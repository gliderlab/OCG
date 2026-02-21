// Package signal provides Signal channel implementation
package signal

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

type SignalChannel struct {
	account string
	cliPath  string
	client   *http.Client
	agentRPC types.AgentRPCInterface
	running  bool
	stopCh   chan struct{}
}

func NewSignalChannel(account, cliPath string, agentRPC types.AgentRPCInterface) *SignalChannel {
	return &SignalChannel{
		account: account,
		cliPath:  cliPath,
		client:   &http.Client{Timeout: 30 * time.Second},
		agentRPC: agentRPC,
		stopCh:  make(chan struct{}),
	}
}

func (c *SignalChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{Name: "Signal", Type: types.ChannelSignal, Version: "1.0.0", Description: "Signal Messenger integration"}
}
func (c *SignalChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *SignalChannel) Start() error { log.Printf("[START] Starting Signal channel..."); c.running = true; return nil }
func (c *SignalChannel) Stop() error { c.running = false; return nil }
func (c *SignalChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	c.sendTyping(req.ChatID)
	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}

func (c *SignalChannel) sendTyping(chatID int64) {
	// Signal typing indicator via signal-cli - placeholder
	log.Printf("[NOTE] Signal typing indicator for chat %d", chatID)
}
func (c *SignalChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ok": true}`) }
func (c *SignalChannel) HealthCheck() error { return nil }

func SignalCreateFromEnv(agentRPC types.AgentRPCInterface) (*SignalChannel, error) {
	account := os.Getenv("SIGNAL_ACCOUNT")
	if account == "" { return nil, fmt.Errorf("SIGNAL_ACCOUNT not set") }
	return NewSignalChannel(account, os.Getenv("SIGNAL_CLI_PATH"), agentRPC), nil
}
