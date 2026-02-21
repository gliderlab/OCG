// Tools module - tool invocation framework
package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gliderlab/cogate/pkg/config"
)

// Tool defines the tool interface
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(args map[string]interface{}) (interface{}, error)
}

// ToolsPolicy holds tool allow/deny policy
type ToolsPolicy struct {
	Profile       string   // "minimal", "coding", "messaging", "full"
	Allow         []string // Tool names or groups to allow
	Deny          []string // Tool names or groups to deny
	WorkspaceOnly bool     // Restrict file ops to workspace
}

// Registry holds registered tools
type Registry struct {
	tools  map[string]Tool
	policy *ToolsPolicy
}

// Tool groups (matching official OCG)
var ToolGroups = map[string][]string{
	"group:runtime":    {"exec", "process"},
	"group:fs":         {"read", "write", "edit", "apply_patch"},
	"group:sessions":   {"sessions_list", "sessions_history", "sessions_send", "sessions_spawn", "session_status"},
	"group:memory":     {"memory_search", "memory_get"},
	"group:web":        {"web_search", "web_fetch"},
	"group:ui":         {"browser", "canvas"},
	"group:automation": {"cron", "gateway"},
	"group:messaging":  {"message"},
	"group:nodes":      {"nodes"},
}

func NewRegistry() *Registry {
	return &Registry{
		tools:  make(map[string]Tool),
		policy: DefaultToolsPolicy(),
	}
}

// NewRegistryWithPolicy creates a registry with custom policy
func NewRegistryWithPolicy(policy *ToolsPolicy) *Registry {
	if policy == nil {
		policy = DefaultToolsPolicy()
	}
	return &Registry{
		tools:  make(map[string]Tool),
		policy: policy,
	}
}

// DefaultToolsPolicy returns default policy (full access)
func DefaultToolsPolicy() *ToolsPolicy {
	return &ToolsPolicy{
		Profile:       "full",
		Allow:         nil, // nil means all
		Deny:          nil,
		WorkspaceOnly: false,
	}
}

// SetPolicy updates the tools policy
func (r *Registry) SetPolicy(policy *ToolsPolicy) {
	r.policy = policy
}

// IsToolAllowed checks if a tool is allowed by policy
func (r *Registry) IsToolAllowed(toolName string) bool {
	// Expand tool name if it's a group
	toolNames := []string{toolName}
	if strings.HasPrefix(toolName, "group:") {
		if group, ok := ToolGroups[toolName]; ok {
			toolNames = group
		}
	}

	for _, name := range toolNames {
		// Check deny first
		if r.policy != nil && len(r.policy.Deny) > 0 {
			for _, denied := range r.policy.Deny {
				if denied == "*" || denied == name || denied == toolName {
					// Check if it's in allow list
					allowedByDeny := false
					if len(r.policy.Allow) > 0 {
						for _, allowItem := range r.policy.Allow {
							if allowItem == "*" || allowItem == name || allowItem == toolName {
								allowedByDeny = true
								break
							}
						}
					}
					if !allowedByDeny {
						return false
					}
				}
			}
		}

		// Check allow list
		if r.policy != nil && len(r.policy.Allow) > 0 {
			allowedByAllow := false
			for _, allowItem := range r.policy.Allow {
				if allowItem == "*" || allowItem == name || allowItem == toolName {
					allowedByAllow = true
					break
				}
			}
			if !allowedByAllow {
				return false
			}
		}
	}

	return true
}

// GetAllowedTools returns list of tools filtered by policy
func (r *Registry) GetAllowedTools() []string {
	var allowed []string
	for name := range r.tools {
		if r.IsToolAllowed(name) {
			allowed = append(allowed, name)
		}
	}
	return allowed
}

// Register a tool
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	log.Printf("[OK] tool registered: %s", t.Name())
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List all tools
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// CallTool and return its result
func (r *Registry) CallTool(name string, args map[string]interface{}) (interface{}, error) {
	// Check policy first
	if !r.IsToolAllowed(name) {
		return nil, fmt.Errorf("tool not allowed by policy: %s", name)
	}

	t, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	log.Printf("[TOOL] calling tool: %s, args: %v", name, args)
	result, err := t.Execute(args)
	if err != nil {
		log.Printf("[ERROR] tool failed: %s - %v", name, err)
		return nil, err
	}

	log.Printf("[OK] tool succeeded: %s", name)
	return result, nil
}

// GetToolSpecs returns OpenAI-format specs with function wrapper (filtered by policy)
func (r *Registry) GetToolSpecs() []map[string]interface{} {
	specs := make([]map[string]interface{}, 0)
	for _, t := range r.tools {
		// Only include tools allowed by policy
		if !r.IsToolAllowed(t.Name()) {
			continue
		}
		specs = append(specs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  t.Parameters(),
			},
		})
	}
	return specs
}

// ParseToolCalls parse OpenAI tool_calls response
func ParseToolCalls(response map[string]interface{}) []map[string]interface{} {
	toolCalls, ok := response["choices"].([]map[string]interface{})
	if !ok || len(toolCalls) == 0 {
		return nil
	}

	message, ok := toolCalls[0]["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	calls, ok := message["tool_calls"].([]map[string]interface{})
	if !ok {
		return nil
	}

	return calls
}

// FormatToolResult formats tool result as a message
func FormatToolResult(toolName string, result interface{}) map[string]interface{} {
	var content string
	switch v := result.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	default:
		b, _ := json.Marshal(v)
		content = string(b)
	}

	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": fmt.Sprintf("call_%s", toolName),
		"content":      content,
	}
}

// ErrorResult returns an error payload
func ErrorResult(toolName string, err error) map[string]interface{} {
	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": fmt.Sprintf("call_%s", toolName),
		"content":      fmt.Sprintf("error: %v", err),
	}
}

// ParseArgs parses JSON args
func ParseArgs(argsJSON string) (map[string]interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		// Try as array
		var arr []interface{}
		if jerr := json.Unmarshal([]byte(argsJSON), &arr); jerr == nil {
			return map[string]interface{}{"args": arr}, nil
		}
		return nil, fmt.Errorf("failed to parse args: %v", err)
	}
	return args, nil
}

// GetString gets a string arg
func GetString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt gets an int arg
func GetInt(args map[string]interface{}, key string) int {
	if v, ok := args[key]; ok {
		switch f := v.(type) {
		case float64:
			return int(f)
		case int:
			return f
		case string:
			var i int
			fmt.Sscanf(f, "%d", &i)
			return i
		}
	}
	return 0
}

// GetFloat64 gets a float arg
func GetFloat64(args map[string]interface{}, key string) float64 {
	if v, ok := args[key]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case int:
			return float64(f)
		case string:
			var x float64
			fmt.Sscanf(f, "%f", &x)
			return x
		}
	}
	return 0
}

// GetBool gets a bool arg
func GetBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Truncate long text
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...\n(content truncated)"
}

// Summarize text (for AI responses)
func Summarize(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	lines := strings.Split(s, "\n")
	if len(lines) > 10 {
		// keep first 5 and last 5 lines
		return strings.Join(append(lines[:5], lines[len(lines)-5:]...), "\n") + "\n...(middle omitted)"
	}

	return s
}

// AllowedDirs defines the directories where file operations are allowed
var AllowedDirs = []string{
	config.DefaultDataDir(),
	config.DefaultInstallDir(),
	os.TempDir(),
}

// GetAllowedDirs returns the list of allowed directories, including OS-specific paths
func GetAllowedDirs() []string {
	dirs := make([]string, len(AllowedDirs))
	copy(dirs, AllowedDirs)

	// Add OS-specific paths
	if runtime.GOOS == "windows" {
		// Add Windows user profile
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			dirs = append(dirs, userProfile)
		}
		// Add Windows temp dirs
		if temp := os.Getenv("TEMP"); temp != "" {
			dirs = append(dirs, temp)
		}
		if tmp := os.Getenv("TMP"); tmp != "" {
			dirs = append(dirs, tmp)
		}
	}

	return dirs
}

// IsPathAllowed checks if a path is within allowed directories (jail mechanism)
// Returns the resolved path if allowed, or empty string + error if not
func IsPathAllowed(path string) (string, error) {
	// Resolve symlinks to prevent bypass
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If symlink resolution fails (e.g. target file for write doesn't exist yet),
		// keep original path and try to canonicalize parent directory below.
		resolvedPath = path
	}

	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	// Resolve any ".." components
	absPath = filepath.Clean(absPath)

	// Build candidate absolute paths to handle OS temp symlink differences
	// (e.g. macOS /var -> /private/var for non-existing files).
	candidatePaths := []string{absPath}
	if parent := filepath.Dir(absPath); parent != "" {
		if resolvedParent, perr := filepath.EvalSymlinks(parent); perr == nil {
			candidatePaths = append(candidatePaths, filepath.Clean(filepath.Join(resolvedParent, filepath.Base(absPath))))
		}
	}

	// Build list of allowed dirs (include Windows equivalents)
	allowed := GetAllowedDirs()
	// Add Windows temp dir if on Windows
	if runtime.GOOS == "windows" {
		allowed = append(allowed, os.Getenv("TEMP"))
		allowed = append(allowed, os.Getenv("TMP"))
		// Add user profile
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			allowed = append(allowed, userProfile)
		}
	}

	for _, dir := range allowed {
		if dir == "" {
			continue
		}
		// Resolve symlinks for allowed dirs too
		cleanDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			cleanDir = dir
		}
		cleanDir = filepath.Clean(cleanDir)

		for _, p := range candidatePaths {
			// Check if path is inside allowed dir
			rel, err := filepath.Rel(cleanDir, p)
			if err != nil {
				continue
			}
			// Ensure we didn't escape (rel starts with .. would mean we're outside)
			sep := string(os.PathSeparator)
			if rel != ".." && !strings.HasPrefix(rel, ".."+sep) {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("path not allowed: %s is outside allowed directories", absPath)
}
