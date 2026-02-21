// Package moonshot provides Moonshot AI provider
package moonshot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
)

func NewFromEnv() *Provider {
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	if apiKey == "" {
		return nil
	}
	cfg := llm.Config{
		Type:    llm.ProviderMoonshot,
		APIKey:  apiKey,
		BaseURL: getEnv("MOONSHOT_BASE_URL", "https://api.moonshot.cn/v1"),
		Model:   getEnv("MOONSHOT_MODEL", "moonshot-v1-8k"),
		Timeout: 60,
	}
	return New(cfg)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type Provider struct {
	config llm.Config
	client *http.Client
}

func New(cfg llm.Config) *Provider {
	return &Provider{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second},
	}
}

func (p *Provider) Name() string           { return "moonshot" }
func (p *Provider) Type() llm.ProviderType { return llm.ProviderMoonshot }
func (p *Provider) GetConfig() llm.Config  { return p.config }

func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return p.doChat(ctx, req, false)
}

func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest, fn func(*llm.StreamChunk)) error {
	_, err := p.doChat(ctx, req, true)
	return err
}

func (p *Provider) Embeddings(ctx context.Context, req *llm.EmbedRequest) (*llm.EmbedResponse, error) {
	url := p.config.BaseURL + "/embeddings"
	body, _ := json.Marshal(req)
	
	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	
	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	data, _ := io.ReadAll(resp.Body)
	var result llm.EmbedResponse
	json.Unmarshal(data, &result)
	return &result, nil
}

func (p *Provider) doChat(ctx context.Context, req *llm.ChatRequest, stream bool) (*llm.ChatResponse, error) {
	url := p.config.BaseURL + "/chat/completions"
	req.Stream = stream
	body, _ := json.Marshal(req)
	
	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	
	resp, err := p.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	data, _ := io.ReadAll(resp.Body)
	var result llm.ChatResponse
	json.Unmarshal(data, &result)
	return &result, nil
}

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
