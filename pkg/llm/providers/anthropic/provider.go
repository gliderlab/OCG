// Package anthropic provides Anthropic Claude provider implementation
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
)

// Provider implements llm.Provider for Anthropic
type Provider struct {
	config llm.Config
	client *http.Client
}

// New creates a new Anthropic provider
func New(cfg llm.Config) *Provider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60
	}
	return &Provider{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

// NewFromEnv creates a new Anthropic provider from environment variables
func NewFromEnv() *Provider {
	cfg := loadConfigFromEnv()
	return New(cfg)
}

func loadConfigFromEnv() llm.Config {
	return llm.Config{
		Type:            llm.ProviderAnthropic,
		APIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL:         getEnvOrDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1"),
		Model:           getEnvOrDefault("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"),
		Timeout:         60,
		EmbedAPIKey:     os.Getenv("ANTHROPIC_EMBED_API_KEY"),
		EmbedBaseURL:    getEnvOrDefault("ANTHROPIC_EMBED_BASE_URL", "https://api.openai.com/v1"),
		EmbedModel:      getEnvOrDefault("ANTHROPIC_EMBED_MODEL", "text-embedding-3-small"),
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Name returns the provider name
func (p *Provider) Name() string { return "anthropic" }

// Type returns the provider type
func (p *Provider) GetConfig() llm.Config { return p.config }
func (p *Provider) Type() llm.ProviderType { return llm.ProviderAnthropic }

// GetConfig returns the provider config

// Chat implements llm.Provider.Chat
func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	anthropicReq := map[string]interface{}{
		"model":       req.Model,
		"max_tokens":  4096,
		"messages":     convertToAnthropicMessages(req.Messages),
		"temperature": req.Temperature,
	}

	httpReq, err := p.buildRequest("/messages", anthropicReq)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Role   string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	content := ""
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	return &llm.ChatResponse{
		ID:   resp.ID,
		Model: req.Model,
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: llm.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream implements llm.Provider.ChatStream
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest, fn func(*llm.StreamChunk)) error {
	// Anthropic uses SSE (Server-Sent Events) for streaming
	anthropicReq := map[string]interface{}{
		"model":       req.Model,
		"max_tokens":  4096,
		"messages":     convertToAnthropicMessages(req.Messages),
		"temperature": req.Temperature,
		"stream":      true,
	}

	httpReq, err := p.buildRequest("/messages", anthropicReq)
	if err != nil {
		return err
	}
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read SSE stream
	buf := make([]byte, 0, 4096)
	chunkBuf := make([]byte, 0, 1024)

	for {
		n, err := resp.Body.Read(chunkBuf)
		if n > 0 {
			buf = append(buf, chunkBuf[:n]...)
			// Process complete events
			for {
				content, remaining, found := strings.Cut(string(buf), "\n")
				buf = []byte(remaining)
				if !found {
					break
				}
				content = strings.TrimSpace(content)
				if strings.HasPrefix(content, "data: ") {
					data := strings.TrimPrefix(content, "data: ")
					if data == "[DONE]" {
						return nil
					}
					// Parse SSE chunk
					var event struct {
						Type string `json:"type"`
						Delta struct {
							Text string `json:"text"`
						} `json:"delta"`
						Usage struct {
							OutputTokens int `json:"output_tokens"`
						} `json:"usage"`
					}
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						if event.Type == "content_block_delta" && event.Delta.Text != "" {
							fn(&llm.StreamChunk{
								Choices: []llm.StreamChoice{
									{
										Index: 0,
										Delta: llm.StreamDelta{
											Content: event.Delta.Text,
										},
									},
								},
							})
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return nil
}

// Embeddings implements llm.Provider.Embeddings
// Anthropic doesn't have a native embeddings API, but supports external embedding services
func (p *Provider) Embeddings(ctx context.Context, req *llm.EmbedRequest) (*llm.EmbedResponse, error) {
	// Use configured embedding service, or fallback to environment variables
	apiKey := p.config.EmbedAPIKey
	baseURL := p.config.EmbedBaseURL
	model := p.config.EmbedModel

	// Fallback to environment variables if not configured
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_EMBED_API_KEY")
	}
	if baseURL == "" {
		baseURL = getEnvOrDefault("ANTHROPIC_EMBED_BASE_URL", "https://api.openai.com/v1")
	}
	if model == "" {
		model = getEnvOrDefault("ANTHROPIC_EMBED_MODEL", "text-embedding-3-small")
	}

	if apiKey == "" {
		// Fallback to OPENAI_API_KEY if no Anthropic embed key
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("anthropic embeddings requires ANTHROPIC_EMBED_API_KEY or OPENAI_API_KEY")
		}
	}

	// Prepare embedding request (OpenAI compatible format)
	embedReq := map[string]interface{}{
		"input": req.Input,
		"model": model,
	}

	body, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var embedResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	// Convert to llm.EmbedResponse
	result := llm.EmbedResponse{
		Data: make([]llm.Embedding, len(embedResp.Data)),
		Usage: llm.Usage{
			PromptTokens: embedResp.Usage.PromptTokens,
		},
	}

	for i, e := range embedResp.Data {
		result.Data[i] = llm.Embedding{
			Object:    "embedding",
			Embedding: e.Embedding,
			Index:     e.Index,
		}
	}

	return &result, nil
}

func convertToAnthropicMessages(msgs []llm.Message) []map[string]string {
	result := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}
	return result
}

func (p *Provider) buildRequest(endpoint string, body any) (*http.Request, error) {
	url := p.config.BaseURL + endpoint
	var bodyStr string
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyStr = string(b)
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (p *Provider) doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	maxRetries := 3
	baseBackoff := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := p.client.Do(req.WithContext(ctx))
		if err != nil {
			if attempt < maxRetries-1 && isRetryable(err) {
				backoff := baseBackoff * time.Duration(1<<attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			if (resp.StatusCode >= 500 || resp.StatusCode == 429) && attempt < maxRetries-1 {
				backoff := baseBackoff * time.Duration(1<<attempt)
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, err := strconv.Atoi(retryAfter); err == nil {
						backoff = time.Duration(seconds) * time.Second
					}
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}
		return body, nil
	}
	return nil, fmt.Errorf("max retries exceeded")
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "reset") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503")
}

// Ensure Provider implements llm.Provider
var _ llm.Provider = (*Provider)(nil)

// Capabilities returns supported capabilities
func (p *Provider) Capabilities() []llm.Capability {
	return []llm.Capability{
		llm.CapabilityEmbeddings,
	}
}

// Vision implements llm.Provider.Vision
func (p *Provider) Vision(ctx context.Context, req *llm.VisionRequest) (*llm.VisionResponse, error) {
	return nil, llm.ErrCapabilityNotSupported
}

// TTS implements llm.Provider.TTS
func (p *Provider) TTS(ctx context.Context, req *llm.TTSRequest) (*llm.TTSResponse, error) {
	return nil, llm.ErrCapabilityNotSupported
}

// Transcription implements llm.Provider.Transcription
func (p *Provider) Transcription(ctx context.Context, req *llm.TranscriptionRequest) (*llm.TranscriptionResponse, error) {
	return nil, llm.ErrCapabilityNotSupported
}

// Realtime implements llm.Provider.Realtime
func (p *Provider) Realtime(ctx context.Context, cfg llm.RealtimeConfig) (llm.RealtimeProvider, error) {
	return nil, llm.ErrCapabilityNotSupported
}
