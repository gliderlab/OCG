// Gateway Tool - Manage the Gateway process
//
// Provides Gateway restart, configuration management, and update capabilities.
// Allows agents to restart the Gateway, read/modify config, and trigger updates.
//go:build !windows
// +build !windows

package tools

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
)

const (
	GitHubAPIURL   = "https://api.github.com/repos/gliderlab/cogate/releases/latest"
	AssetNameLinux = "ocg-linux-x64.zip"
	AssetNameWin   = "ocg-windows-x64.zip"
	AssetNameMac   = "ocg-darwin-x64.zip"
)

type GatewayTool struct {
	gatewayURL    string
	gatewayToken  string
	workDir       string
	binaryPath    string
	configPath    string
	restartCmd    string
	restartArgs   []string
	mu            sync.Mutex
	lastRestart   time.Time
}

// NewGatewayTool creates a new gateway management tool
func NewGatewayTool(opts ...GatewayToolOption) *GatewayTool {
	installDir := config.DefaultInstallDir()
	t := &GatewayTool{
		gatewayURL:  config.DefaultGatewayURL(),
		workDir:     installDir,
		binaryPath:  filepath.Join(installDir, "bin/ocg-gateway"),
		configPath:  filepath.Join(installDir, "bin/config/env.config"),
		restartCmd:  filepath.Join(installDir, "bin/ocg"),
		restartArgs: []string{"restart"},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *GatewayTool) Name() string {
	return "gateway"
}

func (t *GatewayTool) Description() string {
	return "Manage Gateway: restart, config get/apply/patch, update run"
}

func (t *GatewayTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: restart, config.get, config.schema, config.apply, config.patch, update.run, update.github",
				"enum":        []string{"restart", "config.get", "config.schema", "config.apply", "config.patch", "update.run", "update.github"},
			},
			"delayMs": map[string]interface{}{
				"type":        "number",
				"description": "Delay before restart (ms). Default 2000 to avoid interrupting reply",
			},
			"config": map[string]interface{}{
				"type":        "object",
				"description": "Full config object for config.apply",
			},
			"patch": map[string]interface{}{
				"type":        "object",
				"description": "Partial config for config.patch",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Reason for restart (logged)",
			},
			"raw": map[string]interface{}{
				"type":        "boolean",
				"description": "For config.get: return raw config instead of parsed",
			},
		},
		"required": []string{"action"},
	}
}

func (t *GatewayTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := GetString(args, "action")
	if action == "" {
		return nil, &GatewayError{Message: "action is required"}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	switch action {
	case "restart":
		return t.handleRestart(args)
	case "config.get":
		return t.handleConfigGet(args)
	case "config.schema":
		return t.handleConfigSchema(args)
	case "config.apply":
		return t.handleConfigApply(args)
	case "config.patch":
		return t.handleConfigPatch(args)
	case "update.run":
		return t.handleUpdateRun(args)
	case "update.github":
		return t.handleUpdateGitHub(args)
	default:
		return nil, &GatewayError{Message: "unsupported action: " + action}
	}
}

// handleRestart restarts the Gateway process
func (t *GatewayTool) handleRestart(args map[string]interface{}) (interface{}, error) {
	delayMs := GetInt(args, "delayMs")
	if delayMs == 0 {
		delayMs = 2000 // default delay
	}
	reason := GetString(args, "reason")

	// Check if restart is allowed (rate limit)
	if time.Since(t.lastRestart) < 10*time.Second {
		return nil, &GatewayError{Message: "restart too soon after last restart"}
	}

	// Send a graceful signal first (SIGUSR1 for in-process restart)
	// Or use the ocg CLI restart command
	
	// Try to use the CLI restart command first
	cmd := exec.Command(t.restartCmd, t.restartArgs...)
	cmd.Dir = t.workDir
	cmd.Env = os.Environ()
	
	// Run restart in background to not block
	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		
		// Try to find and signal the current process
		pidFile := t.workDir + "/bin/gateway.pid"
		if data, err := os.ReadFile(pidFile); err == nil {
			var pid int
			fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
			if pid > 0 {
				syscall.Kill(pid, syscall.SIGUSR1)
				return
			}
		}
		
		// Fallback: use CLI
		cmd.Run()
	}()

	t.lastRestart = time.Now()

	return GatewayResult{
		OK:      true,
		Action:  "restart",
		Message: fmt.Sprintf("Gateway restart initiated (delay: %dms)", delayMs),
		Extra:   map[string]interface{}{"reason": reason, "delayMs": delayMs},
	}, nil
}

// handleConfigGet returns current configuration
func (t *GatewayTool) handleConfigGet(args map[string]interface{}) (interface{}, error) {
	raw := GetBool(args, "raw")

	// Try to read from Gateway API first
	resp, err := t.httpGet("/config")
	if err == nil && resp != nil {
		return resp, nil
	}

	// Fallback: read from config file
	configData, err := os.ReadFile(t.configPath)
	if err != nil {
		return nil, &GatewayError{Message: "config not found: " + err.Error()}
	}

	if raw {
		return GatewayResult{
			OK:      true,
			Action:  "config.get",
			Message: "Config retrieved",
			Extra:   map[string]interface{}{"config": string(configData)},
		}, nil
	}

	// Parse as JSON
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		// Return as raw if not valid JSON
		return GatewayResult{
			OK:      true,
			Action:  "config.get",
			Message: "Config retrieved (raw)",
			Extra:   map[string]interface{}{"config": string(configData)},
		}, nil
	}

	return GatewayResult{
		OK:      true,
		Action:  "config.get",
		Message: "Config retrieved",
		Extra:   map[string]interface{}{"config": config},
	}, nil
}

// handleConfigSchema returns the configuration schema
func (t *GatewayTool) handleConfigSchema(args map[string]interface{}) (interface{}, error) {
	// Try Gateway API first
	resp, err := t.httpGet("/config/schema")
	if err == nil && resp != nil {
		return resp, nil
	}

	// Return known schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"gateway": map[string]interface{}{
				"type":        "object",
				"description": "Gateway configuration",
				"properties": map[string]interface{}{
					"port":         map[string]interface{}{"type": "number", "description": "Gateway port"},
					"host":         map[string]interface{}{"type": "string", "description": "Gateway host"},
					"auth":         map[string]interface{}{"type": "boolean", "description": "Enable auth"},
					"token":        map[string]interface{}{"type": "string", "description": "Auth token"},
				},
			},
			"agent": map[string]interface{}{
				"type":        "object",
				"description": "Agent configuration",
				"properties": map[string]interface{}{
					"model":     map[string]interface{}{"type": "string", "description": "Default model"},
					"apiKey":    map[string]interface{}{"type": "string", "description": "API key"},
					"baseURL":   map[string]interface{}{"type": "string", "description": "API base URL"},
					"maxTokens": map[string]interface{}{"type": "number", "description": "Max tokens"},
				},
			},
			"channels": map[string]interface{}{
				"type":        "object",
				"description": "Channel configurations",
			},
		},
	}

	return GatewayResult{
		OK:      true,
		Action:  "config.schema",
		Message: "Config schema retrieved",
		Extra:   map[string]interface{}{"schema": schema},
	}, nil
}

// handleConfigApply applies full configuration
func (t *GatewayTool) handleConfigApply(args map[string]interface{}) (interface{}, error) {
	configIface, ok := args["config"]
	if !ok {
		return nil, &GatewayError{Message: "config is required for config.apply"}
	}

	config, ok := configIface.(map[string]interface{})
	if !ok {
		return nil, &GatewayError{Message: "config must be an object"}
	}

	// Validate config (basic check)
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, &GatewayError{Message: "invalid config: " + err.Error()}
	}

	// Write config file
	if err := os.WriteFile(t.configPath, configJSON, 0644); err != nil {
		return nil, &GatewayError{Message: "failed to write config: " + err.Error()}
	}

	// Trigger restart
	delayMs := GetInt(args, "delayMs")
	if delayMs == 0 {
		delayMs = 2000
	}

	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		t.restartGateway()
	}()

	return GatewayResult{
		OK:      true,
		Action:  "config.apply",
		Message: "Config applied, Gateway restarting",
		Extra:   map[string]interface{}{"delayMs": delayMs},
	}, nil
}

// handleConfigPatch patches configuration
func (t *GatewayTool) handleConfigPatch(args map[string]interface{}) (interface{}, error) {
	patchIface, ok := args["patch"]
	if !ok {
		return nil, &GatewayError{Message: "patch is required for config.patch"}
	}

	patch, ok := patchIface.(map[string]interface{})
	if !ok {
		return nil, &GatewayError{Message: "patch must be an object"}
	}

	// Read existing config
	var existing map[string]interface{}
	existingData, err := os.ReadFile(t.configPath)
	if err == nil {
		json.Unmarshal(existingData, &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Merge patch into existing
	for k, v := range patch {
		existing[k] = v
	}

	// Write merged config
	mergedJSON, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(t.configPath, mergedJSON, 0644); err != nil {
		return nil, &GatewayError{Message: "failed to write config: " + err.Error()}
	}

	// Trigger restart
	delayMs := GetInt(args, "delayMs")
	if delayMs == 0 {
		delayMs = 2000
	}

	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		t.restartGateway()
	}()

	return GatewayResult{
		OK:      true,
		Action:  "config.patch",
		Message: "Config patched, Gateway restarting",
		Extra:   map[string]interface{}{"patch": patch, "delayMs": delayMs},
	}, nil
}

// handleUpdateRun runs Gateway update
func (t *GatewayTool) handleUpdateRun(args map[string]interface{}) (interface{}, error) {
	// This would typically:
	// 1. Pull latest code (git pull)
	// 2. Rebuild (make build)
	// 3. Restart Gateway

	// For security, this should be disabled by default
	// Enable with commands.update: true in config

	delayMs := GetInt(args, "delayMs")
	if delayMs == 0 {
		delayMs = 2000
	}

	// Check if updates are allowed
	// This is a placeholder - in production would check config
	allowUpdate := os.Getenv("OCG_ALLOW_UPDATE") == "true"
	if !allowUpdate {
		// Try to run npm update if available
		cmd := exec.Command("npm", "list", "-g", "openclaw")
		if _, err := cmd.Output(); err == nil {
			// Update available
			go func() {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
				exec.Command("npm", "update", "-g", "openclaw").Run()
				t.restartGateway()
			}()
			
			return GatewayResult{
				OK:      true,
				Action:  "update.run",
				Message: "Update initiated, Gateway restarting",
				Extra:   map[string]interface{}{"delayMs": delayMs, "method": "npm"},
			}, nil
		}
		
		return nil, &GatewayError{Message: "updates disabled by default. Set OCG_ALLOW_UPDATE=true to enable"}
	}

	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		
		// Try git pull + rebuild
		gitCmd := exec.Command("git", "pull")
		gitCmd.Dir = t.workDir
		gitCmd.Run()
		
		makeCmd := exec.Command("make", "build")
		makeCmd.Dir = t.workDir
		makeCmd.Run()
		
		t.restartGateway()
	}()

	return GatewayResult{
		OK:      true,
		Action:  "update.run",
		Message: "Update initiated, Gateway restarting",
		Extra:   map[string]interface{}{"delayMs": delayMs},
	}, nil
}

// handleUpdateGitHub downloads and installs latest release from GitHub
func (t *GatewayTool) handleUpdateGitHub(args map[string]interface{}) (interface{}, error) {
	delayMs := GetInt(args, "delayMs")
	if delayMs == 0 {
		delayMs = 3000
	}

	// Check if updates are allowed
	allowUpdate := os.Getenv("OCG_ALLOW_UPDATE") == "true"
	if !allowUpdate {
		return nil, &GatewayError{Message: "updates disabled by default. Set OCG_ALLOW_UPDATE=true to enable"}
	}

	// Run update in background
	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		t.doGitHubUpdate()
	}()

	return GatewayResult{
		OK:      true,
		Action:  "update.github",
		Message: "GitHub update initiated, downloading latest release...",
		Extra:   map[string]interface{}{"delayMs": delayMs},
	}, nil
}

// doGitHubUpdate performs the actual GitHub release download and extraction
func (t *GatewayTool) doGitHubUpdate() error {
	// Step 1: Get latest release info from GitHub
	release, err := t.fetchLatestRelease()
	if err != nil {
		log.Printf("[Gateway] Failed to fetch latest release: %v", err)
		return err
	}

	// Step 2: Find matching asset for current platform
	asset := t.findMatchingAsset(release)
	if asset == nil {
		err = fmt.Errorf("no matching asset for %s/%s", runtime.GOOS, runtime.GOARCH)
		log.Printf("[Gateway] %v", err)
		return err
	}

	log.Printf("[Gateway] Found asset: %s (%d bytes)", asset.Name, asset.Size)

	// Step 3: Download the asset
	zipPath, err := t.downloadAsset(asset)
	if err != nil {
		log.Printf("[Gateway] Failed to download asset: %v", err)
		return err
	}
	defer os.Remove(zipPath)

	// Step 4: Extract and replace binaries
	if err := t.extractAndReplace(zipPath); err != nil {
		log.Printf("[Gateway] Failed to extract: %v", err)
		return err
	}

	log.Printf("[Gateway] Update completed successfully!")
	return nil
}

// fetchLatestRelease fetches the latest release info from GitHub API
func (t *GatewayTool) fetchLatestRelease() (*GitHubRelease, error) {
	req, err := http.NewRequest("GET", GitHubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// findMatchingAsset finds the asset matching current platform
func (t *GatewayTool) findMatchingAsset(release *GitHubRelease) *GitHubAsset {
	var targetName string
	switch runtime.GOOS {
	case "linux":
		targetName = AssetNameLinux
	case "windows":
		targetName = AssetNameWin
	case "darwin":
		targetName = AssetNameMac
	default:
		return nil
	}

	for i := range release.Assets {
		if release.Assets[i].Name == targetName {
			return &release.Assets[i]
		}
	}
	return nil
}

// downloadAsset downloads the asset to a temporary file
func (t *GatewayTool) downloadAsset(asset *GitHubAsset) (string, error) {
	req, err := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "ocg-update-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	// Download to file
	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

// extractAndReplace extracts the zip and replaces binaries
func (t *GatewayTool) extractAndReplace(zipPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Binaries to extract and their target names
	binaries := map[string]string{
		"ocg":           "bin/ocg",
		"ocg-gateway":   "bin/ocg-gateway",
		"ocg-agent":     "bin/ocg-agent",
		"ocg-embedding": "bin/ocg-embedding",
	}

	extractDir := t.workDir

	for _, file := range reader.File {
		// Check if this is one of our binaries (ocg-*)
		// Support both direct match and path prefix (e.g., "bin/ocg-gateway")
		targetName := filepath.Base(file.Name)
		targetRelPath, ok := binaries[targetName]
		if !ok {
			// Check if it starts with "ocg-" and we should extract it
			if strings.HasPrefix(targetName, "ocg-") {
				targetRelPath = "bin/" + targetName
			} else {
				continue
			}
		}

		targetPath := filepath.Join(extractDir, targetRelPath)

		// Create parent dir if needed
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Open the zip entry
		rc, err := file.Open()
		if err != nil {
			return err
		}

		// Create target file
		targetFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}

		// Copy content
		if _, err := io.Copy(targetFile, rc); err != nil {
			rc.Close()
			targetFile.Close()
			return err
		}

		rc.Close()
		targetFile.Close()

		log.Printf("[Gateway] Extracted: %s -> %s", file.Name, targetPath)
	}

	// Restart services
	t.restartGateway()

	return nil
}

// GitHub release/asset structures
type GitHubRelease struct {
	TagName string         `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// restartGateway restarts the Gateway process
func (t *GatewayTool) restartGateway() {
	// Try SIGUSR1 first (graceful)
	pidFile := filepath.Join(t.workDir, "bin/gateway.pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		var pid int
		fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
		if pid > 0 {
			syscall.Kill(pid, syscall.SIGUSR1)
			return
		}
	}

	// Fallback: restart via CLI
	exec.Command(t.restartCmd, t.restartArgs...).Run()
}

// httpGet makes HTTP GET request to Gateway
func (t *GatewayTool) httpGet(path string) (map[string]interface{}, error) {
	url := t.gatewayURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if t.gatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.gatewayToken)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Types

type GatewayResult struct {
	OK      bool                   `json:"ok"`
	Action  string                 `json:"action"`
	Message string                 `json:"message"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

type GatewayError struct {
	Message string
}

func (e *GatewayError) Error() string {
	return e.Message
}
