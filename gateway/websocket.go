// WebSocket handler for real-time chat

package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/gliderlab/cogate/rpcproto"
)

// WebSocket message types
const (
	MsgTypeChat    = "chat"
	MsgTypeChunk   = "chunk"
	MsgTypeDone    = "done"
	MsgTypeError   = "error"
	MsgTypePing    = "ping"
	MsgTypePong    = "pong"
	MsgTypeHistory = "history"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

// WSChatRequest represents a chat request via WebSocket
type WSChatRequest struct {
	Model    string             `json:"model"`
	Messages []rpcproto.Message `json:"messages"`
}

// WSChatResponse represents a chat response via WebSocket
type WSChatResponse struct {
	Content     string `json:"content"`
	Finish      bool   `json:"finish"`
	Error       string `json:"error,omitempty"`
	TotalTokens int    `json:"totalTokens,omitempty"`
}

// HandleWebSocket handles WebSocket upgrade requests
func (g *Gateway) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Use shared validateToken method
	if !g.validateToken(r) {
		if g.cfg.UIAuthToken == "" {
			http.Error(w, "unauthorized (ui token not set)", http.StatusUnauthorized)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Bug #8 Fix: Add WebSocket connection rate limiting (global + per-IP)
	if atomic.AddInt32(&g.wsConnCount, 1) > g.maxWSConns {
		atomic.AddInt32(&g.wsConnCount, -1)
		http.Error(w, "too many WebSocket connections", http.StatusServiceUnavailable)
		return
	}

	// Optimization #3: Per-IP rate limiting
	ip := getClientIP(r)
	g.mu.Lock()
	if g.wsIPConns == nil {
		g.wsIPConns = make(map[string]int32)
	}
	g.wsIPConns[ip]++
	ipLimit := int32(10) // Max 10 connections per IP
	if g.wsIPConns[ip] > ipLimit {
		g.wsIPConns[ip]--
		g.mu.Unlock()
		atomic.AddInt32(&g.wsConnCount, -1)
		http.Error(w, "too many connections from this IP", http.StatusServiceUnavailable)
		return
	}
	g.mu.Unlock()

	// Upgrade to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		log.Printf("[WS] Accept error: %v", err)
		// rollback connection count
		atomic.AddInt32(&g.wsConnCount, -1)
		g.mu.Lock()
		if g.wsIPConns != nil {
			g.wsIPConns[ip]--
			if g.wsIPConns[ip] <= 0 {
				delete(g.wsIPConns, ip)
			}
		}
		g.mu.Unlock()
		return
	}
	conn.SetReadLimit(4 * 1024 * 1024)

	// Create context with timeout for ping/pong handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle the connection
	g.handleWSConnection(ctx, conn, ip)
}

func (g *Gateway) handleWSConnection(ctx context.Context, conn *websocket.Conn, clientIP string) {
	defer func() {
		conn.Close(websocket.StatusNormalClosure, "")
		// Bug #8 Fix: Decrement connection counter on close
		atomic.AddInt32(&g.wsConnCount, -1)
		// Optimization #3: Decrement per-IP counter on close
		if clientIP != "" {
			g.mu.Lock()
			if g.wsIPConns != nil {
				g.wsIPConns[clientIP]--
				if g.wsIPConns[clientIP] <= 0 {
					delete(g.wsIPConns, clientIP)
				}
			}
			g.mu.Unlock()
		}
	}()

	// Mutex to protect all write operations (coder/websocket is not thread-safe)
	writeMu := sync.Mutex{}

	pingCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ping goroutine to detect dead connections (every 30 seconds)
	// Fix A: Use mutex to protect concurrent writes
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				ping := WSMessage{Type: MsgTypePing}
				data, err := json.Marshal(ping)
				if err != nil {
					continue
				}
				writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				writeMu.Lock()
				if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
					writeMu.Unlock()
					cancel()
					log.Printf("[WS] Ping failed, closing connection: %v", err)
					conn.Close(websocket.StatusNormalClosure, "")
					return
				}
				writeMu.Unlock()
				cancel()
			}
		}
	}()

	// Message loop
	for {
		_, msgBytes, err := conn.Read(ctx)
		if err != nil {
			log.Printf("[WS] Read error: %v", err)
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			g.sendWSError(ctx, conn, &writeMu, "invalid message format")
			continue
		}

		switch msg.Type {
		case MsgTypeChat:
			// Fix B: Run in goroutine to avoid blocking the read loop
			// This allows the connection to continue handling ping/pong while waiting for LLM
			go g.handleWSChat(ctx, conn, &writeMu, msg.Content)
		case MsgTypePing:
			// Respond with pong (bounded write timeout) - Fix A: use mutex
			pong := WSMessage{Type: MsgTypePong}
			if data, err := json.Marshal(pong); err == nil {
				writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				writeMu.Lock()
				if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
					writeMu.Unlock()
					log.Printf("[WS] Pong write failed, closing connection: %v", err)
					conn.Close(websocket.StatusNormalClosure, "")
					cancel()
					return
				}
				writeMu.Unlock()
				cancel()
			}
		case MsgTypePong:
			// Connection alive, do nothing
		default:
			log.Printf("[WS] Unknown message type: %s", msg.Type)
		}
	}
}

func (g *Gateway) handleWSChat(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, content json.RawMessage) {
	// Create a cancellable context for this specific chat request
	// This ensures the goroutine exits when the connection closes
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Content can be either a JSON object or a stringified JSON object
	// Try to parse as WSChatRequest first
	var req WSChatRequest
	if err := json.Unmarshal(content, &req); err != nil {
		// Try parsing as string (front-end sends stringified JSON)
		var contentStr string
		if err := json.Unmarshal(content, &contentStr); err != nil {
			g.sendWSError(ctx, conn, writeMu, "invalid request: "+err.Error())
			return
		}
		// Parse the stringified JSON
		if err := json.Unmarshal([]byte(contentStr), &req); err != nil {
			g.sendWSError(ctx, conn, writeMu, "invalid request content: "+err.Error())
			return
		}
	}

	client, err := g.clientOrError()
	if err != nil {
		g.sendWSError(ctx, conn, writeMu, "agent not connected")
		return
	}

	// Log the incoming message
	if len(req.Messages) > 0 {
		last := &req.Messages[len(req.Messages)-1]
		log.Printf("[WS] Received message: role=%s len=%d", last.Role, len(last.Content))
	}

	// Send request to agent via gRPC (streaming)
	// FIX: Use connection context so gRPC call is cancelled when WS closes
	grpcClient := rpcproto.NewAgentGRPCClient(client)
	args := rpcproto.ChatArgs{Messages: rpcproto.ToMessagesPtr(req.Messages)}
	ctxTimeout, cancel := context.WithTimeout(ctx, rpcproto.DefaultGRPCTimeout())
	defer cancel()

	stream, err := grpcClient.ChatStream(ctxTimeout, &args)
	if err != nil {
		g.sendWSError(ctx, conn, writeMu, "chat error: "+err.Error())
		return
	}

	// Stream response chunks to WebSocket
	// FIX: Use strings.Builder for O(1) append instead of O(n) string concat
	var contentBuilder strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk.Done {
			break
		}
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)

			// Send chunk to client with proper error handling
			resp := WSChatResponse{
				Content: chunk.Content,
				Finish:  false,
			}
			msg := WSMessage{
				Type:    MsgTypeChat,
				Content: json.RawMessage{},
			}
			respBytes, err := json.Marshal(resp)
			if err != nil {
				log.Printf("[WS] Marshal error: %v", err)
				continue
			}
			msg.Content = respBytes
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("[WS] Marshal error: %v", err)
				continue
			}

			// FIX: Add write timeout and check errors
			writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
			writeMu.Lock()
			writeErr := conn.Write(writeCtx, websocket.MessageText, data)
			writeMu.Unlock()
			writeCancel()

			if writeErr != nil {
				log.Printf("[WS] Write error: %v", err)
				g.sendWSError(ctx, conn, writeMu, "write error: "+writeErr.Error())
				return
			}
		}
	}

	// Send final message
	// FIX: Use strings.Builder content instead of separate variable
	resp := WSChatResponse{
		Content:     contentBuilder.String(),
		Finish:      true,
		TotalTokens: countTokens([]byte(contentBuilder.String())),
	}

	msg := WSMessage{
		Type:    MsgTypeDone,
		Content: json.RawMessage{},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		g.sendWSError(ctx, conn, writeMu, "marshal error: "+err.Error())
		return
	}
	msg.Content = respBytes

	data, err := json.Marshal(msg)
	if err != nil {
		g.sendWSError(ctx, conn, writeMu, "marshal error: "+err.Error())
		return
	}

	// FIX: Add write timeout
	writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
	writeMu.Lock()
	writeErr := conn.Write(writeCtx, websocket.MessageText, data)
	writeMu.Unlock()
	writeCancel()

	if writeErr != nil {
		log.Printf("[WS] Final write error: %v", writeErr)
	}
}

func (g *Gateway) sendWSError(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, errMsg string) {
	resp := WSChatResponse{
		Error:  errMsg,
		Finish: true,
	}
	msg := WSMessage{
		Type:    MsgTypeError,
		Content: json.RawMessage{},
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[WS] Error marshal: %v", err)
		return
	}
	msg.Content = respBytes
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] Error marshal: %v", err)
		return
	}

	// FIX: Use mutex + timeout + check errors
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	writeMu.Lock()
	writeErr := conn.Write(writeCtx, websocket.MessageText, data)
	writeMu.Unlock()
	if writeErr != nil {
		log.Printf("[WS] Error send: %v", writeErr)
	}
}

// getClientIP extracts client IP from HTTP request (handles proxies)
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for reverse proxy)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
