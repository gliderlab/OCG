// Gateway Tool - Windows implementation
// Provides Gateway restart, configuration management, and update capabilities.
//go:build windows
// +build windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GatewayTool provides Gateway management capabilities
type GatewayTool struct {
	gatewayURL    string
	gatewayToken  string
	workDir       string
	binaryPath    string
	configPath    string
	restartCmd    string
	restartArgs   []string
	lastRestart   time.Time
	minInterval   time.Duration
}

// NewGatewayTool creates a new Gateway tool (Windows version)
// Compatible with Unix version signature
func NewGatewayTool(opts ...GatewayToolOption) *GatewayTool {
	t := &GatewayTool{
		workDir:     "",
		restartCmd:  "taskkill",
		minInterval: 5 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Name returns the tool name
func (t *GatewayTool) Name() string {
	return "gateway"
}

// Description returns the tool description
func (t *GatewayTool) Description() string {
	return "Manage Gateway: restart, config.get, config.apply, config.patch, update.run"
}

// Parameters returns the tool parameters
func (t *GatewayTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: restart, config.get, config.apply, config.patch, update.run",
				"enum":        []string{"restart", "config.get", "config.apply", "config.patch", "update.run"},
			},
			"config": map[string]interface{}{
				"type":        "string",
				"description": "Config JSON for apply/patch actions",
			},
		},
		"required": []string{"action"},
	}
}

// Execute implements ToolExecutor
func (t *GatewayTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := GetString(args, "action")

	switch action {
	case "restart":
		return t.restart()
	case "config.get":
		return t.getConfig()
	case "config.patch":
		return t.patchConfig(args)
	case "config.apply":
		return t.applyConfig(args)
	case "update.run":
		return t.runUpdate()
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *GatewayTool) restart() (interface{}, error) {
	// Check minimum interval
	if time.Since(t.lastRestart) < t.minInterval {
		return nil, fmt.Errorf("restart too frequent, wait %v", t.minInterval)
	}

	// Find and kill gateway process
	killCmd := exec.Command("taskkill", "/F", "/IM", "ocg-gateway.exe")
	if err := killCmd.Run(); err != nil {
		// Process may have already exited
	}

	// Wait a bit then restart
	time.Sleep(500 * time.Millisecond)

	// Start new process
	binPath := filepath.Join(t.workDir, "bin", "ocg-gateway.exe")
	if _, err := os.Stat(binPath); err != nil {
		return nil, fmt.Errorf("gateway binary not found: %w", err)
	}

	startCmd := exec.Command(binPath)
	startCmd.Dir = filepath.Join(t.workDir, "bin")
	startCmd.Start()

	t.lastRestart = time.Now()

	return "Gateway restarted successfully", nil
}

func (t *GatewayTool) getConfig() (interface{}, error) {
	configFile := filepath.Join(t.workDir, "bin", "env.config")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return string(data), nil
}

func (t *GatewayTool) patchConfig(args map[string]interface{}) (interface{}, error) {
	configJSON := GetString(args, "config")
	if configJSON == "" {
		return nil, fmt.Errorf("config JSON required for patch")
	}

	configFile := filepath.Join(t.workDir, "bin", "env.config")
	// Read existing config
	existing, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Simple merge - append new keys
	updated := string(existing) + "\n" + configJSON
	if err := os.WriteFile(configFile, []byte(updated), 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return "Config patched successfully", nil
}

func (t *GatewayTool) applyConfig(args map[string]interface{}) (interface{}, error) {
	configJSON := GetString(args, "config")
	if configJSON == "" {
		return nil, fmt.Errorf("config JSON required for apply")
	}

	configFile := filepath.Join(t.workDir, "bin", "env.config")
	if err := os.WriteFile(configFile, []byte(configJSON), 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return "Config applied successfully", nil
}

func (t *GatewayTool) runUpdate() (interface{}, error) {
	return nil, fmt.Errorf("update.run not implemented on Windows yet")
}
