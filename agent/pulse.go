// Pulse/Heartbeat system for OCG-Go
// Runs every second (60 times per minute) to check for important events

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/hooks"
	"github.com/gliderlab/cogate/storage"
)

// PulseConfig holds configuration for the heartbeat system
type PulseConfig struct {
	Interval      time.Duration // Check interval (default 1 second)
	Enabled       bool          // Enable/disable pulse
	LLMEnabled   bool          // Enable LLM processing
	MaxQueueSize int           // Maximum events in queue
	CleanupHours int           // Hours after which to clear old events
	// Session reset
	SessionResetEnabled bool          // Enable session reset check
	SessionResetMins   int           // Check interval in minutes (default 60)
}

// DefaultPulseConfig returns default configuration
func DefaultPulseConfig() *PulseConfig {
	return &PulseConfig{
		Interval:           1 * time.Second,
		Enabled:            true,
		LLMEnabled:         true,
		MaxQueueSize:       100,
		CleanupHours:       24,
		SessionResetEnabled: false,
		SessionResetMins:   60,
	}
}

// PulseEvent represents an event to be processed by the heartbeat system
type PulseEvent struct {
	Event    *storage.Event
	Response string
	Errors   []string
}

// PulseHandler handles the heartbeat/pulse system
type PulseHandler struct {
	storage *storage.Storage
	config  *PulseConfig
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	eventCh chan *PulseEvent
	// Context for signaling background goroutines to stop
	ctx    context.Context
	cancel context.CancelFunc
	// WaitGroup for tracking background goroutines
	wg sync.WaitGroup
	// Processing state
	isProcessing bool
	currentEvent *storage.Event

	// Hooks registry for handling hook events
	hooksRegistry *hooks.HookRegistry

	// Callbacks
	onEvent      func(*PulseEvent)
	onBroadcast  func(string, int, string) error // (message, priority, channel)
	onLLMProcess func(string) (string, error)
}

// NewPulseHandler creates a new pulse handler
func NewPulseHandler(storage *storage.Storage, config *PulseConfig) *PulseHandler {
	if config == nil {
		config = DefaultPulseConfig()
	}
	return &PulseHandler{
		storage: storage,
		config:  config,
		stopCh:  make(chan struct{}),
		eventCh: make(chan *PulseEvent, config.MaxQueueSize),
	}
}

// SetBroadcastCallback sets the callback for broadcasting messages
func (p *PulseHandler) SetBroadcastCallback(cb func(string, int, string) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onBroadcast = cb
}

// SetLLMCallback sets the callback for LLM processing
func (p *PulseHandler) SetLLMCallback(cb func(string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onLLMProcess = cb
}

// SetEventCallback sets the callback for event processing
func (p *PulseHandler) SetEventCallback(cb func(*PulseEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEvent = cb
}

// Start starts the heartbeat system
func (p *PulseHandler) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	// Initialize context with cancel for background goroutines
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.running = true
	p.stopCh = make(chan struct{})
	p.mu.Unlock()

	log.Printf("[Pulse] Starting heartbeat system (interval: %v)", p.config.Interval)

	// Start the heartbeat loop
	go p.heartbeatLoop()
}

// Stop stops the heartbeat system and waits for background goroutines
func (p *PulseHandler) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	// Cancel context to signal background goroutines to stop
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	// Wait for all background goroutines to complete
	p.wg.Wait()

	log.Printf("[Pulse] Stopped heartbeat system")
}

// IsRunning returns whether the pulse is running
func (p *PulseHandler) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// IsProcessing returns whether currently processing an event
func (p *PulseHandler) IsProcessing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isProcessing
}

// heartbeatLoop runs the main heartbeat loop
func (p *PulseHandler) heartbeatLoop() {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

// Trigger immediately processes pending events
// This can be called to force event processing outside the regular heartbeat cycle
func (p *PulseHandler) Trigger() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.IsRunning() {
		log.Printf("[Pulse] Trigger called but pulse is not running")
		return
	}
	
	log.Printf("[Pulse] Trigger called, processing events immediately")
	p.tick()
}

// tick performs one heartbeat check
func (p *PulseHandler) tick() {
	// First, peek at the next event to check if we should process it
	// This prevents claiming an event only to skip it (which would leave it stuck in "processing")
	event, err := p.storage.PeekNextEvent()
	if err != nil {
		log.Printf("[Pulse] Error peeking next event: %v", err)
		return
	}

	if event == nil {
		// No pending events, do cleanup periodically
		if time.Now().Second() == 0 { // Every minute
			if err := p.storage.ClearOldEvents(p.config.CleanupHours); err != nil {
				log.Printf("[Pulse] Cleanup error: %v", err)
			}
			// Session reset check (every minute when enabled)
			if p.config.SessionResetEnabled {
				p.CheckSessionReset()
			}
		}
		return
	}

	// Check if we should process this event BEFORE claiming
	if !p.shouldProcessEvent(event) {
		return
	}

	// Now atomically claim the event (we know we want to process it)
	claimedEvent, err := p.storage.ClaimNextEvent()
	if err != nil {
		log.Printf("[Pulse] Error claiming event: %v", err)
		return
	}
	if claimedEvent == nil {
		// Another worker claimed it, that's fine
		return
	}

	// Process the event
	p.processEvent(claimedEvent)
}

// CheckSessionReset checks and resets stale sessions based on config
// Called every minute when SessionResetEnabled is true
func (p *PulseHandler) CheckSessionReset() {
	// Use a simple counter to avoid checking every minute
	// Check every SessionResetMins minutes
	now := time.Now()
	if now.Minute()%p.config.SessionResetMins != 0 {
		return
	}

	log.Printf("[Pulse] Checking for sessions to reset...")

	// Get sessions for reset based on idle policy (default: 24 hours)
	sessions, err := p.storage.GetSessionsForReset("idle", 4, 1440)
	if err != nil {
		log.Printf("[Pulse] Session reset check error: %v", err)
		return
	}

	// Reset idle sessions
	resetCount := 0
	for _, sess := range sessions {
		if err := p.storage.ResetSession(sess.SessionKey); err != nil {
			log.Printf("[Pulse] Failed to reset session %s: %v", sess.SessionKey, err)
		} else {
			log.Printf("[Pulse] Reset idle session: %s", sess.SessionKey)
			resetCount++
		}
	}

	if resetCount > 0 {
		log.Printf("[Pulse] Reset %d idle sessions", resetCount)
	}

	// Daily reset check - runs at configured hour (default 4 AM)
	// Check at the configured hour only
	if now.Hour() == 4 && now.Minute() == 0 {
		dailySessions, err := p.storage.GetSessionsForReset("daily", 4, 0)
		if err != nil {
			log.Printf("[Pulse] Daily session reset check error: %v", err)
			return
		}
		for _, sess := range dailySessions {
			if err := p.storage.ResetSession(sess.SessionKey); err != nil {
				log.Printf("[Pulse] Failed to reset daily session %s: %v", sess.SessionKey, err)
			} else {
				log.Printf("[Pulse] Reset daily session: %s", sess.SessionKey)
				resetCount++
			}
		}
		if len(dailySessions) > 0 {
			log.Printf("[Pulse] Reset %d daily sessions", len(dailySessions))
		}
	}
}

// shouldProcessEvent determines if an event should be processed now
func (p *PulseHandler) shouldProcessEvent(event *storage.Event) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// If already processing, only critical events can interrupt
	if p.isProcessing && event.Priority > storage.PriorityCritical {
		return false
	}

	// Priority 0 (Critical) - always process immediately
	// Priority 1 (High) - process when not processing critical
	// Priority 2 (Normal) - process when idle
	// Priority 3 (Low) - process when explicitly idle

	switch event.Priority {
	case storage.PriorityCritical:
		return true
	case storage.PriorityHigh:
		return !p.isProcessing || p.currentEvent == nil
	case storage.PriorityNormal:
		return !p.isProcessing
	case storage.PriorityLow:
		return !p.isProcessing
	}

	return false
}

// processEvent handles processing a single event
func (p *PulseHandler) processEvent(event *storage.Event) {
	p.mu.Lock()
	p.isProcessing = true
	p.currentEvent = event
	p.mu.Unlock()

	log.Printf("[Pulse] Processing event: id=%d, priority=%d, title=%s",
		event.ID, event.Priority, event.Title)

	// Mark as processing (if not already claimed)
	if event.Status != "processing" {
		if err := p.storage.UpdateEventStatus(event.ID, "processing"); err != nil {
			log.Printf("[Pulse] Update status error: %v", err)
		}
	}

	var response string
	var errors []string

	// Handle based on priority
	switch event.Priority {
	case storage.PriorityCritical:
		// Broadcast to all channels immediately
		msg := fmt.Sprintf("[CRITICAL]: %s\n\n%s", event.Title, event.Content)
		p.mu.RLock()
		if p.onBroadcast != nil {
			if err := p.onBroadcast(msg, 0, event.Channel); err != nil {
				errors = append(errors, err.Error())
			}
		}
		p.mu.RUnlock()
		response = "Broadcasted to all channels"

	case storage.PriorityHigh:
		// Check if this is a Hook event (event_type starts with "hook:")
		if event.EventType != "" && strings.HasPrefix(event.EventType, "hook:") {
			// Handle as Hook event
			p.handleHookEvent(event)
			return
		}

		// Broadcast to specified channel(s)
		msg := fmt.Sprintf("%s\n\n%s", event.Title, event.Content)
		p.mu.RLock()
		channel := event.Channel
		if p.onBroadcast != nil {
			// If channel specified, use it; otherwise broadcast to all
			if channel != "" {
				if err := p.onBroadcast(msg, 1, channel); err != nil {
					errors = append(errors, err.Error())
				}
			} else {
				if err := p.onBroadcast(msg, 1, ""); err != nil {
					errors = append(errors, err.Error())
				}
			}
		}
		p.mu.RUnlock()
		response = "Broadcasted to channel"

	case storage.PriorityNormal, storage.PriorityLow:
		// Process LLM asynchronously to avoid blocking heartbeat loop
		// Mark as completed immediately, LLM processing happens in background
		if p.config.LLMEnabled {
			p.mu.RLock()
			llmCb := p.onLLMProcess
			p.mu.RUnlock()

			if llmCb != nil {
				// Update status to "processing_llm" to indicate background work
				if err := p.storage.UpdateEventStatus(event.ID, "processing_llm"); err != nil {
					log.Printf("[Pulse] Update status error: %v", err)
				}

				// Get callbacks under lock, then release before async call
				p.mu.RLock()
				eventCbCopy := p.onEvent
				p.mu.RUnlock()

				// Increment WaitGroup before starting goroutine
				p.wg.Add(1)

				// Run LLM processing in background (with panic recovery)
				go func(ev *storage.Event, cb func(*PulseEvent)) {
					defer p.wg.Done() // Decrement WaitGroup when done
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[Pulse] LLM goroutine panic recovered: %v", r)
							if err := p.storage.UpdateEventStatusWithResponse(ev.ID, "completed_with_errors", fmt.Sprintf("panic recovered: %v", r)); err != nil {
								log.Printf("[Pulse] Update status error: %v", err)
							}
						}
					}()

					// Check if context was cancelled (shutdown)
					select {
					case <-p.ctx.Done():
						if err := p.storage.UpdateEventStatusWithResponse(ev.ID, "cancelled", "service shutdown"); err != nil {
							log.Printf("[Pulse] Update status error: %v", err)
						}
						return
					default:
					}

					input := fmt.Sprintf("Event: %s\n\nDescription: %s\n\nPlease analyze and respond:",
						ev.Title, ev.Content)
					resp, err := llmCb(input)

					var respText string
					var errs []string
					if err != nil {
						errs = append(errs, err.Error())
					} else {
						respText = resp
					}

					// Create result event
					result := &PulseEvent{
						Event:    ev,
						Response: respText,
						Errors:   errs,
					}

					// Trigger callback if provided
					if cb != nil {
						cb(result)
					}

					// Update final status
					if len(errs) > 0 {
						if err := p.storage.UpdateEventStatusWithResponse(ev.ID, "completed_with_errors", strings.Join(errs, "; ")); err != nil {
							log.Printf("[Pulse] Update status error: %v", err)
						}
					} else {
						if err := p.storage.UpdateEventStatusWithResponse(ev.ID, "completed", respText); err != nil {
							log.Printf("[Pulse] Update status error: %v", err)
						}
					}
				}(event, eventCbCopy)

				// Return early - don't block heartbeat loop
				p.mu.Lock()
				p.isProcessing = false
				p.currentEvent = nil
				p.mu.Unlock()
				return
			}
		}
	}

	// Create pulse event result
	pulseEvent := &PulseEvent{
		Event:    event,
		Response: response,
		Errors:   errors,
	}

	// Trigger callback
	p.mu.RLock()
	eventCb := p.onEvent
	p.mu.RUnlock()
	if eventCb != nil {
		eventCb(pulseEvent)
	}

	// Update status
	if len(errors) > 0 {
		if err := p.storage.UpdateEventStatusWithResponse(event.ID, "completed_with_errors", strings.Join(errors, "; ")); err != nil {
			log.Printf("[Pulse] Update status error: %v", err)
		}
	} else {
		if err := p.storage.UpdateEventStatusWithResponse(event.ID, "completed", response); err != nil {
			log.Printf("[Pulse] Update status error: %v", err)
		}
	}

	// Reset processing state
	p.mu.Lock()
	p.isProcessing = false
	p.currentEvent = nil
	p.mu.Unlock()
}

// AddEvent adds a new event to the pulse system
func (p *PulseHandler) AddEvent(title, content string, priority int, channel string) (int64, error) {
	if priority < 0 || priority > 3 {
		priority = 2 // Default to normal
	}
	return p.storage.AddEvent(title, content, storage.EventPriority(priority), channel)
}

// handleHookEvent processes a hook event
func (p *PulseHandler) handleHookEvent(event *storage.Event) {
	log.Printf("[Pulse] Handling hook event: id=%d, type=%s, hook=%s",
		event.ID, event.EventType, event.HookName)

	// Check if hooks registry is available
	p.mu.RLock()
	registry := p.hooksRegistry
	p.mu.RUnlock()

	if registry == nil {
		log.Printf("[Pulse] Hooks registry not available, marking event as failed")
		p.storage.UpdateEventStatusWithResponse(event.ID, "completed_with_errors", "hooks registry not available")
		return
	}

	// Parse event type (remove "hook:" prefix)
	eventType := strings.TrimPrefix(event.EventType, "hook:")

	// Get hooks for this event type
	hookList := registry.GetHooks(hooks.EventType(eventType))
	if len(hookList) == 0 {
		log.Printf("[Pulse] No hooks registered for event type: %s", eventType)
		p.storage.UpdateEventStatus(event.ID, "completed")
		return
	}

	// Create hook event context
	hookCtx := hooks.EventContext{
		Content:    event.Content,
		Metadata:   make(map[string]interface{}),
	}

	// Parse metadata if present
	if event.Metadata != "" {
		if err := json.Unmarshal([]byte(event.Metadata), &hookCtx.Metadata); err != nil {
			log.Printf("[Pulse] Failed to parse hook metadata: %v", err)
		}
	}

	// Dispatch to all matching hooks
	var errors []string
	for _, hook := range hookList {
		if !hook.Enabled {
			log.Printf("[Pulse] Skipping disabled hook: %s", hook.Name)
			continue
		}

		log.Printf("[Pulse] Dispatching hook: %s", hook.Name)

		// Run hook asynchronously
		p.wg.Add(1)
		go func(h *hooks.Hook) {
			defer p.wg.Done()

			// Create hook event
			hookEvent := hooks.NewHookEvent(
				hooks.EventType(eventType),
				"", // action
				event.HookName,
			)
			hookEvent.Context = hookCtx

			// Execute hook handler
			if err := h.Handler.Handle(hookEvent); err != nil {
				errMsg := fmt.Sprintf("hook %s error: %v", h.Name, err)
				log.Printf("[Pulse] %s", errMsg)
				errors = append(errors, errMsg)
			}
		}(hook)
	}

	// Update event status
	if len(errors) > 0 {
		p.storage.UpdateEventStatusWithResponse(event.ID, "completed_with_errors", strings.Join(errors, "; "))
	} else {
		p.storage.UpdateEventStatus(event.ID, "completed")
	}
}

// GetStatus returns the current status of the pulse system
func (p *PulseHandler) GetStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	counts, _ := p.storage.GetEventCount()

	return map[string]interface{}{
		"running":       p.running,
		"is_processing": p.isProcessing,
		"current_event": p.currentEvent,
		"event_counts":  counts,
		"config":        p.config,
	}
}

// ParsePriority parses a priority string to EventPriority
func ParsePriority(s string) storage.EventPriority {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "0", "critical", "crit", "c":
		return storage.PriorityCritical
	case "1", "high", "important", "h":
		return storage.PriorityHigh
	case "2", "normal", "n":
		return storage.PriorityNormal
	case "3", "low", "l":
		return storage.PriorityLow
	default:
		return storage.PriorityNormal
	}
}

// EventToJSON converts an event to JSON
func EventToJSON(event *storage.Event) string {
	data, _ := json.Marshal(event)
	return string(data)
}
