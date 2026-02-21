// Package bundled provides built-in hooks for OCG
package bundled

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlab/cogate/pkg/hooks"
)

// NewSessionMemoryHook creates the session-memory hook
// Saves session context to memory when /new is issued
func NewSessionMemoryHook() *hooks.Hook {
	return &hooks.Hook{
		Name:        "session-memory",
		Description: "Saves session context to memory when /new is issued",
		Emoji:       "SAVE",
		Events:      []hooks.EventType{hooks.EventTypeCommandNew},
		Enabled:     false,
		Priority:    hooks.PriorityNormal,
		Handler:     sessionMemoryHandler{},
	}
}

// NewCommandLoggerHook creates the command-logger hook
// Logs all command events to a file
func NewCommandLoggerHook() *hooks.Hook {
	return &hooks.Hook{
		Name:        "command-logger",
		Description: "Logs all command events to a file",
		Emoji:       "NOTE",
		Events:      []hooks.EventType{hooks.EventTypeCommand},
		Enabled:     false,
		Priority:    hooks.PriorityLow,
		Handler:     commandLoggerHandler{},
	}
}

// NewBootMDHook creates the boot-md hook
// Runs BOOT.md when gateway starts
func NewBootMDHook() *hooks.Hook {
	return &hooks.Hook{
		Name:        "boot-md",
		Description: "Runs BOOT.md when gateway starts",
		Emoji:       "START",
		Events:      []hooks.EventType{hooks.EventTypeGatewayStartup},
		Enabled:     false,
		Priority:    hooks.PriorityHigh,
		Handler:     bootMDHandler{},
	}
}

// NewBootstrapExtraFilesHook creates the bootstrap-extra-files hook
// Injects additional bootstrap files during agent:bootstrap
func NewBootstrapExtraFilesHook() *hooks.Hook {
	return &hooks.Hook{
		Name:        "bootstrap-extra-files",
		Description: "Injects additional bootstrap files during agent:bootstrap",
		Emoji:       "📎",
		Events:      []hooks.EventType{hooks.EventTypeAgentBootstrap},
		Enabled:     false,
		Priority:    hooks.PriorityHigh,
		Handler:     bootstrapExtraFilesHandler{},
	}
}

// sessionMemoryHandler saves session context to memory
type sessionMemoryHandler struct{}

func (h sessionMemoryHandler) Handle(event *hooks.HookEvent) error {
	workspaceDir := os.Getenv("OCG_WORKSPACE")
	if workspaceDir == "" {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/root"
		}
		workspaceDir = filepath.Join(homeDir, ".openclaw", "workspace")
	}

	if workspaceDir == "" {
		return fmt.Errorf("workspace directory not configured")
	}

	// Create memory directory if it doesn't exist
	memoryDir := filepath.Join(workspaceDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Generate filename
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	slug := generateSlug(event.Context.Content)
	
	filename := fmt.Sprintf("%s-%s.md", dateStr, slug)
	if slug == "" {
		timeStr := now.Format("1504")
		filename = fmt.Sprintf("%s-%s.md", dateStr, timeStr)
	}

	filepath := filepath.Join(memoryDir, filename)

	// Create content
	content := fmt.Sprintf(`# Session: %s

**Session Key**: %s
**Source**: %s

---

`, now.Format("2006-01-02 15:04:05 MST"),
		event.SessionKey,
		event.Context.CommandSource,
	)

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	fmt.Printf("[session-memory] Saved session to %s\n", filepath)
	return nil
}

// commandLoggerHandler logs command events
type commandLoggerHandler struct{}

func (h commandLoggerHandler) Handle(event *hooks.HookEvent) error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/root"
	}

	logDir := filepath.Join(homeDir, ".ocg", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "commands.log")

	// Format: JSONL
	entry := fmt.Sprintf(`{"timestamp":"%s","action":"%s","sessionKey":"%s","senderId":"%s","source":"%s"}`,
		event.Timestamp.Format(time.RFC3339),
		event.Action,
		event.SessionKey,
		event.Context.SenderID,
		event.Context.CommandSource,
	)

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// bootMDHandler runs BOOT.md on gateway startup
type bootMDHandler struct{}

func (h bootMDHandler) Handle(event *hooks.HookEvent) error {
	workspaceDir := os.Getenv("OCG_WORKSPACE")
	if workspaceDir == "" {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/root"
		}
		workspaceDir = filepath.Join(homeDir, ".openclaw", "workspace")
	}

	bootMDPath := filepath.Join(workspaceDir, "BOOT.md")
	content, err := os.ReadFile(bootMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[boot-md] BOOT.md not found, skipping\n")
			return nil
		}
		return fmt.Errorf("failed to read BOOT.md: %w", err)
	}

	fmt.Printf("[boot-md] BOOT.md found, content length: %d bytes\n", len(content))

	// Store BOOT.md content to a file for the agent to read
	// The agent will inject this as a system message on first interaction
	dataDir := filepath.Join(workspaceDir, ".ocg", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	bootCachePath := filepath.Join(dataDir, "boot.md.cache")
	if err := os.WriteFile(bootCachePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to cache BOOT.md: %w", err)
	}

	fmt.Printf("[boot-md] BOOT.md cached to %s\n", bootCachePath)
	return nil
}

// bootstrapExtraFilesHandler injects additional bootstrap files
type bootstrapExtraFilesHandler struct{}

func (h bootstrapExtraFilesHandler) Handle(event *hooks.HookEvent) error {
	// This hook runs during agent:bootstrap and can modify
	// the bootstrap files list in event.Context.BootstrapFiles
	workspaceDir := os.Getenv("OCG_WORKSPACE")
	if workspaceDir == "" {
		return nil
	}

	// Look for additional bootstrap files
	patterns := []string{
		"packages/*/AGENTS.md",
		"packages/*/TOOLS.md",
	}

	for _, pattern := range patterns {
		// Expand pattern and find matching files
		matches, err := filepath.Glob(filepath.Join(workspaceDir, pattern))
		if err != nil {
			continue
		}

		event.Context.BootstrapFiles = append(event.Context.BootstrapFiles, matches...)
	}

	return nil
}

// generateSlug generates a URL-friendly slug from content
func generateSlug(content string) string {
	// Simple slug generation - take first few words
	words := strings.Fields(content)
	if len(words) == 0 {
		return ""
	}

	var slug []string
	for _, word := range words {
		if len(slug) >= 3 {
			break
		}
		// Keep only alphanumeric characters
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			if r >= 'A' && r <= 'Z' {
				return r + 32 // to lowercase
			}
			return '-' // replace special chars with hyphen
		}, word)
		
		clean = strings.Trim(clean, "-")
		if len(clean) > 2 {
			slug = append(slug, clean)
		}
	}

	if len(slug) == 0 {
		return ""
	}

	return strings.Join(slug, "-")
}

// GetAllBundledHooks returns all built-in hooks
func GetAllBundledHooks() []*hooks.Hook {
	return []*hooks.Hook{
		NewSessionMemoryHook(),
		NewCommandLoggerHook(),
		NewBootMDHook(),
		NewBootstrapExtraFilesHook(),
	}
}
