// Package minimax provides MiniMax provider implementation
package minimax

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

// Provider implements llm.Provider for MiniMax
type Provider struct {
	config  llm.Config
	client  *http.Client
	groupID string
}

// New creates a new MiniMax provider
func New(cfg llm.Config) *Provider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60
	}
	return &Provider{
		config:  cfg,
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
		groupID: os.Getenv("MINIMAX_GROUP_ID"),
	}
}

// NewFromEnv creates a new MiniMax provider from environment variables
func NewFromEnv() *Provider {
	cfg := loadConfigFromEnv()
	return New(cfg)
}

func loadConfigFromEnv() llm.Config {
	return llm.Config{
		Type:    llm.ProviderMiniMax,
		APIKey:  os.Getenv("MINIMAX_API_KEY"),
		BaseURL: getEnvOrDefault("MINIMAX_BASE_URL", "https://api.minimax.chat/v1"),
		Model:   getEnvOrDefault("MINIMAX_MODEL", "MiniMax-M2.1"),
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
func (p *Provider) Name() string { return "minimax" }

// Type returns the provider type
func (p *Provider) GetConfig() llm.Config { return p.config }
func (p *Provider) Type() llm.ProviderType { return llm.ProviderMiniMax }

// GetConfig returns the provider config

// Chat implements llm.Provider.Chat
func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	minimaxReq := map[string]interface{}{
		"model":       req.Model,
		"messages":     req.Messages,
		"temperature": req.Temperature,
		"tokens":      req.MaxTokens,
	}

	httpReq, err := p.buildRequest("/text/chatcompletion_v2", minimaxReq)
	if err != nil {
		return nil, err
	}
	if p.groupID != "" {
		httpReq.Header.Set("GroupId", p.groupID)
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp struct {
		BaseResp struct {
			StatusCode   int    `json:"status_code"`
			StatusMsg    string `json:"status_msg"`
		} `json:"base_resp"`
		Choices []struct {
			FinishReason string      `json:"finish_reason"`
			Message      llm.Message `json:"message"`
		} `json:"choices"`
		Usage llm.Usage `json:"usage"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax error: %s", resp.BaseResp.StatusMsg)
	}

	if len(resp.Choices) > 0 {
		content := resp.Choices[0].Message.Content
		finishReason := resp.Choices[0].FinishReason
		return &llm.ChatResponse{
			ID:      "",
			Model:   req.Model,
			Choices: []llm.Choice{{Index: 0, Message: llm.Message{Role: "assistant", Content: content}, FinishReason: finishReason}},
			Usage:   resp.Usage,
		}, nil
	}

	// No choices returned - return empty response instead of panicking
	return &llm.ChatResponse{
		ID:      "",
		Model:   req.Model,
		Choices: []llm.Choice{{Index: 0, Message: llm.Message{Role: "assistant", Content: ""}, FinishReason: "stop"}},
		Usage:   resp.Usage,
	}, nil
}

// ChatStream implements llm.Provider.ChatStream
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest, fn func(*llm.StreamChunk)) error {
	minimaxReq := map[string]interface{}{
		"model":       req.Model,
		"messages":     req.Messages,
		"temperature": req.Temperature,
		"tokens":      req.MaxTokens,
		"stream":      true,
	}

	httpReq, err := p.buildRequest("/text/chatcompletion_v2", minimaxReq)
	if err != nil {
		return err
	}
	if p.groupID != "" {
		httpReq.Header.Set("GroupId", p.groupID)
	}

	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if len(chunk.Choices) > 0 {
			fn(&llm.StreamChunk{
				Choices: []llm.StreamChoice{
					{
						Index: 0,
						Delta: llm.StreamDelta{
							Content: chunk.Choices[0].Delta.Content,
						},
						FinishReason: chunk.Choices[0].FinishReason,
					},
				},
			})
			if chunk.Choices[0].FinishReason != "" {
				break
			}
		}
	}
	return nil
}

// Embeddings implements llm.Provider.Embeddings
func (p *Provider) Embeddings(ctx context.Context, req *llm.EmbedRequest) (*llm.EmbedResponse, error) {
	embedReq := map[string]interface{}{
		"model": req.Model,
		"texts": []string{req.Input},
	}

	httpReq, err := p.buildRequest("/embeddings", embedReq)
	if err != nil {
		return nil, err
	}
	if p.groupID != "" {
		httpReq.Header.Set("GroupId", p.groupID)
	}

	body, err := p.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var resp llm.EmbedResponse
	json.Unmarshal(body, &resp)
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
