package types

import (
	"testing"
)

func TestChannelTypeString(t *testing.T) {
	tests := []struct {
		channelType ChannelType
		expected   string
	}{
		{ChannelTelegram, "telegram"},
		{ChannelWhatsApp, "whatsapp"},
		{ChannelSlack, "slack"},
		{ChannelDiscord, "discord"},
		{ChannelWebChat, "webchat"},
		{ChannelSignal, "signal"},
		{ChannelGoogleChat, "googlechat"},
		{ChannelMSTeams, "msteams"},
		{ChannelIRC, "irc"},
		{ChannelMatrix, "matrix"},
		{ChannelFeishu, "feishu"},
		{ChannelZalo, "zalo"},
		{ChannelLINE, "line"},
	}

	for _, tt := range tests {
		if string(tt.channelType) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, string(tt.channelType))
		}
	}
}

func TestChannelTypeConversion(t *testing.T) {
	// Test string to ChannelType conversion
	s := "telegram"
	chType := ChannelType(s)
	if chType != ChannelTelegram {
		t.Errorf("Expected ChannelTelegram, got %v", chType)
	}
}

func TestSendMessageResponse(t *testing.T) {
	resp := &SendMessageResponse{
		OK:        true,
		MessageID: 12345,
	}

	if !resp.OK {
		t.Error("Expected OK to be true")
	}

	if resp.MessageID != 12345 {
		t.Errorf("Expected MessageID 12345, got %d", resp.MessageID)
	}
}

func TestChannelInfo(t *testing.T) {
	info := ChannelInfo{
		Name: "Test Bot",
		Type: ChannelTelegram,
	}

	if info.Type != ChannelTelegram {
		t.Errorf("Expected ChannelTelegram, got %v", info.Type)
	}

	if info.Name != "Test Bot" {
		t.Errorf("Expected 'Test Bot', got '%s'", info.Name)
	}
}
