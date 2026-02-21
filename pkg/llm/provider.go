// Package llm provides LLM provider abstraction layer
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGoogle     ProviderType = "google"
	ProviderMiniMax    ProviderType = "minimax"
	ProviderOllama     ProviderType = "ollama"
	ProviderCustom     ProviderType = "custom"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderBedrock    ProviderType = "bedrock"
	ProviderMoonshot  ProviderType = "moonshot"
	ProviderGLM        ProviderType = "glm"
	ProviderQianfan    ProviderType = "qianfan"
	ProviderVercel     ProviderType = "vercel"
	ProviderZAi        ProviderType = "zai"
)

// Capability represents optional provider capabilities
type Capability string

const (
	CapabilityRealtime     Capability = "realtime"
	CapabilityVision       Capability = "vision"
	CapabilityTTS          Capability = "tts"
	CapabilityTranscription Capability = "transcription"
	CapabilityEmbeddings   Capability = "embeddings"
)

// Message represents a chat message
type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Name    string    `json:"name,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

// ToolCall represents a function tool call
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function *ToolFunction `json:"function"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// Tool represents a function tool
type Tool struct {
	Type     string       `json:"type"`
	Function *ToolFunction `json:"function,omitempty"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int      `json:"index"`
	Message      Message  `json:"message"`
	FinishReason string   `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        StreamDelta  `json:"delta"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

type StreamDelta struct {
	Content   string    `json:"content"`
	Role      string    `json:"role,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ============ New Types for Extended Capabilities ============

// EmbedRequest represents an embedding request
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse represents an embedding response
type EmbedResponse struct {
	Data []Embedding `json:"data"`
	Usage Usage      `json:"usage"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// VisionRequest represents a vision/image analysis request
type VisionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// VisionResponse represents a vision response
type VisionResponse struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// TTSRequest represents a text-to-speech request
type TTSRequest struct {
	Model      string  `json:"model"`
	Input      string  `json:"input"`
	Voice      string  `json:"voice,omitempty"`
	Speed      float64 `json:"speed,omitempty"`
	Format     string  `json:"format,omitempty"` // mp3, pcm, etc.
}

// TTSResponse represents a TTS response (audio bytes)
type TTSResponse struct {
	Audio []byte `json:"audio"`
}

// TranscriptionRequest represents a speech-to-text request
type TranscriptionRequest struct {
	Model       string `json:"model"`
	AudioData   []byte `json:"audio_data"`
	Language    string `json:"language,omitempty"`
}

// TranscriptionResponse represents a transcription response
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// RealtimeConfig represents Realtime API configuration
type RealtimeConfig struct {
	Model        string `json:"model"`
	APIKey       string `json:"apiKey,omitempty"`
	BaseURL      string `json:"baseUrl,omitempty"`
	Voice        string `json:"voice,omitempty"`
	Instructions string `json:"instructions,omitempty"`

	// Generation parameters
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"topP,omitempty"`
	TopK        float32 `json:"topK,omitempty"`
	MaxTokens   int32   `json:"maxTokens,omitempty"`
	Seed        int32   `json:"seed,omitempty"`

	// Thinking
	Thinking        bool  `json:"thinking,omitempty"`
	IncludeThoughts bool  `json:"includeThoughts,omitempty"`
	ThinkingBudget  int32 `json:"thinkingBudget,omitempty"`

	// Tools (Function Calling)
	Tools []Tool `json:"tools,omitempty"`

	// Audio and speech
	InputAudioTranscription  bool   `json:"inputAudioTranscription,omitempty"`
	OutputAudioTranscription bool   `json:"outputAudioTranscription,omitempty"`
	InputLanguage            string `json:"inputLanguage,omitempty"`
	SpeechLanguageCode       string `json:"speechLanguageCode,omitempty"`
	// TODO: EnableAffectiveDialog - not supported by Gemini Live API yet (server returns "Cannot find field")
	// EnableAffectiveDialog *bool `json:"enableAffectiveDialog,omitempty"`
	ProactiveAudio  *bool  `json:"proactiveAudio,omitempty"`
	MediaResolution string `json:"mediaResolution,omitempty"` // low|medium|high

	// Session
	SessionResumption            bool   `json:"sessionResumption,omitempty"`
	SessionResumptionHandle      string `json:"sessionResumptionHandle,omitempty"`
	SessionResumptionTransparent bool   `json:"sessionResumptionTransparent,omitempty"`

	// VAD (Voice Activity Detection)
	ExplicitVAD         bool   `json:"explicitVAD,omitempty"`
	AutoVADDisabled     *bool  `json:"autoVADDisabled,omitempty"`
	VADStartSensitivity string `json:"vadStartSensitivity,omitempty"` // low|high
	VADEndSensitivity   string `json:"vadEndSensitivity,omitempty"`   // low|high
	VADPrefixPaddingMs  int32  `json:"vadPrefixPaddingMs,omitempty"`
	VADSilenceDurationMs int32 `json:"vadSilenceDurationMs,omitempty"`
	VADActivityHandling string `json:"vadActivityHandling,omitempty"` // start_of_activity_interrupts|no_interruption
	VADTurnCoverage     string `json:"vadTurnCoverage,omitempty"`     // turn_includes_only_activity|turn_includes_all_input

	// Context compression
	ContextWindowCompression             bool  `json:"contextWindowCompression,omitempty"`
	ContextCompressionTriggerTokens      int64 `json:"contextCompressionTriggerTokens,omitempty"`
	ContextCompressionTargetTokens       int64 `json:"contextCompressionTargetTokens,omitempty"`
}

// ToolResponse represents a tool response to send back
type ToolResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Result any   `json:"result"`
}

// TranscriptionResult represents transcribed audio
type TranscriptionResult struct {
	Text   string `json:"text"`
	Type   string `json:"type"` // "input" or "output"
}

// VADSignal represents voice activity detection signal
type VADSignal struct {
	Active bool   `json:"active"` // true = speech started, false = speech ended
	Type   string `json:"type"`   // "start" or "end"
}

// RealtimeProvider represents a realtime voice provider interface
type RealtimeProvider interface {
	// Connection management
	Connect(ctx context.Context, cfg RealtimeConfig) error
	Disconnect() error
	IsConnected() bool

	// Audio input
	SendAudio(ctx context.Context, audioData []byte) error
	EndAudio(ctx context.Context) error
	OnAudio(fn func(audio []byte))

	// Messages
	SendText(ctx context.Context, text string) error
	OnText(fn func(text string))

	// Tools (Function Calling)
	OnToolCall(fn func(toolCall ToolCall))
	SendToolResponse(ctx context.Context, resp ToolResponse) error

	// Transcription
	OnTranscription(fn func(result TranscriptionResult))

	// VAD (Voice Activity Detection)
	OnVAD(fn func(signal VADSignal))

	// Session events
	OnGoAway(fn func(reason string))
	OnSessionUpdate(fn func(resumable bool))
	OnUsage(fn func(promptTokens, completionTokens int))

	// Events
	OnError(fn func(err error))
	OnDisconnect(fn func())
}

// ============ Provider Interface ============

// Provider defines the interface for LLM providers
type Provider interface {
	// Basic capabilities (required)
	Name() string
	Type() ProviderType
	GetConfig() Config
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest, fn func(*StreamChunk)) error

	// Optional capabilities - return ErrCapabilityNotSupported if not implemented
	Capabilities() []Capability

	// Embeddings - return nil, ErrCapabilityNotSupported if not supported
	Embeddings(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)

	// Vision - return nil, ErrCapabilityNotSupported if not supported
	Vision(ctx context.Context, req *VisionRequest) (*VisionResponse, error)

	// TTS - return nil, ErrCapabilityNotSupported if not supported
	TTS(ctx context.Context, req *TTSRequest) (*TTSResponse, error)

	// Transcription - return nil, ErrCapabilityNotSupported if not supported
	Transcription(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResponse, error)

	// Realtime - return nil, ErrCapabilityNotSupported if not supported
	Realtime(ctx context.Context, cfg RealtimeConfig) (RealtimeProvider, error)
}

// ErrCapabilityNotSupported is returned when a capability is not supported
var ErrCapabilityNotSupported = fmt.Errorf("capability not supported")

// Config holds provider configuration
type Config struct {
	Type            ProviderType       `json:"type"`
	APIKey          string             `json:"apiKey,omitempty"`
	BaseURL         string             `json:"baseUrl,omitempty"`
	Model           string             `json:"model,omitempty"`
	Timeout         int                `json:"timeout,omitempty"`
	Headers         map[string]string  `json:"headers,omitempty"`
	EmbedAPIKey     string             `json:"embedApiKey,omitempty"`
	EmbedBaseURL    string             `json:"embedBaseUrl,omitempty"`
	EmbedModel      string             `json:"embedModel,omitempty"`
}

// ModelsConfig holds models configuration
type ModelsConfig struct {
	Primary    string            `json:"primary"`
	Fallbacks []string          `json:"fallbacks,omitempty"`
	Models     map[string]ModelConfig `json:"models,omitempty"`
}

// ModelConfig holds individual model configuration
type ModelConfig struct {
	Alias         string `json:"alias,omitempty"`
	APIKey        string `json:"apiKey,omitempty"`
	BaseURL       string `json:"baseUrl,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty"`
}

// GetContextWindow attempts to get context window from API, falls back to config
func GetContextWindow(providerType ProviderType, model, baseURL, apiKey string, modelsCfg ModelsConfig) int {
	// 1. Try to get from API
	if ctxWindow := fetchContextWindowFromAPI(providerType, model, baseURL, apiKey); ctxWindow > 0 {
		return ctxWindow
	}

	// 2. Fallback to config
	if modelsCfg.Models != nil {
		if cfg, ok := modelsCfg.Models[model]; ok && cfg.ContextWindow > 0 {
			return cfg.ContextWindow
		}
		// Try matching prefix (e.g., "gpt-4" matches "gpt-4o")
		for name, cfg := range modelsCfg.Models {
			if strings.HasPrefix(model, name) && cfg.ContextWindow > 0 {
				return cfg.ContextWindow
			}
		}
	}

	// 3. Fallback to default
	return 8192
}

// fetchContextWindowFromAPI tries to get context window from provider API
func fetchContextWindowFromAPI(providerType ProviderType, model, baseURL, apiKey string) int {
	if apiKey == "" || baseURL == "" {
		return 0
	}

	var url string
	switch providerType {
	case ProviderOpenAI, ProviderCustom:
		url = baseURL + "/v1/models/" + model
	case ProviderAnthropic:
		return getAnthropicContextWindow(model)
	case ProviderGoogle:
		return getGoogleContextWindow(model)
	case ProviderMiniMax:
		return getMiniMaxContextWindow(model)
	case ProviderOllama:
		url = baseURL + "/api/tags"
	case ProviderOpenRouter:
		return getOpenRouterContextWindow(model)
	case ProviderMoonshot:
		return getMoonshotContextWindow(model)
	case ProviderGLM:
		return getGLMContextWindow(model)
	case ProviderQianfan:
		return getQianfanContextWindow(model)
	case ProviderBedrock:
		return getBedrockContextWindow(model)
	case ProviderVercel:
		return getVercelContextWindow(model)
	case ProviderZAi:
		return getZAiContextWindow(model)
	}

	if url == "" {
		return 0
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}

	return parseContextWindowFromResponse(providerType, result, model)
}

func parseContextWindowFromResponse(providerType ProviderType, result map[string]interface{}, model string) int {
	switch providerType {
	case ProviderOpenAI, ProviderCustom:
		if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
			if m, ok := data[0].(map[string]interface{}); ok {
				if ctx, ok := m["context_window"].(float64); ok {
					return int(ctx)
				}
				if ctx, ok := m["max_tokens"].(float64); ok {
					return int(ctx)
				}
			}
		}
	case ProviderMiniMax:
		if info, ok := result["model_info"].(map[string]interface{}); ok {
			if modelInfo, ok := info[model].(map[string]interface{}); ok {
				if ctx, ok := modelInfo["context_window"].(float64); ok {
					return int(ctx)
				}
			}
		}
	case ProviderOllama:
		if models, ok := result["models"].([]interface{}); ok {
			for _, m := range models {
				if mm, ok := m.(map[string]interface{}); ok {
					name, _ := mm["name"].(string)
					if strings.Contains(name, strings.Split(model, ":")[0]) {
						if details, ok := mm["details"].(map[string]interface{}); ok {
							if ctx, ok := details["context_length"].(float64); ok {
								return int(ctx)
							}
						}
					}
				}
			}
		}
	}
	return 0
}

// Known context windows for Anthropic models
func getAnthropicContextWindow(model string) int {
	windows := map[string]int{
		"claude-3-5-sonnet-20241022": 200000,
		"claude-3-5-sonnet":          200000,
		"claude-3-opus":              200000,
		"claude-3-sonnet":            200000,
		"claude-3-haiku":             200000,
		"claude-2.1":                200000,
		"claude-2":                  100000,
		"claude-instant":             100000,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for MiniMax models
func getMiniMaxContextWindow(model string) int {
	windows := map[string]int{
		"MiniMax-M2.5":    200000,
		"MiniMax-M2":      200000,
		"MiniMax-M1.2":    128000,
		"MiniMax-Text-01": 32000,
		"abab6.5s-chat":   245760,
		"abab6.5g-chat":   245760,
		"abab6-chat":      32768,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Google models
func getGoogleContextWindow(model string) int {
	windows := map[string]int{
		"gemini-2.0-flash":        1000000,
		"gemini-1.5-pro":          200000,
		"gemini-1.5-flash":        1000000,
		"gemini-1.5-flash-8b":     1000000,
		"gemini-pro":              32000,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for OpenRouter models
func getOpenRouterContextWindow(model string) int {
	windows := map[string]int{
		"claude-3.5-sonnet": 200000,
		"claude-3-opus":     200000,
		"claude-3-sonnet":   200000,
		"claude-3-haiku":    200000,
		"claude-2.1":        200000,
		"claude-2":          100000,
		"gpt-4o":           128000,
		"gpt-4-turbo":      128000,
		"gpt-4":            8192,
		"gpt-3.5-turbo":    16385,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Moonshot models
func getMoonshotContextWindow(model string) int {
	windows := map[string]int{
		"moonshot-v1-8k":    8192,
		"moonshot-v1-32k":   32768,
		"moonshot-v1-128k":  131072,
		"moonshot-v1-8k-v":  8192,
		"moonshot-v1-32k-v": 32768,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for GLM models
func getGLMContextWindow(model string) int {
	windows := map[string]int{
		"glm-4":          128000,
		"glm-4-flash":    128000,
		"glm-4-plus":     128000,
		"glm-4-long":    1000000,
		"glm-3-turbo":    32000,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Qianfan models
func getQianfanContextWindow(model string) int {
	windows := map[string]int{
		"ernie-speed-8k":    8192,
		"ernie-speed-32k":   32768,
		"ernie-speed-128k": 131072,
		"ernie-4-8k":        8192,
		"ernie-3.5-8k":      8192,
		"ernie-bot-8k":      8192,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Bedrock models
func getBedrockContextWindow(model string) int {
	windows := map[string]int{
		"claude-3-opus":    200000,
		"claude-3-sonnet":  200000,
		"claude-3-haiku":   200000,
		"claude-2.1":       200000,
		"claude-2":         100000,
		"claude-instant":   100000,
		"titan-text":       4096,
		"titan-embed":      8192,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Vercel models
func getVercelContextWindow(model string) int {
	windows := map[string]int{
		"gpt-4o":         128000,
		"gpt-4-turbo":    128000,
		"gpt-4":          8192,
		"gpt-3.5-turbo":  16385,
		"claude-3.5-sonnet": 200000,
		"claude-3-haiku":  200000,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// Known context windows for Z.AI models
func getZAiContextWindow(model string) int {
	windows := map[string]int{
		"default": 32000,
	}
	for k, v := range windows {
		if strings.Contains(model, k) {
			return v
		}
	}
	return 0
}

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	config    Config
	client    *http.Client
}

func NewBaseProvider(cfg Config) *BaseProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60
	}
	return &BaseProvider{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

func (p *BaseProvider) GetConfig() Config   { return p.config }
func (p *BaseProvider) GetClient() *http.Client { return p.client }

func (p *BaseProvider) BuildRequest(endpoint string, body interface{}) (*http.Request, error) {
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
	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// isRetryable checks if an error is retryable
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

func (p *BaseProvider) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
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

// ProviderRegistry manages provider instances
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[ProviderType]Provider
}

var globalRegistry = &ProviderRegistry{
	providers: make(map[ProviderType]Provider),
}

// RegisterProvider registers a provider
func RegisterProvider(p Provider) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.providers[p.Type()] = p
}

// GetProvider returns a provider by type
func GetProvider(t ProviderType) (Provider, error) {
	globalRegistry.mu.RLock()
	p, ok := globalRegistry.providers[t]
	globalRegistry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider %s not registered", t)
	}
	return p, nil
}

// ListProviders returns all registered providers
func ListProviders() []ProviderType {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	types := make([]ProviderType, 0, len(globalRegistry.providers))
	for t := range globalRegistry.providers {
		types = append(types, t)
	}
	return types
}

// LoadConfigFromEnv loads provider config from environment variables
func LoadConfigFromEnv(providerType ProviderType) Config {
	cfg := Config{Type: providerType}
	switch providerType {
	case ProviderOpenAI:
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		cfg.BaseURL = getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
		cfg.Model = getEnvOrDefault("OPENAI_MODEL", "gpt-4o")
	case ProviderAnthropic:
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		cfg.BaseURL = getEnvOrDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1")
		cfg.Model = getEnvOrDefault("ANTHROPIC_MODEL", "claude-sonnet-4-20250514")
	case ProviderGoogle:
		cfg.APIKey = os.Getenv("GOOGLE_API_KEY")
		cfg.BaseURL = getEnvOrDefault("GOOGLE_BASE_URL", "https://generativelanguage.googleapis.com/v1")
		cfg.Model = getEnvOrDefault("GOOGLE_MODEL", "gemini-2.0-flash")
	case ProviderMiniMax:
		cfg.APIKey = os.Getenv("MINIMAX_API_KEY")
		cfg.BaseURL = getEnvOrDefault("MINIMAX_BASE_URL", "https://api.minimax.chat/v1")
		cfg.Model = getEnvOrDefault("MINIMAX_MODEL", "MiniMax-M2.1")
	case ProviderOllama:
		cfg.BaseURL = getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434")
		cfg.Model = getEnvOrDefault("OLLAMA_MODEL", "llama3")
	case ProviderCustom:
		cfg.APIKey = os.Getenv("CUSTOM_API_KEY")
		cfg.BaseURL = os.Getenv("CUSTOM_BASE_URL")
		cfg.Model = os.Getenv("CUSTOM_MODEL")
	case ProviderOpenRouter:
		cfg.APIKey = os.Getenv("OPENROUTER_API_KEY")
		cfg.BaseURL = getEnvOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")
		cfg.Model = getEnvOrDefault("OPENROUTER_MODEL", "anthropic/claude-3.5-sonnet")
	case ProviderBedrock:
		cfg.APIKey = os.Getenv("AWS_ACCESS_KEY_ID")
		cfg.BaseURL = os.Getenv("AWS_REGION")
		cfg.Model = getEnvOrDefault("BEDROCK_MODEL", "anthropic.claude-3-sonnet-20240229-v1:0")
	case ProviderMoonshot:
		cfg.APIKey = os.Getenv("MOONSHOT_API_KEY")
		cfg.BaseURL = getEnvOrDefault("MOONSHOT_BASE_URL", "https://api.moonshot.cn/v1")
		cfg.Model = getEnvOrDefault("MOONSHOT_MODEL", "moonshot-v1-8k")
	case ProviderGLM:
		cfg.APIKey = os.Getenv("ZHIPU_API_KEY")
		cfg.BaseURL = getEnvOrDefault("ZHIPU_BASE_URL", "https://open.bigmodel.cn/api/paas/v4")
		cfg.Model = getEnvOrDefault("ZHIPU_MODEL", "glm-4")
	case ProviderQianfan:
		cfg.APIKey = os.Getenv("QIANFAN_ACCESS_KEY")
		cfg.BaseURL = getEnvOrDefault("QIANFAN_BASE_URL", "https://qianfan.baidubce.com/v2")
		cfg.Model = getEnvOrDefault("QIANFAN_MODEL", "ernie-speed-8k")
	case ProviderVercel:
		cfg.APIKey = os.Getenv("VERCEL_API_TOKEN")
		cfg.BaseURL = getEnvOrDefault("VERCEL_AI_SDK_BASE_URL", "https://api.vercel.ai/v1")
		cfg.Model = getEnvOrDefault("VERCEL_MODEL", "gpt-4o")
	case ProviderZAi:
		cfg.APIKey = os.Getenv("ZAI_API_KEY")
		cfg.BaseURL = getEnvOrDefault("ZAI_BASE_URL", "https://api.ziai.com/v1")
		cfg.Model = getEnvOrDefault("ZAI_MODEL", "default")
	}
	return cfg
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ============ Default Implementations for Optional Capabilities ============

// DefaultCapabilities returns basic capabilities that all providers support
func DefaultCapabilities() []Capability {
	return []Capability{CapabilityEmbeddings}
}

// DefaultEmbeddings returns error (must be overridden)
func DefaultEmbeddings(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	return nil, ErrCapabilityNotSupported
}

// DefaultVision returns error (must be overridden)
func DefaultVision(ctx context.Context, req *VisionRequest) (*VisionResponse, error) {
	return nil, ErrCapabilityNotSupported
}

// DefaultTTS returns error (must be overridden)
func DefaultTTS(ctx context.Context, req *TTSRequest) (*TTSResponse, error) {
	return nil, ErrCapabilityNotSupported
}

// DefaultTranscription returns error (must be overridden)
func DefaultTranscription(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResponse, error) {
	return nil, ErrCapabilityNotSupported
}

// DefaultRealtime returns error (must be overridden)
func DefaultRealtime(ctx context.Context, cfg RealtimeConfig) (RealtimeProvider, error) {
	return nil, ErrCapabilityNotSupported
}
