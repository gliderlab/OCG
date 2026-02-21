// Package ollama provides Ollama local provider implementation
package ollama

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

// Provider implements llm.Provider for Ollama
type Provider struct {
	config llm.Config
	client *http.Client
}

// New creates a new Ollama provider
func New(cfg llm.Config) *Provider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 300 // Ollama can be slow
	}
	return &Provider{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

// NewFromEnv creates a new Ollama provider from environment variables
func NewFromEnv() *Provider {
	cfg := loadConfigFromEnv()
	return New(cfg)
}

func loadConfigFromEnv() llm.Config {
	return llm.Config{
		Type:    llm.ProviderOllama,
		APIKey:  "", // Ollama doesn't need API key
		BaseURL: getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
		Model:   getEnvOrDefault("OLLAMA_MODEL", "llama3"),
		Timeout: 300,
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Name returns the provider name
func (p *Provider) Name() string { return "ollama" }

// Type returns the provider type
func (p *Provider) GetConfig() llm.Config { return p.config }
func (p *Provider) Type() llm.ProviderType { return llm.ProviderOllama }

// GetConfig returns the provider config

// Chat implements llm.Provider.Chat
func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	ollamaReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"stream":     false,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	httpReq, err := p.buildRequest("/api/chat", ollamaReq)
	if err != nil {
		return nil, err
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Message      llm.Message `json:"message"`
		Done         bool        `json:"done"`
		PromptEvalCount int      `json:"prompt_eval_count"`
		EvalCount    int         `json:"eval_count"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &llm.ChatResponse{
		ID:      "",
		Model:   req.Model,
		Choices: []llm.Choice{{Index: 0, Message: resp.Message, FinishReason: "stop"}},
		Usage: llm.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}, nil
}

// ChatStream implements llm.Provider.ChatStream
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest, fn func(*llm.StreamChunk)) error {
	ollamaReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"stream":     true,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	httpReq, err := p.buildRequest("/api/chat", ollamaReq)
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
		var chunk struct {
			Message struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"message"`
			Done         bool `json:"done"`
			PromptEvalCount int `json:"prompt_eval_count"`
			EvalCount    int  `json:"eval_count"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		fn(&llm.StreamChunk{
			Choices: []llm.StreamChoice{
				{
					Index: 0,
					Delta: llm.StreamDelta{
						Content: chunk.Message.Content,
						Role:    chunk.Message.Role,
					},
				},
			},
		})
		if chunk.Done {
			break
		}
	}
	return nil
}

// Embeddings implements llm.Provider.Embeddings
func (p *Provider) Embeddings(ctx context.Context, req *llm.EmbedRequest) (*llm.EmbedResponse, error) {
	embedReq := map[string]interface{}{
		"model":  req.Model,
		"prompt": req.Input,
	}

	httpReq, err := p.buildRequest("/api/embeddings", embedReq)
	if err != nil {
		return nil, err
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &llm.EmbedResponse{
		Data: []llm.Embedding{{Object: "embedding", Embedding: resp.Embedding, Index: 0}},
		Usage: llm.Usage{},
	}, nil
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
