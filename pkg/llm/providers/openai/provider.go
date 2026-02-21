// Package openai provides OpenAI provider implementation
package openai

import (
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

// Provider implements llm.Provider for OpenAI
type Provider struct {
	config llm.Config
	client *http.Client
}

// New creates a new OpenAI provider
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

// NewFromEnv creates a new OpenAI provider from environment variables
func NewFromEnv() *Provider {
	cfg := loadConfigFromEnv()
	return New(cfg)
}

func loadConfigFromEnv() llm.Config {
	return llm.Config{
		Type:    llm.ProviderOpenAI,
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL:  getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		Model:   getEnvOrDefault("OPENAI_MODEL", "gpt-4o"),
		Timeout: 60,
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Name returns the provider name
func (p *Provider) Name() string { return "openai" }

// Type returns the provider type
func (p *Provider) GetConfig() llm.Config { return p.config }
func (p *Provider) Type() llm.ProviderType { return llm.ProviderOpenAI }

// GetConfig returns the provider config

// Chat implements llm.Provider.Chat
func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	httpReq, err := p.buildRequest("/chat/completions", req)
	if err != nil {
		return nil, err
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp llm.ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ChatStream implements llm.Provider.ChatStream
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest, fn func(*llm.StreamChunk)) error {
	req.Stream = true
	httpReq, err := p.buildRequest("/chat/completions", req)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk llm.StreamChunk
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		fn(&chunk)
		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
			break
		}
	}
	return nil
}

// Embeddings implements llm.Provider.Embeddings
func (p *Provider) Embeddings(ctx context.Context, req *llm.EmbedRequest) (*llm.EmbedResponse, error) {
	httpReq, err := p.buildRequest("/embeddings", req)
	if err != nil {
		return nil, err
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp llm.EmbedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
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

// Capabilities returns supported capabilities
func (p *Provider) Capabilities() []llm.Capability {
	return []llm.Capability{
		llm.CapabilityEmbeddings,
		llm.CapabilityVision,
		llm.CapabilityTTS,
		llm.CapabilityTranscription,
		llm.CapabilityRealtime,
	}
}

// Vision implements llm.Provider.Vision
func (p *Provider) Vision(ctx context.Context, req *llm.VisionRequest) (*llm.VisionResponse, error) {
	// Use GPT-4V for vision
	httpReq, err := p.buildRequest("/chat/completions", map[string]interface{}{
		"model":       "gpt-4o",
		"messages":    req.Messages,
		"max_tokens": req.MaxTokens,
		"temperature": req.Temperature,
	})
	if err != nil {
		return nil, err
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage llm.Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no vision response")
	}

	return &llm.VisionResponse{
		Content: resp.Choices[0].Message.Content,
		Usage:   resp.Usage,
	}, nil
}

// TTS implements llm.Provider.TTS
func (p *Provider) TTS(ctx context.Context, req *llm.TTSRequest) (*llm.TTSResponse, error) {
	voice := req.Voice
	if voice == "" {
		voice = "alloy"
	}

	bodyMap := map[string]interface{}{
		"model": "tts-1",
		"input": req.Input,
		"voice": voice,
	}
	if req.Speed > 0 {
		bodyMap["speed"] = req.Speed
	}
	if req.Format != "" {
		bodyMap["format"] = req.Format
	}

	httpReq, err := p.buildRequest("/audio/speech", bodyMap)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("TTS API error: %s", string(audio))
	}

	return &llm.TTSResponse{Audio: audio}, nil
}

// Transcription implements llm.Provider.Transcription
func (p *Provider) Transcription(ctx context.Context, req *llm.TranscriptionRequest) (*llm.TranscriptionResponse, error) {
	// Note: Need multipart form request for actual implementation
	// This is a placeholder
	return nil, llm.ErrCapabilityNotSupported
}

// Realtime implements llm.Provider.Realtime
func (p *Provider) Realtime(ctx context.Context, cfg llm.RealtimeConfig) (llm.RealtimeProvider, error) {
	return nil, llm.ErrCapabilityNotSupported
}

// Ensure Provider implements llm.Provider
var _ llm.Provider = (*Provider)(nil)
