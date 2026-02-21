package telegram

import (
	"sync"
	"testing"
	"time"
)

// mockAgentRPC implements a minimal mock for testing
type mockAgentRPC struct {
	msgs   []string
	mu     sync.Mutex
}

func (m *mockAgentRPC) Chat(sessionKey string, msg string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return "mock response"
}

func (m *mockAgentRPC) GetSessionKey() string {
	return "test-session"
}

func TestNewTelegramBot(t *testing.T) {
	bot := NewTelegramBot("test-token", nil)

	if bot == nil {
		t.Fatal("NewTelegramBot returned nil")
	}

	if bot.token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", bot.token)
	}

	if bot.pollInterval != 1*time.Second {
		t.Errorf("expected pollInterval 1s, got %v", bot.pollInterval)
	}

	if bot.workerCnt != 8 {
		t.Errorf("expected workerCnt 8, got %d", bot.workerCnt)
	}

	if cap(bot.msgCh) != 100 {
		t.Errorf("expected msgCh capacity 100, got %d", cap(bot.msgCh))
	}
}

func TestTelegramBotWorkerPool(t *testing.T) {
	bot := NewTelegramBot("test-token", nil)
	bot.workerCnt = 2 // Use small pool for test
	bot.msgCh = make(chan TelegramUpdate, 10)

	received := make([]TelegramUpdate, 0)
	var mu sync.Mutex

	// Start workers
	bot.wg.Add(bot.workerCnt)
	for i := 0; i < bot.workerCnt; i++ {
		go func(id int) {
			defer bot.wg.Done()
			for update := range bot.msgCh {
				mu.Lock()
				received = append(received, update)
				mu.Unlock()
			}
		}(i)
	}

	// Send updates
	testUpdates := []TelegramUpdate{
		{UpdateID: 1, Message: TelegramMessage{Text: "hello"}},
		{UpdateID: 2, Message: TelegramMessage{Text: "world"}},
		{UpdateID: 3, Message: TelegramMessage{Text: "test"}},
	}

	for _, u := range testUpdates {
		bot.msgCh <- u
	}

	// Close and wait
	close(bot.msgCh)
	bot.wg.Wait()

	// Verify
	if len(received) != len(testUpdates) {
		t.Errorf("expected %d updates, got %d", len(testUpdates), len(received))
	}
}

func TestTelegramBotBackpressure(t *testing.T) {
	bot := NewTelegramBot("test-token", nil)
	bot.workerCnt = 1
	bot.msgCh = make(chan TelegramUpdate, 2) // Small buffer

	// Fill the queue
	bot.msgCh <- TelegramUpdate{UpdateID: 1}
	bot.msgCh <- TelegramUpdate{UpdateID: 2}

	// This should not block (non-blocking send)
	select {
	case bot.msgCh <- TelegramUpdate{UpdateID: 3}:
		t.Error("expected queue to be full, but send succeeded")
	default:
		// Expected - queue is full
	}
}

func TestTelegramOffsetSync(t *testing.T) {
	bot := NewTelegramBot("test-token", nil)

	// Simulate concurrent offset updates
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bot.muOffset.Lock()
			if id > bot.offset {
				bot.offset = id
			}
			bot.muOffset.Unlock()
		}(i)
	}

	wg.Wait()

	bot.muOffset.Lock()
	defer bot.muOffset.Unlock()
	if bot.offset != 99 {
		t.Errorf("expected offset 99, got %d", bot.offset)
	}
}
