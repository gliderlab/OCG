package config

import (
	"os"
	"testing"
)

func TestDefaultGatewayPort(t *testing.T) {
	port := DefaultGatewayPort
	if port != 55003 {
		t.Errorf("Expected 55003, got %d", port)
	}
}

func TestDefaultEmbeddingPortRange(t *testing.T) {
	min := DefaultEmbeddingPortMin
	max := DefaultEmbeddingPortMax
	
	if min != 50000 {
		t.Errorf("Expected min 50000, got %d", min)
	}
	if max != 60000 {
		t.Errorf("Expected max 60000, got %d", max)
	}
}

func TestDefaultLlamaPortRange(t *testing.T) {
	min := DefaultLlamaPortMin
	max := DefaultLlamaPortMax
	
	if min != 18000 {
		t.Errorf("Expected min 18000, got %d", min)
	}
	if max != 19000 {
		t.Errorf("Expected max 19000, got %d", max)
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir := DefaultDataDir()
	if dir == "" {
		t.Error("DefaultDataDir should not be empty")
	}
}

func TestDefaultSocketPath(t *testing.T) {
	path := DefaultSocketPath()
	if path == "" {
		t.Error("DefaultSocketPath should not be empty")
	}
}

func TestDefaultDBPath(t *testing.T) {
	path := DefaultDBPath()
	if path == "" {
		t.Error("DefaultDBPath should not be empty")
	}
}

func TestDefaultGatewayURL(t *testing.T) {
	url := DefaultGatewayURL()
	if url == "" {
		t.Error("DefaultGatewayURL should not be empty")
	}
	
	expected := "http://127.0.0.1:55003"
	if url != expected {
		t.Errorf("Expected '%s', got '%s'", expected, url)
	}
}

func TestDefaultEmbeddingURL(t *testing.T) {
	url := DefaultEmbeddingURL()
	if url == "" {
		t.Error("DefaultEmbeddingURL should not be empty")
	}
}

func TestDefaultCDPPort(t *testing.T) {
	port := DefaultCDPPort
	if port != 18800 {
		t.Errorf("Expected 18800, got %d", port)
	}
}

func TestDefaultIRCPort(t *testing.T) {
	port := DefaultIRCPort
	if port != 6667 {
		t.Errorf("Expected 6667, got %d", port)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set custom environment variables
	os.Setenv("OCG_DATA_DIR", "/tmp/test-ocg")
	defer os.Unsetenv("OCG_DATA_DIR")
	
	dir := DefaultDataDir()
	if dir != "/tmp/test-ocg" {
		t.Errorf("Expected '/tmp/test-ocg', got '%s'", dir)
	}
}

func TestGatewayConfig(t *testing.T) {
	cfg := GatewayConfig{
		Port: 55003,
		Host: "127.0.0.1",
	}
	
	if cfg.Port != 55003 {
		t.Errorf("Expected port 55003, got %d", cfg.Port)
	}
	
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", cfg.Host)
	}
}

func TestAgentConfig(t *testing.T) {
	cfg := AgentConfig{
		Model: "test-model",
	}
	
	if cfg.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", cfg.Model)
	}
}

func TestServerConfig(t *testing.T) {
	cfg := ServerConfig{
		Gateway: &GatewayConfig{Port: 55003},
		Agent:   &AgentConfig{Model: "test"},
	}
	
	if cfg.Gateway == nil {
		t.Fatal("Gateway should not be nil")
	}
	
	if cfg.Agent == nil {
		t.Fatal("Agent should not be nil")
	}
}
