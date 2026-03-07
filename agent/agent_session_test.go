package agent_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gliderlab/cogate/agent"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/storage"
)

// TestAgentSession_TaskOperations tests task storage and retrieval
func TestAgentSession_TaskOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_session.db"

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	a := agent.NewAgentDI().
		WithConfig(config.AgentConfig{Model: "test-model"}).
		WithStorage(store).
		Build()

	session := "test-session-tasks"

	t.Run("StoreUserTask", func(t *testing.T) {
		instructions := "Test task: buy groceries"
		subtasks := []string{"1. Make list", "2. Go to store"}

		result := a.RunTestStoreUserTask(session, instructions, subtasks)

		if result == "" {
			t.Error("StoreUserTask should return confirmation message")
		}

		if !strings.Contains(result, "task") && !strings.Contains(result, "stored") {
			t.Logf("Result: %s", result)
		}
	})

	t.Run("TaskList", func(t *testing.T) {
		// List tasks for the session
		result := a.RunTestTaskList(session, 10)

		if result == "" {
			t.Error("TaskList should return task list or empty message")
		}

		t.Logf("Task list result: %s", result)
	})
}

// TestAgentSession_Compaction tests context compaction logic
func TestAgentSession_Compaction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_compact.db"

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	a := agent.NewAgentDI().
		WithConfig(config.AgentConfig{
			Model:               "test-model",
			CompactionThreshold: 0.5, // Lower threshold for testing
			KeepMessages:        5,
		}).
		WithStorage(store).
		Build()

	t.Run("RunCompact", func(t *testing.T) {
		// Compact with no messages should handle gracefully
		result := a.RunTestCompact("Compact old messages")

		// Should not panic, may return empty or status message
		t.Logf("Compact result: %s", result)
	})
}

// TestAgentSession_SessionLifecycle tests session creation and reset
func TestAgentSession_SessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_lifecycle.db"

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	a := agent.NewAgentDI().
		WithConfig(config.AgentConfig{Model: "test-model"}).
		WithStorage(store).
		Build()

	t.Run("NewSession", func(t *testing.T) {
		result := a.RunTestNewSession()

		if result == "" {
			t.Error("NewSession should return confirmation")
		}

		t.Logf("NewSession: %s", result)
	})

	t.Run("ResetSession", func(t *testing.T) {
		result := a.RunTestResetSession()

		if result == "" {
			t.Error("ResetSession should return confirmation")
		}

		t.Logf("ResetSession: %s", result)
	})
}

// TestAgentSession_TimeFormatting tests Unix millisecond formatting
func TestAgentSession_TimeFormatting(t *testing.T) {
	// Test formatUnixMilli helper
	now := time.Now()
	ms := now.UnixMilli()

	// Format it back (uses "2006-01-02 15:04:05" format)
	formatted := agent.FormatUnixMilliForTest(ms)

	// Parse with the correct format
	layout := "2006-01-02 15:04:05"
	parsed, err := time.ParseInLocation(layout, formatted, time.Local)
	if err != nil {
		t.Errorf("formatUnixMilli produced invalid time: %v (got: %s)", err, formatted)
		return
	}

	// Allow 1 second tolerance (due to millisecond truncation)
	diff := parsed.Sub(now.Truncate(time.Second)).Abs()
	if diff > time.Second {
		t.Errorf("Formatted time differs too much: %v vs %v (diff: %v)", formatted, now, diff)
	}

	t.Logf("Unix milli %d -> %s (parsed: %v)", ms, formatted, parsed)
}
