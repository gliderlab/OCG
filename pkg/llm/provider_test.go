package llm

import (
	"testing"
)

func TestProviderTypes(t *testing.T) {
	// Test provider type constants
	types := []ProviderType{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGoogle,
		ProviderMiniMax,
		ProviderOllama,
		ProviderCustom,
	}
	
	if len(types) == 0 {
		t.Error("Provider types should not be empty")
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}
	
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}
	
	if msg.Content != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", msg.Content)
	}
}

func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Temperature: 0.7,
	}
	
	if req.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", req.Model)
	}
	
	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}
}

func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		Model: "gpt-4",
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: "Hi there!",
				},
				Index: 0,
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:     15,
		},
	}
	
	if resp.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", resp.Model)
	}
	
	if len(resp.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(resp.Choices))
	}
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:     150,
	}
	
	if usage.PromptTokens != 100 {
		t.Errorf("Expected 100 prompt tokens, got %d", usage.PromptTokens)
	}
	
	if usage.CompletionTokens != 50 {
		t.Errorf("Expected 50 completion tokens, got %d", usage.CompletionTokens)
	}
	
	if usage.TotalTokens != 150 {
		t.Errorf("Expected 150 total tokens, got %d", usage.TotalTokens)
	}
}
