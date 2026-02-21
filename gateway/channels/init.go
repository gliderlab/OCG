// Package channels - Channel registry and initialization
// Provides unified channel loading and management

package channels

import (
	"log"
	"os"

	"github.com/gliderlab/cogate/gateway/channels/discord"
	"github.com/gliderlab/cogate/gateway/channels/feishu"
	"github.com/gliderlab/cogate/gateway/channels/imessage"
	"github.com/gliderlab/cogate/gateway/channels/line"
	"github.com/gliderlab/cogate/gateway/channels/matrix"
	"github.com/gliderlab/cogate/gateway/channels/mattermost"
	"github.com/gliderlab/cogate/gateway/channels/session"
	"github.com/gliderlab/cogate/gateway/channels/signal"
	"github.com/gliderlab/cogate/gateway/channels/slack"
	"github.com/gliderlab/cogate/gateway/channels/threema"
	"github.com/gliderlab/cogate/gateway/channels/tox"
	"github.com/gliderlab/cogate/gateway/channels/types"
	"github.com/gliderlab/cogate/gateway/channels/webchat"
	"github.com/gliderlab/cogate/gateway/channels/whatsapp"
	"github.com/gliderlab/cogate/gateway/channels/zalo"
)

// ChannelRegistry holds all available channel implementations
var channelRegistry = make(map[types.ChannelType]func(agentRPC AgentRPCInterface) (ChannelLoader, error))

// RegisterChannel registers a channel implementation
func RegisterChannel(channelType types.ChannelType, factory func(agentRPC AgentRPCInterface) (ChannelLoader, error)) {
	channelRegistry[channelType] = factory
	log.Printf("Registered channel factory: %s", channelType)
}

// GetChannelFactory returns a channel factory by type
func GetChannelFactory(channelType types.ChannelType) (func(agentRPC AgentRPCInterface) (ChannelLoader, error), bool) {
	factory, ok := channelRegistry[channelType]
	return factory, ok
}

// ListAvailableChannels returns all registered channel types
func ListAvailableChannels() []types.ChannelType {
	types := make([]types.ChannelType, 0, len(channelRegistry))
	for t := range channelRegistry {
		types = append(types, t)
	}
	return types
}

// Deprecated: InitializeAllChannels is not used in the current implementation.
// Channel initialization is done directly in gateway.go via adapter.RegisterChannel().
// This function remains for potential future use or as a reference.
func InitializeAllChannels(agentRPC AgentRPCInterface) *ChannelAdapter {
	adapter := NewChannelAdapter(DefaultChannelAdapterConfig(), agentRPC)
	count := 0

	// Register Telegram (uses legacy bot.go)
	if os.Getenv("TELEGRAM_BOT_TOKEN") != "" {
		RegisterChannel(types.ChannelTelegram, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return nil, nil
		})
		count++
	}

	// Register Discord
	if f, err := discord.DiscordCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelDiscord, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return discord.DiscordCreateFromEnv(rpc)
		})
		count++
	}

	// Register Slack
	if f, err := slack.SlackCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelSlack, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return slack.SlackCreateFromEnv(rpc)
		})
		count++
	}

	// Register WhatsApp
	if f, err := whatsapp.WhatsAppCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelWhatsApp, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return whatsapp.WhatsAppCreateFromEnv(rpc)
		})
		count++
	}

	// Register Signal
	if f, err := signal.SignalCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelSignal, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return signal.SignalCreateFromEnv(rpc)
		})
		count++
	}

	// New channels - Mattermost
	if f, err := mattermost.MattermostCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelMattermost, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return mattermost.MattermostCreateFromEnv(rpc)
		})
		count++
	}

	// LINE
	if f, err := line.LineCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelLINE, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return line.LineCreateFromEnv(rpc)
		})
		count++
	}

	// Matrix
	if f, err := matrix.MatrixCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelMatrix, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return matrix.MatrixCreateFromEnv(rpc)
		})
		count++
	}

	// Feishu
	if f, err := feishu.FeishuCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelFeishu, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return feishu.FeishuCreateFromEnv(rpc)
		})
		count++
	}

	// Zalo
	if f, err := zalo.ZaloCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelZalo, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return zalo.ZaloCreateFromEnv(rpc)
		})
		count++
	}

	// Threema
	if f, err := threema.ThreemaCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelThreema, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return threema.ThreemaCreateFromEnv(rpc)
		})
		count++
	}

	// Session
	if f, err := session.SessionCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelSession, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return session.SessionCreateFromEnv(rpc)
		})
		count++
	}

	// Tox
	if f, err := tox.ToxCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelTox, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return tox.ToxCreateFromEnv(rpc)
		})
		count++
	}

	// iMessage (via BlueBubbles/AirMessage REST API)
	if f, err := imessage.IMessageCreateFromEnv(agentRPC); err == nil && f != nil {
		RegisterChannel(types.ChannelIMessage, func(rpc AgentRPCInterface) (ChannelLoader, error) {
			return imessage.IMessageCreateFromEnv(rpc)
		})
		count++
	}

	// WebChat (always available)
	RegisterChannel(types.ChannelWebChat, func(rpc AgentRPCInterface) (ChannelLoader, error) {
		return webchat.NewWebChatChannel(rpc), nil
	})
	count++

	log.Printf("[OK] Registered %d channel types", count)
	return adapter
}
