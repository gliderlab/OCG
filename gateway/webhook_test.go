package gateway

import (
	"testing"
)

func TestGenerateRandomIDLength(t *testing.T) {
	tests := []struct {
		length   int
		expected int
	}{
		{0, 0},
		{1, 1},
		{8, 8},
		{16, 16},
		{32, 32},
	}

	for _, tt := range tests {
		result := generateRandomID(tt.length)
		if len(result) != tt.expected {
			t.Errorf("generateRandomID(%d) returned length %d, expected %d", tt.length, len(result), tt.expected)
		}
	}
}

func TestGenerateRandomIDUnique(t *testing.T) {
	// Generate multiple IDs and verify they're different
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRandomID(16)
		if ids[id] {
			t.Errorf("generateRandomID generated duplicate ID: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestGenerateRandomIDChars(t *testing.T) {
	// Verify all characters are from the allowed set
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"

	id := generateRandomID(1000)
	for _, c := range id {
		found := false
		for _, allowed := range chars {
			if c == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("generateRandomID produced invalid character: %c", c)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
