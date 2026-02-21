// Package plugin provides plugin utilities for the tool adapter
package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/gliderlab/cogate/tools/adapter"
)

// PluginLoader loads plugins from shared libraries (.so files)
type PluginLoader struct {
	adapter    *adapter.ToolAdapter
	pluginDir  string
	symbolName string
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader(adapter *adapter.ToolAdapter, pluginDir string) *PluginLoader {
	return &PluginLoader{
		adapter:    adapter,
		pluginDir:  pluginDir,
		symbolName: "ToolPlugin",
	}
}

// LoadPlugin loads a plugin from a .so file
func (l *PluginLoader) LoadPlugin(filePath string) error {
	// Check file extension
	if filepath.Ext(filePath) != ".so" {
		return fmt.Errorf("plugin file must be .so: %s", filePath)
	}

	// Load the shared library
	p, err := plugin.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Lookup the plugin symbol
	sym, err := p.Lookup(l.symbolName)
	if err != nil {
		return fmt.Errorf("symbol %s not found: %w", l.symbolName, err)
	}

	// Type assert to the plugin interface
	pluginInst, ok := sym.(adapter.PluginLoader)
	if !ok {
		return fmt.Errorf("symbol %s is not a PluginLoader", l.symbolName)
	}

	// Get plugin info
	info := pluginInst.PluginInfo()
	log.Printf("[PKG] loading plugin: %s v%s from %s", info.Name, info.Version, filePath)

	// Initialize the plugin
	if err := pluginInst.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Register with the adapter
	if err := l.adapter.RegisterPlugin(info.Name, pluginInst); err != nil {
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	log.Printf("[OK] plugin loaded: %s", info.Name)
	return nil
}

// LoadAllPlugins loads all plugins from the plugin directory
func (l *PluginLoader) LoadAllPlugins() error {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[DIR] plugin directory not found: %s", l.pluginDir)
			return nil
		}
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) == ".so" {
			filePath := filepath.Join(l.pluginDir, entry.Name())
			if err := l.LoadPlugin(filePath); err != nil {
				log.Printf("[WARN] failed to load plugin %s: %v", entry.Name(), err)
				continue
			}
		}
	}

	return nil
}

// UnloadPlugin unloads a plugin by name
func (l *PluginLoader) UnloadPlugin(name string) error {
	return l.adapter.UnregisterPlugin(name)
}

// ReloadPlugin reloads a plugin (unload + load)
func (l *PluginLoader) ReloadPlugin(name string, filePath string) error {
	if err := l.adapter.UnregisterPlugin(name); err != nil {
		return err
	}
	return l.LoadPlugin(filePath)
}

// JSONPluginLoader loads plugins from JSON configuration files
type JSONPluginLoader struct {
	adapter       *adapter.ToolAdapter
	configDir     string
	externalProcs map[string]*exec.Cmd // Track external plugin processes
}

// NewJSONPluginLoader creates a new JSON plugin loader
func NewJSONPluginLoader(adapter *adapter.ToolAdapter, configDir string) *JSONPluginLoader {
	return &JSONPluginLoader{
		adapter:       adapter,
		configDir:     configDir,
		externalProcs: make(map[string]*exec.Cmd),
	}
}

// PluginConfig represents a plugin configuration file
type PluginConfig struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Tags        []string               `json:"tags,omitempty"`
	Type        string                 `json:"type"` // "builtin", "external", "wasm"
	Builtin     *BuiltinConfig         `json:"builtin,omitempty"`
	External    *ExternalConfig        `json:"external,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

type BuiltinConfig struct {
	Module string `json:"module"` // e.g., "tools.read"
	Symbol string `json:"symbol"` // e.g., "ReadTool"
}

type ExternalConfig struct {
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"`
	Stdin     bool     `json:"stdin"`
	Transport string   `json:"transport"` // "stdio", "websocket", "http"
	Endpoint  string   `json:"endpoint,omitempty"`
}

// LoadPlugin loads a plugin from a JSON config file
func (l *JSONPluginLoader) LoadPlugin(filePath string) error {
	// Read the config file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config PluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	log.Printf("[PKG] loading JSON plugin: %s v%s", config.Name, config.Version)

	switch config.Type {
	case "builtin":
		return l.loadBuiltinPlugin(config)
	case "external":
		return l.loadExternalPlugin(config)
	default:
		return fmt.Errorf("unknown plugin type: %s", config.Type)
	}
}

func (l *JSONPluginLoader) loadBuiltinPlugin(config PluginConfig) error {
	// Try to load the builtin module
	switch config.Builtin.Module {
	case "tools.read":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &ReadPluginWrapper{}
		})
	case "tools.write":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &WritePluginWrapper{}
		})
	case "tools.exec":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &ExecPluginWrapper{}
		})
	case "tools.memory":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &MemoryPluginWrapper{}
		})
	case "tools.process":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &ProcessPluginWrapper{}
		})
	case "tools.web":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &WebPluginWrapper{}
		})
	case "tools.telegram":
		return l.loadAsAdapterPlugin(config, func() adapter.PluginLoader {
			return &TelegramPluginWrapper{}
		})
	default:
		return fmt.Errorf("unknown builtin module: %s", config.Builtin.Module)
	}
}

func (l *JSONPluginLoader) loadAsAdapterPlugin(config PluginConfig, factory func() adapter.PluginLoader) error {
	pluginInst := factory()
	if err := pluginInst.Initialize(config.Config); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	return l.adapter.RegisterPlugin(config.Name, pluginInst)
}

func (l *JSONPluginLoader) loadExternalPlugin(config PluginConfig) error {
	// External plugins run as separate processes
	externalCfg := config.External
	log.Printf("[PKG] external plugin %s: %s %v (transport: %s)", config.Name, externalCfg.Command, externalCfg.Args, externalCfg.Transport)

	// Determine transport mechanism
	switch externalCfg.Transport {
	case "stdio":
		return l.loadExternalPluginStdio(config)
	case "http":
		return l.loadExternalPluginHTTP(config)
	default:
		// Default to stdio
		return l.loadExternalPluginStdio(config)
	}
}

// loadExternalPluginStdio loads an external plugin using stdin/stdout communication
func (l *JSONPluginLoader) loadExternalPluginStdio(config PluginConfig) error {
	externalCfg := config.External

	// Prepare command
	cmd := externalCfg.Command
	if cmd == "" {
		return fmt.Errorf("external command is required")
	}

	// Build args
	args := externalCfg.Args
	if args == nil {
		args = []string{}
	}

	// Prepare environment
	env := append(os.Environ(), externalCfg.Env...)

	// Create process
	proc := l.externalProcs[config.Name]
	if proc != nil {
		log.Printf("[WARN] plugin %s already running, reusing existing process", config.Name)
		// Reuse existing plugin
		return l.adapter.RegisterPlugin(config.Name, &externalPluginAdapter{
			name:   config.Name,
			config: externalCfg,
			proc:   proc,
		})
	}

	// Start new process
	process := exec.Command(cmd, args...)
	process.Env = env
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr

	if err := process.Start(); err != nil {
		return fmt.Errorf("failed to start external plugin: %w", err)
	}

	// Track the process
	l.externalProcs[config.Name] = process
	log.Printf("[OK] external plugin started: %s (pid: %d)", config.Name, process.Process.Pid)

	// Register the plugin adapter
	return l.adapter.RegisterPlugin(config.Name, &externalPluginAdapter{
		name:   config.Name,
		config: externalCfg,
		proc:   process,
	})
}

// loadExternalPluginHTTP loads an external plugin using HTTP communication
func (l *JSONPluginLoader) loadExternalPluginHTTP(config PluginConfig) error {
	externalCfg := config.External

	if externalCfg.Endpoint == "" {
		return fmt.Errorf("http transport requires endpoint URL")
	}

	// Register HTTP-based plugin adapter
	return l.adapter.RegisterPlugin(config.Name, &externalHTTPPluginAdapter{
		name:     config.Name,
		endpoint: externalCfg.Endpoint,
	})
}

// externalPluginAdapter implements adapter.PluginLoader for external stdio plugins
type externalPluginAdapter struct {
	name   string
	config *ExternalConfig
	proc   *exec.Cmd
}

// PluginInfo returns plugin information
func (a *externalPluginAdapter) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        a.name,
		Version:     "external",
		Description: "External plugin via stdio",
	}
}

// Initialize initializes the plugin
func (a *externalPluginAdapter) Initialize(config map[string]interface{}) error {
	return nil
}

// Execute runs the plugin (for stdio, this sends to stdin)
func (a *externalPluginAdapter) Execute(args map[string]interface{}) (interface{}, error) {
	// For stdio plugins, we could send JSON via stdin and read from stdout
	// This is a simplified implementation
	return nil, fmt.Errorf("external stdio plugin Execute not implemented")
}

// Shutdown stops the plugin process
func (a *externalPluginAdapter) Shutdown() error {
	if a.proc != nil && a.proc.Process != nil {
		return a.proc.Process.Kill()
	}
	return nil
}

// HealthCheck checks if the plugin is still running
func (a *externalPluginAdapter) HealthCheck() error {
	if a.proc != nil && a.proc.Process != nil {
		err := a.proc.Process.Signal(syscall.Signal(0))
		if err != nil {
			return fmt.Errorf("plugin process not running: %w", err)
		}
		return nil
	}
	return fmt.Errorf("no process reference")
}

// externalHTTPPluginAdapter implements adapter.PluginLoader for external HTTP plugins
type externalHTTPPluginAdapter struct {
	name     string
	endpoint string
	client   *http.Client
}

// PluginInfo returns plugin information
func (a *externalHTTPPluginAdapter) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        a.name,
		Version:     "external",
		Description: "External plugin via HTTP: " + a.endpoint,
	}
}

// Initialize initializes the plugin
func (a *externalHTTPPluginAdapter) Initialize(config map[string]interface{}) error {
	a.client = &http.Client{Timeout: 30 * time.Second}
	return nil
}

// Execute runs the plugin via HTTP
func (a *externalHTTPPluginAdapter) Execute(args map[string]interface{}) (interface{}, error) {
	if a.client == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}

	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := a.client.Post(a.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plugin returned status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result["result"], nil
}

// Shutdown cleans up the plugin
func (a *externalHTTPPluginAdapter) Shutdown() error {
	a.client = nil
	return nil
}

// HealthCheck checks if the plugin is reachable
func (a *externalHTTPPluginAdapter) HealthCheck() error {
	if a.client == nil {
		return fmt.Errorf("plugin not initialized")
	}
	resp, err := a.client.Get(a.endpoint + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned: %d", resp.StatusCode)
	}
	return nil
}
func (a *externalHTTPPluginAdapter) Invoke(method string, args map[string]interface{}) (interface{}, error) {
	if a.client == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}

	// Build request
	reqBody := map[string]interface{}{
		"method": method,
		"args":   args,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := a.client.Post(a.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plugin returned status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result["result"], nil
}

// LoadAllPlugins loads all JSON plugins from the config directory
func (l *JSONPluginLoader) LoadAllPlugins() error {
	entries, err := os.ReadDir(l.configDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[DIR] plugin config directory not found: %s", l.configDir)
			return nil
		}
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(l.configDir, entry.Name())
			if err := l.LoadPlugin(filePath); err != nil {
				log.Printf("[WARN] failed to load plugin %s: %v", entry.Name(), err)
				continue
			}
		}
	}

	return nil
}

// Plugin wrappers for built-in tools
type ReadPluginWrapper struct{}

func (p *ReadPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "read",
		Version:     "1.0.0",
		Description: "Read file contents with 50KB limit",
		Author:      "OCG-Go",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to read",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (p *ReadPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *ReadPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	path := ""
	if v, ok := args["path"].(string); ok {
		path = v
	}
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	// The actual read logic is in tools/read.go
	// This is a wrapper that delegates
	return map[string]interface{}{"path": path, "status": "mock"}, nil
}

func (p *ReadPluginWrapper) Shutdown() error { return nil }

func (p *ReadPluginWrapper) HealthCheck() error { return nil }

// WritePluginWrapper
type WritePluginWrapper struct{}

func (p *WritePluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "write",
		Version:     "1.0.0",
		Description: "Write content to a file",
	}
}

func (p *WritePluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *WritePluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "write_mock"}, nil
}

func (p *WritePluginWrapper) Shutdown() error { return nil }

func (p *WritePluginWrapper) HealthCheck() error { return nil }

// ExecPluginWrapper
type ExecPluginWrapper struct{}

func (p *ExecPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "exec",
		Version:     "1.0.0",
		Description: "Execute shell commands",
	}
}

func (p *ExecPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *ExecPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "exec_mock"}, nil
}

func (p *ExecPluginWrapper) Shutdown() error { return nil }

func (p *ExecPluginWrapper) HealthCheck() error { return nil }

// MemoryPluginWrapper
type MemoryPluginWrapper struct{}

func (p *MemoryPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "memory",
		Version:     "1.0.0",
		Description: "Vector memory storage and retrieval",
	}
}

func (p *MemoryPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *MemoryPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "memory_mock"}, nil
}

func (p *MemoryPluginWrapper) Shutdown() error { return nil }

func (p *MemoryPluginWrapper) HealthCheck() error { return nil }

// ProcessPluginWrapper
type ProcessPluginWrapper struct{}

func (p *ProcessPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "process",
		Version:     "1.0.0",
		Description: "Manage background processes",
	}
}

func (p *ProcessPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *ProcessPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "process_mock"}, nil
}

func (p *ProcessPluginWrapper) Shutdown() error { return nil }

func (p *ProcessPluginWrapper) HealthCheck() error { return nil }

// WebPluginWrapper
type WebPluginWrapper struct{}

func (p *WebPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "web",
		Version:     "1.0.0",
		Description: "Web search and fetch",
	}
}

func (p *WebPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *WebPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "web_mock"}, nil
}

func (p *WebPluginWrapper) Shutdown() error { return nil }

func (p *WebPluginWrapper) HealthCheck() error { return nil }

// TelegramPluginWrapper
type TelegramPluginWrapper struct{}

func (p *TelegramPluginWrapper) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "telegram",
		Version:     "1.0.0",
		Description: "Telegram bot integration",
	}
}

func (p *TelegramPluginWrapper) Initialize(config map[string]interface{}) error { return nil }

func (p *TelegramPluginWrapper) Execute(args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "telegram_mock"}, nil
}

func (p *TelegramPluginWrapper) Shutdown() error { return nil }

func (p *TelegramPluginWrapper) HealthCheck() error { return nil }

// CreatePluginManifest generates a manifest file for a plugin
func CreatePluginManifest(name, version, description, author string, tags []string) PluginConfig {
	return PluginConfig{
		Name:        name,
		Version:     version,
		Description: description,
		Author:      author,
		Tags:        tags,
	}
}

// ExportPlugin exports a plugin to a JSON manifest
func ExportPlugin(plugin adapter.PluginLoader, outputPath string) error {
	info := plugin.PluginInfo()
	config := CreatePluginManifest(info.Name, info.Version, info.Description, info.Author, info.Tags)
	config.Type = "builtin"

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}

// ListAvailablePlugins lists all available plugins in the directories
func ListAvailablePlugins(pluginDir, configDir string) ([]string, error) {
	var plugins []string

	// Check .so files
	if entries, err := os.ReadDir(pluginDir); err == nil {
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".so" {
				plugins = append(plugins, filepath.Join(pluginDir, entry.Name()))
			}
		}
	}

	// Check .json configs
	if entries, err := os.ReadDir(configDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				plugins = append(plugins, filepath.Join(configDir, entry.Name()))
			}
		}
	}

	return plugins, nil
}

// ValidatePluginConfig validates a plugin configuration
func ValidatePluginConfig(config PluginConfig) error {
	if config.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if config.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if config.Type == "" {
		return fmt.Errorf("plugin type is required")
	}
	if config.Type == "external" && config.External.Command == "" {
		return fmt.Errorf("external plugin command is required")
	}
	return nil
}

// AutoDiscoverAndLoad auto-discovers and loads all plugins
func AutoDiscoverAndLoad(adapter *adapter.ToolAdapter, pluginDir, configDir string) error {
	loader := NewPluginLoader(adapter, pluginDir)
	jsonLoader := NewJSONPluginLoader(adapter, configDir)

	// Load .so plugins
	if err := loader.LoadAllPlugins(); err != nil {
		log.Printf("[WARN] .so plugin loading error: %v", err)
	}

	// Load JSON plugins
	if err := jsonLoader.LoadAllPlugins(); err != nil {
		log.Printf("[WARN] JSON plugin loading error: %v", err)
	}

	return nil
}

// Helper to create plugin from function
func MakePluginFromFunc(name, version, description string, fn interface{}) adapter.PluginLoader {
	typ := reflect.TypeOf(fn)
	if typ == nil || typ.Kind() != reflect.Func {
		return &invalidPlugin{err: fmt.Errorf("plugin %s must be a function", name)}
	}

	return &funcPlugin{
		name:        name,
		version:     version,
		description: description,
		fn:          fn,
	}
}

type funcPlugin struct {
	name        string
	version     string
	description string
	fn          interface{}
}

type invalidPlugin struct {
	err error
}

func (p *funcPlugin) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        p.name,
		Version:     p.version,
		Description: p.description,
	}
}

func (p *funcPlugin) Initialize(config map[string]interface{}) error { return nil }

func (p *funcPlugin) Execute(args map[string]interface{}) (interface{}, error) {
	rv := reflect.ValueOf(p.fn)
	results := rv.Call([]reflect.Value{reflect.ValueOf(args)})
	if len(results) > 0 {
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
	}
	return nil, nil
}

func (p *funcPlugin) Shutdown() error { return nil }

func (p *funcPlugin) HealthCheck() error { return nil }

func (p *invalidPlugin) PluginInfo() adapter.PluginInfo {
	return adapter.PluginInfo{
		Name:        "invalid",
		Version:     "0.0.0",
		Description: p.err.Error(),
	}
}

func (p *invalidPlugin) Initialize(config map[string]interface{}) error { return p.err }

func (p *invalidPlugin) Execute(args map[string]interface{}) (interface{}, error) {
	return nil, p.err
}

func (p *invalidPlugin) Shutdown() error { return p.err }

func (p *invalidPlugin) HealthCheck() error { return p.err }
