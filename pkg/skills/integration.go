// Skills integration with OCG Agent
package skills

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gliderlab/cogate/tools"
)

// SetupSkills loads skills and registers them with the agent
func SetupSkills(agentRegistry interface {
	Register(tools.Tool)
}) error {
	// Find skills directory
	skillsDir := findSkillsDir()
	if skillsDir == "" {
		log.Printf("[skills] No skills directory found")
		return nil
	}

	log.Printf("[skills] Loading skills from: %s", skillsDir)

	// Create registry and load skills
	registry := NewRegistry(skillsDir)
	if err := registry.Load(); err != nil {
		return err
	}

	// Create adapter and generate tools
	adapter := NewAdapter(registry)
	skillTools := adapter.GenerateTools()

	// Register tools with agent
	for _, tool := range skillTools {
		agentRegistry.Register(tool)
	}

	log.Printf("[skills] Registered %d skill tools", len(skillTools))

	// Print available skills
	available := registry.FilterAvailable()
	if len(available) > 0 {
		log.Printf("[skills] Available skills:")
		for _, s := range available {
			log.Printf("  - %s: %s", s.Name, s.Description)
		}
	}

	return nil
}

// findSkillsDir finds the skills directory
func findSkillsDir() string {
	// Priority order:
	// 1. ./skills (workspace)
	// 2. ~/.openclaw/skills
	// 3. Built-in skills from npm package

	locations := []string{
		"./skills",
		"skills",
		filepath.Join(os.Getenv("HOME"), ".openclaw", "skills"),
		"/usr/lib/node_modules/openclaw/skills",
	}

	for _, loc := range locations {
		absPath, err := filepath.Abs(loc)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	return ""
}

// LoadSkills loads skills from a specific directory
func LoadSkills(skillsDir string) (*Registry, *Adapter, error) {
	registry := NewRegistry(skillsDir)
	if err := registry.Load(); err != nil {
		return nil, nil, err
	}

	adapter := NewAdapter(registry)
	adapter.GenerateTools()

	return registry, adapter, nil
}

// ImportFromOpenClaw imports skills from OCG's bundled skills
func ImportFromOpenClaw(targetDir string) error {
	srcDirs := []string{
		"/usr/lib/node_modules/openclaw/skills",
		filepath.Join(os.Getenv("HOME"), ".npm", "lib", "node_modules", "openclaw", "skills"),
	}

	var srcDir string
	for _, dir := range srcDirs {
		if _, err := os.Stat(dir); err == nil {
			srcDir = dir
			break
		}
	}

	if srcDir == "" {
		return nil // No OCG skills found, that's OK
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Copy skills
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		tgtPath := filepath.Join(targetDir, entry.Name())

		// Skip if already exists
		if _, err := os.Stat(tgtPath); err == nil {
			continue
		}

		// Copy directory
		if err := copyDir(srcPath, tgtPath); err != nil {
			log.Printf("[skills] Failed to copy %s: %v", entry.Name(), err)
			continue
		}

		log.Printf("[skills] Imported: %s", entry.Name())
	}

	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		tgt := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(tgt, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(tgt, data, info.Mode())
	})
}
