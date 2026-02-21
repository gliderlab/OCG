// Package channels - Channel adapter system for communication platforms
// Provides plugin-based channel integration with hot-reload support
package channels

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// Alias types for backward compatibility
type ChannelType = types.ChannelType
type ChannelInfo = types.ChannelInfo
type ChannelLoader = types.ChannelLoader
type SendMessageRequest = types.SendMessageRequest
type SendMessageResponse = types.SendMessageResponse
type Button = types.Button
type ChannelMessage = types.ChannelMessage
type ChannelMedia = types.ChannelMedia
type Message = types.Message
type AgentRPCInterface = types.AgentRPCInterface

// Keep old constants for backward compatibility
var (
	ChannelTelegram   = types.ChannelTelegram
	ChannelWhatsApp   = types.ChannelWhatsApp
	ChannelSlack      = types.ChannelSlack
	ChannelDiscord    = types.ChannelDiscord
	ChannelWebChat    = types.ChannelWebChat
	ChannelSignal    = types.ChannelSignal
	ChannelGoogleChat = types.ChannelGoogleChat
	ChannelIMessage   = types.ChannelIMessage
	ChannelMSTeams    = types.ChannelMSTeams
	ChannelIRC        = types.ChannelIRC
	ChannelLINE       = types.ChannelLINE
	ChannelMatrix     = types.ChannelMatrix
	ChannelFeishu     = types.ChannelFeishu
	ChannelZalo       = types.ChannelZalo
	ChannelMattermost = types.ChannelMattermost
	ChannelThreema    = types.ChannelThreema
	ChannelSession    = types.ChannelSession
	ChannelTox        = types.ChannelTox
)

// ChannelAdapterConfig holds adapter configuration
type ChannelAdapterConfig struct {
	Enabled      bool              `json:"enabled"`
	Channels     map[ChannelType]bool `json:"channels"`
	WebhookPath  string            `json:"webhookPath"`
	WebhookHost  string            `json:"webhookHost"`
	WebhookPort  int               `json:"webhookPort"`
	Polling      bool              `json:"pollingEnabled"`
	PollingLimit int               `json:"pollingLimit"`
	MaxRetries   int               `json:"maxRetries"`
	Timeout      int               `json:"defaultTimeoutSeconds"`
}

func DefaultChannelAdapterConfig() ChannelAdapterConfig {
	return ChannelAdapterConfig{
		Enabled:      true,
		Channels:     make(map[ChannelType]bool),
		WebhookPath:  "/webhook",
		WebhookHost:  "127.0.0.1",
		WebhookPort:  8787,
		Polling:      false,
		PollingLimit: 100,
		MaxRetries:   3,
		Timeout:      30,
	}
}

// ChannelAdapter is the main adapter that manages channel plugins
type ChannelAdapter struct {
	mu        sync.RWMutex
	channels  map[ChannelType]ChannelLoader
	registry  *ChannelRegistry
	config    ChannelAdapterConfig
	agentRPC  AgentRPCInterface
}

func NewChannelAdapter(cfg ChannelAdapterConfig, agentRPC AgentRPCInterface) *ChannelAdapter {
	return &ChannelAdapter{
		channels: make(map[ChannelType]ChannelLoader),
		registry:  NewChannelRegistry(),
		config:   cfg,
		agentRPC: agentRPC,
	}
}

func (a *ChannelAdapter) RegisterChannel(channel ChannelLoader) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	info := channel.ChannelInfo()
	channelType := info.Type

	if _, exists := a.channels[channelType]; exists {
		return fmt.Errorf("channel %s already registered", channelType)
	}

	if err := channel.Initialize(info.Config); err != nil {
		return fmt.Errorf("failed to initialize channel %s: %w", channelType, err)
	}

	a.channels[channelType] = channel
	a.registry.Add(info)

	log.Printf("[OK] registered channel: %s v%s", info.Name, info.Version)
	return nil
}

func (a *ChannelAdapter) UnregisterChannel(channelType ChannelType) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	channel, exists := a.channels[channelType]
	if !exists {
		return fmt.Errorf("channel %s not found", channelType)
	}

	if err := channel.Stop(); err != nil {
		log.Printf("[WARN] channel %s stop error: %v", channelType, err)
	}

	delete(a.channels, channelType)
	a.registry.Remove(string(channelType))

	return nil
}

func (a *ChannelAdapter) StartChannel(channelType ChannelType) error {
	a.mu.RLock()
	channel, exists := a.channels[channelType]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", channelType)
	}

	return channel.Start()
}

func (a *ChannelAdapter) StartAllChannels() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for channelType, channel := range a.channels {
		if err := channel.Start(); err != nil {
			log.Printf("[WARN] failed to start channel %s: %v", channelType, err)
			continue
		}
		log.Printf("[START] started channel: %s", channelType)
	}

	return nil
}

func (a *ChannelAdapter) StopAllChannels() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for channelType, channel := range a.channels {
		if err := channel.Stop(); err != nil {
			log.Printf("[WARN] failed to stop channel %s: %v", channelType, err)
		}
	}

	a.channels = make(map[ChannelType]ChannelLoader)
	return nil
}

func (a *ChannelAdapter) SendMessage(channelType ChannelType, req *SendMessageRequest) (*SendMessageResponse, error) {
	a.mu.RLock()
	channel, exists := a.channels[channelType]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelType)
	}

	return channel.SendMessage(req)
}

func (a *ChannelAdapter) HandleWebhook(channelType ChannelType, w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	channel, exists := a.channels[channelType]
	a.mu.RUnlock()

	if !exists {
		http.Error(w, "channel not found", 404)
		return
	}

	channel.HandleWebhook(w, r)
}

func (a *ChannelAdapter) ProcessMessage(msg *ChannelMessage) (*ChannelResult, error) {
	if a.agentRPC == nil {
		return nil, fmt.Errorf("agent RPC not configured")
	}

	// Generate session key from channel and chat ID
	sessionKey := fmt.Sprintf("%s_%d", msg.Channel, msg.ChatID)

	messages := []Message{
		{Role: "system", Content: fmt.Sprintf("You are an AI assistant. Received message from %s channel, chat ID: %d, user: @%s", msg.Channel, msg.ChatID, msg.Username)},
		{Role: "user", Content: msg.Text},
	}

	// Use ChatWithSession to load conversation history
	response, err := a.agentRPC.ChatWithSession(sessionKey, messages)
	if err != nil {
		return &ChannelResult{Success: false, Error: err.Error(), Timestamp: time.Now().Unix()}, err
	}

	sendReq := &SendMessageRequest{ChatID: msg.ChatID, Text: response}
	if msg.ThreadID > 0 {
		sendReq.ThreadID = msg.ThreadID
	}

	resp, err := a.SendMessage(msg.Channel, sendReq)
	if err != nil {
		return &ChannelResult{Success: false, Error: err.Error(), Timestamp: time.Now().Unix()}, err
	}

	return &ChannelResult{Success: true, Data: resp, Timestamp: time.Now().Unix()}, nil
}

func (a *ChannelAdapter) ListChannels() []ChannelType {
	a.mu.RLock()
	defer a.mu.RUnlock()

	types := make([]ChannelType, 0, len(a.channels))
	for t := range a.channels {
		types = append(types, t)
	}
	return types
}

func (a *ChannelAdapter) HasChannel(channelType ChannelType) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, exists := a.channels[channelType]
	return exists
}

func (a *ChannelAdapter) GetChannelInfo(channelType ChannelType) (*ChannelInfo, error) {
	a.mu.RLock()
	channel, exists := a.channels[channelType]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelType)
	}

	info := channel.ChannelInfo()
	return &info, nil
}

func (a *ChannelAdapter) GetRegistry() *ChannelRegistry {
	return a.registry
}

func (a *ChannelAdapter) HealthCheck() map[ChannelType]error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	results := make(map[ChannelType]error)
	for channelType, channel := range a.channels {
		if err := channel.HealthCheck(); err != nil {
			results[channelType] = err
		}
	}

	return results
}

// ChannelResult represents a channel operation result
type ChannelResult struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

func NewChannelResult(data interface{}) *ChannelResult {
	return &ChannelResult{Success: true, Data: data, Timestamp: time.Now().Unix()}
}

func NewChannelErrorResult(err error) *ChannelResult {
	return &ChannelResult{Success: false, Error: err.Error(), Timestamp: time.Now().Unix()}
}

// ChannelRegistry maintains a registry of all loaded channels
type ChannelRegistry struct {
	mu       sync.RWMutex
	channels map[string]ChannelInfo
}

func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{channels: make(map[string]ChannelInfo)}
}

func (r *ChannelRegistry) Add(info ChannelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[string(info.Type)] = info
}

func (r *ChannelRegistry) Remove(channelType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, channelType)
}

func (r *ChannelRegistry) Get(channelType string) (ChannelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.channels[channelType]
	return info, ok
}

func (r *ChannelRegistry) List() []ChannelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ChannelInfo, 0, len(r.channels))
	for _, info := range r.channels {
		infos = append(infos, info)
	}
	return infos
}
