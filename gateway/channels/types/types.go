// Package types - Shared types and interfaces for channels
// This package is imported by both the main channels package and individual channel packages
package types

import (
	"net/http"
)

// ChannelType represents the type of communication channel
type ChannelType string

const (
	ChannelTelegram   ChannelType = "telegram"
	ChannelWhatsApp   ChannelType = "whatsapp"
	ChannelSlack      ChannelType = "slack"
	ChannelDiscord    ChannelType = "discord"
	ChannelWebChat    ChannelType = "webchat"
	ChannelSignal     ChannelType = "signal"
	ChannelGoogleChat ChannelType = "googlechat"
	ChannelIMessage   ChannelType = "imessage"
	ChannelMSTeams    ChannelType = "msteams"
	ChannelIRC        ChannelType = "irc"
	ChannelLINE       ChannelType = "line"
	ChannelMatrix     ChannelType = "matrix"
	ChannelFeishu     ChannelType = "feishu"
	ChannelZalo       ChannelType = "zalo"
	ChannelMattermost ChannelType = "mattermost"
	ChannelThreema    ChannelType = "threema"
	ChannelSession    ChannelType = "session"
	ChannelTox        ChannelType = "tox"
)

const MaxWebhookBodyBytes int64 = 1 << 20

func LimitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxWebhookBodyBytes)
}

// ChannelInfo contains metadata about a channel
type ChannelInfo struct {
	Name         string                 `json:"name"`
	Type         ChannelType            `json:"type"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
}

// ChannelLoader defines the interface for channel plugins
type ChannelLoader interface {
	ChannelInfo() ChannelInfo
	Initialize(config map[string]interface{}) error
	Start() error
	Stop() error
	SendMessage(req *SendMessageRequest) (*SendMessageResponse, error)
	HandleWebhook(w http.ResponseWriter, r *http.Request)
	HealthCheck() error
}

// SendMessageRequest represents a message to send
type SendMessageRequest struct {
	ChatID    int64      `json:"chatId"`
	Text      string     `json:"text"`
	ParseMode string     `json:"parseMode,omitempty"`
	Media     string     `json:"media,omitempty"`
	Buttons   [][]Button `json:"buttons,omitempty"`
	ReplyTo   int64      `json:"replyToMessageId,omitempty"`
	ThreadID  int64      `json:"messageThreadId,omitempty"`
}

// Button represents an inline button
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// SendMessageResponse represents the response from sending a message
type SendMessageResponse struct {
	OK        bool   `json:"ok"`
	MessageID int64  `json:"messageId"`
	ChatID    int64  `json:"chatId"`
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

// ChannelMessage represents an incoming channel message
type ChannelMessage struct {
	ID        string                 `json:"id"`
	Channel   ChannelType            `json:"channel"`
	ChatID    int64                  `json:"chatId"`
	UserID    int64                  `json:"userId"`
	Username  string                 `json:"username"`
	Text      string                 `json:"text"`
	Timestamp int64                  `json:"timestamp"`
	ThreadID  int64                  `json:"threadId,omitempty"`
	Media     *ChannelMedia          `json:"media,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ChannelMedia represents media attachments
type ChannelMedia struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Caption  string `json:"caption"`
	MimeType string `json:"mimeType"`
	Duration int    `json:"duration,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AgentRPCInterface defines the interface for agent communication
type AgentRPCInterface interface {
	Chat(messages []Message) (string, error)
	ChatWithSession(sessionKey string, messages []Message) (string, error) // New: with session context
	GetStats() (map[string]int, error)
}
