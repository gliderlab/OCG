// Agent module - external LLM API, SQLite storage, config persistence, and tool calls

package agent

import (
	"bufio"
	"context"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlab/cogate/memory"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/pkg/kv"
	"github.com/gliderlab/cogate/pkg/llm"
	googleprovider "github.com/gliderlab/cogate/pkg/llm/providers/google"
	"github.com/gliderlab/cogate/pkg/skills"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
	"github.com/gliderlab/cogate/tools"
	"github.com/pkoukk/tiktoken-go"
)

const CONFIG_SECTION = "llm"

func init() {
	// Register gob types to avoid interface{} serialization issues
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

// Agent provides dependency injection for all agent components
type Agent struct {
	mu            sync.RWMutex
	cfg           config.AgentConfig // Injected configuration
	client        *http.Client       // Injected HTTP client
	store         *storage.Storage
	memoryStore   *memory.VectorMemoryStore
	registry      *tools.Registry
	systemTools   []rpcproto.Tool
	pulse         *PulseHandler
	compactMu     sync.Mutex // Mutex for compaction (replaces channel)
	kv            *kv.KV     // Fast KV cache (BadgerDB)

	// Rate limiting
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

// Tool spec for OpenAI-compatible schema
func (a *Agent) ToolSpecs() []map[string]any {
	if a.registry == nil {
		return nil
	}
	return a.registry.GetToolSpecs()
}

// update tool specs cache
func (a *Agent) refreshToolSpecs() {
	if a.registry == nil {
		return
	}
	specs := a.registry.GetToolSpecs()

	// Check if tools have actually changed before updating
	newTools := make([]rpcproto.Tool, 0, len(specs))
	for _, s := range specs {
		// Get the function object
		functionObj, ok := s["function"].(map[string]interface{})
		if !ok {
			log.Printf("[WARN] Could not extract function object from spec: %+v", s)
			continue
		}

		name, _ := functionObj["name"].(string)
		desc, _ := functionObj["description"].(string)
		params, _ := functionObj["parameters"].(map[string]interface{})
		paramsJSON, _ := json.Marshal(params)
		newTools = append(newTools, rpcproto.Tool{
			Type: "function",
			Function: &rpcproto.ToolFunction{
				Name:        name,
				Description: desc,
				Parameters:  string(paramsJSON),
			},
		})
	}

	// Only update and log if tools have changed
	if len(newTools) != len(a.systemTools) || !toolsEqual(a.systemTools, newTools) {
		a.systemTools = newTools
		log.Printf("[TOOL] Tools updated: count=%d", len(a.systemTools))
	}
}

// toolsEqual compares two tool slices for equality
func toolsEqual(a, b []rpcproto.Tool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Function.Name != b[i].Function.Name {
			return false
		}
	}
	return true
}

type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []rpcproto.Tool `json:"tools,omitempty"`
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

// Config wraps config.AgentConfig with injected dependencies
// Keeps DI concerns inside the agent package to avoid import cycles
// golangci-lint:ignore
// Config groups agent settings and dependencies
// (This is intentionally verbose to keep dependencies injectable.)
//
//nolint:revive // config naming matches package
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
		cfg:              cfg.AgentConfig,
		client:           &http.Client{Timeout: cfg.HTTPTimeout},
		store:            cfg.Storage,
		memoryStore:      cfg.MemoryStore,
		registry:         cfg.Registry,
		timeProvider:     &defaultTimeProvider{},
		idGenerator:      &defaultIDGenerator{},
		logger:           &defaultLogger{},
		realtimeSessions:   make(map[string]llm.RealtimeProvider),
		realtimeLastUsed:   make(map[string]time.Time),
		realtimeSessionMu:  make(map[string]*sync.Mutex),
	}

	// Use default registry if none is provided
	if a.registry == nil {
		a.registry = tools.NewDefaultRegistry()
	}

	// Apply defaults - Try dynamic context window from API, fallback to config
	if a.cfg.ContextTokens == 0 {
		// Try to get from provider API first, then fallback to config
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

	// Limit compaction concurrency (using mutex instead of channel for safety)
	// compactLocked removed

	// Load OCG skills
	a.loadSkills()

	// Initialize tool enhancement features
	a.toolLoopDetector = NewToolLoopDetector(DefaultToolLoopDetectionConfig)
	// Use config thinking mode if set, otherwise default to off
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
	// Find skills directory
	skillsDir := findSkillsDir()
	if skillsDir == "" {
		log.Printf("[Agent] No skills directory found, skipping skills load")
		return
	}

	log.Printf("[Agent] Loading skills from: %s", skillsDir)

	// Create registry and load skills
	registry := skills.NewRegistry(skillsDir)
	if err := registry.Load(); err != nil {
		log.Printf("[Agent] Failed to load skills: %v", err)
		return
	}

	// Create adapter and generate tools
	adapter := skills.NewAdapter(registry)
	skillTools := adapter.GenerateTools()

	// Register tools with agent registry
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
		"/usr/lib/node_modules/openclaw/skills",
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
// priority: 0=critical, 1=high, 2=normal, 3=low
func (a *Agent) AddPulseEvent(title, content string, priority int, channel string) (int64, error) {
	if a.pulse == nil {
		return 0, fmt.Errorf("pulse system not enabled")
	}
	return a.pulse.AddEvent(title, content, priority, channel)
}

// loadBootMD loads BOOT.md content from cache and returns it as a system message
// Returns nil if no BOOT.md is found
func (a *Agent) loadBootMD() *Message {
	// Check common workspace locations
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

// Load configuration from database
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
		// No config to load - will use defaults or env vars
		return
	}

	log.Printf("[LOAD] loading config from database...")
	config, _ := a.store.GetConfigSection(CONFIG_SECTION)

	if v, ok := config["apiKey"]; ok && v != "" {
		a.cfg.APIKey = v
	}
	if v, ok := config["baseUrl"]; ok && v != "" {
		a.cfg.BaseURL = v
	}
	if v, ok := config["model"]; ok && v != "" {
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
	defer a.mu.Unlock()
	a.cfg.APIKey = apiKey
	a.cfg.BaseURL = baseURL
	a.cfg.Model = model
	if a.store != nil {
		// Fix: Pass values to avoid race - saveConfigToDB reads unlocked fields
		a.saveConfigToDB(apiKey, baseURL, model)
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
		// Avoid polluting the default session when the caller doesn't know the session key.
		return
	}
	if err := a.store.AddMessage(sessionKey, role, content); err != nil {
		log.Printf("[WARN] store message failed: %v", err)
	}
}

// runCommandIfRequested executes explicit user command requests via process tool.
func (a *Agent) runCommandIfRequested(msg string) (string, bool) {
	msg = strings.TrimSpace(msg)
	if msg == "" || a.registry == nil {
		return "", false
	}

	// Handle /compact command
	if strings.HasPrefix(msg, "/compact") || strings.HasPrefix(msg, "/compact ") {
		instructions := strings.TrimPrefix(msg, "/compact")
		instructions = strings.TrimSpace(instructions)
		return a.runCompact(instructions), true
	}

	// Handle /new command (start new session)
	if msg == "/new" || msg == "/new " {
		return a.runNewSession(), true
	}

	// Handle /reset command (reset current session)
	if msg == "/reset" || msg == "/reset " {
		return a.runResetSession(), true
	}

	// Handle /split command (explicit task splitting)
	if strings.HasPrefix(msg, "/split ") || msg == "/split" {
		taskMsg := strings.TrimPrefix(msg, "/split ")
		if taskMsg == "/split" || taskMsg == "" {
			return "Usage: /split <task>\nExample: /split summarize today's meeting notes", true
		}
		// Execute task split explicitly
		return a.executeSplitTask(taskMsg), true
	}

	// Resolve task marker(s) quickly: [task_done:task-...]
	if strings.Contains(msg, "[task_done:") {
		re := regexp.MustCompile(`\[task_done:(task-[^\]]+)\]`)
		all := re.FindAllStringSubmatch(msg, -1)
		if len(all) > 0 {
			seen := map[string]bool{}
			parts := make([]string, 0, len(all))
			for _, m := range all {
				if len(m) == 2 {
					id := m[1]
					if seen[id] {
						continue
					}
					seen[id] = true
					parts = append(parts, a.runTaskSummary(id))
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n---\n\n"), true
			}
		}
	}

	// Handle /debug archive
	if strings.HasPrefix(msg, "/debug archive") {
		parts := strings.Fields(msg)
		session := "default"
		if len(parts) >= 3 {
			session = parts[2]
		}
		return a.runArchiveDebug(session), true
	}

	// Handle /debug live
	if strings.HasPrefix(msg, "/debug live") {
		parts := strings.Fields(msg)
		session := ""
		if len(parts) >= 3 {
			session = parts[2]
		}
		return a.runLiveDebug(session), true
	}

	// Handle /task commands
	if strings.HasPrefix(msg, "/task") {
		parts := strings.Fields(msg)
		if len(parts) == 1 {
			return "Usage:\n/task list [limit]\n/task detail <task-id> [page] [pageSize]\n/task summary <task-id>", true
		}
		sub := strings.ToLower(parts[1])
		switch sub {
		case "list":
			limit := 10
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(parts[2]); err == nil && n > 0 && n <= 100 {
					limit = n
				}
			}
			return a.runTaskList("default", limit), true
		case "detail":
			if len(parts) < 3 {
				return "Usage: /task detail <task-id> [page] [pageSize]", true
			}
			page := 1
			pageSize := 20
			if len(parts) >= 4 {
				if n, err := strconv.Atoi(parts[3]); err == nil && n > 0 {
					page = n
				}
			}
			if len(parts) >= 5 {
				if n, err := strconv.Atoi(parts[4]); err == nil && n > 0 && n <= 200 {
					pageSize = n
				}
			}
			return a.runTaskDetail(parts[2], page, pageSize), true
		case "summary":
			if len(parts) < 3 {
				return "Usage: /task summary <task-id>", true
			}
			return a.runTaskSummary(parts[2]), true
		default:
			return "Unknown /task subcommand. Use: list | detail | summary", true
		}
	}

	// Match explicit run/execute patterns
	reCmd := regexp.MustCompile(`^(run|exec)\s+(.+)$`)
	cmd := ""
	if m := reCmd.FindStringSubmatch(msg); m != nil {
		cmd = strings.TrimSpace(m[2])
	}
	if strings.Contains(msg, "uname -r") {
		cmd = "uname -r"
	}
	if cmd == "" {
		return "", false
	}

	// Block dangerous commands
	danger := []string{"rm ", "rm -", "shutdown", "reboot", "mkfs", "dd ", "sudo ", "kill ", ":(){"}
	for _, d := range danger {
		if strings.Contains(cmd, d) {
			return "This command may be dangerous. Please confirm before executing.", true
		}
	}

	res, err := a.registry.CallTool("process", map[string]interface{}{"action": "start", "command": cmd})
	if err != nil {
		return fmt.Sprintf("Command execution failed: %v", err), true
	}
	// Extract sessionId
	sessionID := ""
	switch v := res.(type) {
	case tools.ProcessStartResult:
		sessionID = v.SessionID
	case *tools.ProcessStartResult:
		sessionID = v.SessionID
	case map[string]interface{}:
		if id, ok := v["sessionId"].(string); ok {
			sessionID = id
		}
	}
	if sessionID == "" {
		return "Command started but no sessionId returned.", true
	}

	// Poll command output with context + backoff
	content := a.pollCommandOutput(sessionID, 5*time.Second) // 5s deadline
	if content != "" {
		return content, true
	}
	return "Command completed but no output.", true
}

// pollCommandOutput polls command logs with exponential backoff and deadline
func (a *Agent) pollCommandOutput(sessionID string, deadline time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	getContent := func(res interface{}) string {
		switch v := res.(type) {
		case tools.ProcessLogResult:
			return strings.TrimSpace(v.Content)
		case *tools.ProcessLogResult:
			return strings.TrimSpace(v.Content)
		case map[string]interface{}:
			if c, ok := v["content"].(string); ok {
				return strings.TrimSpace(c)
			}
		}
		return ""
	}

	// Initial delay
	delay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return "" // Timeout
		default:
		}

		logRes, err := a.registry.CallTool("process", map[string]interface{}{"action": "log", "sessionId": sessionID})
		if err == nil {
			if content := getContent(logRes); content != "" {
				return content
			}
		}

		// Exponential backoff
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ""
			case <-time.After(delay):
			}
			delay = delay * 3 / 2 // 1.5x backoff
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return ""
}

func realtimeDirective(last string) string {
	l := strings.ToLower(strings.TrimSpace(last))
	switch {
	case strings.HasPrefix(l, "/text "), strings.HasPrefix(l, "/http "):
		return "force_http"
	case strings.HasPrefix(l, "/live-audio-file "), strings.HasPrefix(l, "/live "), strings.HasPrefix(l, "/voice "), strings.HasPrefix(l, "/audio "):
		return "force_live"
	default:
		return "auto"
	}
}

func looksLikeAudioInput(last string) bool {
	l := strings.ToLower(strings.TrimSpace(last))
	return strings.HasPrefix(l, "data:audio/") ||
		strings.HasPrefix(l, "[audio]") ||
		strings.Contains(l, "mime:audio/") ||
		strings.Contains(l, `\"type\":\"audio\"`) ||
		strings.Contains(l, "voice message")
}

func (a *Agent) shouldUseRealtime(sessionKey string, messages []Message) bool {
	if sessionKey == "" || len(messages) == 0 {
		return false
	}

	last := strings.TrimSpace(messages[len(messages)-1].Content)
	switch realtimeDirective(last) {
	case "force_http":
		return false
	case "force_live":
		return true
	}

	if looksLikeAudioInput(last) {
		return true
	}

	if strings.HasPrefix(sessionKey, "live:") || strings.HasPrefix(sessionKey, "realtime:") {
		return true
	}
	if a.store != nil {
		if meta, err := a.store.GetSessionMeta(sessionKey); err == nil && meta.ProviderType == "live" {
			return true
		}
	}
	return false
}

func (a *Agent) realtimeModel() string {
	m := strings.TrimSpace(a.cfg.Model)
	if strings.Contains(strings.ToLower(m), "gemini") {
		return m
	}
	return "models/gemini-2.5-flash-native-audio-preview-12-2025"
}

func (a *Agent) getRealtimeProvider(sessionKey string) (llm.RealtimeProvider, error) {
	if p, ok := a.getCachedRealtime(sessionKey); ok {
		return p, nil
	}

	// For Google Realtime, check environment variables first (consistent with regular LLM)
	// Priority: GEMINI_API_KEY -> GOOGLE_API_KEY -> config API key
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("google live API key not configured (set GEMINI_API_KEY or GOOGLE_API_KEY env var)")
	}
	shortKey := apiKey
	if len(apiKey) > 15 {
		shortKey = apiKey[:15]
	}
	log.Printf("[realtime] Using API key: %s...", shortKey)

	cfg := llm.RealtimeConfig{
		Model:                  a.realtimeModel(),
		APIKey:                 apiKey,
		Voice:                  "Kore",
		InputAudioTranscription:  true,
		OutputAudioTranscription: true,
	}
	provider := googleprovider.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: cfg.Model})
	rt, err := provider.Realtime(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if err := rt.Connect(context.Background(), cfg); err != nil {
		return nil, err
	}

	a.cacheRealtime(sessionKey, rt)
	return rt, nil
}

func (a *Agent) chatWithRealtimeSession(sessionKey string, messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	unlock := a.getRealtimeSessionLock(sessionKey)
	defer unlock()

	lastMsg := strings.TrimSpace(messages[len(messages)-1].Content)
	audioFile := ""
	lowerLast := strings.ToLower(lastMsg)
	if strings.HasPrefix(lowerLast, "/live-audio-file ") {
		audioFile = strings.TrimSpace(lastMsg[len("/live-audio-file "):])
		lastMsg = ""
	} else {
		for _, pfx := range []string{"/live ", "/voice ", "/audio ", "[audio]"} {
			if strings.HasPrefix(strings.ToLower(lastMsg), pfx) {
				lastMsg = strings.TrimSpace(lastMsg[len(pfx):])
				break
			}
		}
	}
	if lastMsg == "" && audioFile == "" {
		return "(live) empty input"
	}

	rt, err := a.getRealtimeProvider(sessionKey)
	if err != nil {
		log.Printf("[realtime] init failed: %v, falling back to http", err)
		return a.fallbackToHTTP(sessionKey, messages, err.Error())
	}

	if a.store != nil {
		storedUser := lastMsg
		if audioFile != "" && storedUser == "" {
			storedUser = "[voice_input]"
		}
		a.storeMessage(sessionKey, "user", storedUser)
		_ = a.store.SetSessionProviderType(sessionKey, "live")
		_ = a.store.TouchRealtimeSession(sessionKey)
	}

	var (
		mu         sync.Mutex
		textBuf    strings.Builder
		lastUpdate time.Time
		errReason  string
	)
	rt.OnText(func(text string) {
		if text == "" {
			return
		}
		mu.Lock()
		textBuf.WriteString(text)
		lastUpdate = time.Now()
		mu.Unlock()
	})
	rt.OnError(func(err error) {
		mu.Lock()
		if errReason == "" {
			errReason = err.Error()
		}
		mu.Unlock()
	})

	if audioFile != "" {
		pcmData, err := os.ReadFile(audioFile)
		_ = os.Remove(audioFile)
		if err != nil {
			log.Printf("[realtime] read audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
		if err := rt.SendAudio(context.Background(), pcmData); err != nil {
			log.Printf("[realtime] send audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
		if err := rt.EndAudio(context.Background()); err != nil {
			log.Printf("[realtime] end audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
	}
	if lastMsg != "" {
		if err := rt.SendText(context.Background(), lastMsg); err != nil {
			log.Printf("[realtime] send text failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
	}
	a.touchRealtimeInMemory(sessionKey)

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(120 * time.Millisecond)
		mu.Lock()
		content := strings.TrimSpace(textBuf.String())
		lu := lastUpdate
		errMsg := errReason
		mu.Unlock()
		if errMsg != "" {
			log.Printf("[realtime] error during session: %s, falling back to http", errMsg)
			return a.fallbackToHTTP(sessionKey, messages, errMsg)
		}
		if content != "" && !lu.IsZero() && time.Since(lu) > 800*time.Millisecond {
			if a.store != nil {
				a.storeMessage(sessionKey, "assistant", content)
				_ = a.store.TouchRealtimeSession(sessionKey)
			}
			a.touchRealtimeInMemory(sessionKey)
			return content
		}
	}

	mu.Lock()
	content := strings.TrimSpace(textBuf.String())
	errMsg := errReason
	mu.Unlock()
	if content == "" {
		if errMsg != "" {
			log.Printf("[realtime] session error: %s, falling back to http", errMsg)
			return a.fallbackToHTTP(sessionKey, messages, errMsg)
		}
		content = "(live) no response received"
	}
	if a.store != nil {
		a.storeMessage(sessionKey, "assistant", content)
		_ = a.store.TouchRealtimeSession(sessionKey)
	}
	a.touchRealtimeInMemory(sessionKey)
	return content
}

// SendAudioChunk sends audio data to an active live session (for streaming voice input)
func (a *Agent) SendAudioChunk(sessionKey string, pcmData []byte) error {
	a.realtimeMu.Lock()
	rt, ok := a.realtimeSessions[sessionKey]
	a.realtimeMu.Unlock()
	if !ok || rt == nil || !rt.IsConnected() {
		return fmt.Errorf("no active live session for %s", sessionKey)
	}
	if len(pcmData) == 0 {
		return nil
	}
	return rt.SendAudio(context.Background(), pcmData)
}

// EndAudioStream signals end of audio stream for a live session
func (a *Agent) EndAudioStream(sessionKey string) error {
	a.realtimeMu.Lock()
	rt, ok := a.realtimeSessions[sessionKey]
	a.realtimeMu.Unlock()
	if !ok || rt == nil || !rt.IsConnected() {
		return fmt.Errorf("no active live session for %s", sessionKey)
	}
	return rt.EndAudio(context.Background())
}

func (a *Agent) fallbackToHTTP(sessionKey string, messages []Message, reason string) string {
	log.Printf("[realtime->http] falling back: %s", reason)
	cleaned := make([]Message, len(messages))
	copy(cleaned, messages)
	for i := range cleaned {
		cleaned[i].Content = strings.TrimSpace(cleaned[i].Content)
		for _, pfx := range []string{"/live ", "/voice ", "/audio ", "/live-audio-file ", "[audio]"} {
			if strings.HasPrefix(strings.ToLower(cleaned[i].Content), pfx) {
				cleaned[i].Content = strings.TrimSpace(cleaned[i].Content[len(pfx):])
				break
			}
		}
	}
	if len(cleaned) > 0 {
		cleaned[len(cleaned)-1].Content = "[realtime-fallback] " + cleaned[len(cleaned)-1].Content
	}
	return a.ChatWithSession(sessionKey, cleaned)
}

func (a *Agent) ChatWithSession(sessionKey string, messages []Message) string {
	if len(messages) > 0 {
		idx := len(messages) - 1
		last := strings.TrimSpace(messages[idx].Content)
		if strings.HasPrefix(strings.ToLower(last), "/text ") {
			messages[idx].Content = strings.TrimSpace(last[len("/text "):])
		} else if strings.HasPrefix(strings.ToLower(last), "/http ") {
			messages[idx].Content = strings.TrimSpace(last[len("/http "):])
		}
	}

	if a.shouldUseRealtime(sessionKey, messages) {
		return a.chatWithRealtimeSession(sessionKey, messages)
	}

	isNewSession := false

	// Load history for this session if store is available
	if a.store != nil && sessionKey != "" {
		history, err := a.store.GetMessages(sessionKey, 100) // Load last 100 messages
		if err == nil && len(history) > 0 {
			// Prepend history to current messages
			histMsgs := make([]Message, len(history))
			for i, m := range history {
				histMsgs[i] = Message{
					Role:    m.Role,
					Content: m.Content,
				}
			}
			// Combine: history + new messages
			messages = append(histMsgs, messages...)
			log.Printf("[HISTORY] Loaded %d historical messages for session %s", len(history), sessionKey)
		} else {
			// No history = new session
			isNewSession = true
		}
	}

	// Inject BOOT.md for new sessions
	if isNewSession {
		if bootMsg := a.loadBootMD(); bootMsg != nil {
			// Prepend BOOT.md as system message
			messages = append([]Message{*bootMsg}, messages...)
			log.Printf("[BOOT] BOOT.md injected as system message")
		}
	}

	// Delegate to Chat with the combined messages
	return a.Chat(messages)
}

func (a *Agent) Chat(messages []Message) string {
	lastMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastMsg = messages[i].Content
			break
		}
	}
	finalize := func(reply string) string {
		// Strip thinking tags if thinking mode is not enabled (default: off)
		if a.thinkingConfig.Mode == "" || a.thinkingConfig.Mode == ThinkingModeOff {
			reply = StripThinkingTags(reply)
		}
		if a.store != nil && lastMsg != "" {
			a.storeMessage("default", "user", lastMsg)
			a.storeMessage("default", "assistant", reply)
		}
		return reply
	}

	// Check if we should use realtime/live mode (handles /live, /voice, /audio commands)
	if a.shouldUseRealtime("default", messages) {
		return a.chatWithRealtimeSession("default", messages)
	}

	if out, ok := a.runCommandIfRequested(lastMsg); ok {
		return finalize(out)
	}

	// Note: Task splitting is now explicit via /split command
	// Automatic triggering removed - use /split <task> instead

	if a.store != nil {
		if lastMsg != "" {
			if a.memoryStore != nil && tools.ShouldCapture(lastMsg) {
				category := tools.DetectCategory(lastMsg)
				results, _ := a.memoryStore.Search(lastMsg, 1, 0.95)
				if len(results) == 0 {
					_, err := a.memoryStore.StoreWithSource(lastMsg, category, 0.6, "auto")
					if err != nil {
						log.Printf("[WARN] auto memory write failed")
					}
				}
			}
			// Soft-trigger memory flush (based on message count + time)
			a.maybeFlushMemory(lastMsg)
			// compaction check (async with timeout to prevent blocking)
			go func() {
				// Try to acquire lock (non-blocking)
				if !a.compactMu.TryLock() {
					log.Printf("[WARN] maybeCompact skipped: another compaction in progress")
					return
				}
				// Lock acquired - ensure we unlock on exit
				locked := true
				defer func() {
					if locked {
						if r := recover(); r != nil {
							log.Printf("[WARN] maybeCompact recovered from panic: %v", r)
						}
						a.compactMu.Unlock()
					}
				}()

				done := make(chan struct{}, 1) // buffered to prevent goroutine leak
				// Run maybeCompact directly (already in goroutine, no need for nested one)
				go func() {
					defer func() { done <- struct{}{} }()
					a.maybeCompact("default", messages, nil)
				}()
				select {
				case <-done:
					locked = false // compaction completed, unlock immediately
					return
				case <-time.After(30 * time.Second):
					log.Printf("[WARN] maybeCompact timed out, continuing in background")
					// Don't wait for it - let it run in background, unlock after timeout
					locked = false
					a.compactMu.Unlock()
					return
				}
			}()
		}
	}

	// Handle tool calls
	if len(messages) > 0 && len(messages[len(messages)-1].ToolCalls) > 0 {
		return finalize(a.handleToolCalls(messages, messages[len(messages)-1].ToolCalls, nil, 0, nil))
	}

	// Detect edit intent
	if len(messages) > 0 {
		lastUserMsg := messages[len(messages)-1].Content
		if editArgs := detectEditIntent(lastUserMsg); editArgs != nil {
			return finalize(a.handleEdit(editArgs))
		}
	}

	// Explicit recall trigger: user can request recall via keywords
	if len(messages) > 0 && a.memoryStore != nil {
		lastUserMsg := messages[len(messages)-1].Content
		if isRecallRequest(lastUserMsg) {
			if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
				log.Printf("recall command injected %d memories", strings.Count(memories, "- ["))
				injected := Message{Role: "system", Content: memories}
				messages = append([]Message{injected}, messages...)
			}
		}
	}

	// Auto recall: inject relevant memories as a system message before sending to model
	if a.cfg.AutoRecall && a.memoryStore != nil && len(messages) > 0 {
		lastUserMsg := messages[len(messages)-1].Content
		if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
			log.Printf("auto-recall injected %d memories", strings.Count(memories, "- ["))
			injected := Message{Role: "system", Content: memories}
			messages = append([]Message{injected}, messages...)
		}
	}

	// overflow handling: estimate Proactive context tokens and apply pruning/compaction
	// Official OCG behavior: check if current + new > context_window, then prune/compact
	if a.store != nil {
		messages = a.handleContextOverflow("default", messages)
	}

	if a.cfg.APIKey == "" {
		return finalize(a.simpleResponse(messages))
	}

	return finalize(a.callAPI(messages))
}

// ChatStream sends chat messages and streams the response via callback
func (a *Agent) ChatStream(messages []Message, callback func(string)) {
	lastMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastMsg = messages[i].Content
			break
		}
	}

	if out, ok := a.runCommandIfRequested(lastMsg); ok {
		callback(out)
		if a.store != nil && lastMsg != "" {
			a.storeMessage("default", "user", lastMsg)
			a.storeMessage("default", "assistant", out)
		}
		return
	}

	// Handle /split command (explicit task splitting)
	if strings.HasPrefix(lastMsg, "/split ") || lastMsg == "/split" {
		taskMsg := strings.TrimPrefix(lastMsg, "/split ")
		if taskMsg == "/split" || taskMsg == "" {
			callback("Usage: /split <task>\nExample: /split summarize today's meeting notes")
		} else {
			result := a.executeSplitTask(taskMsg)
			callback(result)
		}
		return
	}

	// Note: Task splitting is now explicit via /split command
	// Automatic triggering removed

	if a.cfg.APIKey == "" {
		// Simple mode - no streaming, just send the full response
		response := a.simpleResponse(messages)
		callback(response)
		if a.store != nil && lastMsg != "" {
			a.storeMessage("default", "user", lastMsg)
			a.storeMessage("default", "assistant", response)
		}
		return
	}

	if a.store != nil && lastMsg != "" {
		if a.memoryStore != nil && tools.ShouldCapture(lastMsg) {
			category := tools.DetectCategory(lastMsg)
			results, _ := a.memoryStore.Search(lastMsg, 1, 0.95)
			if len(results) == 0 {
				_, err := a.memoryStore.StoreWithSource(lastMsg, category, 0.6, "auto")
				if err != nil {
					log.Printf("[WARN] auto memory write failed")
				}
			}
		}
		// Soft-trigger memory flush (based on message count + time)
		a.maybeFlushMemory(lastMsg)
		// compaction check (async with timeout to prevent blocking)
		go func() {
			// Try to acquire lock (non-blocking)
			if !a.compactMu.TryLock() {
				log.Printf("[WARN] maybeCompact skipped: another compaction in progress")
				return
			}
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WARN] maybeCompact recovered from panic: %v", r)
				}
				a.compactMu.Unlock()
			}()

			done := make(chan struct{}, 1)
			go func() {
				defer func() { done <- struct{}{} }()
				a.maybeCompact("default", messages, nil)
			}()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				log.Printf("[WARN] maybeCompact timed out, continuing in background")
				// Don't wait for it - let it run in background
				return
			}
		}()
	}

	// Overflow handling: estimate context tokens and apply pruning/compaction
	if a.store != nil {
		messages = a.handleContextOverflow("default", messages)
	}

	// Use streaming HTTP request
	reqBody := ChatRequest{
		Model:       a.cfg.Model,
		Messages:    messages,
		Temperature: a.cfg.Temperature,
		MaxTokens:   a.cfg.MaxTokens,
		Stream:      true,
	}
	if len(a.systemTools) == 0 {
		a.refreshToolSpecs()
	}
	reqBody.Tools = a.systemTools

	body, _ := json.Marshal(reqBody)
	url := a.cfg.BaseURL + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		callback(fmt.Sprintf("request build error: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		callback(fmt.Sprintf("API error: %v", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		callback(fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody)))
		return
	}

	// Update rate limiter for Anthropic
	a.updateAnthropicRateLimit()

	// Read SSE stream and check for tool calls
	reader := bufio.NewReader(resp.Body)
	var contentBuilder strings.Builder
	var toolCalls []ToolCall

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			// Parse SSE data
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							// Check for tool calls
							if tcRaw, ok := delta["tool_calls"].([]interface{}); ok && len(tcRaw) > 0 {
								// Collect tool calls from chunk
								for _, tcItem := range tcRaw {
									if tcMap, ok := tcItem.(map[string]interface{}); ok {
										tcData, _ := json.Marshal(tcMap)
										var tcCall ToolCall
										if err := json.Unmarshal(tcData, &tcCall); err == nil {
											if tcCall.Function.Name != "" {
												toolCalls = append(toolCalls, tcCall)
											}
										}
									}
								}
							}
							// Send content
							if content, ok := delta["content"].(string); ok {
								contentBuilder.WriteString(content)
								callback(content)
							}
						}
					}
				}
			}
		}
	}

	// Handle tool calls if any
	if len(toolCalls) > 0 && a.registry != nil {
		// Normalize and validate tool call arguments to avoid invalid JSON errors
		validCalls := make([]ToolCall, 0, len(toolCalls))
		for _, tc := range toolCalls {
			args := strings.TrimSpace(tc.Function.Arguments)
			if args == "" {
				args = "{}"
			}
			if !json.Valid([]byte(args)) {
				// Skip invalid tool calls to avoid API 400 errors
				continue
			}
			tc.Function.Arguments = args
			if tc.Function.Name != "" {
				validCalls = append(validCalls, tc)
			}
		}
		if len(validCalls) == 0 {
			// Fallback to non-streaming flow if tool calls are invalid
			reply := a.Chat(messages)
			if reply != "" {
				callback(reply)
			}
			return
		}
		toolCalls = validCalls

		// Send tool execution start event (for streaming)
		log.Printf("[TOOL] TOOL_EVENT: sending tool_start event")
		log.Printf("[TOOL] DEBUG: Sending TOOL_EVENT to callback, toolCalls=%d", len(toolCalls))
		callback(`[TOOL_EVENT]{"type":"tool_start","tools":[` + 
			strings.Join(func() []string {
				var names []string
				for _, tc := range toolCalls {
					names = append(names, fmt.Sprintf(`{"name":"%s","id":"%s"}`, tc.Function.Name, tc.ID))
				}
				return names
			}(), ",") + `]}`)

		// Execute tool calls
		results := a.executeToolCalls(toolCalls)

		// Send tool result event (for streaming)
		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			// Check if result contains error
			hasError := false
			if resultMap, ok := tr.Result.(map[string]interface{}); ok {
				if _, exists := resultMap["error"]; exists {
					hasError = true
				}
			}
			log.Printf("[TOOL] TOOL_EVENT: sending tool_result event for %s", toolCalls[i].ID)
			callback(fmt.Sprintf(`[TOOL_EVENT]{"type":"tool_result","tool_id":"%s","success":%t,"result":%s}`,
				toolCalls[i].ID, !hasError, string(resultBytes)))
		}

		// Build tool result messages
		newMessages := make([]Message, 0, len(messages)+len(results)+1)
		newMessages = append(newMessages, messages...)
		newMessages = append(newMessages, Message{Role: "assistant", Content: contentBuilder.String(), ToolCalls: toolCalls})

		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			toolMsg := Message{
				Role:       "tool",
				Content:    string(resultBytes),
				ToolCallID: toolCalls[i].ID,
			}
			newMessages = append(newMessages, toolMsg)
		}

		// Call API again with tool results and stream the response
		a.ChatStream(newMessages, callback)
		return
	}

	// No tool calls: store final assistant reply
	if a.store != nil && lastMsg != "" {
		reply := strings.TrimSpace(contentBuilder.String())
		if reply != "" {
			a.storeMessage("default", "user", lastMsg)
			a.storeMessage("default", "assistant", reply)
		}
	}
}

func (a *Agent) executeToolCalls(toolCalls []ToolCall) []ToolResult {
	results := make([]ToolResult, 0, len(toolCalls))

	// Tool loop detection - check before executing
	if a.toolLoopDetector != nil {
		for _, call := range toolCalls {
			hasLoop, reason := a.toolLoopDetector.CheckLoop()
			if hasLoop {
				log.Printf("[Agent] Tool loop detected: %s", reason)
				return []ToolResult{{
					ID:   call.ID,
					Type: "function",
					Result: map[string]interface{}{
						"error":   "Tool loop detected: " + reason,
						"tool":    call.Function.Name,
						"success": false,
					},
				}}
			}
			// Record this tool call
			a.toolLoopDetector.RecordCall(call.Function.Name, call.Function.Arguments)
		}
	}

	for _, call := range toolCalls {
		var result interface{}
		var err error

		if a.registry != nil {
			result, err = a.registry.CallTool(call.Function.Name, parseArgs(call.Function.Arguments))
		} else {
			err = fmt.Errorf("tool registry not initialized")
		}

		if err != nil {
			if call.Function.Name == "exec" && err.Error() == "shell features are disabled; use a simple command or enable OCG_EXEC_ALLOW_SHELL" {
				result = err.Error()
			} else {
				result = map[string]interface{}{
					"error":   err.Error(),
					"tool":    call.Function.Name,
					"success": false,
				}
			}
		} else {
			// Simplify exec output to plain text
			if call.Function.Name == "exec" {
				switch v := result.(type) {
				case tools.ExecResult:
					out := strings.TrimSpace(v.Stdout)
					if out == "" {
						out = strings.TrimSpace(v.Stderr)
					}
					if out == "" {
						out = "OK"
					}
					result = out
				case *tools.ExecResult:
					out := strings.TrimSpace(v.Stdout)
					if out == "" {
						out = strings.TrimSpace(v.Stderr)
					}
					if out == "" {
						out = "OK"
					}
					result = out
				}
			}
			result = map[string]interface{}{
				"result":  result,
				"tool":    call.Function.Name,
				"success": true,
			}
		}

		results = append(results, ToolResult{
			ID:     call.ID,
			Type:   "function",
			Result: result,
		})
	}

	// Apply tool result truncation
	results = TruncateToolResults(results, DefaultToolResultTruncationConfig)

	return results
}

func (a *Agent) handleToolCalls(messages []Message, toolCalls []ToolCall, assistantMsg *Message, depth int, callback func(string)) string {
	// Send tool execution start event
	if callback != nil && len(toolCalls) > 0 {
		callback(`[TOOL_EVENT]{"type":"tool_start","tools":[`)
		for i, tc := range toolCalls {
			if i > 0 {
				callback(",")
			}
			callback(fmt.Sprintf(`{"name":"%s","id":"%s"}`, tc.Function.Name, tc.ID))
		}
		callback(`]}`)
	}
	
	results := a.executeToolCalls(toolCalls)
	
	// Send tool result events
	if callback != nil && len(results) > 0 {
		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			callback(fmt.Sprintf(`[TOOL_EVENT]{"type":"tool_result","tool_id":"%s","success":true,"result":%s}`, toolCalls[i].ID, string(resultBytes)))
		}
	}

	resp := ToolResponse{
		ToolResults: results,
	}
	respBytes, _ := json.Marshal(resp)

	if a.cfg.APIKey == "" || depth >= 2 {
		return string(respBytes)
	}

	newMessages := make([]Message, 0, len(messages)+2)
	newMessages = append(newMessages, messages...)

	if assistantMsg != nil {
		newMessages = append(newMessages, *assistantMsg)
	} else if len(messages) == 0 || len(messages[len(messages)-1].ToolCalls) == 0 {
		newMessages = append(newMessages, Message{Role: "assistant", ToolCalls: toolCalls})
	}

	// OpenAI-style tool messages
	for i, tr := range results {
		contentBytes, _ := json.Marshal(tr.Result)
		toolMsg := Message{Role: "tool", Content: string(contentBytes)}
		if i < len(toolCalls) {
			toolMsg.ToolCallID = toolCalls[i].ID
		} else {
			toolMsg.ToolCallID = tr.ID
		}
		newMessages = append(newMessages, toolMsg)
	}

	return a.callAPIWithDepth(newMessages, depth+1)
}

func parseArgs(argsJSON string) map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		log.Printf("[WARN] Failed to parse tool args: %q -> %v", argsJSON, err)
		args = make(map[string]interface{})
	}
	return args
}

// parseCustomToolCalls parses custom tool call format from MiniMax and similar models
// Now tries JSON first (more robust), then falls back to XML-like format
func parseCustomToolCalls(content string) []ToolCall {
	var toolCalls []ToolCall

	// Pre-validation: check if content looks like it has tool calls
	hasToolIndicator := strings.Contains(content, "tool_calls") ||
		strings.Contains(content, "invoke name=") ||
		strings.Contains(content, "function_call") ||
		strings.Contains(content, "=\"")
	
	if !hasToolIndicator && !strings.Contains(content, "[{") && !strings.Contains(content, "{\"") {
		return nil
	}

	// First, try JSON format (more robust and standard)
	// Look for JSON array or object in the content
	jsonMatches := []string{}
	// Match {...} or [...] blocks - improved regex for better detection
	for _, match := range reJSONBlock.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && len(match[1]) > 10 {
			// Quick validation: must contain "name" or "function" key
			if !strings.Contains(match[1], "name") && !strings.Contains(match[1], "function") {
				continue
			}
			// Try to parse as JSON
			var parsed interface{}
			if err := json.Unmarshal([]byte(match[1]), &parsed); err == nil {
				jsonMatches = append(jsonMatches, match[1])
			}
		}
	}

	// If we found JSON that looks like tool calls, parse it
	for _, jsonStr := range jsonMatches {
		// Try to parse as array of tool calls
		var calls []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &calls); err != nil {
			log.Printf("[WARN] Failed to parse tool calls JSON: %v", err)
			continue
		}
		for _, call := range calls {
			name, _ := call["name"].(string)
			if name == "" {
				name, _ = call["function"].(map[string]interface{})["name"].(string)
			}
			if name == "" {
				continue
			}

			// Get arguments
			argsMap := make(map[string]interface{})
			if args, ok := call["arguments"].(map[string]interface{}); ok {
				argsMap = args
			} else if args, ok := call["function"].(map[string]interface{})["arguments"].(map[string]interface{}); ok {
				argsMap = args
			} else if argsStr, ok := call["arguments"].(string); ok {
				// Try to parse arguments string as JSON
				var argsParsed map[string]interface{}
				if err := json.Unmarshal([]byte(argsStr), &argsParsed); err != nil {
					log.Printf("[WARN] Failed to parse arguments JSON: %v", err)
					argsMap["raw"] = argsStr
				} else {
					argsMap = argsParsed
				}
			}

			argsJSON, _ := json.Marshal(argsMap)
			toolCalls = append(toolCalls, ToolCall{
				ID:   fmt.Sprintf("call_%d", len(toolCalls)),
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      mapToolName(name),
					Arguments: string(argsJSON),
				},
			})
		}
		if len(toolCalls) > 0 {
			log.Printf("[DEBUG] parseCustomToolCalls: parsed %d JSON tool calls", len(toolCalls))
			return toolCalls
		}
	}

	// Fall back to XML-like format (more fragile, but some models use it)
	// Pattern 1: <minimax:tool_call>...<invoke name="...">...</invoke>...</minimax:tool_call> (with newlines)
	matches1 := reXMLTool1.FindAllStringSubmatch(content, -1)

	// Pattern 2: <minimax:tool_call><invoke name="..."><parameter>...</invoke>...</invoke> (without newlines)
	matches2 := reXMLTool2.FindAllStringSubmatch(content, -1)

	matches := append(matches1, matches2...)

	// Validate content size to prevent DoS
	if len(content) > 50000 {
		log.Printf("[WARN] parseCustomToolCalls: content too large, truncating")
		content = content[:50000]
	}

	log.Printf("[DEBUG] parseCustomToolCalls: content length=%d, XML matches found=%d", len(content), len(matches))

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		toolName := m[1]
		if len(toolName) > 100 || toolName == "" {
			continue
		}
		paramsStr := m[2]

		// Parse parameters
		args := make(map[string]interface{})

		// Match <parameter name="key">value</parameter>
		paramMatches := reXMLParam.FindAllStringSubmatch(paramsStr, -1)

		for _, pm := range paramMatches {
			if len(pm) >= 3 {
				key := pm[1]
				value := strings.TrimSpace(pm[2])
				if len(key) <= 100 && len(value) <= 10000 {
					args[key] = value
				}
			}
		}

		// Map tool names if needed
		actualToolName := mapToolName(toolName)
		argsJSON, _ := json.Marshal(args)

		toolCalls = append(toolCalls, ToolCall{
			ID:   fmt.Sprintf("call_%d", len(toolCalls)),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      actualToolName,
				Arguments: string(argsJSON),
			},
		})
	}

	// If still no tool calls found, try simple name/arguments extraction
	if len(toolCalls) == 0 {
		toolCalls = parseSimpleToolCalls(content)
	}

	return toolCalls
}

// parseSimpleToolCalls is a fallback parser for basic tool call extraction
// when JSON/XML parsing fails
func parseSimpleToolCalls(content string) []ToolCall {
	var toolCalls []ToolCall
	
	// Look for patterns like: "tool_name" or 'tool_name' followed by arguments
	// This is a last-resort fallback
	reSimple := regexp.MustCompile(`(?i)(?:tool_calls?|function_call|invoke)[:\s]+["']?([a-zA-Z_][a-zA-Z0-9_]*)["']?`)
	matches := reSimple.FindAllStringSubmatch(content, -1)
	
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		toolName := strings.TrimSpace(m[1])
		if toolName == "" || len(toolName) > 100 {
			continue
		}
		
		// Only accept known tool names to avoid false positives
		knownTools := map[string]bool{
			"read": true, "write": true, "edit": true, "exec": true,
			"process": true, "browser": true, "message": true, "cron": true,
			"memory_search": true, "memory_get": true, "sessions_send": true,
			"subagents": true, "image": true, "tts": true, "web_fetch": true,
		}
		
		if !knownTools[toolName] {
			continue
		}
		
		toolCalls = append(toolCalls, ToolCall{
			ID:   fmt.Sprintf("call_simple_%d", len(toolCalls)),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      toolName,
				Arguments: "{}",
			},
		})
	}
	
	if len(toolCalls) > 0 {
		log.Printf("[DEBUG] parseSimpleToolCalls: extracted %d tool calls (fallback)", len(toolCalls))
	}
	
	return toolCalls
}

// mapToolName maps model-specific tool names to actual tool names
func mapToolName(modelToolName string) string {
	switch modelToolName {
	case "read_file":
		return "read"
	case "write_file":
		return "write"
	case "execute_command", "exec_cmd":
		return "exec"
	case "cat":
		return "read" // cat is similar to read
	default:
		return modelToolName // return as-is if no mapping
	}
}

type ToolResponse struct {
	ToolResults []ToolResult `json:"tool_results"`
}

var (
	reEdit1     = regexp.MustCompile(`(?i)Edit\s+([^:]+):\s*replace\s+(.+)\s+with\s+(.+)`)
	reEdit2     = regexp.MustCompile(`(?i)Edit\s+([^:]+):\s*change\s+(.+)\s+to\s+(.+)`)
	reEdit3     = regexp.MustCompile(`(?i)Replace\s+(.+)\s+with\s+(.+)\s+in\s+(.+)`)
	reEdit4     = regexp.MustCompile(`(?i)replace\s+(.+)\s+with\s+(.+)\s+in\s+([^ ]+)`)
	reJSONBlock = regexp.MustCompile(`(?s)(\[[^\]]*\]|\{[^}]*\})`)
	reXMLTool1  = regexp.MustCompile(`(?i)<minimax:tool_call>\s*<invoke\s+name="([^"]+)"[^>]*>(.*?)</invoke>\s*</minimax:tool_call>`)
	reXMLTool2  = regexp.MustCompile(`(?i)<minimax:tool_call>\s*<invoke\s+name="([^"]+)"[^>]*>(.*?)</invoke>\s*`)
	reXMLParam  = regexp.MustCompile(`<parameter\s+name="([^"]+)">([^<]*)</parameter>`)
)

// Edit intent detection - detect natural language edit requests
func detectEditIntent(msg string) map[string]interface{} {
	if m := reEdit1.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[1]),
			"oldText": strings.TrimSpace(m[2]),
			"newText": strings.TrimSpace(m[3]),
		}
	}

	if m := reEdit2.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[1]),
			"oldText": strings.TrimSpace(m[2]),
			"newText": strings.TrimSpace(m[3]),
		}
	}

	if m := reEdit3.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[3]),
			"oldText": strings.TrimSpace(m[1]),
			"newText": strings.TrimSpace(m[2]),
		}
	}

	if m := reEdit4.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[3]), // path is 3rd capture
			"oldText": strings.TrimSpace(m[1]), // old is 1st capture
			"newText": strings.TrimSpace(m[2]), // new is 2nd capture
		}
	}

	return nil
}

// handleEdit processes edit requests
func (a *Agent) handleEdit(args map[string]interface{}) string {
	if a.registry == nil {
		return "Error: tool registry not initialized"
	}

	result, err := a.registry.CallTool("edit", args)
	if err != nil {
		return fmt.Sprintf("Edit failed: %v", err)
	}

	// Format result
	b, _ := json.Marshal(result)
	return fmt.Sprintf("Edit completed: %s", string(b))
}

// recallRelevantMemories automatically retrieves memories related to the prompt
func (a *Agent) recallRelevantMemories(prompt string) string {
	if a.memoryStore == nil {
		return ""
	}
	limit := a.cfg.RecallLimit
	if limit <= 0 {
		limit = 3
	}
	minScore := float32(a.cfg.RecallMinScore)
	if minScore <= 0 {
		minScore = 0.3
	}

	results, err := a.memoryStore.Search(prompt, limit*2, minScore)
	if err != nil || len(results) == 0 {
		return ""
	}

	// re-rank by category/importance weighting
	catBoost := map[string]float32{
		"decision":   0.2,
		"preference": 0.15,
		"fact":       0.1,
		"entity":     0.05,
	}
	sort.Slice(results, func(i, j int) bool {
		ri := results[i]
		rj := results[j]
		wi := ri.Score * (1 + float32(ri.Entry.Importance)) * (1 + catBoost[strings.ToLower(ri.Entry.Category)])
		wj := rj.Score * (1 + float32(rj.Entry.Importance)) * (1 + catBoost[strings.ToLower(rj.Entry.Category)])
		return wi > wj
	})
	if len(results) > limit {
		results = results[:limit]
	}

	return tools.FormatMemoriesForContext(results)
}

func isRecallRequest(msg string) bool {
	low := strings.ToLower(strings.TrimSpace(msg))
	return strings.HasPrefix(low, "/recall") ||
		strings.HasPrefix(low, "recall") ||
		strings.HasPrefix(low, "remember")
}

// maybeFlushMemory soft-triggers long memory flush (SQLite storage)
// Rules: trigger every 50 messages with a minimum interval of 10 minutes
func (a *Agent) maybeFlushMemory(lastMsg string) {
	if a.store == nil || a.memoryStore == nil {
		return
	}

	stats, err := a.store.Stats()
	if err != nil {
		return
	}
	msgCount := stats["messages"]
	// Optimization: Check less frequently (every 200 messages) to reduce DB load
	// Also add random factor to avoid thundering herd
	if msgCount == 0 || msgCount%200 != 0 {
		return
	}

	lastFlushAtStr, _ := a.store.GetConfig("memory", "lastFlushAt")
	lastFlushCountStr, _ := a.store.GetConfig("memory", "lastFlushCount")
	lastFlushAt, _ := strconv.ParseInt(lastFlushAtStr, 10, 64)
	lastFlushCount, _ := strconv.Atoi(lastFlushCountStr)

	if lastFlushCount == msgCount {
		return
	}
	if time.Now().Unix()-lastFlushAt < 600 {
		return
	}

	if lastMsg != "" && tools.ShouldCapture(lastMsg) {
		category := tools.DetectCategory(lastMsg)
		_, _ = a.memoryStore.StoreWithSource(lastMsg, category, 0.5, "flush")
	}

	_ = a.store.SetConfig("memory", "lastFlushAt", fmt.Sprintf("%d", time.Now().Unix()))
	_ = a.store.SetConfig("memory", "lastFlushCount", fmt.Sprintf("%d", msgCount))
}

// runCompact triggers manual compaction with optional instructions
func (a *Agent) runCompact(instructions string) string {
	if a.store == nil {
		return "Storage not available"
	}

	sessionKey := "default"
	messages, err := a.store.GetMessages(sessionKey, 500)
	if err != nil || len(messages) == 0 {
		return "No messages to compress"
	}

	// Get session meta
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		meta = storage.SessionMeta{
			SessionKey: sessionKey,
		}
	}

	// Build summary with optional instructions (using LLM)
	var summary string
	if instructions != "" {
		summary = a.buildSummaryWithInstructionsLLM(messages, instructions)
	} else {
		summary = a.buildSummaryWithInstructionsLLM(messages, "Concisely summarize the key points of the conversation")
	}

	// Archive and clear
	if len(messages) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, messages[len(messages)-1].ID)
		meta.LastCompactedMessageID = messages[len(messages)-1].ID
	}
	_ = a.store.ClearMessages(sessionKey)

	// Add summary
	if summary != "" {
		_ = a.store.AddMessage(sessionKey, "system", "[summary]\n"+summary)
	}

	// Update meta
	meta.CompactionCount += 1
	meta.LastSummary = summary
	_ = a.store.UpsertSessionMeta(meta)

	return fmt.Sprintf("Compaction complete. Kept %d messages, summary length: %d chars", len(messages), len(summary))
}

// runNewSession starts a new session (new session key)
func (a *Agent) runNewSession() string {
	if a.store == nil {
		return "Storage not available"
	}

	// Generate new session key
	newKey := fmt.Sprintf("session-%d", time.Now().UnixNano())
	_ = a.store.AddMessage(newKey, "system", "New session started at "+time.Now().Format("2006-01-02 15:04:05"))

	return fmt.Sprintf("New session created: %s", newKey)
}

// runResetSession resets the current session
func (a *Agent) runResetSession() string {
	if a.store == nil {
		return "Storage not available"
	}

	sessionKey := "default"

	// Archive current messages
	messages, _ := a.store.GetMessages(sessionKey, 1000)
	if len(messages) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, messages[len(messages)-1].ID)
	}

	// Clear messages
	_ = a.store.ClearMessages(sessionKey)

	// Add reset marker
	_ = a.store.AddMessage(sessionKey, "system", "[session reset] "+time.Now().Format("2006-01-02 15:04:05"))

	// Update meta
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err == nil {
		meta.CompactionCount = 0
		meta.LastSummary = ""
		_ = a.store.UpsertSessionMeta(meta)
	}

	return "Session reset"
}

// executeSplitTask explicitly splits and executes a task
func (a *Agent) executeSplitTask(task string) string {
	if a.store == nil {
		return "Storage not available"
	}

	log.Printf("[TaskSplit] Splitting task: %s", task[:min(50, len(task))])

	// Split the task using LLM
	subtasks, err := a.SplitTask(task)
	if err != nil {
		log.Printf("[TaskSplit] Failed to split task: %v", err)
		return fmt.Sprintf("Task split failed: %v", err)
	}

	if len(subtasks) == 0 {
		return "Cannot split task"
	}

	// Create task in storage
	taskID, err := a.storeUserTask("default", task, subtasks)
	if err != nil {
		log.Printf("[TaskSplit] Failed to create task: %v", err)
		return fmt.Sprintf("Create task failed: %v", err)
	}

	// Execute subtasks
	_, err = a.ExecuteSubtasks(taskID, "default")
	if err != nil {
		log.Printf("[TaskSplit] Failed to execute subtasks: %v", err)
		return fmt.Sprintf("Execute task failed: %v", err)
	}

	// Keep main-session context small: return marker only.
	// Full process/details remain in DB (user_tasks + user_subtasks).
	return fmt.Sprintf("Task completed [OK]\nTask ID: %s\nMarker: [task_done:%s]\nUse /task detail %s to view full details.", taskID, taskID, taskID)
}

// storeUserTask stores a task in SQLite
func (a *Agent) storeUserTask(session, instructions string, subtasks []string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not available")
	}
	// Generate task ID
	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	return a.store.CreateUserTask(id, session, instructions, subtasks)
}

// runTaskList lists recent tasks for a session
func (a *Agent) runTaskList(session string, limit int) string {
	if a.store == nil {
		return "Storage not available"
	}
	tasks, err := a.store.GetUserTasksBySession(session, limit)
	if err != nil {
		return fmt.Sprintf("Failed to list tasks: %v", err)
	}
	if len(tasks) == 0 {
		return "No tasks found"
	}
	var sb strings.Builder
	sb.WriteString("Recent tasks:\n")
	for _, t := range tasks {
		fmt.Fprintf(&sb, "- %s | %s | %d/%d\n", t.ID, t.Status, t.Completed, t.Total)
	}
	return sb.String()
}

// runTaskDetail returns full task details from DB by task ID
func (a *Agent) runTaskDetail(taskID string, page, pageSize int) string {
	if a.store == nil {
		return "Storage not available"
	}
	t, err := a.store.GetUserTask(taskID)
	if err != nil {
		return fmt.Sprintf("Task not found: %s", taskID)
	}
	subs, err := a.store.GetUserSubtasks(taskID)
	if err != nil {
		return fmt.Sprintf("Failed to load subtasks: %v", err)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	totalSubs := len(subs)
	start := (page - 1) * pageSize
	if start > totalSubs {
		start = totalSubs
	}
	end := start + pageSize
	if end > totalSubs {
		end = totalSubs
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Task: %s\n", t.ID)
	fmt.Fprintf(&sb, "Status: %s (%d/%d)\n", t.Status, t.Completed, t.Total)
	fmt.Fprintf(&sb, "CreatedAt: %s\n", formatUnixMilli(t.CreatedAt))
	if t.CompletedAt != nil {
		fmt.Fprintf(&sb, "CompletedAt: %s\n", formatUnixMilli(*t.CompletedAt))
		fmt.Fprintf(&sb, "DurationMs: %d\n", *t.CompletedAt-t.CreatedAt)
	}
	fmt.Fprintf(&sb, "Instructions: %s\n\n", t.Instructions)
	if t.Result != "" {
		sb.WriteString("Result:\n")
		sb.WriteString(t.Result)
		sb.WriteString("\n\n")
	}
	fmt.Fprintf(&sb, "Subtasks (page %d, size %d, total %d):\n", page, pageSize, totalSubs)
	for _, s := range subs[start:end] {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", s.IndexNum+1, s.Status, s.Description)
		if s.Result != "" {
			sb.WriteString("   result: ")
			sb.WriteString(s.Result)
			sb.WriteString("\n")
		}
	}
	if end < totalSubs {
		fmt.Fprintf(&sb, "\nMore subtasks available. Use: /task detail %s %d %d\n", taskID, page+1, pageSize)
	}
	return sb.String()
}

// runTaskSummary returns compact task status for low-context follow-ups
func (a *Agent) runTaskSummary(taskID string) string {
	if a.store == nil {
		return "Storage not available"
	}
	t, err := a.store.GetUserTask(taskID)
	if err != nil {
		return fmt.Sprintf("Task not found: %s", taskID)
	}
	subs, _ := a.store.GetUserSubtasks(taskID)
	completed := 0
	for _, s := range subs {
		if s.Status == "completed" {
			completed++
		}
	}
	res := t.Result
	if len(res) > 280 {
		res = res[:280] + "..."
	}
	dur := int64(0)
	if t.CompletedAt != nil {
		dur = *t.CompletedAt - t.CreatedAt
	}
	completedAt := "-"
	if t.CompletedAt != nil {
		completedAt = formatUnixMilli(*t.CompletedAt)
	}
	return fmt.Sprintf("[task_done:%s]\nstatus=%s progress=%d/%d\ncreated_at=%s completed_at=%s duration_ms=%d\ninstructions=%s\nresult=%s\nUse /task detail %s for full process.",
		t.ID, t.Status, completed, t.Total, formatUnixMilli(t.CreatedAt), completedAt, dur, t.Instructions, res, t.ID)
}

func formatUnixMilli(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}

func (a *Agent) runArchiveDebug(sessionKey string) string {
	if a.store == nil {
		return "Storage not available"
	}
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		return fmt.Sprintf("Failed to load session meta: %v", err)
	}
	stats, err := a.store.GetArchiveStats(sessionKey)
	if err != nil {
		return fmt.Sprintf("Failed to load archive stats: %v", err)
	}
	return fmt.Sprintf("Archive debug (%s)\nlast_compacted_message_id=%d\narchived_count=%d\narchive_max_source_message_id=%d\ncompaction_count=%d\nlast_summary_len=%d",
		sessionKey,
		meta.LastCompactedMessageID,
		stats.ArchivedCount,
		stats.LastSourceMessage,
		meta.CompactionCount,
		len(meta.LastSummary),
	)
}

func (a *Agent) runLiveDebug(sessionKey string) string {
	now := time.Now()
	a.realtimeMu.Lock()
	activeCount := 0
	lines := make([]string, 0, len(a.realtimeSessions))
	for key, p := range a.realtimeSessions {
		if p == nil || !p.IsConnected() {
			continue
		}
		activeCount++
		last := a.realtimeLastUsed[key]
		idle := now.Sub(last).Round(time.Second)
		if sessionKey == "" || sessionKey == key {
			lines = append(lines, fmt.Sprintf("- %s connected=true last_used=%s idle=%s", key, last.Format("2006-01-02 15:04:05"), idle))
		}
	}
	a.realtimeMu.Unlock()

	metaLine := ""
	if sessionKey != "" && a.store != nil {
		if meta, err := a.store.GetSessionMeta(sessionKey); err == nil {
			metaLine = fmt.Sprintf("\nmeta: provider_type=%s realtime_last_active_at=%s", meta.ProviderType, meta.RealtimeLastActiveAt.Format("2006-01-02 15:04:05"))
		}
	}

	if len(lines) == 0 {
		if sessionKey != "" {
			return fmt.Sprintf("Live debug\nactive_connections=%d\nno active live connection for session=%s%s", activeCount, sessionKey, metaLine)
		}
		return fmt.Sprintf("Live debug\nactive_connections=%d\nno active live connections", activeCount)
	}

	return fmt.Sprintf("Live debug\nactive_connections=%d\n%s%s", activeCount, strings.Join(lines, "\n"), metaLine)
}

// buildSummaryWithInstructions builds a summary with custom instructions using LLM
func (a *Agent) buildSummaryWithInstructionsLLM(messages []storage.Message, instructions string) string {
	if len(messages) == 0 {
		return ""
	}

	// Build context for summary
	var sb strings.Builder
	fmt.Fprintf(&sb, "Summarize the following conversation history. Key points: %s\n\n", instructions)

	for _, m := range messages {
		if m.Role == "system" && strings.HasPrefix(m.Content, "[summary]") {
			continue // Skip old summary
		}
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, content)
	}

	// Call LLM to generate summary
	summary, err := a.callLLMForSummary(sb.String())
	if err != nil {
		log.Printf("[WARN] LLM summary failed: %v, using fallback", err)
		// Fallback to simple concatenation
		return buildSummary(messages)
	}

	return summary
}

// callLLMForSummary makes a non-streaming LLM call to generate a summary
func (a *Agent) callLLMForSummary(prompt string) (string, error) {
	// Prepare request body for non-streaming
	reqBody := ChatRequest{
		Model:       a.cfg.Model,
		Messages:    []Message{{Role: "user", Content: prompt}},
		Temperature: 0.3, // Lower temperature for deterministic summary
		MaxTokens:   2048,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := a.cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Update rate limiter for Anthropic
	a.updateAnthropicRateLimit()

	// Parse response
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return response.Choices[0].Message.Content, nil
}

// pruneMessages removes old tool results from messages based on config
// This is called before LLM calls to reduce context size
func (a *Agent) pruneMessages(sessionKey string, messages []Message) []Message {
	if a.cfg.ContextPruning.Mode != "cache-ttl" {
		return messages
	}

	// Rate limiting is handled by updateAnthropicRateLimit() after each API call

	cfg := a.cfg.ContextPruning
	if cfg.Mode == "off" {
		return messages
	}

	// Find tool result messages and apply pruning
	result := make([]Message, 0, len(messages))
	var lastAssistantIdx int

	for i, msg := range messages {
		if msg.Role == "assistant" {
			lastAssistantIdx = i
		}
	}

	// Protect last N assistant messages
	protectedFrom := 0
	if lastAssistantIdx > 0 && cfg.KeepLastAssistants > 0 {
		protectedFrom = len(messages) - (cfg.KeepLastAssistants * 2) // rough estimate
		if protectedFrom < 0 {
			protectedFrom = 0
		}
	}

	for i, msg := range messages {
		// Skip non-tool-result messages
		if len(msg.ToolExecutionResults) == 0 {
			result = append(result, msg)
			continue
		}

		// Skip if protected
		if i >= protectedFrom {
			result = append(result, msg)
			continue
		}

		// Skip if contains images (check Result field)
		hasImage := false
		for _, tr := range msg.ToolExecutionResults {
			if tr.Result != nil {
				_, ok := tr.Result.(string)
				if !ok {
					hasImage = true
					break
				}
			}
		}
		if hasImage {
			result = append(result, msg)
			continue
		}

		// Apply soft trim or hard clear
		for j := range msg.ToolExecutionResults {
			if msg.ToolExecutionResults[j].Result != nil {
				content, ok := msg.ToolExecutionResults[j].Result.(string)
				if ok && len(content) > cfg.MinPrunableToolChars {
					// Calculate trim sizes
					headChars := cfg.SoftTrim.HeadChars
					if headChars == 0 {
						headChars = 1500
					}
					tailChars := cfg.SoftTrim.TailChars
					if tailChars == 0 {
						tailChars = 1500
					}

					if len(content) > headChars+tailChars {
						newContent := content[:headChars] + "\n...[truncated, original size: " + fmt.Sprintf("%d", len(content)) + " bytes]...\n" + content[len(content)-tailChars:]
						msg.ToolExecutionResults[j].Result = newContent
					} else if cfg.HardClear.Enabled && len(content) > 0 {
						placeholder := cfg.HardClear.Placeholder
						if placeholder == "" {
							placeholder = "[Old tool result content cleared]"
						}
						msg.ToolExecutionResults[j].Result = placeholder
					}
				}
			}
		}
		result = append(result, msg)
	}

	return result
}

// maybeCompact checks if compaction is needed and executes it if necessary
// Uses channel to notify when compaction is done (for retry support)
// If compactChan is not nil, sends true on it when compaction is performed
func (a *Agent) maybeCompact(sessionKey string, messages []Message, compactChan chan<- bool) {
	if a.store == nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	stored, err := a.store.GetMessages(sessionKey, 500)
	if err != nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	tokens := estimateTokensFromStore(stored)
	meta.TotalTokens = tokens
	_ = a.store.UpsertSessionMeta(meta)

	// Dynamic context window for current model (matches official OCG behavior)
	providerType := a.detectProviderType()
	contextWindow := llm.GetContextWindow(providerType, a.cfg.Model, a.cfg.BaseURL, a.cfg.APIKey, a.cfg.Models)
	if contextWindow <= 0 {
		contextWindow = a.cfg.ContextTokens // fallback to config
	}

	// Calculate threshold: context tokens * compaction threshold
	// E.g., 128000 * 0.7 = 89600 tokens
	threshold := int(float64(contextWindow) * a.cfg.CompactionThreshold)
	log.Printf("[maybeCompact] Model=%s context_window=%d threshold=%d tokens=%d",
		a.cfg.Model, contextWindow, threshold, tokens)

	if tokens < threshold || len(stored) <= a.cfg.KeepMessages {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	cut := len(stored) - a.cfg.KeepMessages
	old := stored[:cut]
	keep := stored[cut:]

	summary := buildSummary(old)

	// archive old messages first (uses session_meta.last_compacted_message_id as watermark)
	if len(old) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, old[len(old)-1].ID)
		meta.LastCompactedMessageID = old[len(old)-1].ID
	}

	meta.CompactionCount += 1
	meta.LastSummary = summary
	meta.MemoryFlushCompactionCnt = meta.CompactionCount
	meta.MemoryFlushAt = time.Now()
	_ = a.store.UpsertSessionMeta(meta)

	_ = a.store.ClearMessages(sessionKey)
	for _, m := range keep {
		_ = a.store.AddMessage(sessionKey, m.Role, m.Content)
	}
	if summary != "" {
		_ = a.store.AddMessage(sessionKey, "system", "[summary]\n"+summary)
	}
	log.Printf("[CLEAN] Compaction done: session=%s, kept=%d, totalTokens=%d", sessionKey, len(keep), tokens)

	// Notify via channel (for retry support)
	if compactChan != nil {
		compactChan <- true
	}
}

// tokenCounter is a package-level tiktoken instance for accurate counting
var (
	tokenCounter     *tiktoken.Tiktoken
	tokenCounterOnce sync.Once
)

// initTokenCounter initializes tiktoken for accurate token counting
func initTokenCounter() {
	tokenCounterOnce.Do(func() {
		// cl100k_base is used by GPT-3.5 Turbo, GPT-4, GPT-4 Turbo
		tk, err := tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			log.Printf("[WARN] Token estimation will use fallback method: %v", err)
			return
		}
		tokenCounter = tk
	})
}

func estimateTokensFromStore(messages []storage.Message) int {
	// Optimization #6: Use tiktoken for accurate BPE token counting
	initTokenCounter()

	if tokenCounter != nil {
		total := 0
		for _, m := range messages {
			tokens := tokenCounter.Encode(m.Content, nil, nil)
			total += len(tokens)
		}
		return total
	}

	// Fallback: rough estimate if tokenizer unavailable
	total := 0
	for _, m := range messages {
		ascii := 0
		nonASCII := 0
		// FIX: range over string directly instead of converting to []rune
		for _, r := range m.Content {
			if r <= 127 {
				ascii++
			} else {
				nonASCII++
			}
		}
		// Rough estimate: ASCII ~4 chars/token, non-ASCII (e.g., CJK) ~0.5 token/char
		total += ascii/4 + nonASCII*2 + 4
	}
	return total
}

func buildSummary(msgs []storage.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s: %s", m.Role, content))
	}
	return strings.Join(lines, "\n")
}

// detectProviderType detects LLM provider type from BaseURL
func (a *Agent) detectProviderType() llm.ProviderType {
	if strings.Contains(a.cfg.BaseURL, "anthropic") {
		return llm.ProviderAnthropic
	} else if strings.Contains(a.cfg.BaseURL, "google") || strings.Contains(a.cfg.BaseURL, "generativelanguage") {
		return llm.ProviderGoogle
	} else if strings.Contains(a.cfg.BaseURL, "minimax") {
		return llm.ProviderMiniMax
	} else if strings.Contains(a.cfg.BaseURL, "ollama") {
		return llm.ProviderOllama
	} else if strings.Contains(a.cfg.BaseURL, "openrouter") {
		return llm.ProviderOpenRouter
	} else if strings.Contains(a.cfg.BaseURL, "moonshot") {
		return llm.ProviderMoonshot
	} else if strings.Contains(a.cfg.BaseURL, "zhipu") || strings.Contains(a.cfg.BaseURL, "glm") {
		return llm.ProviderGLM
	} else if strings.Contains(a.cfg.BaseURL, "qianfan") {
		return llm.ProviderQianfan
	} else if strings.Contains(a.cfg.BaseURL, "bedrock") {
		return llm.ProviderBedrock
	} else if strings.Contains(a.cfg.BaseURL, "vercel") {
		return llm.ProviderVercel
	} else if strings.Contains(a.cfg.BaseURL, "z.ai") || strings.Contains(a.cfg.BaseURL, "z ai") {
		return llm.ProviderZAi
	}
	return llm.ProviderOpenAI // default
}

// updateAnthropicRateLimit updates the last Anthropic API call timestamp
// Should be called after each successful Anthropic API request
func (a *Agent) updateAnthropicRateLimit() {
	if a.detectProviderType() == llm.ProviderAnthropic {
		a.lastAnthropicCall = time.Now()
	}
}

// handleContextOverflow estimates context tokens and applies pruning/compaction if needed
// This matches official OCG behavior: check if current + new > context_window, then prune/compact
func (a *Agent) handleContextOverflow(sessionKey string, messages []Message) []Message {
	if a.store == nil {
		return messages
	}

	// Dynamically get current model's context window (matches official OCG behavior)
	providerType := a.detectProviderType()
	contextWindow := llm.GetContextWindow(providerType, a.cfg.Model, a.cfg.BaseURL, a.cfg.APIKey, a.cfg.Models)
	if contextWindow <= 0 {
		contextWindow = a.cfg.ContextTokens // fallback to config
	}
	reserveTokens := a.cfg.ReserveTokens
	if reserveTokens <= 0 {
		reserveTokens = 5000 // default reserve
	}
	threshold := contextWindow - reserveTokens

	log.Printf("[Context] Model=%s context_window=%d reserve=%d threshold=%d",
		a.cfg.Model, contextWindow, reserveTokens, threshold)

	// Get stored messages
	stored, err := a.store.GetMessages(sessionKey, 500)
	if err != nil || len(stored) == 0 {
		return messages
	}

	// Estimate current tokens from stored messages
	currentTokens := estimateTokensFromStore(stored)

	// Estimate tokens from new messages (messages that haven't been stored yet)
	newMessageTokens := estimateTokens(messages)

	// Total estimated tokens
	totalTokens := currentTokens + newMessageTokens

	// Check if we need to handle overflow
	if totalTokens <= threshold {
		// No overflow, continue normally
		return messages
	}

	log.Printf("[STATS] Context overflow detected: current=%d + new=%d = %d > threshold=%d",
		currentTokens, newMessageTokens, totalTokens, threshold)

	// Step 1: Try pruning first (in-memory, per-request)
	if a.cfg.ContextPruning.Mode == "cache-ttl" {
		// Rate limiting handled by updateAnthropicRateLimit()
		originalLen := len(messages)
		messages = a.pruneMessages(sessionKey, messages)
		if len(messages) != originalLen {
			log.Printf("[PRUNE] Pruning reduced messages: %d -> %d", originalLen, len(messages))
		}

		// Re-estimate after pruning
		prunedTokens := estimateTokens(messages)
		if currentTokens+prunedTokens <= threshold {
			log.Printf("[OK] Pruning sufficient: %d + %d = %d <= %d", currentTokens, prunedTokens, currentTokens+prunedTokens, threshold)
			return messages
		}
	}

	// Step 2: If pruning not enough, trigger compaction
	log.Printf("[CLEAN] Pruning not enough, triggering compaction...")
	compactChan := make(chan bool, 1)
	go func() {
		// Convert stored messages to agent messages for compaction
		storedMsgs := convertStoredMessages(stored)
		a.maybeCompact(sessionKey, storedMsgs, compactChan)
	}()

	// Wait for compaction with timeout
	select {
	case compacted := <-compactChan:
		if compacted {
			log.Printf("[RELOAD] Compaction happened, reloading messages...")
			// Reload messages from storage after compaction
			reloaded, err := a.store.GetMessages(sessionKey, 500)
			if err == nil && len(reloaded) > 0 {
				messages = convertStoredMessages(reloaded)
				// Re-inject recall if needed
				if a.cfg.AutoRecall && a.memoryStore != nil && len(messages) > 0 {
					lastUserMsg := messages[len(messages)-1].Content
					if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
						injected := Message{Role: "system", Content: memories}
						messages = append([]Message{injected}, messages...)
					}
				}
			}
		}
	case <-time.After(2 * time.Second):
		log.Printf("[WARN] Compaction timed out, continuing...")
	}

	return messages
}

// estimateTokens estimates token count for agent Messages (not storage.Message)
func estimateTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		// Rough estimate: chars / 4
		total += len(m.Content) / 4
		// Add tool calls tokens
		for _, tc := range m.ToolCalls {
			total += len(tc.Function.Arguments) / 4
		}
		// Add tool results tokens
		for _, tr := range m.ToolExecutionResults {
			if tr.Result != nil {
				if s, ok := tr.Result.(string); ok {
					total += len(s) / 4
				}
			}
		}
	}
	return total
}

// convertStoredMessages converts storage.Message to agent Message
func convertStoredMessages(stored []storage.Message) []Message {
	result := make([]Message, 0, len(stored))
	for _, m := range stored {
		result = append(result, Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return result
}

func (a *Agent) callAPI(messages []Message) string {
	return a.callAPIWithDepth(messages, 0)
}

func (a *Agent) callAPIWithDepth(messages []Message, depth int) string {
	reqBody := ChatRequest{
		Model:       a.cfg.Model,
		Messages:    messages,
		Temperature: a.cfg.Temperature,
		MaxTokens:   a.cfg.MaxTokens,
	}
	// Only refresh tools if empty (done once at startup)
	if len(a.systemTools) == 0 {
		a.refreshToolSpecs()
	}

	reqBody.Tools = a.systemTools

	body, _ := json.Marshal(reqBody)
	url := a.cfg.BaseURL + "/chat/completions"

	// For tool result processing (depth > 0), use shorter timeout
	ctx := context.Background()
	if depth > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		log.Printf("[FAST] depth=%d: using 30s timeout", depth)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("request build error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Sprintf("API error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("read error: %v", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Update rate limiter for Anthropic
	a.updateAnthropicRateLimit()

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return fmt.Sprintf("parse error: %v", err)
	}

	// handle tool call chain if returned (standard format)
	if len(chatResp.Choices) > 0 && len(chatResp.Choices[0].Message.ToolCalls) > 0 {
		// Filter out invalid tool calls (empty name)
		validCalls := make([]ToolCall, 0)
		for _, tc := range chatResp.Choices[0].Message.ToolCalls {
			if tc.Function.Name != "" && tc.Function.Arguments != "" {
				validCalls = append(validCalls, tc)
			}
		}
		if len(validCalls) > 0 {
			assistantMsg := chatResp.Choices[0].Message
			return a.handleToolCalls(messages, validCalls, &assistantMsg, depth, nil)
		}
		// If all invalid, try custom format
	}

	// handle custom tool call format (MiniMax, etc.)
	if len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content

		// Try to parse custom tool call format: minimax:tool_call
		toolCalls := parseCustomToolCalls(content)
		if len(toolCalls) > 0 {
			assistantMsg := Message{Role: "assistant", Content: content, ToolCalls: toolCalls}
			return a.handleToolCalls(messages, toolCalls, &assistantMsg, depth, nil)
		}

		return content
	}

	return "no response"
}

func (a *Agent) simpleResponse(messages []Message) string {
	var userMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}

	input := strings.TrimSpace(strings.ToLower(userMsg))
	response := ""

	switch {
	case strings.Contains(input, "hello") || strings.Contains(input, "hi"):
		// Use custom greeting if configured, otherwise default
		if a.cfg.Greeting != "" {
			response = a.cfg.Greeting
		} else {
			response = "Hello! I am assistant.\n\nAvailable tools:\n- exec: run commands\n- read: read files\n- write: write files"
		}
	case strings.Contains(input, "time"):
		if a.timeProvider != nil {
			response = a.timeProvider.Now().Format("2006-01-02 15:04:05")
		} else {
			response = time.Now().Format("2006-01-02 15:04:05")
		}
	case strings.Contains(input, "stat"):
		stats, _ := a.store.Stats()
		response = fmt.Sprintf("Storage stats:\n- messages: %d\n- memories: %d\n- files: %d", stats["messages"], stats["memories"], stats["files"])
	case strings.Contains(input, "tools"):
		if a.registry != nil {
			toolList := a.registry.List()
			response = "Available tools:\n- " + strings.Join(toolList, "\n- ")
		} else {
			response = "tools not initialized"
		}
	case strings.Contains(input, "help") || strings.Contains(input, "aid"):
		response = "OCG-Go\n\nCommands:\n- hello - greeting\n- time - time\n- stat - stats\n- tools - list tools\n- help - help"
	default:
		response = "I received: " + userMsg
	}

	return response
}
