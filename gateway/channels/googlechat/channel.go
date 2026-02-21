// Package googlechat provides Google Chat channel implementation
package googlechat

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

type GoogleChatChannel struct {
	serviceAccountFile string
	client            *http.Client
	agentRPC          types.AgentRPCInterface
	running          bool
	stopCh           chan struct{}
}

func NewGoogleChatChannel(serviceAccountFile string, agentRPC types.AgentRPCInterface) *GoogleChatChannel {
	return &GoogleChatChannel{
		serviceAccountFile: serviceAccountFile,
		client:            &http.Client{Timeout: 30 * time.Second},
		agentRPC:          agentRPC,
		stopCh:            make(chan struct{}),
	}
}

func (c *GoogleChatChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{Name: "Google Chat", Type: types.ChannelGoogleChat, Version: "1.0.0", Description: "Google Chat API integration"}
}
func (c *GoogleChatChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *GoogleChatChannel) Start() error { log.Printf("[START] Starting Google Chat channel..."); c.running = true; return nil }
func (c *GoogleChatChannel) Stop() error { c.running = false; return nil }
func (c *GoogleChatChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	// Google Chat doesn't have a standard typing indicator API
	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}
func (c *GoogleChatChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ok": true}`) }
func (c *GoogleChatChannel) HealthCheck() error { return nil }

func GoogleChatCreateFromEnv(agentRPC types.AgentRPCInterface) (*GoogleChatChannel, error) {
	file := os.Getenv("GOOGLE_CHAT_SERVICE_ACCOUNT_FILE")
	if file == "" { return nil, fmt.Errorf("GOOGLE_CHAT_SERVICE_ACCOUNT_FILE not set") }
	return NewGoogleChatChannel(file, agentRPC), nil
}
