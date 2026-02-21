// Package hooks provides hook discovery and loading functionality
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookDiscovery handles discovering hooks from directories
type HookDiscovery struct {
	workspaceDir string
	managedDir   string
	bundledDir   string
}

// NewHookDiscovery creates a new hook discovery instance
func NewHookDiscovery(workspaceDir, managedDir, bundledDir string) *HookDiscovery {
	return &HookDiscovery{
		workspaceDir: workspaceDir,
		managedDir:   managedDir,
		bundledDir:   bundledDir,
	}
}

// Discover scans all hook directories and returns discovered hooks
func (d *HookDiscovery) Discover() ([]*Hook, error) {
	var allHooks []*Hook

	// 1. Scan workspace hooks (highest priority)
	if d.workspaceDir != "" {
		hooks, err := d.scanDir(d.workspaceDir)
		if err != nil {
			fmt.Printf("[Hooks] Error scanning workspace hooks: %v\n", err)
		} else {
			allHooks = append(allHooks, hooks...)
			fmt.Printf("[Hooks] Discovered %d workspace hooks\n", len(hooks))
		}
	}

	// 2. Scan managed hooks
	if d.managedDir != "" {
		hooks, err := d.scanDir(d.managedDir)
		if err != nil {
			fmt.Printf("[Hooks] Error scanning managed hooks: %v\n", err)
		} else {
			allHooks = append(allHooks, hooks...)
			fmt.Printf("[Hooks] Discovered %d managed hooks\n", len(hooks))
		}
	}

	// 3. Scan bundled hooks (lowest priority)
	if d.bundledDir != "" {
		hooks, err := d.scanDir(d.bundledDir)
		if err != nil {
			fmt.Printf("[Hooks] Error scanning bundled hooks: %v\n", err)
		} else {
			allHooks = append(allHooks, hooks...)
			fmt.Printf("[Hooks] Discovered %d bundled hooks\n", len(hooks))
		}
	}

	return allHooks, nil
}

// scanDir scans a single directory for hooks
func (d *HookDiscovery) scanDir(dir string) ([]*Hook, error) {
	var hooks []*Hook

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hookPath := filepath.Join(dir, entry.Name())
		hook, err := d.loadHook(hookPath)
		if err != nil {
			fmt.Printf("[Hooks] Failed to load hook %s: %v\n", entry.Name(), err)
			continue
		}

		if hook != nil {
			hooks = append(hooks, hook)
		}
	}

	return hooks, nil
}

// loadHook loads a single hook from a directory
func (d *HookDiscovery) loadHook(hookPath string) (*Hook, error) {
	// Look for HOOK.md
	hookMDPath := filepath.Join(hookPath, "HOOK.md")
	metadata, err := ParseHookMetadata(hookMDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HOOK.md: %w", err)
	}

	// Create the hook
	hook := &Hook{
		Name:        metadata.Name,
		Description: metadata.Description,
		Emoji:       metadata.Emoji,
		Events:      make([]EventType, 0),
		Enabled:     false, // Disabled by default
		Config:      metadata.Config,
		Requires:    metadata.Requires,
		OS:          metadata.OS,
		Always:      metadata.Always,
	}

	// Parse event types
	for _, eventStr := range metadata.Events {
		eventType := ParseEventType(eventStr)
		hook.Events = append(hook.Events, eventType)
	}

	// For now, we create a simple handler that can be extended
	// In the future, this could load actual handler code
	hook.Handler = HookHandlerFunc(func(event *HookEvent) error {
		fmt.Printf("[Hooks] Hook '%s' triggered by event: %s\n", hook.Name, event.Type)
		return nil
	})

	return hook, nil
}

// HookMetadata represents the parsed metadata from HOOK.md
type HookMetadata struct {
	Name        string
	Description string
	Emoji       string
	Homepage    string
	Events      []string
	Config      map[string]interface{}
	Requires    *HookRequirements
	OS          []string
	Always      bool
}

// ParseHookMetadata parses HOOK.md and extracts YAML frontmatter
func ParseHookMetadata(path string) (*HookMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse YAML frontmatter
	metadata := &HookMetadata{
		Config:   make(map[string]interface{}),
		Requires: &HookRequirements{},
	}

	// Simple YAML parsing for frontmatter
	lines := strings.Split(string(content), "\n")
	inFrontmatter := false
	frontmatterLines := []string{}

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				break // End of frontmatter
			}
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	// Parse frontmatter lines
	for _, line := range frontmatterLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key: value pairs
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			metadata.Name = strings.Trim(value, "\"")
		case "description":
			metadata.Description = strings.Trim(value, "\"")
		case "emoji":
			metadata.Emoji = strings.Trim(value, "\"")
		case "homepage":
			metadata.Homepage = strings.Trim(value, "\"")
		case "events":
			// Parse array: ["event1", "event2"]
			metadata.Events = parseStringArray(value)
		case "os":
			metadata.OS = parseStringArray(value)
		}
	}

	// If name is empty, use directory name
	if metadata.Name == "" {
		metadata.Name = filepath.Base(filepath.Dir(path))
	}

	return metadata, nil
}

// parseStringArray parses a YAML string array
func parseStringArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]\"")
	
	if s == "" {
		return nil
	}

	// Handle quoted strings
	var result []string
	var current strings.Builder
	inQuote := false
	
	for _, c := range s {
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if c == ',' && !inQuote {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(c)
	}
	
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	
	return result
}

// DefaultHooksDir returns the default hooks directory
func DefaultHooksDir() string {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/root"
	}
	return filepath.Join(homeDir, ".ocg", "hooks")
}
