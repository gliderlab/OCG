// Agent module - dependency injection wrapper
// This file provides DI-friendly constructors for Agent

package agent

import (
	"net/http"
	"time"

	"github.com/gliderlab/cogate/memory"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/storage"
	"github.com/gliderlab/cogate/tools"
)

// AgentDI provides dependency injection for Agent
type AgentDI struct {
	cfg          config.AgentConfig
	storage      *storage.Storage
	memoryStore  *memory.VectorMemoryStore
	registry     *tools.Registry
	pulseConfig  *PulseConfig
	client       *http.Client
	timeProvider TimeProvider
	idGenerator  IDGenerator
	logger       Logger
}

// NewAgentDI creates a new AgentDI builder
func NewAgentDI() *AgentDI {
	return &AgentDI{
		cfg: *config.DefaultAgentConfig(),
	}
}

// WithConfig sets the agent configuration
func (d *AgentDI) WithConfig(cfg config.AgentConfig) *AgentDI {
	d.cfg = cfg
	return d
}

// WithStorage sets the storage instance
func (d *AgentDI) WithStorage(store *storage.Storage) *AgentDI {
	d.storage = store
	return d
}

// WithMemoryStore sets the vector memory store
func (d *AgentDI) WithMemoryStore(store *memory.VectorMemoryStore) *AgentDI {
	d.memoryStore = store
	return d
}

// WithRegistry sets the tool registry
func (d *AgentDI) WithRegistry(registry *tools.Registry) *AgentDI {
	d.registry = registry
	return d
}

// WithPulseConfig sets the pulse config
func (d *AgentDI) WithPulseConfig(cfg *PulseConfig) *AgentDI {
	d.pulseConfig = cfg
	return d
}

// WithHTTPClient sets a custom HTTP client
func (d *AgentDI) WithHTTPClient(client *http.Client) *AgentDI {
	d.client = client
	return d
}

// WithTimeout sets the HTTP timeout
func (d *AgentDI) WithTimeout(timeout time.Duration) *AgentDI {
	d.cfg.HTTPTimeout = timeout
	return d
}

// WithModel sets the LLM model
func (d *AgentDI) WithModel(model string) *AgentDI {
	d.cfg.Model = model
	return d
}

// WithAPIKey sets the API key
func (d *AgentDI) WithAPIKey(apiKey string) *AgentDI {
	d.cfg.APIKey = apiKey
	return d
}

// WithBaseURL sets the base URL
func (d *AgentDI) WithBaseURL(baseURL string) *AgentDI {
	d.cfg.BaseURL = baseURL
	return d
}

// WithAutoRecall enables auto memory recall
func (d *AgentDI) WithAutoRecall(enabled bool, limit int, minScore float64) *AgentDI {
	d.cfg.AutoRecall = enabled
	d.cfg.RecallLimit = limit
	d.cfg.RecallMinScore = minScore
	return d
}

// WithTimeProvider sets a custom time provider
func (d *AgentDI) WithTimeProvider(tp TimeProvider) *AgentDI {
	d.timeProvider = tp
	return d
}

// WithIDGenerator sets a custom ID generator
func (d *AgentDI) WithIDGenerator(gen IDGenerator) *AgentDI {
	d.idGenerator = gen
	return d
}

// WithLogger sets a custom logger
func (d *AgentDI) WithLogger(logger Logger) *AgentDI {
	d.logger = logger
	return d
}

// Build creates the Agent instance
func (d *AgentDI) Build() *Agent {
	cfg := Config{
		AgentConfig: d.cfg,
		Storage:     d.storage,
		MemoryStore: d.memoryStore,
		Registry:    d.registry,
		PulseConfig: d.pulseConfig,
	}
	agent := New(cfg)

	if d.client != nil {
		agent.client = d.client
	}
	if d.timeProvider != nil {
		agent.timeProvider = d.timeProvider
	}
	if d.idGenerator != nil {
		agent.idGenerator = d.idGenerator
	}
	if d.logger != nil {
		agent.logger = d.logger
	}

	return agent
}

// Default implementation factories

// NewDefaultTimeProvider returns a default time provider
func NewDefaultTimeProvider() TimeProvider {
	return &defaultTimeProvider{}
}

// NewDefaultIDGenerator returns a default ID generator
func NewDefaultIDGenerator() IDGenerator {
	return &defaultIDGenerator{}
}

// NewDefaultLogger returns a default logger
func NewDefaultLogger() Logger {
	return &defaultLogger{}
}
