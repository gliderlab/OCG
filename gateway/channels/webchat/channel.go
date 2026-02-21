// Package webchat provides WebChat channel implementation
package webchat

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

type WebChatChannel struct {
	agentRPC types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
	config    WebChatConfig
}

type WebChatConfig struct {
	Path string
}

func NewWebChatChannel(agentRPC types.AgentRPCInterface) *WebChatChannel {
	return &WebChatChannel{
		agentRPC: agentRPC,
		stopCh:   make(chan struct{}),
		config:   WebChatConfig{Path: "/ws/chat"},
	}
}

func (c *WebChatChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{Name: "WebChat", Type: types.ChannelWebChat, Version: "1.0.0", Description: "Web-based chat via WebSocket"}
}
func (c *WebChatChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *WebChatChannel) Start() error { log.Printf("[START] Starting WebChat channel..."); c.running = true; return nil }
func (c *WebChatChannel) Stop() error { c.running = false; return nil }
func (c *WebChatChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}
func (c *WebChatChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/chat/status":
		fmt.Fprintf(w, `{"enabled":true,"path":"%s"}`, c.config.Path)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}
func (c *WebChatChannel) HealthCheck() error { return nil }
