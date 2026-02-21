// Package slack provides Slack bot channel implementation
package slack

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// SlackChannel implements the types.ChannelLoader interface
type SlackChannel struct {
	botToken  string
	appToken  string
	client    *http.Client
	agentRPC  types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
}

// NewSlackChannel creates a new Slack channel
func NewSlackChannel(botToken, appToken string, agentRPC types.AgentRPCInterface) *SlackChannel {
	return &SlackChannel{
		botToken: botToken,
		appToken: appToken,
		client:   &http.Client{Timeout: 30 * time.Second},
		agentRPC: agentRPC,
		stopCh:   make(chan struct{}),
	}
}

func (c *SlackChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Slack",
		Type:        types.ChannelSlack,
		Version:     "1.0.0",
		Description: "Slack Bot API integration",
	}
}

func (c *SlackChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *SlackChannel) Start() error {
	if c.running { return nil }
	log.Printf("[START] Starting Slack channel...")
	c.running = true
	return nil
}
func (c *SlackChannel) Stop() error {
	if !c.running { return nil }
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Slack channel stopped")
	return nil
}

func (c *SlackChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	// Send typing indicator first
	c.sendTyping(req.ChatID)

	text := req.Text
	if len(text) > 4000 { text = text[:4000] }

	payload := map[string]interface{}{
		"channel": strconv.FormatInt(req.ChatID, 10),
		"text":    text,
	}
	payloadBytes, _ := json.Marshal(payload)
	url := "https://slack.com/api/chat.postMessage"

	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	httpReq.Header.Set("Authorization", "Bearer "+c.botToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}

// sendTyping sends a typing indicator to Slack
func (c *SlackChannel) sendTyping(channelID int64) {
	url := "https://slack.com/api/users.typing"
	payload := map[string]interface{}{
		"channel": strconv.FormatInt(channelID, 10),
	}
	payloadBytes, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	httpReq.Header.Set("Authorization", "Bearer "+c.botToken)
	httpReq.Header.Set("Content-Type", "application/json")
	c.client.Do(httpReq) // Fire and forget
}

func (c *SlackChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}
func (c *SlackChannel) HealthCheck() error { return nil }

func SlackCreateFromEnv(agentRPC types.AgentRPCInterface) (*SlackChannel, error) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" { return nil, fmt.Errorf("SLACK_BOT_TOKEN not set") }
	return NewSlackChannel(token, os.Getenv("SLACK_APP_TOKEN"), agentRPC), nil
}
