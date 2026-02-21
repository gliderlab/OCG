// Package line provides LINE messaging channel implementation
package line

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

// LineChannel implements the types.ChannelLoader interface
type LineChannel struct {
	channelSecret string
	channelToken  string
	client        *http.Client
	agentRPC      types.AgentRPCInterface
	running       bool
	stopCh        chan struct{}
}

// NewLineChannel creates a new LINE channel
func NewLineChannel(channelSecret, channelToken string, agentRPC types.AgentRPCInterface) *LineChannel {
	return &LineChannel{
		channelSecret: channelSecret,
		channelToken:  channelToken,
		client:        &http.Client{Timeout: 30 * time.Second},
		agentRPC:      agentRPC,
		stopCh:        make(chan struct{}),
	}
}

func (c *LineChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "LINE",
		Type:        types.ChannelLINE,
		Version:     "1.0.0",
		Description: "LINE Messaging API integration",
		Capabilities: []string{
			"text",
			"media",
			"sticker",
			"buttons",
			"carousel",
			"webhooks",
		},
	}
}

func (c *LineChannel) Initialize(config map[string]interface{}) error {
	return nil
}

func (c *LineChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting LINE channel...")

	// Verify token
	url := "https://api.line.me/v2/bot/info"
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+c.channelToken)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect to LINE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LINE API error: %d", resp.StatusCode)
	}

	c.running = true
	log.Printf("[OK] LINE channel connected")
	return nil
}

func (c *LineChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 LINE channel stopped")
	return nil
}

func (c *LineChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 5000 {
		text = text[:5000]
	}

	// LINE API expects array of messages
	messages := []map[string]interface{}{
		{"type": "text", "text": text},
	}

	// Handle buttons if present
	if len(req.Buttons) > 0 {
		actions := make([]map[string]interface{}, 0)
		for _, row := range req.Buttons {
			for _, btn := range row {
				actions = append(actions, map[string]interface{}{
					"type":  "message",
					"label": btn.Text,
					"text":  btn.Text,
				})
			}
		}
		buttonsTemplate := map[string]interface{}{
			"type":    "buttons",
			"text":    text,
			"actions": actions,
		}
		messages = []map[string]interface{}{buttonsTemplate}
	}

	payload := map[string]interface{}{
		"to":       fmt.Sprintf("%d", req.ChatID),
		"messages": messages,
	}

	payloadBytes, _ := json.Marshal(payload)
	url := "https://api.line.me/v2/bot/message/push"

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("Authorization", "Bearer "+c.channelToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 202 {
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("LINE API error: %d", resp.StatusCode)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *LineChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("LINE webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *LineChannel) HealthCheck() error {
	url := "https://api.line.me/v2/bot/info"
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+c.channelToken)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LINE unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// LineCreateFromEnv creates LINE channel from environment
func LineCreateFromEnv(agentRPC types.AgentRPCInterface) (*LineChannel, error) {
	channelSecret := os.Getenv("LINE_CHANNEL_SECRET")
	channelToken := os.Getenv("LINE_CHANNEL_TOKEN")

	if channelToken == "" {
		return nil, fmt.Errorf("LINE_CHANNEL_TOKEN not set")
	}

	channel := NewLineChannel(channelSecret, channelToken, agentRPC)
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
