package storage

import (
	"testing"
	"time"
)

func TestStorageMessage(t *testing.T) {
	// Test message structure
	msg := Message{
		ID:         1,
		SessionKey: "session-1",
		Role:       "user",
		Content:    "Hello",
		CreatedAt:  time.Now(),
	}
	
	if msg.ID != 1 {
		t.Errorf("Expected ID 1, got %d", msg.ID)
	}
	
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}
	
	if msg.Content != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", msg.Content)
	}
}

func TestStorageMemory(t *testing.T) {
	memory := Memory{
		ID:         1,
		Key:        "test-key",
		Text:       "Test memory",
		Category:   "fact",
		Importance: 0.8,
		CreatedAt:  time.Now(),
	}
	
	if memory.ID != 1 {
		t.Errorf("Expected ID 1, got %d", memory.ID)
	}
	
	if memory.Category != "fact" {
		t.Errorf("Expected category 'fact', got '%s'", memory.Category)
	}
}

func TestStorageEvent(t *testing.T) {
	event := Event{
		ID:       1,
		Title:    "test-event",
		Content:  "Test content",
		Priority: PriorityNormal,
		Status:   "pending",
	}
	
	if event.ID != 1 {
		t.Errorf("Expected ID 1, got %d", event.ID)
	}
	
	if event.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", event.Status)
	}
	
	if event.Priority != PriorityNormal {
		t.Errorf("Expected PriorityNormal, got %d", event.Priority)
	}
}

func TestStorageFileRecord(t *testing.T) {
	record := FileRecord{
		ID:        1,
		Path:      "/test/file.txt",
		Content:   "test content",
		MimeType:  "text/plain",
		CreatedAt: time.Now(),
	}
	
	if record.ID != 1 {
		t.Errorf("Expected ID 1, got %d", record.ID)
	}
	
	if record.MimeType != "text/plain" {
		t.Errorf("Expected MimeType 'text/plain', got '%s'", record.MimeType)
	}
}

func TestStorageConfig(t *testing.T) {
	cfg := Config{
		ID:        1,
		Section:   "llm",
		Key:       "apiKey",
		Value:     "test-key",
		UpdatedAt: time.Now(),
	}
	
	if cfg.ID != 1 {
		t.Errorf("Expected ID 1, got %d", cfg.ID)
	}
	
	if cfg.Section != "llm" {
		t.Errorf("Expected section 'llm', got '%s'", cfg.Section)
	}
}
