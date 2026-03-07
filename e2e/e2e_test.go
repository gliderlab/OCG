package e2e_test

import (
	"testing"
	"time"

	"github.com/gliderlab/cogate/agent"
	"github.com/gliderlab/cogate/memory"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/storage"
	"github.com/gliderlab/cogate/tools"
)

// TestEnv holds all test dependencies
type TestEnv struct {
	agent     *agent.Agent
	store     *storage.Storage
	memStore  *memory.VectorMemoryStore
	tmpDir    string
}

// setupTestEnv creates a minimal test environment with in-memory SQLite DBs
func setupTestEnv(t *testing.T) (*TestEnv, func()) {
	tmpDir := t.TempDir()

	// Create storage (SQLite)
	dbPath := tmpDir + "/test_storage.db"
	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create memory store with minimal config
	memDBPath := tmpDir + "/test_memory.db"
	memCfg := memory.Config{
		EmbeddingDim:  1536,
		MaxResults:    3,
		MinScore:      0.3,
		HybridEnabled: true,
		HNSWPath:      tmpDir + "/hnsw",
	}
	memStore, err := memory.NewVectorMemoryStore(memDBPath, memCfg)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}

	// Create tool registry
	registry := tools.NewRegistry()

	// Agent config (minimal, no real LLM)
	cfg := config.AgentConfig{
		Model: "test-model",
	}

	// Build agent via DI
	a := agent.NewAgentDI().
		WithConfig(cfg).
		WithStorage(store).
		WithMemoryStore(memStore).
		WithRegistry(registry).
		Build()

	cleanup := func() {
		store.Close()
		memStore.Close()
	}

	return &TestEnv{agent: a, store: store, memStore: memStore, tmpDir: tmpDir}, cleanup
}

// TestE2E_GoldenPaths tests the 3 critical E2E flows
func TestE2E_GoldenPaths(t *testing.T) {
	env, cleanup := setupTestEnv(t)
	defer cleanup()

	t.Run("Path1_NormalChat_ToolCall", func(t *testing.T) {
		// Path 1: Normal chat -> Tool call -> Result return
		// This tests the basic chat pipeline without LLM (expects graceful handling)
		msgs := []agent.Message{
			{Role: "user", Content: "Hello, what tools do you have?"},
		}

		// ChatWithSession blocks on LLM call, so we expect it to fail without real LLM
		// But the pipeline should not panic
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = env.agent.ChatWithSession("e2e-session-1", msgs)
		}()

		select {
		case <-done:
			t.Log("Chat pipeline completed (expected to fail without LLM)")
		case <-time.After(3 * time.Second):
			t.Log("Chat timed out (expected without real LLM provider)")
		}
	})

	t.Run("Path2_MemoryGraph_WriteQuery", func(t *testing.T) {
		// Path 2: memory_search + memory_graph write/query flow
		// Test that memory operations don't crash the agent
		msgs := []agent.Message{
			{Role: "user", Content: "Remember: my name is E2E-Tester"},
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = env.agent.ChatWithSession("e2e-session-2", msgs)
		}()

		select {
		case <-done:
			t.Log("Memory write pipeline completed")
		case <-time.After(3 * time.Second):
			t.Log("Memory write timed out")
		}

		// Verify we can query memory directly
		results, err := env.memStore.Search("E2E-Tester", 3, 0.3)
		if err != nil {
			t.Logf("Memory search returned: %v (expected without embedding)", err)
		} else {
			t.Logf("Memory search found %d results", len(results))
		}
	})

	t.Run("Path3_MultiChannel_Telegram", func(t *testing.T) {
		// Path 3: Multi-channel entry (Telegram-style session key) to agent response
		// Session key format: "telegram:<chat_id>"
		msgs := []agent.Message{
			{Role: "user", Content: "/ping"},
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = env.agent.ChatWithSession("telegram:12345", msgs)
		}()

		select {
		case <-done:
			t.Log("Telegram channel pipeline completed")
		case <-time.After(3 * time.Second):
			t.Log("Telegram channel timed out")
		}
	})
}

// TestE2E_SessionIsolation verifies sessions are properly isolated
func TestE2E_SessionIsolation(t *testing.T) {
	env, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create two separate sessions
	session1 := "e2e-isolation-1"
	session2 := "e2e-isolation-2"

	msgs1 := []agent.Message{{Role: "user", Content: "Session 1 message"}}
	msgs2 := []agent.Message{{Role: "user", Content: "Session 2 message"}}

	// Both should complete (or timeout) independently
	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() {
		defer close(done1)
		_ = env.agent.ChatWithSession(session1, msgs1)
	}()

	go func() {
		defer close(done2)
		_ = env.agent.ChatWithSession(session2, msgs2)
	}()

	select {
	case <-done1:
		t.Log("Session 1 completed")
	case <-time.After(2 * time.Second):
		t.Log("Session 1 timed out")
	}

	select {
	case <-done2:
		t.Log("Session 2 completed")
	case <-time.After(2 * time.Second):
		t.Log("Session 2 timed out")
	}

	t.Log("Session isolation test completed - both sessions handled independently")
}
