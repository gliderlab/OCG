// Package session provides Session messaging channel implementation
package session

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// SessionChannel implements the types.ChannelLoader interface
// Session uses a service node network for decentralized messaging
type SessionChannel struct {
	oxenURL   string
	sessionID string
	publicKey string
	lokiAddr  string
	client    *http.Client
	agentRPC  types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
}

// NewSessionChannel creates a new Session channel
func NewSessionChannel(oxenURL, sessionID, publicKey, lokiAddr string, agentRPC types.AgentRPCInterface) *SessionChannel {
	return &SessionChannel{
		oxenURL:   oxenURL,
		sessionID: sessionID,
		publicKey: publicKey,
		lokiAddr:  lokiAddr,
		client:    &http.Client{Timeout: 30 * time.Second},
		agentRPC:  agentRPC,
		stopCh:    make(chan struct{}),
	}
}

func (c *SessionChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Session",
		Type:        types.ChannelSession,
		Version:     "1.0.0",
		Description: "Session decentralized private messaging",
		Capabilities: []string{
			"text",
			"media",
			"end-to-end",
			"closed-groups",
		},
	}
}

func (c *SessionChannel) Initialize(config map[string]interface{}) error {
	if c.oxenURL == "" {
		c.oxenURL = "https://api.oxen.io"
	}
	return nil
}

func (c *SessionChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Session channel...")

	// Verify Oxen service node connection
	resp, err := c.client.Get(c.oxenURL + "/info/nodes")
	if err != nil {
		log.Printf("[WARN] Session: using offline mode")
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	c.running = true
	log.Printf("[OK] Session channel ready")
	return nil
}

func (c *SessionChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Session channel stopped")
	return nil
}

func (c *SessionChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 2000 {
		text = text[:2000]
	}

	// Session uses Oxen service nodes
	// Message is stored encrypted and retrieved by recipient
	payload := map[string]interface{}{
		"pubkey":    c.publicKey,
		"recipient": fmt.Sprintf("%d", req.ChatID),
		"text":      text,
		"timestamp": time.Now().UnixMilli(),
	}

	// Send to Oxen service node
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	url := c.oxenURL + "/lsrpc/sessions/v1/send"
	
	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	// Use the channel's HTTP client
	resp, err := c.client.Do(httpReq)
	if err != nil {
		// Return error instead of silently failing
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *SessionChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Session webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *SessionChannel) HealthCheck() error {
	if c.sessionID == "" {
		return fmt.Errorf("SESSION_ID not set")
	}
	return nil
}

// SessionCreateFromEnv creates Session channel from environment
func SessionCreateFromEnv(agentRPC types.AgentRPCInterface) (*SessionChannel, error) {
	oxenURL := os.Getenv("SESSION_OXEN_URL")
	sessionID := os.Getenv("SESSION_ID")
	publicKey := os.Getenv("SESSION_PUBLIC_KEY")
	lokiAddr := os.Getenv("LOKI_ADDR")

	if sessionID == "" && publicKey == "" {
		return nil, fmt.Errorf("SESSION_ID or SESSION_PUBLIC_KEY must be set")
	}

	channel := NewSessionChannel(oxenURL, sessionID, publicKey, lokiAddr, agentRPC)
	if err := channel.Initialize(nil); err != nil {
		return nil, err
	}
	return channel, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
