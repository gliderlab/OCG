// Skills loader - parse SKILL.md files
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill
type Skill struct {
	Name        string
	Description string
	Emoji       string
	Metadata    SkillMetadata
	Content     string      // Full SKILL.md content
	Path        string      // Skill directory path
	BinRequires []string    // Required binaries
	EnvRequires []string    // Required environment variables
	OS          []string    // Supported OSes
	Always      bool        // Always load regardless of requirements
}

// InstallItem represents a skill installation method
type InstallItem struct {
	ID       string   `json:"id" yaml:"id"`
	Kind     string   `json:"kind" yaml:"kind"`     // brew, apt, npm, node, pip, etc.
	Formula  string   `json:"formula" yaml:"formula"`  // for brew
	Package  string   `json:"package" yaml:"package"`  // for apt/npm
	Bins     []string `json:"bins" yaml:"bins"`     // required binaries
	Label    string   `json:"label" yaml:"label"`    // human-readable label
	Command  string   `json:"command" yaml:"command"`  // custom install command
	OS       string   `json:"os" yaml:"os"`       // specific OS (linux, darwin, windows)
}

// SkillMetadata parsed from YAML frontmatter
type SkillMetadata struct {
	Requires struct {
		Bins    []string `json:"bins" yaml:"bins"`
		Env     []string `json:"env" yaml:"env"`
		AnyBins []string `json:"anyBins" yaml:"anyBins"`
	} `json:"requires" yaml:"requires"`
	OS      []string     `json:"os" yaml:"os"`
	Emoji   string       `json:"emoji" yaml:"emoji"`
	Always  bool         `json:"always" yaml:"always"`
	Install []InstallItem `json:"install" yaml:"install"`
}

// LoadSkillFromDir loads a skill from a directory containing SKILL.md
func LoadSkillFromDir(dir string) (*Skill, error) {
	skillPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	skill := &Skill{
		Path:    dir,
		Content: string(data),
	}

	// Parse frontmatter
	if err := parseFrontmatter(string(data), skill); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return skill, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown
func parseFrontmatter(content string, skill *Skill) error {
	// Check for frontmatter delimiters
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		// No frontmatter, try to parse name from first heading
		return parseNoFrontmatter(content, skill)
	}

	// Find frontmatter block
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return fmt.Errorf("invalid frontmatter")
	}

	// Extract between --- delimiters
	var frontmatter []string
	inFrontmatter := false
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(line, "---") {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(line, "---") {
			break
		}
		if inFrontmatter {
			frontmatter = append(frontmatter, line)
		}
	}

	if len(frontmatter) == 0 {
		return parseNoFrontmatter(content, skill)
	}

	// Try to parse frontmatter as YAML first
	yamlContent := strings.Join(frontmatter, "\n")
	var meta struct {
		Name        string         `yaml:"name"`
		Description string         `yaml:"description"`
		Emoji       string         `yaml:"emoji"`
		Requires    struct {
			Bins    []string `yaml:"bins"`
			Env     []string `yaml:"env"`
			AnyBins []string `yaml:"anyBins"`
		} `yaml:"requires"`
		OS      []string `yaml:"os"`
		Always  bool     `yaml:"always"`
		Install []struct {
			ID      string   `yaml:"id"`
			Kind    string   `yaml:"kind"`
			Formula string   `yaml:"formula"`
			Package string   `yaml:"package"`
			Bins    []string `yaml:"bins"`
			Label   string   `yaml:"label"`
			OS      string   `yaml:"os"`
		} `yaml:"install"`
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &meta); err == nil {
		if meta.Name != "" {
			skill.Name = meta.Name
		}
		if meta.Description != "" {
			skill.Description = meta.Description
		}
		if meta.Emoji != "" {
			skill.Emoji = meta.Emoji
		}
		skill.BinRequires = meta.Requires.Bins
		skill.EnvRequires = meta.Requires.Env
		skill.OS = meta.OS
		skill.Always = meta.Always

		// Parse install items
		for _, item := range meta.Install {
			skill.Metadata.Install = append(skill.Metadata.Install, InstallItem{
				ID:       item.ID,
				Kind:     item.Kind,
				Formula:  item.Formula,
				Package:  item.Package,
				Bins:     item.Bins,
				Label:    item.Label,
				OS:       item.OS,
			})
		}

		// If we got name from YAML, we're done
		if skill.Name != "" {
			return nil
		}
	}

	// Fallback: parse key-value pairs from frontmatter (simple parser)
	for _, line := range frontmatter {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "key: value" or "key: |" multiline
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = strings.Trim(value, "\"")
		case "emoji":
			skill.Emoji = value
		}
	}

	// Try to parse metadata JSON as fallback
	for _, line := range frontmatter {
		if strings.Contains(line, "metadata") {
			if meta := extractMetadataJSON(line); meta != nil {
				skill.Metadata = *meta
				skill.BinRequires = meta.Requires.Bins
				skill.EnvRequires = meta.Requires.Env
				skill.OS = meta.OS
				skill.Always = meta.Always
			}
		}
	}

	// If no name from frontmatter, try first heading
	if skill.Name == "" {
		return parseNoFrontmatter(content, skill)
	}

	return nil
}

// extractMetadataJSON extracts metadata object from frontmatter line
func extractMetadataJSON(line string) *SkillMetadata {
	// Find metadata: { ... } block - need to handle nested braces
	// Use a more robust approach: find "metadata:" then parse JSON object
	
	re := regexp.MustCompile(`metadata\s*:\s*`)
	loc := re.FindStringIndex(line)
	if loc == nil {
		return nil
	}
	
	// Find the JSON object starting after "metadata:"
	rest := line[loc[1]:]
	
	// Skip whitespace
	rest = strings.TrimSpace(rest)
	
	if !strings.HasPrefix(rest, "{") {
		return nil
	}
	
	// Find matching closing brace by counting braces
	depth := 0
	end := 0
	for i, ch := range rest {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	
	if end == 0 {
		return nil
	}
	
	jsonStr := rest[:end]
	meta := &SkillMetadata{}

	// Simple JSON parsing for known fields
	if strings.Contains(jsonStr, `"bins"`) {
		reBins := regexp.MustCompile(`"bins"\s*:\s*\[([^\]]+)\]`)
		if bins := reBins.FindStringSubmatch(jsonStr); len(bins) > 1 {
			meta.Requires.Bins = parseStringArray(bins[1])
		}
	}
	if strings.Contains(jsonStr, `"env"`) {
		reEnv := regexp.MustCompile(`"env"\s*:\s*\[([^\]]+)\]`)
		if envs := reEnv.FindStringSubmatch(jsonStr); len(envs) > 1 {
			meta.Requires.Env = parseStringArray(envs[1])
		}
	}
	if strings.Contains(jsonStr, `"os"`) {
		reOS := regexp.MustCompile(`"os"\s*:\s*\[([^\]]+)\]`)
		if oses := reOS.FindStringSubmatch(jsonStr); len(oses) > 1 {
			meta.OS = parseStringArray(oses[1])
		}
	}

	// Parse install array
	if strings.Contains(jsonStr, `"install"`) {
		meta.Install = parseInstallArray(jsonStr)
	}

	return meta
}

// parseStringArray parses ["a", "b", "c"] format
func parseStringArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]\"")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseInstallArray parses install: [...] array from metadata
func parseInstallArray(jsonStr string) []InstallItem {
	var items []InstallItem

	// Find install array content
	re := regexp.MustCompile(`"install"\s*:\s*\[([^\]]+)\]`)
	matches := re.FindStringSubmatch(jsonStr)
	if len(matches) < 2 {
		return items
	}

	installContent := matches[1]

	// Split by }{ to get each install item
	// This is a simple parser - handles basic cases
	objects := strings.Split(installContent, "},{")
	for _, obj := range objects {
		obj = strings.Trim(obj, "{} ")
		if obj == "" {
			continue
		}

		item := InstallItem{}

		// Parse ID
		if reID := regexp.MustCompile(`"id"\s*:\s*"([^"]+)"`); reID.MatchString(obj) {
			item.ID = reID.FindStringSubmatch(obj)[1]
		}

		// Parse Kind
		if reKind := regexp.MustCompile(`"kind"\s*:\s*"([^"]+)"`); reKind.MatchString(obj) {
			item.Kind = reKind.FindStringSubmatch(obj)[1]
		}

		// Parse Formula
		if reFormula := regexp.MustCompile(`"formula"\s*:\s*"([^"]+)"`); reFormula.MatchString(obj) {
			item.Formula = reFormula.FindStringSubmatch(obj)[1]
		}

		// Parse Package
		if rePkg := regexp.MustCompile(`"package"\s*:\s*"([^"]+)"`); rePkg.MatchString(obj) {
			item.Package = rePkg.FindStringSubmatch(obj)[1]
		}

		// Parse Label
		if reLabel := regexp.MustCompile(`"label"\s*:\s*"([^"]+)"`); reLabel.MatchString(obj) {
			item.Label = reLabel.FindStringSubmatch(obj)[1]
		}

		// Parse Bins
		if reBins := regexp.MustCompile(`"bins"\s*:\s*\[([^\]]+)\]`); reBins.MatchString(obj) {
			binsMatch := reBins.FindStringSubmatch(obj)
			item.Bins = parseStringArray(binsMatch[1])
		}

		// Parse OS
		if reOS := regexp.MustCompile(`"os"\s*:\s*"([^"]+)"`); reOS.MatchString(obj) {
			item.OS = reOS.FindStringSubmatch(obj)[1]
		}

		// Generate install command based on kind
		item.Command = generateInstallCommand(item)

		if item.ID != "" || item.Kind != "" {
			items = append(items, item)
		}
	}

	return items
}

// generateInstallCommand generates install command based on install item
func generateInstallCommand(item InstallItem) string {
	switch item.Kind {
	case "brew":
		if item.Formula != "" {
			return "brew install " + item.Formula
		}
		return "brew install " + item.Package
	case "apt":
		return "sudo apt install " + item.Package
	case "npm":
		return "npm install -g " + item.Package
	case "node":
		return "npm i -g " + item.Package
	case "pip":
		return "pip install " + item.Package
	case "go":
		return "go install " + item.Package
	default:
		if item.Package != "" {
			return item.Package
		}
		return ""
	}
}

// parseNoFrontmatter handles SKILL.md without YAML frontmatter
func parseNoFrontmatter(content string, skill *Skill) error {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			skill.Name = strings.TrimPrefix(line, "# ")
			skill.Name = strings.ToLower(strings.ReplaceAll(skill.Name, " ", "-"))
			break
		}
	}
	if skill.Description == "" {
		// Try to get first paragraph
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "```") {
				skill.Description = line
				break
			}
		}
	}
	return nil
}

// LoadSkillsFromDir loads all skills from a directory
func LoadSkillsFromDir(dir string) ([]*Skill, error) {
	var skills []*Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name())
		skill, err := LoadSkillFromDir(skillPath)
		if err != nil {
			// Log but continue
			fmt.Printf("[WARN]  Failed to load skill %s: %v\n", entry.Name(), err)
			continue
		}

		if skill.Name == "" {
			skill.Name = entry.Name()
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// ExtractInstructions extracts the markdown body (instructions) from SKILL.md
func ExtractInstructions(skill *Skill) string {
	content := skill.Content

	// Remove frontmatter
	lines := strings.Split(content, "\n")
	var body []string
	inFrontmatter := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				inFrontmatter = false
				continue
			}
		}
		if !inFrontmatter {
			body = append(body, line)
		}
	}

	// Remove first few lines that are just name/description
	result := strings.TrimSpace(strings.Join(body, "\n"))
	return result
}

// FormatForPrompt formats skill for system prompt injection
func FormatForPrompt(skill *Skill) string {
	emoji := skill.Emoji
	if emoji == "" {
		emoji = "TOOL"
	}

	return fmt.Sprintf("### %s **%s**\n%s\n", 
		emoji, skill.Name, skill.Description)
}

// GetInstallInstructions returns install instructions for the skill
func GetInstallInstructions(skill *Skill) []InstallItem {
	return skill.Metadata.Install
}

// FormatInstallInstructions formats install instructions as markdown
func FormatInstallInstructions(skill *Skill) string {
	items := skill.Metadata.Install
	if len(items) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "## Installation")
	
	for _, item := range items {
		if item.Label != "" {
			lines = append(lines, fmt.Sprintf("- **%s**: `%s`", item.Label, item.Command))
		} else if item.Command != "" {
			lines = append(lines, fmt.Sprintf("- `%s`", item.Command))
		}
	}
	
	return strings.Join(lines, "\n")
}
