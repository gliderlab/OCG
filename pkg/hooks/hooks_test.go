package hooks

import (
	"testing"
	"time"
)

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected string
	}{
		{EventType("session-start"), "session-start"},
		{EventType("session-end"), "session-end"},
		{EventType("message-received"), "message-received"},
		{EventType("tool-call"), "tool-call"},
	}
	
	for _, tt := range tests {
		if tt.eventType.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.eventType.String())
		}
	}
}

func TestParseEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected EventType
	}{
		{"session-start", EventType("session-start")},
		{"session-end", EventType("session-end")},
		{"unknown", EventType("unknown")},
		{"", ""},
	}
	
	for _, tt := range tests {
		result := ParseEventType(tt.input)
		if result != tt.expected {
			t.Errorf("Input %s: expected %v, got %v", tt.input, tt.expected, result)
		}
	}
}

func TestNewHookEvent(t *testing.T) {
	event := NewHookEvent(EventType("session-start"), "test-action", "default")
	
	if event.Type != EventType("session-start") {
		t.Errorf("Expected Type 'session-start', got %v", event.Type)
	}
	
	if event.Action != "test-action" {
		t.Errorf("Expected Action 'test-action', got '%s'", event.Action)
	}
	
	if event.SessionKey != "default" {
		t.Errorf("Expected SessionKey 'default', got '%s'", event.SessionKey)
	}
}

func TestPriority(t *testing.T) {
	high := PriorityHigh
	normal := PriorityNormal
	low := PriorityLow
	
	if high > normal || normal > low {
		t.Error("Priority order should be High > Normal > Low")
	}
}

func TestHookRegistry(t *testing.T) {
	registry := NewHookRegistry()
	
	// Test initial state
	if !registry.IsEnabled() {
		t.Error("Registry should be enabled by default")
	}
	
	// Register a hook
	hook := &Hook{
		Name:    "test-hook",
		Events:  []EventType{EventType("session-start")},
		Enabled: true,
		Priority: PriorityNormal,
	}
	
	registry.Register(hook)
	
	// Get hooks for event type
	hooks := registry.GetHooks(EventType("session-start"))
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
	
	if hooks[0].Name != "test-hook" {
		t.Errorf("Expected hook name 'test-hook', got '%s'", hooks[0].Name)
	}
}

func TestHookRegistryUnregister(t *testing.T) {
	registry := NewHookRegistry()
	
	// Register a hook
	hook := &Hook{
		Name:   "remove-test",
		Events: []EventType{EventType("test-event")},
	}
	
	registry.Register(hook)
	hooks := registry.GetHooks(EventType("test-event"))
	if len(hooks) != 1 {
		t.Fatal("Hook should be registered")
	}
	
	// Unregister
	registry.Unregister("remove-test")
	hooks = registry.GetHooks(EventType("test-event"))
	if len(hooks) != 0 {
		t.Errorf("Expected 0 hooks after unregister, got %d", len(hooks))
	}
}

func TestHookRegistrySetEnabled(t *testing.T) {
	registry := NewHookRegistry()
	
	// Disable
	registry.SetEnabled(false)
	if registry.IsEnabled() {
		t.Error("Registry should be disabled")
	}
	
	// Enable
	registry.SetEnabled(true)
	if !registry.IsEnabled() {
		t.Error("Registry should be enabled")
	}
}

func TestHookEventTimestamp(t *testing.T) {
	event := &HookEvent{
		Type:       EventType("test"),
		Action:     "test-action",
		SessionKey: "default",
		Timestamp:  time.Now(),
	}
	
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}
