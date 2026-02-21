// Package mattermost provides Mattermost channel implementation
package mattermost

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

// MattermostChannel implements the types.ChannelLoader interface
type MattermostChannel struct {
	url      string
	token    string
	teamID   string
	client   *http.Client
	agentRPC types.AgentRPCInterface
	running  bool
	stopCh   chan struct{}
}

// NewMattermostChannel creates a new Mattermost channel
func NewMattermostChannel(url, token, teamID string, agentRPC types.AgentRPCInterface) *MattermostChannel {
	return &MattermostChannel{
		url:      strings.TrimRight(url, "/"),
		token:    token,
		teamID:   teamID,
		client:   &http.Client{Timeout: 30 * time.Second},
		agentRPC: agentRPC,
		stopCh:   make(chan struct{}),
	}
}

func (c *MattermostChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Mattermost",
		Type:        types.ChannelMattermost,
		Version:     "1.0.0",
		Description: "Mattermost self-hosted chat integration",
		Capabilities: []string{
			"text",
			"media",
			"buttons",
			"webhooks",
			"threads",
		},
	}
}

func (c *MattermostChannel) Initialize(config map[string]interface{}) error {
	if c.url == "" {
		c.url = "http://localhost:8065"
	}
	return nil
}

func (c *MattermostChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Mattermost channel: %s", c.url)

	// Verify connection
	resp, err := c.client.Get(c.url + "/api/v4/system/ping")
	if err != nil {
		return fmt.Errorf("failed to connect to Mattermost: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// FIX: lowercase error string
		return fmt.Errorf("mattermost ping failed: %d", resp.StatusCode)
	}

	c.running = true
	log.Printf("[OK] Mattermost channel connected")
	return nil
}

func (c *MattermostChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Mattermost channel stopped")
	return nil
}

func (c *MattermostChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 4000 {
		text = text[:4000]
	}

	payload := map[string]interface{}{
		"channel_id": fmt.Sprintf("%d", req.ChatID),
		"message":    text,
	}

	if req.ThreadID > 0 {
		payload["root_id"] = fmt.Sprintf("%d", req.ThreadID)
	}

	payloadBytes, _ := json.Marshal(payload)
	url := c.url + "/api/v4/posts"

	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		// FIX: lowercase error string
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("mattermost API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	messageID := int64(0)
	if id, ok := result["id"].(string); ok {
		// Generate consistent ID from string
		for _, c := range id {
			messageID = messageID*31 + int64(c)
		}
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: messageID,
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *MattermostChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Mattermost webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *MattermostChannel) HealthCheck() error {
	resp, err := c.client.Get(c.url + "/api/v4/system/ping")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mattermost unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// MattermostCreateFromEnv creates Mattermost channel from environment
func MattermostCreateFromEnv(agentRPC types.AgentRPCInterface) (*MattermostChannel, error) {
	url := os.Getenv("MATTERMOST_URL")
	token := os.Getenv("MATTERMOST_TOKEN")
	teamID := os.Getenv("MATTERMOST_TEAM_ID")

	if token == "" {
		return nil, fmt.Errorf("MATTERMOST_TOKEN not set")
	}

	channel := NewMattermostChannel(url, token, teamID, agentRPC)
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
