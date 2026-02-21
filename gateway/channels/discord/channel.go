// Package discord provides Discord bot channel implementation
package discord

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

// DiscordChannel implements the types.ChannelLoader interface
type DiscordChannel struct {
	token    string
	client   *http.Client
	agentRPC types.AgentRPCInterface
	running  bool
	stopCh   chan struct{}
}

// NewDiscordChannel creates a new Discord channel
func NewDiscordChannel(token string, agentRPC types.AgentRPCInterface) *DiscordChannel {
	return &DiscordChannel{
		token:    token,
		client:   &http.Client{Timeout: 30 * time.Second},
		agentRPC: agentRPC,
		stopCh:   make(chan struct{}),
	}
}

func (c *DiscordChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Discord",
		Type:        types.ChannelDiscord,
		Version:     "1.0.0",
		Description: "Discord Bot API integration",
	}
}

func (c *DiscordChannel) Initialize(config map[string]interface{}) error {
	return nil
}

func (c *DiscordChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Discord channel...")
	c.running = true
	return nil
}

func (c *DiscordChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Discord channel stopped")
	return nil
}

func (c *DiscordChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	// Send typing indicator first
	c.sendTyping(req.ChatID)

	text := req.Text
	if len(text) > 2000 {
		text = text[:2000]
	}

	payload := map[string]interface{}{"content": text}
	payloadBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%d/messages", req.ChatID)

	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	httpReq.Header.Set("Authorization", "Bot "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

// sendTyping sends a typing indicator to Discord
func (c *DiscordChannel) sendTyping(channelID int64) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%d/typing", channelID)
	httpReq, _ := http.NewRequest("POST", url, nil)
	httpReq.Header.Set("Authorization", "Bot "+c.token)
	c.client.Do(httpReq) // Fire and forget
}

func (c *DiscordChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Discord webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *DiscordChannel) HealthCheck() error {
	return nil
}

// DiscordCreateFromEnv creates Discord channel from environment
func DiscordCreateFromEnv(agentRPC types.AgentRPCInterface) (*DiscordChannel, error) {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN not set")
	}
	return NewDiscordChannel(token, agentRPC), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
