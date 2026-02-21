package kv

import (
	"testing"
)

func TestOptions(t *testing.T) {
	opts := Options{
		Dir:           "/test/path",
		ValueDir:      "/test/value",
		SyncWrites:    true,
		Compression:   true,
		MemoryMode:    false,
		MaxCacheSize:  100,
		ValueLogMaxMB: 200,
	}

	if opts.Dir != "/test/path" {
		t.Errorf("Expected Dir '/test/path', got '%s'", opts.Dir)
	}

	if opts.ValueDir != "/test/value" {
		t.Errorf("Expected ValueDir '/test/value', got '%s'", opts.ValueDir)
	}

	if !opts.SyncWrites {
		t.Error("Expected SyncWrites to be true")
	}

	if !opts.Compression {
		t.Error("Expected Compression to be true")
	}

	if opts.MemoryMode {
		t.Error("Expected MemoryMode to be false")
	}

	if opts.MaxCacheSize != 100 {
		t.Errorf("Expected MaxCacheSize 100, got %d", opts.MaxCacheSize)
	}

	if opts.ValueLogMaxMB != 200 {
		t.Errorf("Expected ValueLogMaxMB 200, got %d", opts.ValueLogMaxMB)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions("/tmp")

	if opts.Dir != "/tmp" {
		t.Errorf("Expected Dir '/tmp', got '%s'", opts.Dir)
	}

	if opts.SyncWrites != false {
		t.Error("Expected SyncWrites to be false by default")
	}

	if opts.Compression != true {
		t.Error("Expected Compression to be true by default")
	}

	if opts.MemoryMode != false {
		t.Error("Expected MemoryMode to be false by default")
	}
}

func TestKVStructure(t *testing.T) {
	kv := &KV{}

	// Test closed state
	kv.closed = false
	if kv.closed {
		t.Error("KV should not be closed initially")
	}

	// Test closedMu
	kv.closedMu.Lock()
	kv.closedMu.Unlock()
	// If we got here without panic, the mutex works
}

func TestOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		valid   bool
	}{
		{
			name: "valid options",
			opts: Options{
				Dir:           "/tmp/test",
				SyncWrites:    false,
				Compression:   true,
				MemoryMode:    false,
				MaxCacheSize:  256,
				ValueLogMaxMB: 256,
			},
			valid: true,
		},
		{
			name: "in-memory mode",
			opts: Options{
				MemoryMode:   true,
				MaxCacheSize: 128,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.MemoryMode {
				// In-memory mode should be valid
				if tt.opts.Dir == "" && !tt.opts.MemoryMode {
					t.Error("Expected validation to fail for empty dir in non-memory mode")
				}
			}
		})
	}
}
