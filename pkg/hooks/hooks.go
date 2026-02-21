// Package hooks provides an event-driven hooks system for OCG
// Similar to OCG hooks, allowing automation in response to agent events
package hooks

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// Command events
	EventTypeCommand       EventType = "command"
	EventTypeCommandNew   EventType = "command:new"
	EventTypeCommandReset EventType = "command:reset"
	EventTypeCommandStop  EventType = "command:stop"

	// Agent events
	EventTypeAgentBootstrap EventType = "agent:bootstrap"

	// Gateway events
	EventTypeGatewayStartup EventType = "gateway:startup"

	// Message events
	EventTypeMessage         EventType = "message"
	EventTypeMessageReceived EventType = "message:received"
	EventTypeMessageSent     EventType = "message:sent"
)

// EventContext contains context information for an event
type EventContext struct {
	// Command events
	SessionEntry   interface{} // SessionEntry from storage
	SessionID      string
	SessionFile    string
	CommandSource  string // whatsapp, telegram, discord
	SenderID       string
	WorkspaceDir   string
	BootstrapFiles []string

	// Message events
	From          string
	To            string
	Content       string
	ChannelID     string
	AccountID     string
	ConversationID string
	MessageID     string
	Success       bool
	Error         string

	// Additional metadata
	Metadata map[string]interface{}
}

// HookEvent represents an event that triggers hooks
type HookEvent struct {
	Type       EventType
	Action     string
	SessionKey string
	Timestamp  time.Time
	Messages   []string // Messages to push to the user
	Context    EventContext
}

// NewHookEvent creates a new hook event
func NewHookEvent(eventType EventType, action, sessionKey string) *HookEvent {
	return &HookEvent{
		Type:       eventType,
		Action:     action,
		SessionKey: sessionKey,
		Timestamp:  time.Now(),
		Messages:   make([]string, 0),
		Context:    EventContext{},
	}
}

// String returns the string representation of EventType
func (e EventType) String() string {
	return string(e)
}

// ParseEventType parses a string into EventType
func ParseEventType(s string) EventType {
	switch strings.ToLower(s) {
	case "command":
		return EventTypeCommand
	case "command:new":
		return EventTypeCommandNew
	case "command:reset":
		return EventTypeCommandReset
	case "command:stop":
		return EventTypeCommandStop
	case "agent:bootstrap":
		return EventTypeAgentBootstrap
	case "gateway:startup":
		return EventTypeGatewayStartup
	case "message":
		return EventTypeMessage
	case "message:received":
		return EventTypeMessageReceived
	case "message:sent":
		return EventTypeMessageSent
	default:
		return EventType(s)
	}
}

// Priority levels for hooks (lower = higher priority)
type Priority int

const (
	PriorityHigh Priority = iota
	PriorityNormal
	PriorityLow
)

// Hook represents a single hook
type Hook struct {
	Name        string
	Description string
	Emoji       string
	Events      []EventType
	Handler     HookHandler
	Enabled     bool
	Priority    Priority
	Config      map[string]interface{}
	Requires    *HookRequirements
	OS          []string // Supported operating systems
	Always      bool     // Bypass eligibility checks
}

// HookRequirements defines what a hook needs to run
type HookRequirements struct {
	Bins       []string // Required binaries
	AnyBins    []string // At least one of these must exist
	Env        []string // Required environment variables
	Config     []string // Required config keys
}

// HookHandler is the interface for hook handlers
type HookHandler interface {
	Handle(event *HookEvent) error
}

// HookHandlerFunc is a function type that implements HookHandler
type HookHandlerFunc func(event *HookEvent) error

// Handle implements HookHandler interface
func (f HookHandlerFunc) Handle(event *HookEvent) error {
	return f(event)
}

// HookRegistry manages hook registration and event dispatching
type HookRegistry struct {
	mu      sync.RWMutex
	hooks   map[EventType][]*Hook
	enabled bool
}

// NewHookRegistry creates a new hook registry
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks:   make(map[EventType][]*Hook),
		enabled: true,
	}
}

// Register registers a hook for one or more event types
func (r *HookRegistry) Register(hook *Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, eventType := range hook.Events {
		r.hooks[eventType] = append(r.hooks[eventType], hook)
	}

	// Also register for generic events
	for _, eventType := range hook.Events {
		parts := strings.Split(string(eventType), ":")
		if len(parts) > 1 {
			// Register for parent event type too
			parent := EventType(parts[0])
			r.hooks[parent] = append(r.hooks[parent], hook)
		}
	}
}

// Unregister removes a hook by name
func (r *HookRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for eventType, hooks := range r.hooks {
		newHooks := make([]*Hook, 0)
		for _, h := range hooks {
			if h.Name != name {
				newHooks = append(newHooks, h)
			}
		}
		r.hooks[eventType] = newHooks
	}
}

// GetHooks returns all hooks for a specific event type
func (r *HookRegistry) GetHooks(eventType EventType) []*Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Hook
	hooks, ok := r.hooks[eventType]
	if ok {
		result = append(result, hooks...)
	}

	// Also get parent event hooks
	parts := strings.Split(string(eventType), ":")
	if len(parts) > 1 {
		parent := EventType(parts[0])
		if parentHooks, ok := r.hooks[parent]; ok {
			result = append(result, parentHooks...)
		}
	}

	return result
}

// SetEnabled enables or disables all hooks
func (r *HookRegistry) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// IsEnabled returns whether hooks are enabled
func (r *HookRegistry) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// Dispatch dispatches an event to all registered hooks
func (r *HookRegistry) Dispatch(event *HookEvent) {
	if !r.IsEnabled() {
		return
	}

	hooks := r.GetHooks(event.Type)

	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}

		// Run hook asynchronously to avoid blocking
		go func(h *Hook) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[Hooks] Hook %s panic: %v\n", h.Name, r)
				}
			}()

			if err := h.Handler.Handle(event); err != nil {
				fmt.Printf("[Hooks] Hook %s error: %v\n", h.Name, err)
			}
		}(hook)
	}
}

// List returns all registered hooks
func (r *HookRegistry) List() []*Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var result []*Hook

	for _, hooks := range r.hooks {
		for _, h := range hooks {
			if !seen[h.Name] {
				seen[h.Name] = true
				result = append(result, h)
			}
		}
	}

	return result
}

// Enable enables a hook by name
func (r *HookRegistry) Enable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, hooks := range r.hooks {
		for _, h := range hooks {
			if h.Name == name {
				h.Enabled = true
				return true
			}
		}
	}
	return false
}

// Disable disables a hook by name
func (r *HookRegistry) Disable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, hooks := range r.hooks {
		for _, h := range hooks {
			if h.Name == name {
				h.Enabled = false
				return true
			}
		}
	}
	return false
}
