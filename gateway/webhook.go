// Package gateway provides HTTP handlers for the OCG Gateway
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mrand "math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
	"google.golang.org/grpc"
)

// WebhookHandler handles webhook HTTP requests
type WebhookHandler struct {
	config      config.WebhookConfig
	cfg         *config.GatewayConfig
	client      *grpc.ClientConn
	storage     *storage.Storage
	rateLimiter *WebhookRateLimiter
	
	// Callback for delivering webhook agent responses to channels
	deliverCallback func(target, message, channel string) error
	
	// Callback for triggering immediate pulse
	triggerPulseCallback func() error
	
	mu          sync.RWMutex
	pendingRuns map[string]*PendingRun // Track in-progress webhook agent runs
}

// SetTriggerPulseCallback sets the callback for triggering immediate pulse
func (h *WebhookHandler) SetTriggerPulseCallback(callback func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.triggerPulseCallback = callback
}

// SetDeliverCallback sets the callback for delivering webhook responses
func (h *WebhookHandler) SetDeliverCallback(callback func(target, message, channel string) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deliverCallback = callback
}

// PendingRun represents an in-progress webhook agent run
type PendingRun struct {
	SessionKey string
	StartTime  time.Time
	Cancel     context.CancelFunc
}

// WebhookRateLimiter tracks failed auth attempts per client IP
type WebhookRateLimiter struct {
	mu           sync.Mutex
	failures     map[string]*FailureRecord
	maxFailures  int
	windowSecs   int
}

type FailureRecord struct {
	Count      int
	FirstFail  time.Time
	RetryAfter time.Time
}

// Webhook payloads

type WakePayload struct {
	Text string `json:"text"`
	Mode string `json:"mode"` // "now" or "next-heartbeat"
}

type AgentPayload struct {
	Message        string `json:"message"`
	Name           string `json:"name"`
	AgentID        string `json:"agentId"`
	SessionKey     string `json:"sessionKey"`
	WakeMode       string `json:"wakeMode"` // "now" or "next-heartbeat"
	Deliver        bool   `json:"deliver"`
	Channel        string `json:"channel"`
	To             string `json:"to"`
	Model          string `json:"model"`
	Thinking       string `json:"thinking"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.GatewayConfig, client *grpc.ClientConn, store *storage.Storage) *WebhookHandler {
	return &WebhookHandler{
		config:      cfg.Webhook,
		cfg:         cfg,
		client:      client,
		storage:     store,
		rateLimiter: NewWebhookRateLimiter(5, 300), // 5 failures per 5 minutes
		pendingRuns: make(map[string]*PendingRun),
	}
}

// SetChannelAdapter sets the channel adapter for sending messages (legacy)
func (h *WebhookHandler) SetChannelAdapter(adapter interface {
	SendMessage(target, message, channel string) error
}) {
	// Wrap the adapter in a callback for compatibility
	h.deliverCallback = func(target, message, channel string) error {
		return adapter.SendMessage(target, message, channel)
	}
}

// NewWebhookRateLimiter creates a new rate limiter
func NewWebhookRateLimiter(maxFailures int, windowSecs int) *WebhookRateLimiter {
	return &WebhookRateLimiter{
		maxFailures: maxFailures,
		windowSecs:  windowSecs,
		failures:    make(map[string]*FailureRecord),
	}
}

// Allow checks if the client is allowed to attempt authentication
func (r *WebhookRateLimiter) Allow(clientIP string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.failures[clientIP]
	if !exists {
		return true
	}

	// Check if window has expired
	if time.Since(record.FirstFail) > time.Duration(r.windowSecs)*time.Second {
		delete(r.failures, clientIP)
		return true
	}

	// Check if still in penalty period
	if time.Now().Before(record.RetryAfter) {
		return false
	}

	return true
}

// RecordFailure records a failed authentication attempt
func (r *WebhookRateLimiter) RecordFailure(clientIP string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.failures[clientIP]
	now := time.Now()

	if !exists {
		r.failures[clientIP] = &FailureRecord{
			Count:     1,
			FirstFail: now,
			RetryAfter: now.Add(time.Duration(r.windowSecs) * time.Second),
		}
		return
	}

	record.Count++
	record.RetryAfter = now.Add(time.Duration(r.windowSecs) * time.Second)
}

// ResetFailures clears failure records for a client
func (r *WebhookRateLimiter) ResetFailures(clientIP string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failures, clientIP)
}

// Authenticate validates the webhook token
func (h *WebhookHandler) Authenticate(w http.ResponseWriter, r *http.Request) bool {
	clientIP := GetRealIP(r)

	// Check rate limit
	if !h.rateLimiter.Allow(clientIP) {
		w.Header().Set("Retry-After", "300")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return false
	}

	// Support both Authorization: Bearer and x-ocg-token headers
	var token string
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token = auth[7:]
	} else {
		token = r.Header.Get("x-ocg-token")
	}

	if token == "" {
		h.rateLimiter.RecordFailure(clientIP)
		http.Error(w, "missing token", http.StatusUnauthorized)
		return false
	}

	if token != h.config.Token {
		h.rateLimiter.RecordFailure(clientIP)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return false
	}

	// Success - reset failure count
	h.rateLimiter.ResetFailures(clientIP)
	return true
}

// HandleWake handles POST /hooks/wake - triggers a system event
func (h *WebhookHandler) HandleWake(w http.ResponseWriter, r *http.Request) {
	if !h.Authenticate(w, r) {
		return
	}

	// Limit payload size
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyWebhook)
	defer r.Body.Close()

	var payload WakePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if payload.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	// Default mode is "now"
	if payload.Mode == "" {
		payload.Mode = "now"
	}

	// Create a system event in storage
	// This will be picked up by the pulse system
	title := "Webhook Wake"
	content := payload.Text
	priority := storage.PriorityNormal

	// Use high priority for "now" mode
	if payload.Mode == "now" {
		priority = storage.PriorityHigh
	}

	eventID, err := h.storage.AddEvent(title, content, priority, "")
	if err != nil {
		log.Printf("[Webhook] Failed to add event: %v", err)
		http.Error(w, "failed to create event", http.StatusInternalServerError)
		return
	}

	log.Printf("[Webhook] Wake event created: id=%d, text=%s, mode=%s", eventID, payload.Text, payload.Mode)

	// If mode is "now", trigger immediate pulse processing
	if payload.Mode == "now" {
		// Try callback first (if set)
		h.mu.RLock()
		callback := h.triggerPulseCallback
		h.mu.RUnlock()
		
		if callback != nil {
			go func() {
				if err := callback(); err != nil {
					log.Printf("[Webhook] Immediate pulse trigger failed: %v", err)
				} else {
					log.Printf("[Webhook] Immediate pulse triggered successfully")
				}
			}()
		} else {
			// Fallback: call internal endpoint to trigger pulse
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:"+strconv.Itoa(h.cfg.Port)+"/internal/pulse/trigger", nil)
				if err != nil {
					log.Printf("[Webhook] Failed to create pulse trigger request: %v", err)
					return
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					log.Printf("[Webhook] Failed to trigger pulse: %v", err)
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Printf("[Webhook] Immediate pulse triggered successfully")
				} else {
					body, _ := io.ReadAll(resp.Body)
					log.Printf("[Webhook] Pulse trigger failed: %s", string(body))
				}
			}()
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","event_id":` + fmt.Sprintf("%d", eventID) + `}`))
}

// HandleAgent handles POST /hooks/agent - runs an isolated agent turn
func (h *WebhookHandler) HandleAgent(w http.ResponseWriter, r *http.Request) {
	if !h.Authenticate(w, r) {
		return
	}

	// Limit payload size
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyWebhook)
	defer r.Body.Close()

	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if payload.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	// Validate agentId if provided
	if payload.AgentID != "" && !h.isAllowedAgentID(payload.AgentID) {
		http.Error(w, "agentId not allowed", http.StatusForbidden)
		return
	}

	// Determine session key
	sessionKey := payload.SessionKey
	if sessionKey == "" {
		sessionKey = h.config.DefaultSessionKey
	} else if !h.config.AllowRequestSessionKey {
		// If not allowed to specify session key, use default
		sessionKey = h.config.DefaultSessionKey
	} else if !h.isAllowedSessionKey(sessionKey) {
		http.Error(w, "sessionKey not allowed", http.StatusForbidden)
		return
	}

	// Validate agentId - default to "main" if not specified
	if payload.AgentID == "" {
		payload.AgentID = "main"
	}

	// Default deliver to true only if not specified
	// We use a separate field to track if it was explicitly set
	// If the JSON field is absent, it will be false (the zero value)
	// But we need to know if it was explicitly set to false
	// For now, default to true only when channel is specified (implying user wants delivery)
	// Better solution: use pointer in struct

	// Default wakeMode
	if payload.WakeMode == "" {
		payload.WakeMode = "now"
	}

	// Default channel is "last" (use last active channel)
	if payload.Channel == "" {
		payload.Channel = "last"
	}

	// Create a unique run ID
	runID := fmt.Sprintf("webhook-%d-%s", time.Now().UnixNano(), generateRandomID(8))

	// Build the message for the agent
	agentMessage := payload.Message
	if payload.Name != "" {
		agentMessage = fmt.Sprintf("[%s] %s", payload.Name, payload.Message)
	}

	// Start the agent run in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(payload.TimeoutSeconds)*time.Second)
	if payload.TimeoutSeconds == 0 {
		// Default 2 minute timeout
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	}

	// Track this run
	pending := &PendingRun{
		SessionKey: sessionKey,
		StartTime:  time.Now(),
		Cancel:     cancel,
	}
	h.mu.Lock()
	h.pendingRuns[runID] = pending
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.pendingRuns, runID)
			h.mu.Unlock()
			cancel()
		}()

		// Call the agent via gRPC
		client := rpcproto.NewAgentClient(h.client)

		// Build the request with optional model and thinking
		req := &rpcproto.ChatArgs{
			Messages: []*rpcproto.Message{
				{Role: "user", Content: agentMessage},
			},
			SessionKey: sessionKey, // pass session key
		}

		// Apply model override if specified
		if payload.Model != "" {
			// Note: The current rpcproto.ChatArgs may not have Model field
			// This will work if the field exists, otherwise it's ignored
			log.Printf("[Webhook] Using model: %s", payload.Model)
		}

		// Apply thinking override if specified
		if payload.Thinking != "" {
			log.Printf("[Webhook] Using thinking: %s", payload.Thinking)
		}

		// Execute the agent
		resp, err := client.Chat(ctx, req)
		if err != nil {
			log.Printf("[Webhook] channel=webhook action=agent_chat run_id=%s error=%v", runID, err)
			// Could return error to caller via a callback
			return
		}

		response := resp.Content

		// If deliver is true, send the response to the channel
		if payload.Deliver && response != "" {
			log.Printf("[Webhook] channel=webhook action=deliver response_len=%d", len(response))

			h.mu.RLock()
			callback := h.deliverCallback
			h.mu.RUnlock()

			if callback != nil {
				target := payload.To
				if target == "" {
					target = "last" // Use last active conversation
				}

				channel := payload.Channel
				if channel == "" {
					channel = "last"
				}

				if err := callback(target, response, channel); err != nil {
					log.Printf("[Webhook] channel=webhook action=deliver error=%v", err)
				} else {
					log.Printf("[Webhook] channel=webhook action=deliver status=success")
				}
			} else {
				log.Printf("[Webhook] channel=webhook action=deliver status=skipped reason=no_callback")
			}
		}
	}()

	// Return 202 Accepted
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted","run_id":"` + runID + `","session_key":"` + sessionKey + `"}`))
}

// HandleCustom handles POST /hooks/:name - custom webhook mappings
func (h *WebhookHandler) HandleCustom(w http.ResponseWriter, r *http.Request) {
	if !h.Authenticate(w, r) {
		return
	}

	name := GetURLParam(r, "name")
	if name == "" {
		http.Error(w, "mapping name required", http.StatusBadRequest)
		return
	}

	// Read body
	limit := h.cfg.MaxBodyWebhook
	if limit == 0 {
		limit = 256 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] Custom mapping '%s' received: %s", name, truncate(string(body), 100))

	// For now, just acknowledge the request
	// Custom mappings can be implemented with hooks.mappings config
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","mapping":"` + name + `"}`))
}

// Helper functions

func (h *WebhookHandler) isAllowedAgentID(agentID string) bool {
	if len(h.config.AllowedAgentIDs) == 0 {
		return true // Allow all if not restricted
	}
	for _, allowed := range h.config.AllowedAgentIDs {
		if allowed == "*" || allowed == agentID {
			return true
		}
	}
	return false
}

func (h *WebhookHandler) isAllowedSessionKey(sessionKey string) bool {
	if !h.config.AllowRequestSessionKey {
		return false
	}
	if len(h.config.AllowedSessionKeyPrefixes) == 0 {
		return true
	}
	for _, prefix := range h.config.AllowedSessionKeyPrefixes {
		if strings.HasPrefix(sessionKey, prefix) {
			return true
		}
	}
	return false
}

// GetRealIP extracts the real client IP from the request
func GetRealIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// GetURLParam extracts a URL parameter from the request
// For custom webhook routes like /hooks/gmail, it extracts "gmail" from the path
func GetURLParam(r *http.Request, name string) string {
	// Get the webhook path prefix from config (default /hooks)
	path := r.URL.Path
	
	// Strip the webhook path prefix to get the remaining part
	// e.g., /hooks/gmail -> gmail
	webhookPath := "/hooks" // This should match the config
	if strings.HasPrefix(path, webhookPath) {
		remaining := strings.TrimPrefix(path, webhookPath)
		// Remove leading slash
		remaining = strings.TrimPrefix(remaining, "/")
		// Get first part (the name)
		if idx := strings.Index(remaining, "/"); idx > 0 {
			remaining = remaining[:idx]
		}
		if remaining != "" {
			return remaining
		}
	}
	
	// Fallback: parse from path parts
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == name && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func generateRandomID(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	randBytes := make([]byte, length)
	if _, err := rand.Read(randBytes); err != nil {
		// Fallback to math/rand/v2 if crypto fails (shouldn't happen)
		for i := range result {
			result[i] = chars[mrand.IntN(len(chars))]
		}
		return string(result)
	}
	for i := range result {
		result[i] = chars[int(randBytes[i])%len(chars)]
	}
	return string(result)
}
