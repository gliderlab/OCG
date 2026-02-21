package agent

import (
	"testing"
)

func TestAgentMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}
	
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}
	
	if msg.Content != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", msg.Content)
	}
}

func TestDefaultIDGenerator(t *testing.T) {
	gen := &defaultIDGenerator{}
	
	id1 := gen.New()
	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}
	
	id2 := gen.New()
	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func TestDefaultTimeProvider(t *testing.T) {
	provider := &defaultTimeProvider{}
	
	now := provider.Now()
	if now.IsZero() {
		t.Error("Now should not return zero time")
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := &defaultLogger{}
	
	// Should not panic
	logger.Print("test")
	logger.Printf("test %s", "format")
}

func TestToolLoopDetector(t *testing.T) {
	detector := NewToolLoopDetector(DefaultToolLoopDetectionConfig)
	
	// Record some tool calls
	detector.RecordCall("read", "file=test.txt")
	detector.RecordCall("exec", "cmd=ls")
	
	// Check for loops (should not find any)
	hasLoop, _ := detector.CheckLoop()
	if hasLoop {
		t.Error("Should not detect a loop with only 2 calls")
	}
}

func TestToolLoopDetectorMaxCalls(t *testing.T) {
	detector := NewToolLoopDetector(ToolLoopDetectionConfig{
		MaxCalls:     3,
		TimeWindow:   5 * 60,
		SameToolLimit: 2,
	})
	
	// Record 3+ calls
	detector.RecordCall("read", "{}")
	detector.RecordCall("read", "{}")
	detector.RecordCall("read", "{}")
	
	// Should detect loop
	hasLoop, reason := detector.CheckLoop()
	if !hasLoop {
		t.Error("Should detect loop after max calls exceeded")
	}
	t.Logf("Loop detected: %s", reason)
}

func TestThinkingModeParsing(t *testing.T) {
	// Test thinking mode parsing
	tests := []struct {
		name     string
		mode     string
		expected ThinkingMode
	}{
		{"off", "off", ThinkingModeOff},
		{"on", "on", ThinkingModeOn},
		{"stream", "stream", ThinkingModeStream},
		{"invalid", "invalid", ThinkingModeOff},
		{"empty", "", ThinkingModeOff},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseThinkingMode(tt.mode)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}







func TestStripThinkingTagsSimple(t *testing.T) {
	// Simple test - no tags
	result := StripThinkingTags("Hello world")
	if result != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", result)
	}
}
