// Package channels - Telegram channel wrapper for backward compatibility
// This file re-exports the telegram package types
package channels

import (
	telegram "github.com/gliderlab/cogate/gateway/channels/telegram"
	types "github.com/gliderlab/cogate/gateway/channels/types"
)

// NewTelegramBot creates a new Telegram bot (wrapper for telegram package)
func NewTelegramBot(token string, agentRPC types.AgentRPCInterface) ChannelLoader {
	return telegram.NewTelegramBot(token, agentRPC)
}

// TelegramChannelLoader alias for backward compatibility
type TelegramChannelLoader = telegram.TelegramBot
