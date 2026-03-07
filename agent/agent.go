// Agent module - core struct, constructor, configuration, and lifecycle management
// The following logic is split into sub-files:
//   agent_commands.go  - slash/inline command dispatcher
//   agent_chat.go      - Chat/ChatWithSession/chatInternal + realtime
//   agent_stream.go    - ChatStream SSE streaming
//   agent_tools.go     - tool execution and parsing
//   agent_memory.go    - memory recall and flush
//   agent_session.go   - session management, task scheduling, LLM summary
//   agent_compact.go   - context overflow/compaction/token estimation
//   agent_api.go       - callAPI, callAPIWithDepth, simpleResponse

package agent

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlab/cogate/memory"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/pkg/kv"
	"github.com/gliderlab/cogate/pkg/llm"
	"github.com/gliderlab/cogate/pkg/skills"
	"github.com/gliderlab/cogate/storage"
	"github.com/gliderlab/cogate/tools"
)

const CONFIG_SECTION = "llm"

func init() {
	// Register gob types to avoid interface{} serialization issues
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

// Agent provides dependency injection for all agent components
type Agent struct {
	mu          sync.RWMutex
	cfg         config.AgentConfig // Injected configuration
	client      *http.Client       // Injected HTTP client
	store       *storage.Storage
	memoryStore *memory.VectorMemoryStore
	registry    *tools.Registry
	systemTools []llm.Tool
	pulse       *PulseHandler
	compactMu   sync.Mutex // Mutex for compaction (replaces channel)
	kv          *kv.KV     // Fast KV cache (BadgerDB)

	// Rate limiting (protected by rateLimitMu)
	rateLimitMu       sync.Mutex
	lastAnthropicCall time.Time // Track last Anthropic API call for rate limit

	// Tool enhancement features
	toolLoopDetector *ToolLoopDetector // Tool loop detection
	thinkingConfig   ThinkingConfig    // Thinking mode config

	// Injected dependencies (optional)
	timeProvider TimeProvider
	idGenerator  IDGenerator
	logger       Logger

	// Realtime session manager (opt-in, per-session)
	realtimeMu       sync.Mutex
	realtimeSessions map[string]llm.RealtimeProvider
	realtimeLastUsed map[string]time.Time

	// Per-session locks to prevent concurrent requests on same live session
	realtimeSessionMu map[string]*sync.Mutex
}

// TimeProvider interface for dependency injection
type TimeProvider interface {
	Now() time.Time
	Sleep(duration time.Duration)
}

// IDGenerator interface for dependency injection
type IDGenerator interface {
	New() string
}

// Logger interface for dependency injection
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
}

// Default implementations
type defaultTimeProvider struct{}

func (d *defaultTimeProvider) Now() time.Time { return time.Now() }
func (d *defaultTimeProvider) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

type defaultIDGenerator struct {
	seq uint64
}

func (d *defaultIDGenerator) New() string {
	ns := time.Now().UnixNano()
	n := atomic.AddUint64(&d.seq, 1)
	return fmt.Sprintf("%d-%d", ns, n)
}

type defaultLogger struct{}

func (d *defaultLogger) Print(v ...interface{})                 { log.Print(v...) }
func (d *defaultLogger) Printf(format string, v ...interface{}) { log.Printf(format, v...) }

type Message struct {
	Role                 string       `json:"role"`
	Content              string       `json:"content"`
	ToolCalls            []ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID           string       `json:"tool_call_id,omitempty"`
	ToolExecutionResults []ToolResult `json:"tool_results,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ToolResult struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Result any    `json:"result"`
}

// ToolSpecs returns OpenAI-compatible tool specs
func (a *Agent) ToolSpecs() []map[string]any {
	if a.registry == nil {
		return nil
	}
	return a.registry.GetToolSpecs()
}

// refreshToolSpecs updates the cached tool spec list
func (a *Agent) refreshToolSpecs() {
	if a.registry == nil {
		return
	}
	specs := a.registry.GetToolSpecs()

	newTools := make([]llm.Tool, 0, len(specs))
	for _, s := range specs {
		functionObj, ok := s["function"].(map[string]interface{})
		if !ok {
			log.Printf("[WARN] Could not extract function object from spec: %+v", s)
			continue
		}

		name, _ := functionObj["name"].(string)
		desc, _ := functionObj["description"].(string)
		params, _ := functionObj["parameters"].(map[string]interface{})
		newTools = append(newTools, llm.Tool{
			Type: "function",
			Function: &llm.ToolFunction{
				Name:        name,
				Description: desc,
				Parameters:  params,
			},
		})
	}

	if len(newTools) != len(a.systemTools) || !toolsEqual(a.systemTools, newTools) {
		a.systemTools = newTools
		log.Printf("[TOOL] Tools updated: count=%d", len(a.systemTools))
	}
}

// toolsEqual compares two tool slices for equality
func toolsEqual(a, b []llm.Tool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Function.Name != b[i].Function.Name ||
			a[i].Function.Description != b[i].Function.Description {
			return false
		}
		if aj, _ := json.Marshal(a[i].Function.Parameters); string(aj) != "" {
			bj, _ := json.Marshal(b[i].Function.Parameters)
			if string(aj) != string(bj) {
				return false
			}
		}
	}
	return true
}

type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []llm.Tool      `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Config groups agent settings and dependencies (keeps DI concerns inside the agent package)
//
//nolint:revive
//nolint:stylecheck
//lint:ignore ST1003 allow Config name
type Config struct {
	config.AgentConfig
	Storage     *storage.Storage
	MemoryStore *memory.VectorMemoryStore
	Registry    *tools.Registry
	PulseConfig *PulseConfig
}

// New creates a new Agent with the given configuration and optional dependencies
func New(cfg Config) *Agent {
	a := &Agent{
		cfg:               cfg.AgentConfig,
		client:            &http.Client{Timeout: cfg.HTTPTimeout},
		store:             cfg.Storage,
		memoryStore:       cfg.MemoryStore,
		registry:          cfg.Registry,
		timeProvider:      &defaultTimeProvider{},
		idGenerator:       &defaultIDGenerator{},
		logger:            &defaultLogger{},
		realtimeSessions:  make(map[string]llm.RealtimeProvider),
		realtimeLastUsed:  make(map[string]time.Time),
		realtimeSessionMu: make(map[string]*sync.Mutex),
	}

	// Use default registry if none is provided
	if a.registry == nil {
		a.registry = tools.NewDefaultRegistry()
	}

	// Apply defaults - Try dynamic context window from API, fallback to config
	if a.cfg.ContextTokens == 0 {
		providerType := llm.ProviderOpenAI
		if strings.Contains(a.cfg.BaseURL, "anthropic") {
			providerType = llm.ProviderAnthropic
		} else if strings.Contains(a.cfg.BaseURL, "google") {
			providerType = llm.ProviderGoogle
		} else if strings.Contains(a.cfg.BaseURL, "minimax") {
			providerType = llm.ProviderMiniMax
		} else if strings.Contains(a.cfg.BaseURL, "ollama") {
			providerType = llm.ProviderOllama
		}

		ctxWindow := llm.GetContextWindow(
			providerType,
			a.cfg.Model,
			a.cfg.BaseURL,
			a.cfg.APIKey,
			a.cfg.Models,
		)
		a.cfg.ContextTokens = ctxWindow
		log.Printf("[Agent] Context window for %s: %d", a.cfg.Model, a.cfg.ContextTokens)
	}
	if a.cfg.ReserveTokens == 0 {
		a.cfg.ReserveTokens = 1024
	}
	if a.cfg.SoftTokens == 0 {
		a.cfg.SoftTokens = 800
	}
	if a.cfg.CompactionThreshold == 0 {
		a.cfg.CompactionThreshold = 0.7
	}
	if a.cfg.KeepMessages == 0 {
		a.cfg.KeepMessages = 30
	}
	if a.cfg.HTTPTimeout == 0 {
		a.cfg.HTTPTimeout = 120 * time.Second
	}
	if a.cfg.Temperature == 0 {
		a.cfg.Temperature = 0.7
	}
	if a.cfg.MaxTokens == 0 {
		a.cfg.MaxTokens = 1000
	}
	if a.cfg.RecallLimit == 0 {
		a.cfg.RecallLimit = 3
	}
	if a.cfg.RecallMinScore == 0 {
		a.cfg.RecallMinScore = 0.3
	}

	// Load configuration from database
	if cfg.Storage != nil {
		a.loadConfigFromDB()
	}

	// Initialize pulse/heartbeat system
	if cfg.PulseEnabled && cfg.Storage != nil {
		a.pulse = NewPulseHandler(cfg.Storage, cfg.PulseConfig)
		a.pulse.SetLLMCallback(func(input string) (string, error) {
			return a.Chat([]Message{{Role: "user", Content: input}}), nil
		})
		a.pulse.SetEventCallback(func(evt *PulseEvent) {
			if evt == nil || evt.Event == nil {
				return
			}
			if len(evt.Errors) > 0 {
				log.Printf("[Pulse] Event %d completed with errors: %s", evt.Event.ID, strings.Join(evt.Errors, "; "))
				return
			}
			if evt.Response != "" {
				log.Printf("[Pulse] Event %d completed: %s", evt.Event.ID, tools.Truncate(evt.Response, 200))
			}
		})
		a.pulse.SetBroadcastCallback(func(message string, priority int, channel string) error {
			log.Printf("[Pulse] Broadcast requested (priority=%d channel=%s): %s", priority, channel, tools.Truncate(message, 200))
			return nil
		})
		a.pulse.Start()
		log.Printf("[Agent] Pulse/Heartbeat system started")
	}

	// Load OCG skills
	a.loadSkills()

	// Initialize tool enhancement features
	a.toolLoopDetector = NewToolLoopDetector(DefaultToolLoopDetectionConfig)
	if a.cfg.ThinkingMode != "" {
		a.thinkingConfig = ThinkingConfig{
			Mode: ParseThinkingMode(a.cfg.ThinkingMode),
		}
	} else {
		a.thinkingConfig = DefaultThinkingConfig
	}

	log.Printf("[Agent] Tool enhancements initialized: loop_detection=%v, thinking=%s",
		a.toolLoopDetector != nil, a.thinkingConfig.Mode)

	a.startRealtimeJanitor()

	return a
}

// loadSkills loads OCG skills and registers them as tools
func (a *Agent) loadSkills() {
	skillsDir := findSkillsDir()
	if skillsDir == "" {
		log.Printf("[Agent] No skills directory found, skipping skills load")
		return
	}

	log.Printf("[Agent] Loading skills from: %s", skillsDir)

	registry := skills.NewRegistry(skillsDir)
	if err := registry.Load(); err != nil {
		log.Printf("[Agent] Failed to load skills: %v", err)
		return
	}

	adapter := skills.NewAdapter(registry)
	skillTools := adapter.GenerateTools()

	for _, tool := range skillTools {
		a.registry.Register(tool)
		log.Printf("[Agent] Registered skill tool: %s", tool.Name())
	}

	log.Printf("[Agent] Loaded %d skill tools", len(skillTools))
}

// findSkillsDir finds the skills directory
func findSkillsDir() string {
	locations := []string{
		"./skills",
		"skills",
		config.DefaultWorkspaceDir() + "/skills",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// WithHTTPClient injects a custom HTTP client
func (a *Agent) WithHTTPClient(client *http.Client) *Agent {
	a.client = client
	return a
}

// WithTimeProvider injects a custom time provider
func (a *Agent) WithTimeProvider(tp TimeProvider) *Agent {
	a.timeProvider = tp
	return a
}

// WithIDGenerator injects a custom ID generator
func (a *Agent) WithIDGenerator(gen IDGenerator) *Agent {
	a.idGenerator = gen
	return a
}

// WithLogger injects a custom logger
func (a *Agent) WithLogger(logger Logger) *Agent {
	a.logger = logger
	return a
}

func (a *Agent) Store() *storage.Storage {
	return a.store
}

func (a *Agent) Registry_() *tools.Registry {
	return a.registry
}

func (a *Agent) MemoryStore() *memory.VectorMemoryStore {
	return a.memoryStore
}

// Pulse returns the pulse handler if available
func (a *Agent) Pulse() *PulseHandler {
	return a.pulse
}

// Stop gracefully stops the agent and all background goroutines
func (a *Agent) Stop() {
	if a.pulse != nil {
		a.pulse.Stop()
		log.Printf("[Agent] Pulse/Heartbeat system stopped")
	}
	if a.kv != nil {
		a.kv.Close()
		log.Printf("[Agent] KV store closed")
	}
}

// SetKV sets the KV store for fast caching
func (a *Agent) SetKV(k *kv.KV) {
	a.kv = k
}

// GetKV returns the KV store
func (a *Agent) GetKV() *kv.KV {
	return a.kv
}

// AddPulseEvent adds a new event to the pulse system
func (a *Agent) AddPulseEvent(title, content string, priority int, channel string) (int64, error) {
	if a.pulse == nil {
		return 0, fmt.Errorf("pulse system not enabled")
	}
	return a.pulse.AddEvent(title, content, priority, channel)
}

// loadBootMD loads BOOT.md content from cache and returns it as a system message
func (a *Agent) loadBootMD() *Message {
	locations := []string{
		os.Getenv("OCG_WORKSPACE"),
		filepath.Join(os.Getenv("HOME"), ".openclaw", "workspace"),
		config.DefaultWorkspaceDir(),
	}

	for _, workspaceDir := range locations {
		if workspaceDir == "" {
			continue
		}
		bootCachePath := filepath.Join(workspaceDir, ".ocg", "data", "boot.md.cache")
		content, err := os.ReadFile(bootCachePath)
		if err == nil && len(content) > 0 {
			log.Printf("[BOOT] Loaded BOOT.md from cache (%d bytes)", len(content))
			return &Message{Role: "system", Content: string(content)}
		}
	}
	return nil
}

// GetPulseStatus returns the current status of the pulse system
func (a *Agent) GetPulseStatus() (map[string]interface{}, error) {
	if a.pulse == nil {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}
	return a.pulse.GetStatus(), nil
}

// loadConfigFromDB loads configuration from database
func (a *Agent) loadConfigFromDB() {
	if a.store == nil {
		return
	}

	exists, err := a.store.ConfigExists(CONFIG_SECTION)
	if err != nil {
		log.Printf("[WARN] failed to check config: %v", err)
		return
	}

	if !exists {
		log.Printf("[NOTE] first start, no existing config in database")
		return
	}

	log.Printf("[LOAD] loading config from database...")
	cfg, err := a.store.GetConfigSection(CONFIG_SECTION)
	if err != nil {
		log.Printf("[WARN] failed to load config from DB: %v", err)
		return
	}

	if v, ok := cfg["apiKey"]; ok && v != "" {
		a.cfg.APIKey = v
	}
	if v, ok := cfg["baseUrl"]; ok && v != "" {
		a.cfg.BaseURL = v
	}
	if v, ok := cfg["model"]; ok && v != "" {
		a.cfg.Model = v
	}

	log.Printf("[OK] config loaded from database")
}

func (a *Agent) saveConfigToDB(apiKey, baseURL, model string) {
	if a.store == nil {
		return
	}

	if apiKey != "" {
		if err := a.store.SetConfig(CONFIG_SECTION, "apiKey", apiKey); err != nil {
			log.Printf("[WARN] failed to save apiKey to DB: %v", err)
		}
	}
	if baseURL != "" {
		if err := a.store.SetConfig(CONFIG_SECTION, "baseUrl", baseURL); err != nil {
			log.Printf("[WARN] failed to save baseUrl to DB: %v", err)
		}
	}
	if model != "" {
		if err := a.store.SetConfig(CONFIG_SECTION, "model", model); err != nil {
			log.Printf("[WARN] failed to save model to DB: %v", err)
		}
	}
}

func (a *Agent) UpdateConfig(apiKey, baseURL, model string) {
	a.mu.Lock()
	a.cfg.APIKey = apiKey
	a.cfg.BaseURL = baseURL
	a.cfg.Model = model
	a.mu.Unlock()

	if a.store != nil {
		go a.saveConfigToDB(apiKey, baseURL, model)
	}
}

func (a *Agent) GetConfig() (apiKey, baseURL, model string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.APIKey, a.cfg.BaseURL, a.cfg.Model
}

func (a *Agent) storeMessage(sessionKey, role, content string) {
	if a.store == nil {
		return
	}
	if sessionKey == "" {
		return
	}
	if err := a.store.AddMessage(sessionKey, role, content); err != nil {
		log.Printf("[WARN] store message failed: %v", err)
	}
}
