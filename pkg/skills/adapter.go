// Skills adapter - converts skills to OCG tools
package skills

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/tools"
	"github.com/google/shlex"
)

// Adapter converts skills to OCG tools
type Adapter struct {
	registry  *Registry
	execTool  tools.Tool //nolint:unused // kept for future shell skill execution
	mu        sync.RWMutex
	tools     map[string]tools.Tool
}

// NewAdapter creates a new skills adapter
func NewAdapter(reg *Registry) *Adapter {
	return &Adapter{
		registry: reg,
		tools:    make(map[string]tools.Tool),
	}
}

// GenerateTools generates OCG tools from loaded skills
func (a *Adapter) GenerateTools() []tools.Tool {
	var result []tools.Tool
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, skill := range a.registry.List() {
		// Check requirements
		ok, missing := CheckRequirements(skill)
		if !ok && !skill.Always {
			log.Printf("[skills] %s skipped (missing: %v)", skill.Name, missing)
			continue
		}

		tool := a.createSkillTool(skill)
		if tool != nil {
			a.tools[skill.Name] = tool
			result = append(result, tool)
			log.Printf("[skills] Generated tool: %s", skill.Name)
		}
	}

	return result
}

// createSkillTool creates an OCG tool from a skill
func (a *Adapter) createSkillTool(skill *Skill) tools.Tool {
	// Determine skill type and create appropriate tool
	switch {
	case isShellSkill(skill):
		return newShellSkillTool(skill)
	case isCLISkill(skill):
		return newCLISkillTool(skill)
	default:
		return newGenericSkillTool(skill)
	}
}

// isShellSkill checks if skill is primarily shell/curl based
func isShellSkill(skill *Skill) bool {
	content := strings.ToLower(skill.Content)
	return strings.Contains(content, "curl") || 
	       strings.Contains(content, "http") ||
	       strings.Contains(content, "bash")
}

// isCLISkill checks if skill requires a specific CLI tool
func isCLISkill(skill *Skill) bool {
	return len(skill.BinRequires) > 0
}

// shellSkillTool - executes shell commands (curl, etc)
type shellSkillTool struct {
	skill *Skill
}

func newShellSkillTool(skill *Skill) tools.Tool {
	return &shellSkillTool{skill: skill}
}

func (t *shellSkillTool) Name() string {
	return "skill_" + t.skill.Name
}

func (t *shellSkillTool) Description() string {
	return t.skill.Description
}

func (t *shellSkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to execute (analyze skill instructions and generate appropriate command)",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default 30, max 120)",
				"default":     30,
			},
		},
		"required": []string{"command"},
	}
}

func (t *shellSkillTool) Execute(args map[string]interface{}) (interface{}, error) {
	command := tools.GetString(args, "command")
	timeout := tools.GetInt(args, "timeout")
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 120 {
		timeout = 120
	}

	// Sandbox: restrict to bin/work
	workspaceDir := ""
	if exePath, exeErr := os.Executable(); exeErr == nil {
		binDir := filepath.Dir(exePath)
		workspaceDir = filepath.Join(binDir, "work")
	}
	if workspaceDir == "" {
		workspaceDir = config.DefaultWorkspaceDir()
	}
	workdir := workspaceDir

	// Execute the command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workdir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %d seconds", timeout)
	}
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// cliSkillTool - wraps specific CLI tools
type cliSkillTool struct {
	skill *Skill
	bin   string
}

func newCLISkillTool(skill *Skill) tools.Tool {
	bin := ""
	if len(skill.BinRequires) > 0 {
		bin = skill.BinRequires[0]
	}
	return &cliSkillTool{skill: skill, bin: bin}
}

func (t *cliSkillTool) Name() string {
	return "skill_" + t.skill.Name
}

func (t *cliSkillTool) Description() string {
	return t.skill.Description
}

func (t *cliSkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"args": map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("Arguments for %s CLI (see skill instructions)", t.bin),
			},
		},
	}
}

func (t *cliSkillTool) Execute(args map[string]interface{}) (interface{}, error) {
	argsStr := tools.GetString(args, "args")

	// Sandbox: restrict to bin/work
	workspaceDir := ""
	if exePath, exeErr := os.Executable(); exeErr == nil {
		binDir := filepath.Dir(exePath)
		workspaceDir = filepath.Join(binDir, "work")
	}
	if workspaceDir == "" {
		workspaceDir = config.DefaultWorkspaceDir()
	}
	workdir := workspaceDir

	var cmd *exec.Cmd
	if argsStr == "" {
		cmd = exec.Command(t.bin)
	} else {
		parts, err := shlex.Split(argsStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse args: %w", err)
		}
		cmd = exec.Command(t.bin, parts...)
	}
	cmd.Dir = workdir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w", t.bin, err)
	}

	return string(output), nil
}

// genericSkillTool - generic skill that returns instructions
type genericSkillTool struct {
	skill *Skill
}

func newGenericSkillTool(skill *Skill) tools.Tool {
	return &genericSkillTool{skill: skill}
}

func (t *genericSkillTool) Name() string {
	return "skill_" + t.skill.Name
}

func (t *genericSkillTool) Description() string {
	return t.skill.Description + " (see instructions for usage)"
}

func (t *genericSkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform (see skill instructions)",
			},
			"params": map[string]interface{}{
				"type":        "object",
				"description": "Additional parameters",
			},
		},
	}
}

func (t *genericSkillTool) Execute(args map[string]interface{}) (interface{}, error) {
	instructions := ExtractInstructions(t.skill)
	
	action := tools.GetString(args, "action")
	if action != "" {
		return fmt.Sprintf("Skill: %s\nInstructions:\n%s\n\nRequested action: %s", 
			t.skill.Name, instructions, action), nil
	}
	
	return fmt.Sprintf("Skill: %s\n\nInstructions:\n%s", t.skill.Name, instructions), nil
}

// FormatForPrompt formats all available skills for system prompt
func (a *Adapter) FormatForPrompt() string {
	available := a.registry.FilterAvailable()
	if len(available) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Skills\n\n")

	for _, s := range available {
		sb.WriteString(FormatForPrompt(s))
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetTool returns a tool by skill name
func (a *Adapter) GetTool(name string) tools.Tool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tools["skill_"+name]
}

// ListTools returns all generated tools
func (a *Adapter) ListTools() []tools.Tool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := make([]tools.Tool, 0, len(a.tools))
	for _, t := range a.tools {
		result = append(result, t)
	}
	return result
}

// BuiltinSkillsDir returns the path to built-in skills
func BuiltinSkillsDir() string {
	// Check multiple locations
	locations := []string{
		"./skills",
		"../skills",
		"/usr/lib/node_modules/openclaw/skills",
		filepath.Join(os.Getenv("HOME"), ".openclaw", "workspace", "skills"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Default to workspace skills
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw", "workspace", "skills")
}

// IsSkillTool checks if a tool name is a skill tool
func IsSkillTool(name string) bool {
	return strings.HasPrefix(name, "skill_")
}

// ExtractSkillName extracts skill name from tool name
func ExtractSkillName(toolName string) string {
	if strings.HasPrefix(toolName, "skill_") {
		return strings.TrimPrefix(toolName, "skill_")
	}
	return ""
}
