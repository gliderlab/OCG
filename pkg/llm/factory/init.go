// Package factory provides the provider factory and initialization
package factory

import (
	"context"
	"fmt"

	"github.com/gliderlab/cogate/pkg/llm"
	"github.com/gliderlab/cogate/pkg/llm/providers/openai"
	"github.com/gliderlab/cogate/pkg/llm/providers/anthropic"
	"github.com/gliderlab/cogate/pkg/llm/providers/google"
	"github.com/gliderlab/cogate/pkg/llm/providers/minimax"
	"github.com/gliderlab/cogate/pkg/llm/providers/ollama"
	"github.com/gliderlab/cogate/pkg/llm/providers/custom"
	"github.com/gliderlab/cogate/pkg/llm/providers/openrouter"
	"github.com/gliderlab/cogate/pkg/llm/providers/bedrock"
	"github.com/gliderlab/cogate/pkg/llm/providers/moonshot"
	"github.com/gliderlab/cogate/pkg/llm/providers/glm"
	"github.com/gliderlab/cogate/pkg/llm/providers/qianfan"
	"github.com/gliderlab/cogate/pkg/llm/providers/vercel"
	"github.com/gliderlab/cogate/pkg/llm/providers/zai"
)

// InitProviders initializes all available LLM providers
func InitProviders() error {
	// OpenAI
	if openaiProvider := openai.NewFromEnv(); openaiProvider != nil {
		llm.RegisterProvider(openaiProvider)
		fmt.Printf("[OK] Registered provider: OpenAI (model: %s)\n", openaiProvider.GetConfig().Model)
	}

	// Anthropic
	if anthropicProvider := anthropic.NewFromEnv(); anthropicProvider != nil {
		llm.RegisterProvider(anthropicProvider)
		fmt.Printf("[OK] Registered provider: Anthropic (model: %s)\n", anthropicProvider.GetConfig().Model)
	}

	// Google
	if googleProvider := google.NewFromEnv(); googleProvider != nil {
		llm.RegisterProvider(googleProvider)
		fmt.Printf("[OK] Registered provider: Google (model: %s)\n", googleProvider.GetConfig().Model)
	}

	// MiniMax
	if minimaxProvider := minimax.NewFromEnv(); minimaxProvider != nil {
		llm.RegisterProvider(minimaxProvider)
		fmt.Printf("[OK] Registered provider: MiniMax (model: %s)\n", minimaxProvider.GetConfig().Model)
	}

	// Ollama (always available if running)
	ollamaProvider := ollama.NewFromEnv()
	llm.RegisterProvider(ollamaProvider)
	fmt.Printf("[OK] Registered provider: Ollama (model: %s)\n", ollamaProvider.GetConfig().Model)

	// Custom (for any OpenAI-compatible API)
	if customProvider := custom.NewFromEnv(); customProvider != nil {
		llm.RegisterProvider(customProvider)
		fmt.Printf("[OK] Registered provider: Custom (model: %s)\n", customProvider.GetConfig().Model)
	}

	// OpenRouter
	if openrouterProvider := openrouter.NewFromEnv(); openrouterProvider != nil {
		llm.RegisterProvider(openrouterProvider)
		fmt.Printf("[OK] Registered provider: OpenRouter (model: %s)\n", openrouterProvider.GetConfig().Model)
	}

	// Amazon Bedrock
	if bedrockProvider := bedrock.NewFromEnv(); bedrockProvider != nil {
		llm.RegisterProvider(bedrockProvider)
		fmt.Printf("[OK] Registered provider: Bedrock (model: %s)\n", bedrockProvider.GetConfig().Model)
	}

	// Moonshot AI
	if moonshotProvider := moonshot.NewFromEnv(); moonshotProvider != nil {
		llm.RegisterProvider(moonshotProvider)
		fmt.Printf("[OK] Registered provider: Moonshot (model: %s)\n", moonshotProvider.GetConfig().Model)
	}

	// Zhipu AI (GLM)
	if glmProvider := glm.NewFromEnv(); glmProvider != nil {
		llm.RegisterProvider(glmProvider)
		fmt.Printf("[OK] Registered provider: GLM (model: %s)\n", glmProvider.GetConfig().Model)
	}

	// Baidu Qianfan
	if qianfanProvider := qianfan.NewFromEnv(); qianfanProvider != nil {
		llm.RegisterProvider(qianfanProvider)
		fmt.Printf("[OK] Registered provider: Qianfan (model: %s)\n", qianfanProvider.GetConfig().Model)
	}

	// Vercel AI
	if vercelProvider := vercel.NewFromEnv(); vercelProvider != nil {
		llm.RegisterProvider(vercelProvider)
		fmt.Printf("[OK] Registered provider: Vercel (model: %s)\n", vercelProvider.GetConfig().Model)
	}

	// Z.AI
	if zaiProvider := zai.NewFromEnv(); zaiProvider != nil {
		llm.RegisterProvider(zaiProvider)
		fmt.Printf("[OK] Registered provider: Z.AI (model: %s)\n", zaiProvider.GetConfig().Model)
	}

	return nil
}

// GetDefaultProvider returns the default provider based on available API keys
func GetDefaultProvider() (llm.Provider, error) {
	// Priority: OpenAI > Anthropic > Google > MiniMax > Ollama > Custom
	providers := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
	}

	for _, t := range providers {
		if p, err := llm.GetProvider(t); err == nil {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no provider available")
}

// ChatWithProvider sends a chat request to the specified provider
func ChatWithProvider(providerType llm.ProviderType, messages []llm.Message, model string, temp float64) (string, error) {
	p, err := llm.GetProvider(providerType)
	if err != nil {
		// Fallback to default
		p, err = GetDefaultProvider()
		if err != nil {
			return "", err
		}
	}

	req := &llm.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
	}

	resp, err := p.Chat(context.Background(), req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from provider")
	}

	return resp.Choices[0].Message.Content, nil
}

// SimpleChat is a convenience function for quick chat
func SimpleChat(prompt string) (string, error) {
	p, err := GetDefaultProvider()
	if err != nil {
		return "", err
	}

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	req := &llm.ChatRequest{
		Messages:    messages,
		Temperature: 0.7,
	}

	resp, err := p.Chat(context.Background(), req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}

	return resp.Choices[0].Message.Content, nil
}
