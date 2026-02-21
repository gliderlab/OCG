// Package config provides configuration types for OCG services
// Supports dependency injection for customizable behavior
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
)

// GatewayConfig holds all configurable Gateway parameters
type GatewayConfig struct {
	Host             string        // Host to bind (default: "0.0.0.0")
	Port             int           // Port to listen (default: 18789)
	UIAuthToken      string        // Token for UI authentication
	ReadTimeout      time.Duration // HTTP read timeout (default: 120s)
	WriteTimeout     time.Duration // HTTP write timeout (default: 180s)
	IdleTimeout      time.Duration // HTTP idle timeout (default: 300s)
	MaxBodyChat      int64         // Max body size for chat (default: 2MB)
	MaxBodyProcess   int64         // Max body size for process (default: 512KB)
	MaxBodyMemory    int64         // Max body size for memory (default: 512KB)
	MaxBodyCron      int64         // Max body size for cron (default: 256KB)
	MaxBodyWebhook   int64         // Max body size for webhook (default: 256KB)
	MaxProcessLogCap int           // Max lines/chars per log request (default: 10KB)
	AgentAddr        string        // Agent RPC address (optional)
	StaticDir        string        // Static files directory
	EnvConfigPath    string        // Path to env.config
	PidDir           string        // PID directory
	DBPath           string        // Database path
	RateLimitWindow  time.Duration // Rate limit window (default: 1h)

	GatewayDir    string // Base gateway directory (static + data)
	CronJobsPath  string // Cron jobs file path (override)
	TelegramToken string // Telegram bot token (override)

	// Hot reload config
	ReloadMode    string        // "hot", "hybrid", "restart", "off" (default: "hybrid")
	ReloadDebounce time.Duration // Debounce for file changes (default: 300ms)

	// Session config
	Session SessionConfig // Session routing and reset config

	// Webhook config
	Webhook WebhookConfig // Webhook configuration
}

// DefaultGatewayConfig returns the default gateway configuration
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		Host:             "0.0.0.0",
		Port:             55003, // Match DefaultGatewayPort in defaults.go
		ReadTimeout:      120 * time.Second,
		WriteTimeout:     180 * time.Second,
		IdleTimeout:      300 * time.Second,
		MaxBodyChat:      2 * 1024 * 1024, // 2MB
		MaxBodyProcess:   512 * 1024,      // 512KB
		MaxBodyMemory:    512 * 1024,      // 512KB
		MaxBodyCron:      256 * 1024,      // 256KB
		MaxBodyWebhook:   256 * 1024,      // 256KB
		MaxProcessLogCap: 10 * 1024,       // 10KB
		PidDir:           DefaultPidDir(),
		RateLimitWindow:  time.Hour,
		ReloadMode:      "hybrid",         // hot-apply safe changes, auto-restart for critical
		ReloadDebounce:  300 * time.Millisecond,
		Session: SessionConfig{
			DMScope: "main",
			Reset: SessionResetConfig{
				Mode:        "daily",
				AtHour:      4,
				IdleMinutes: 0,
			},
			MainKey: "main",
		},
		Webhook: WebhookConfig{
			Enabled:                  false,
			Path:                    "/hooks",
			AllowedAgentIDs:          []string{"main"},
			DefaultSessionKey:        "hook:ingress",
			AllowRequestSessionKey:   false,
			AllowedSessionKeyPrefixes: []string{"hook:"},
		},
	}
}

// SessionResetConfig holds session reset configuration
type SessionResetConfig struct {
	Mode        string        // "daily", "idle", "off" (default: "daily")
	AtHour      int           // Hour of day for daily reset (default: 4, UTC)
	IdleMinutes int           // Idle minutes before reset (default: 0 = disabled)
}

// SessionConfig holds session routing and management configuration
type SessionConfig struct {
	DMScope      string                   // "main", "per-peer", "per-channel-peer", "per-account-channel-peer"
	Reset        SessionResetConfig          // Session reset policy
	ResetByType  map[string]SessionResetConfig // Per-type overrides (direct, group, thread)
	MainKey      string                   // Main session key (default: "main")
}

// GenerateSessionKey generates a session key based on dmScope and message metadata
// agentId: the agent ID
// channel: the channel type (e.g., "telegram", "whatsapp")
// peerId: the sender ID (user ID)
// accountId: the account ID (for multi-account channels)
// chatType: "direct" or "group"
// Returns: the session key
func (s *SessionConfig) GenerateSessionKey(agentId, channel, peerId, accountId, chatType string) string {
	switch s.DMScope {
	case "per-peer":
		// agent:<agentId>:dm:<peerId>
		return fmt.Sprintf("agent:%s:dm:%s", agentId, peerId)
	case "per-channel-peer":
		// agent:<agentId>:<channel>:dm:<peerId>
		return fmt.Sprintf("agent:%s:%s:dm:%s", agentId, channel, peerId)
	case "per-account-channel-peer":
		// agent:<agentId>:<channel>:<accountId>:dm:<peerId>
		if accountId == "" {
			accountId = "default"
		}
		return fmt.Sprintf("agent:%s:%s:%s:dm:%s", agentId, channel, accountId, peerId)
	case "main":
		fallthrough
	default:
		// agent:<agentId>:<mainKey> (default: main)
		mainKey := s.MainKey
		if mainKey == "" {
			mainKey = "main"
		}
		return fmt.Sprintf("agent:%s:%s", agentId, mainKey)
	}
}

// ContextPruningConfig holds session pruning configuration
type ContextPruningConfig struct {
	Mode                string        // "off", "cache-ttl" (default: "off")
	TTL                 time.Duration // TTL for cache-ttl mode (default: 5m)
	KeepLastAssistants  int           // Number of assistant messages to protect (default: 3)
	SoftTrimRatio      float64       // Soft trim ratio (default: 0.3)
	HardClearRatio     float64       // Hard clear ratio (default: 0.5)
	MinPrunableToolChars int         // Min chars to consider for pruning (default: 50000)
	SoftTrim          struct {
		MaxChars   int // Max chars after soft trim (default: 4000)
		HeadChars  int // Head chars to keep (default: 1500)
		TailChars  int // Tail chars to keep (default: 1500)
	}
	HardClear struct {
		Enabled     bool   // Enable hard clear (default: true)
		Placeholder string // Placeholder text (default: "[Old tool result content cleared]")
	}
	Tools struct {
		Allow []string // Tools to allow pruning
		Deny  []string // Tools to deny pruning
	}
}

// AgentConfig holds all configurable Agent parameters
type AgentConfig struct {
	Model            string        // LLM model name
	APIKey           string        // API key for LLM provider
	BaseURL          string        // Base URL for LLM API
	ContextTokens    int           // Context window size (default: 8192, auto-detect if 0)
	ReserveTokens    int           // Reserved tokens (default: 1024)
	Models           llm.ModelsConfig `json:"models,omitempty"` // Models configuration with context window fallback
	SoftTokens       int           // Soft limit tokens (default: 800)
	Greeting         string        // Custom greeting message (default: English greeting)
	ThinkingMode     string        // Thinking mode: off, on, stream (default: off)
	CompactionThreshold float64    // Compress when context usage exceeds this ratio (default: 0.7 = 70%)
	KeepMessages     int           // Messages to keep after compaction (default: 30)
	AutoRecall       bool          // Enable automatic memory recall
	RecallLimit      int           // Max memories to recall (default: 3)
	RecallMinScore   float64       // Min similarity score for recall (default: 0.3)
	HTTPTimeout      time.Duration // HTTP client timeout (default: 120s)
	Temperature      float64       // Default temperature (default: 0.7)
	MaxTokens        int           // Default max tokens (default: 1000)
	PulseEnabled     bool          // Enable pulse/heartbeat system
	PulseInterval    time.Duration // Pulse interval (default: 30s)
	StoragePath      string        // SQLite storage path
	MemoryDBPath     string        // Vector memory database path
	HNSWPath         string        // HNSW index path
	EmbeddingServer  string        // Local embedding server URL
	EmbeddingModel   string        // OpenAI embedding model
	EmbeddingApiKey  string        // OpenAI embedding API key
	EmbeddingDim     int           // Embedding dimension
	HybridEnabled    bool          // Enable hybrid search (default: true)
	VectorWeight     float32       // Vector search weight (default: 0.7)
	TextWeight       float32       // Text search weight (default: 0.3)
	CandidateMult    int           // Candidate multiplier (default: 4)
	MaxResults       int           // Max search results (default: 5)
	MinScore         float32       // Min similarity score (default: 0.7)
	ContextPruning   ContextPruningConfig // Session pruning config
}

// DefaultAgentConfig returns the default agent configuration
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		ContextTokens:  8192,
		ReserveTokens:  1024,
		SoftTokens:     800,
		CompactionThreshold: 0.7, // Compress when context usage exceeds 70%
		KeepMessages:   30,
		AutoRecall:     false,
		RecallLimit:    3,
		RecallMinScore: 0.3,
		HTTPTimeout:    120 * time.Second,
		Temperature:    0.7,
		MaxTokens:      1000,
		PulseEnabled:   false,
		PulseInterval:  30 * time.Second,
		HybridEnabled:  true,
		VectorWeight:   0.7,
		TextWeight:     0.3,
		CandidateMult:  4,
		MaxResults:     5,
		MinScore:       0.7,
		ContextPruning: ContextPruningConfig{
			Mode:               "off",
			TTL:                5 * time.Minute,
			KeepLastAssistants:  3,
			SoftTrimRatio:      0.3,
			HardClearRatio:     0.5,
			MinPrunableToolChars: 50000,
		},
	}
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	DBPath          string        // Database path
	MaxOpenConns    int           // Max open connections (default: 4)
	MaxIdleConns    int           // Max idle connections (default: 4)
	ConnMaxLifetime time.Duration // Connection max lifetime (default: 5m)
	WalMode         bool          // Enable WAL mode (default: true)
	SyncMode        string        // Sync mode (default: "NORMAL")
}

// DefaultStorageConfig returns the default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		DBPath:          DefaultDBPath(),
		MaxOpenConns:    4,
		MaxIdleConns:    4,
		ConnMaxLifetime: 5 * time.Minute,
		WalMode:         true,
		SyncMode:        "NORMAL",
	}
}

// MemoryConfig holds vector memory configuration
type MemoryConfig struct {
	DBPath          string  // Database path
	HNSWPath        string  // HNSW index file path
	EmbeddingServer string  // Local embedding server URL
	EmbeddingModel  string  // OpenAI embedding model
	EmbeddingApiKey string  // OpenAI embedding API key
	EmbeddingDim    int     // Embedding dimension (auto-detected)
	MaxResults      int     // Max results per search (default: 5)
	MinScore        float32 // Minimum similarity score (default: 0.7)
	HybridEnabled   bool    // Enable hybrid search (default: true)
	VectorWeight    float32 // Vector weight (default: 0.7)
	TextWeight      float32 // Text weight (default: 0.3)
	CandidateMult   int     // Candidate multiplier (default: 4)
	BatchSize       int     // Vector load batch size (default: 1000)
}

// DefaultMemoryConfig returns the default memory configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxResults:    5,
		MinScore:      0.7,
		HybridEnabled: true,
		VectorWeight:  0.7,
		TextWeight:    0.3,
		CandidateMult: 4,
		BatchSize:     1000,
	}
}

// EmbeddingConfig holds embedding service configuration
type EmbeddingConfig struct {
	ServerURL string        // Local embedding server URL
	Model     string        // OpenAI embedding model
	APIKey    string        // OpenAI API key
	Dim       int           // Embedding dimension (auto-detected)
	Timeout   time.Duration // Request timeout (default: 60s)

	// Local embedding server (llama.cpp) config
	Host         string
	Port         int
	LlamaHost    string
	LlamaPort    int
	LlamaBin     string
	ModelPath    string
	Verbose      bool
	PortMin      int
	PortMax      int
	LlamaPortMin int
	LlamaPortMax int
}

// DefaultEmbeddingConfig returns the default embedding configuration
func DefaultEmbeddingConfig() *EmbeddingConfig {
	return &EmbeddingConfig{
		Timeout:      60 * time.Second,
		Host:         "0.0.0.0",
		PortMin:      50000,
		PortMax:      60000,
		LlamaHost:    "0.0.0.0",
		LlamaPortMin: 18000,
		LlamaPortMax: 19000,
	}
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	Enabled                  bool     // Enable/disable webhooks
	Token                   string   // Shared secret token for authentication
	Path                    string   // Webhook endpoint path (default: /hooks)
	AllowedAgentIDs         []string // Allowed agent IDs for explicit routing
	DefaultSessionKey       string   // Default session key for webhook agent runs
	AllowRequestSessionKey  bool     // Allow sessionKey in request (default: false)
	AllowedSessionKeyPrefixes []string // Allowed session key prefixes
}

// DefaultWebhookConfig returns the default webhook configuration
func DefaultWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		Enabled:                  false,
		Path:                    "/hooks",
		AllowedAgentIDs:          []string{"main"},
		DefaultSessionKey:        "hook:ingress",
		AllowRequestSessionKey:   false,
		AllowedSessionKeyPrefixes: []string{"hook:"},
	}
}

// CronConfig holds cron scheduler configuration
type CronConfig struct {
	JobsPath     string        // Path to jobs storage
	PollInterval time.Duration // Poll interval (default: 1s)
}

// DefaultCronConfig returns the default cron configuration
func DefaultCronConfig() *CronConfig {
	return &CronConfig{
		PollInterval: time.Second,
	}
}

// ProcessConfig holds process tool configuration
type ProcessConfig struct {
	WorkDir    string        // Default working directory
	EnvPrefix  string        // Environment variable prefix
	Timeout    time.Duration // Default command timeout
	MaxLogSize int           // Max log size (default: 1MB)
	CleanupAge time.Duration // Old process cleanup age (default: 5m)
}

// DefaultProcessConfig returns the default process configuration
func DefaultProcessConfig() *ProcessConfig {
	return &ProcessConfig{
		Timeout:    0, // No default timeout
		MaxLogSize: 1024 * 1024,
		CleanupAge: 5 * time.Minute,
	}
}

// EmbeddingDimensions maps model names to their embedding dimensions
var EmbeddingDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1024,
}

// MemoryCategories defines valid memory categories
var MemoryCategories = []string{
	"preference",
	"decision",
	"fact",
	"entity",
	"other",
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	WindowSize    time.Duration // Time window for rate limiting
	MaxRequests   int           // Max requests per window (0 = unlimited)
	EndpointRules map[string]EndpointRule
}

// EndpointRule defines rate limit rules for a specific endpoint
type EndpointRule struct {
	MaxRequests int           // Max requests (0 = use default)
	Window      time.Duration // Custom window for this endpoint
}

// DefaultRateLimitConfig returns the default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		WindowSize:    time.Hour,
		MaxRequests:   100,
		EndpointRules: make(map[string]EndpointRule),
	}
}

// ServerConfig combines all server configurations
type ServerConfig struct {
	Gateway   *GatewayConfig
	Agent     *AgentConfig
	Storage   *StorageConfig
	Memory    *MemoryConfig
	Cron      *CronConfig
	Process   *ProcessConfig
	RateLimit *RateLimitConfig
}

// DefaultServerConfig returns a complete default configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Gateway:   DefaultGatewayConfig(),
		Agent:     DefaultAgentConfig(),
		Storage:   DefaultStorageConfig(),
		Memory:    DefaultMemoryConfig(),
		Cron:      DefaultCronConfig(),
		Process:   DefaultProcessConfig(),
		RateLimit: DefaultRateLimitConfig(),
	}
}

// LoadFromEnv overrides configuration with environment variables
func (c *ServerConfig) LoadFromEnv(prefix string) {
	// Gateway overrides
	if v := getEnv(prefix + "PORT"); v != "" {
		c.Gateway.Port = parseInt(v, c.Gateway.Port)
	}
	if v := getEnv(prefix + "HOST"); v != "" {
		c.Gateway.Host = v
	}
	if v := getEnv(prefix + "UI_TOKEN"); v != "" {
		c.Gateway.UIAuthToken = v
	}

	// Agent overrides
	if v := getEnv(prefix + "MODEL"); v != "" {
		c.Agent.Model = v
	}
	if v := getEnv(prefix + "API_KEY"); v != "" {
		c.Agent.APIKey = v
	}
	if v := getEnv(prefix + "BASE_URL"); v != "" {
		c.Agent.BaseURL = v
	}

	// Storage overrides
	if v := getEnv(prefix + "DB_PATH"); v != "" {
		c.Storage.DBPath = v
		c.Memory.DBPath = v
		c.Gateway.DBPath = v
	}

	// Memory overrides
	if v := getEnv(prefix + "EMBEDDING_SERVER"); v != "" {
		c.Memory.EmbeddingServer = v
		c.Agent.EmbeddingServer = v
	}
	if v := getEnv(prefix + "EMBEDDING_MODEL"); v != "" {
		c.Memory.EmbeddingModel = v
		c.Agent.EmbeddingModel = v
	}
	if v := getEnv(prefix + "HNSW_PATH"); v != "" {
		c.Memory.HNSWPath = v
	}
}

// Helper functions
func getEnv(key string) string {
	return os.Getenv(key)
}

func parseInt(s string, defaultVal int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}
