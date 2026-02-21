// Package llmhealth provides LLM health check and auto-failover functionality
package llmhealth

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
)

// Config holds health check configuration
type Config struct {
	Enabled        bool          // Enable health check
	Interval       time.Duration // Check interval (default 1 hour)
	FailureThreshold int         // Failures before failover (default 3)
	SuccessThreshold int        // Successes before recover (default 2)
	TestPrompt     string        // Test prompt (default "hello")
	Timeout        time.Duration // Request timeout (default 30s)
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Interval:       time.Hour,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		TestPrompt:     "hello",
		Timeout:        30 * time.Second,
	}
}

// HealthStatus represents the health status of a provider
type HealthStatus struct {
	Provider    llm.ProviderType
	Healthy     bool
	Latency     time.Duration // Response time
	LastCheck   time.Time
	FailCount   int
	SuccessCount int
	Error       string
}

// FailoverEvent represents a failover event
type FailoverEvent struct {
	Time        time.Time
	FromProvider llm.ProviderType
	ToProvider  llm.ProviderType
	Reason      string
	Manual      bool // True if manually triggered
}

// Manager handles health checking and failover
type Manager struct {
	config       *Config
	status       map[llm.ProviderType]*HealthStatus
	failoverCh   chan FailoverEvent
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
	currentPrimary llm.ProviderType // Current primary provider
}

// NewManager creates a new health check manager
func NewManager(cfg *Config) *Manager {
	return &Manager{
		config:       cfg,
		status:       make(map[llm.ProviderType]*HealthStatus),
		failoverCh:   make(chan FailoverEvent, 10),
		stopCh:       make(chan struct{}),
		currentPrimary: llm.ProviderOpenAI, // Default
	}
}

// SetPrimary sets the current primary provider
func (m *Manager) SetPrimary(p llm.ProviderType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentPrimary = p
}

// GetPrimary returns the current primary provider
func (m *Manager) GetPrimary() llm.ProviderType {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPrimary
}

// Start begins health checking
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return fmt.Errorf("health check not enabled")
	}
	if m.running {
		return fmt.Errorf("health check already running")
	}

	m.running = true
	log.Printf("[LLMHealth] Starting health check (interval: %v, failure threshold: %d)", 
		m.config.Interval, m.config.FailureThreshold)

	// Initialize status for all providers
	providers := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
	}
	for _, p := range providers {
		m.status[p] = &HealthStatus{
			Provider:  p,
			Healthy:   true,
			FailCount: 0,
		}
	}

	// Run initial check
	m.checkAll()

	// Start periodic check
	go m.runLoop()

	return nil
}

// Stop stops health checking
func (m *Manager) Stop() {
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Printf("[LLMHealth] Stopped")
}

// GetStatus returns health status for all providers
func (m *Manager) GetStatus() map[llm.ProviderType]*HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[llm.ProviderType]*HealthStatus)
	for k, v := range m.status {
		status[k] = v
	}
	return status
}

// GetFailoverEvents returns recent failover events (last 10)
func (m *Manager) GetFailoverEvents() []FailoverEvent {
	// In a real implementation, we'd store events in a slice
	// For now, return empty slice
	return []FailoverEvent{}
}

func (m *Manager) runLoop() {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

func (m *Manager) checkAll() {
	providers := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
	}

	for _, p := range providers {
		m.checkProvider(p)
	}

	// Check if we need to failover
	m.evaluateFailover()
}

func (m *Manager) checkProvider(pType llm.ProviderType) {
	p, err := llm.GetProvider(pType)
	if err != nil {
		m.updateStatus(pType, false, 0, err.Error())
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	req := &llm.ChatRequest{
		Model: p.GetConfig().Model,
		Messages: []llm.Message{
			{Role: "user", Content: m.config.TestPrompt},
		},
		MaxTokens: 10,
	}

	_, err = p.Chat(ctx, req)
	latency := time.Since(start)

	if err != nil {
		m.updateStatus(pType, false, latency, err.Error())
		log.Printf("[LLMHealth] %s: UNHEALTHY (%v) - %v", pType, latency, err)
	} else {
		m.updateStatus(pType, true, latency, "")
		log.Printf("[LLMHealth] %s: HEALTHY (%v)", pType, latency)
	}
}

func (m *Manager) updateStatus(pType llm.ProviderType, healthy bool, latency time.Duration, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.status[pType]
	if !ok {
		status = &HealthStatus{Provider: pType}
		m.status[pType] = status
	}

	status.LastCheck = time.Now()
	status.Latency = latency
	status.Error = errMsg

	if healthy {
		status.Healthy = true
		status.FailCount = 0
		status.SuccessCount++
		if status.SuccessCount >= m.config.SuccessThreshold {
			// Provider recovered, could switch back if it's higher priority
			log.Printf("[LLMHealth] %s: RECOVERED", pType)
		}
	} else {
		status.Healthy = false
		status.SuccessCount = 0
		status.FailCount++
	}
}

func (m *Manager) evaluateFailover() {
	m.mu.Lock()
	defer m.mu.Unlock()

	primaryStatus := m.status[m.currentPrimary]
	if primaryStatus == nil {
		return
	}

	// Check if primary is unhealthy and exceeded failure threshold
	if !primaryStatus.Healthy && primaryStatus.FailCount >= m.config.FailureThreshold {
		// Find best fallback
		newPrimary := m.findBestFallback()
		if newPrimary != m.currentPrimary {
			log.Printf("[LLMHealth] FAILOVER: %s -> %s (failures: %d)", 
				m.currentPrimary, newPrimary, primaryStatus.FailCount)
			
			event := FailoverEvent{
				Time:        time.Now(),
				FromProvider: m.currentPrimary,
				ToProvider:  newPrimary,
				Reason:      fmt.Sprintf("failure threshold reached (%d failures)", primaryStatus.FailCount),
			}
			m.failoverCh <- event

			m.currentPrimary = newPrimary
		}
	}
}

func (m *Manager) findBestFallback() llm.ProviderType {
	// Priority order: OpenAI > Anthropic > Google > MiniMax > Ollama > Custom
	priority := []llm.ProviderType{
		llm.ProviderOpenAI,
		llm.ProviderAnthropic,
		llm.ProviderGoogle,
		llm.ProviderMiniMax,
		llm.ProviderOllama,
		llm.ProviderCustom,
	}

	for _, p := range priority {
		if p == m.currentPrimary {
			continue
		}
		status, ok := m.status[p]
		if ok && status.Healthy {
			return p
		}
	}

	// If no healthy fallback found, stay with current
	return m.currentPrimary
}

// ManualFailover triggers a manual failover to specified provider
func (m *Manager) ManualFailover(target llm.ProviderType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure status entry exists
	if _, ok := m.status[target]; !ok {
		m.status[target] = &HealthStatus{
			Provider:  target,
			Healthy:   true, // Assume healthy for manual
			FailCount: 0,
		}
	}

	status := m.status[target]
	if !status.Healthy {
		return fmt.Errorf("provider %s is not healthy", target)
	}

	oldPrimary := m.currentPrimary
	m.currentPrimary = target

	event := FailoverEvent{
		Time:        time.Now(),
		FromProvider: oldPrimary,
		ToProvider:  target,
		Reason:      "manual failover",
		Manual:      true,
	}
	m.failoverCh <- event

	log.Printf("[LLMHealth] MANUAL FAILOVER: %s -> %s", oldPrimary, target)
	return nil
}

// LoadConfigFromEnv loads health check config from environment variables
func LoadConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if v := os.Getenv("LLM_HEALTH_CHECK"); v != "" {
		cfg.Enabled = v == "1" || v == "true"
	}

	if v := os.Getenv("LLM_HEALTH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Interval = d
		}
	}

	if v := os.Getenv("LLM_HEALTH_FAILURE_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.FailureThreshold = n
		}
	}

	return cfg
}
