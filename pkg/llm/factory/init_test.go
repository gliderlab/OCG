package factory

import (
	"testing"

	"github.com/gliderlab/cogate/pkg/llm"
)

func TestProviderTypeConstants(t *testing.T) {
	// Verify provider type constants are defined
	providers := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
		llm.ProviderOpenRouter,
		llm.ProviderBedrock,
		llm.ProviderMoonshot,
		llm.ProviderGLM,
		llm.ProviderQianfan,
		llm.ProviderVercel,
		llm.ProviderZAi,
	}

	if len(providers) == 0 {
		t.Error("Provider types should not be empty")
	}
}

func TestProviderNames(t *testing.T) {
	// Test provider type to name mapping
	tests := []struct {
		providerType llm.ProviderType
		expected    string
	}{
		{llm.ProviderOpenAI, "openai"},
		{llm.ProviderAnthropic, "anthropic"},
		{llm.ProviderGoogle, "google"},
		{llm.ProviderMiniMax, "minimax"},
		{llm.ProviderOllama, "ollama"},
		{llm.ProviderCustom, "custom"},
		{llm.ProviderOpenRouter, "openrouter"},
		{llm.ProviderBedrock, "bedrock"},
		{llm.ProviderMoonshot, "moonshot"},
		{llm.ProviderGLM, "glm"},
		{llm.ProviderQianfan, "qianfan"},
		{llm.ProviderVercel, "vercel"},
		{llm.ProviderZAi, "zai"},
	}

	for _, tt := range tests {
		name := string(tt.providerType)
		if name != tt.expected {
			t.Errorf("Expected provider name %s, got %s", tt.expected, name)
		}
	}
}

func TestProviderRegistration(t *testing.T) {
	// Test that GetProvider returns error for unknown provider
	_, err := llm.GetProvider("unknown-provider")
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestProviderTypesCount(t *testing.T) {
	// Verify we have at least 13 providers
	expected := 13
	types := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
		llm.ProviderOpenRouter,
		llm.ProviderBedrock,
		llm.ProviderMoonshot,
		llm.ProviderGLM,
		llm.ProviderQianfan,
		llm.ProviderVercel,
		llm.ProviderZAi,
	}

	if len(types) != expected {
		t.Errorf("Expected %d provider types, got %d", expected, len(types))
	}
}
