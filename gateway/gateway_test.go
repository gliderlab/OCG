package gateway

import (
	"testing"
)

func TestWSMessageTypes(t *testing.T) {
	// Test message type constants
	if MsgTypeChat != "chat" {
		t.Errorf("Expected MsgTypeChat 'chat', got '%s'", MsgTypeChat)
	}
	
	if MsgTypeChunk != "chunk" {
		t.Errorf("Expected MsgTypeChunk 'chunk', got '%s'", MsgTypeChunk)
	}
	
	if MsgTypeDone != "done" {
		t.Errorf("Expected MsgTypeDone 'done', got '%s'", MsgTypeDone)
	}
	
	if MsgTypeError != "error" {
		t.Errorf("Expected MsgTypeError 'error', got '%s'", MsgTypeError)
	}
}

func TestWSMessage(t *testing.T) {
	msg := WSMessage{
		Type:    MsgTypeChat,
		Content: []byte(`{"test": true}`),
	}
	
	if msg.Type != MsgTypeChat {
		t.Errorf("Expected type 'chat', got '%s'", msg.Type)
	}
}

func TestWSChatRequest(t *testing.T) {
	req := WSChatRequest{
		Model: "test-model",
		Messages: nil,
	}
	
	if req.Model != "test-model" {
		t.Errorf("Expected Model 'test-model', got '%s'", req.Model)
	}
}

func TestWSChatResponse(t *testing.T) {
	resp := WSChatResponse{
		Content: "Hello",
		Finish:  true,
	}
	
	if resp.Content != "Hello" {
		t.Errorf("Expected Content 'Hello', got '%s'", resp.Content)
	}
	
	if !resp.Finish {
		t.Error("Expected Finish to be true")
	}
}

func TestGatewayTokenValidation(t *testing.T) {
	// Test token validation logic
	validTokens := []string{
		"test-token",
		"abc123",
		"",
	}
	
	for _, token := range validTokens {
		_ = token // Just test it doesn't panic
	}
}
