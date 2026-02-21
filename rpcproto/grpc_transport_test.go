package rpcproto

import (
	"testing"
	"time"
)

func TestConvertStats(t *testing.T) {
	input := map[string]int32{
		"tokens_in":  100,
		"tokens_out": 50,
		"requests":   10,
	}

	result := ConvertStats(input)

	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}

	if result["tokens_in"] != 100 {
		t.Errorf("expected tokens_in=100, got %d", result["tokens_in"])
	}

	if result["tokens_out"] != 50 {
		t.Errorf("expected tokens_out=50, got %d", result["tokens_out"])
	}

	if result["requests"] != 10 {
		t.Errorf("expected requests=10, got %d", result["requests"])
	}
}

func TestToMessagesPtr(t *testing.T) {
	input := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you?"},
	}

	result := ToMessagesPtr(input)

	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}

	// Verify pointers
	for i := range input {
		if result[i].Role != input[i].Role {
			t.Errorf("expected role %s at index %d, got %s", input[i].Role, i, result[i].Role)
		}
		if result[i].Content != input[i].Content {
			t.Errorf("expected content %s at index %d, got %s", input[i].Content, i, result[i].Content)
		}
	}

	// Verify they are pointers (modifying result affects input)
	result[0].Content = "modified"
	if input[0].Content != "modified" {
		t.Error("ToMessagesPtr should return pointers to original messages")
	}
}

func TestToMessagesPtrEmpty(t *testing.T) {
	input := []Message{}
	result := ToMessagesPtr(input)

	if len(result) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result))
	}
}

func TestDefaultGRPCTimeout(t *testing.T) {
	timeout := DefaultGRPCTimeout()

	if timeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", timeout)
	}
}
