// Package zalo provides Zalo messaging channel implementation
package zalo

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

// ZaloChannel implements the types.ChannelLoader interface
type ZaloChannel struct {
	appID    string
	secret   string
	token    string
	client   *http.Client
	agentRPC types.AgentRPCInterface
	running  bool
	stopCh   chan struct{}
}

// NewZaloChannel creates a new Zalo channel
func NewZaloChannel(appID, secret string, agentRPC types.AgentRPCInterface) *ZaloChannel {
	return &ZaloChannel{
		appID:    appID,
		secret:   secret,
		client:   &http.Client{Timeout: 30 * time.Second},
		agentRPC: agentRPC,
		stopCh:   make(chan struct{}),
	}
}

func (c *ZaloChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Zalo",
		Type:        types.ChannelZalo,
		Version:     "1.0.0",
		Description: "Zalo messaging platform integration",
		Capabilities: []string{
			"text",
			"media",
			"buttons",
			"webhooks",
		},
	}
}

func (c *ZaloChannel) Initialize(config map[string]interface{}) error {
	return nil
}

func (c *ZaloChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Zalo channel...")

	// Zalo Official Account API requires app_id and secret for verification
	if c.appID == "" {
		return fmt.Errorf("ZALO_APP_ID not set")
	}

	c.running = true
	log.Printf("[OK] Zalo channel ready")
	return nil
}

func (c *ZaloChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Zalo channel stopped")
	return nil
}

func (c *ZaloChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 2000 {
		text = text[:2000]
	}

	// Zalo API payload
	payload := map[string]interface{}{
		"recipient": map[string]interface{}{
			"user_id": fmt.Sprintf("%d", req.ChatID),
		},
		"message": map[string]interface{}{
			"text": text,
		},
	}

	// Handle buttons (Zalo uses quick reply)
	if len(req.Buttons) > 0 {
		actions := make([]map[string]interface{}, 0)
		for _, row := range req.Buttons {
			for _, btn := range row {
				actions = append(actions, map[string]interface{}{
					"type":  "open_url",
					"label": btn.Text,
				})
			}
		}
		payload["message"].(map[string]interface{})["quick_reply"] = map[string]interface{}{
			"actions": actions,
		}
	}

	payloadBytes, _ := json.Marshal(payload)
	url := "https://openapi.zalo.me/v3/oa/message"

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("access_token", c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode != http.StatusOK || result["error"] != nil {
		errMsg := "unknown error"
		if msg, ok := result["message"].(string); ok {
			errMsg = msg
		}
		// FIX: lowercase error string
		return &types.SendMessageResponse{OK: false, Error: errMsg}, fmt.Errorf("zalo API error: %s", errMsg)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *ZaloChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Zalo webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *ZaloChannel) HealthCheck() error {
	if c.appID == "" {
		return fmt.Errorf("ZALO_APP_ID not set")
	}
	return nil
}

// ZaloCreateFromEnv creates Zalo channel from environment
func ZaloCreateFromEnv(agentRPC types.AgentRPCInterface) (*ZaloChannel, error) {
	appID := os.Getenv("ZALO_APP_ID")
	secret := os.Getenv("ZALO_SECRET")
	token := os.Getenv("ZALO_ACCESS_TOKEN")

	if appID == "" {
		return nil, fmt.Errorf("ZALO_APP_ID not set")
	}

	channel := NewZaloChannel(appID, secret, agentRPC)
	channel.token = token
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
