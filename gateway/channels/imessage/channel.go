// Package imessage provides iMessage channel implementation via BlueBubbles/AirMessage REST API
package imessage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// iMessageChannel implements the types.ChannelLoader interface
// Works with BlueBubbles REST API or AirMessage REST API
type iMessageChannel struct {
	serverURL string
	password  string
	client    *http.Client
	agentRPC  types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
}

// NewIMessageChannel creates a new iMessage channel
func NewIMessageChannel(serverURL, password string, agentRPC types.AgentRPCInterface) *iMessageChannel {
	return &iMessageChannel{
		serverURL: serverURL,
		password:  password,
		client:    &http.Client{Timeout: 30 * time.Second},
		agentRPC:  agentRPC,
		stopCh:    make(chan struct{}),
	}
}

func (c *iMessageChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "iMessage",
		Type:        types.ChannelIMessage,
		Version:     "1.0.0",
		Description: "iMessage via BlueBubbles/AirMessage REST API",
		Capabilities: []string{
			"text",
			"media",
			"group",
			"read",
			"delivery",
		},
	}
}

func (c *iMessageChannel) Initialize(config map[string]interface{}) error {
	if c.serverURL == "" {
		c.serverURL = "http://localhost:1234"
	}
	return nil
}

func (c *iMessageChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting iMessage channel: %s", c.serverURL)

	// Verify connection to BlueBubbles/AirMessage server
	resp, err := c.client.Get(c.serverURL + "/api/v1/version")
	if err != nil {
		return fmt.Errorf("failed to connect to iMessage server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("iMessage server error: %d", resp.StatusCode)
	}

	c.running = true
	log.Printf("[OK] iMessage channel connected")
	return nil
}

func (c *iMessageChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 iMessage channel stopped")
	return nil
}

func (c *iMessageChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 5000 {
		text = text[:5000]
	}

	// BlueBubbles REST API format
	payload := map[string]interface{}{
		"guid":     fmt.Sprintf("%d", req.ChatID),
		"message":  text,
		"tempGuid": fmt.Sprintf("msg_%d", time.Now().UnixMilli()),
	}

	// Handle group chats (if ChatID indicates group)
	if req.ThreadID > 0 {
		payload["method"] = "group"
	}

	payloadBytes, _ := json.Marshal(payload)
	url := c.serverURL + "/api/v1/message/text"

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	if c.password != "" {
		httpReq.Header.Set("X-Blubbub-Password", c.password)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("iMessage API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	messageID := time.Now().Unix()
	if id, ok := result["id"].(float64); ok {
		messageID = int64(id)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: messageID,
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *iMessageChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("iMessage webhook received: %s", string(body[:min(200, len(body))]))

	// Parse incoming message
	var incoming map[string]interface{}
	json.Unmarshal(body, &incoming)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *iMessageChannel) HealthCheck() error {
	resp, err := c.client.Get(c.serverURL + "/api/v1/version")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("iMessage server unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// iMessageCreateFromEnv creates iMessage channel from environment
func IMessageCreateFromEnv(agentRPC types.AgentRPCInterface) (*iMessageChannel, error) {
	serverURL := os.Getenv("IMESSAGE_SERVER_URL")
	password := os.Getenv("IMESSAGE_PASSWORD")

	if serverURL == "" {
		serverURL = "http://localhost:1234" // Default BlueBubbles port
	}

	channel := NewIMessageChannel(serverURL, password, agentRPC)
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
