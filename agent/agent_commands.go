// agent_commands.go - slash command dispatcher and inline command execution
package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlab/cogate/tools"
)

// runCommandIfRequested executes explicit user command requests via process tool.
func (a *Agent) runCommandIfRequested(msg string) (string, bool) {
	msg = strings.TrimSpace(msg)
	if msg == "" || a.registry == nil {
		return "", false
	}

	// Handle /compact command
	if strings.HasPrefix(msg, "/compact") || strings.HasPrefix(msg, "/compact ") {
		instructions := strings.TrimPrefix(msg, "/compact")
		instructions = strings.TrimSpace(instructions)
		return a.runCompact(instructions), true
	}

	// Handle /new command (start new session)
	if msg == "/new" || msg == "/new " {
		return a.runNewSession(), true
	}

	// Handle /reset command (reset current session)
	if msg == "/reset" || msg == "/reset " {
		return a.runResetSession(), true
	}

	// Handle /split command (explicit task splitting)
	if strings.HasPrefix(msg, "/split ") || msg == "/split" {
		taskMsg := strings.TrimPrefix(msg, "/split ")
		if taskMsg == "/split" || taskMsg == "" {
			return "Usage: /split <task>\nExample: /split summarize today's meeting notes", true
		}
		// Execute task split explicitly
		return a.executeSplitTask(taskMsg), true
	}

	// Resolve task marker(s) quickly: [task_done:task-...]
	if strings.Contains(msg, "[task_done:") {
		re := regexp.MustCompile(`\[task_done:(task-[^\]]+)\]`)
		all := re.FindAllStringSubmatch(msg, -1)
		if len(all) > 0 {
			seen := map[string]bool{}
			parts := make([]string, 0, len(all))
			for _, m := range all {
				if len(m) == 2 {
					id := m[1]
					if seen[id] {
						continue
					}
					seen[id] = true
					parts = append(parts, a.runTaskSummary(id))
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n---\n\n"), true
			}
		}
	}

	// Handle /debug archive
	if strings.HasPrefix(msg, "/debug archive") {
		parts := strings.Fields(msg)
		session := "default"
		if len(parts) >= 3 {
			session = parts[2]
		}
		return a.runArchiveDebug(session), true
	}

	// Handle /debug live
	if strings.HasPrefix(msg, "/debug live") {
		parts := strings.Fields(msg)
		session := ""
		if len(parts) >= 3 {
			session = parts[2]
		}
		return a.runLiveDebug(session), true
	}

	// Handle /task commands
	if strings.HasPrefix(msg, "/task") {
		parts := strings.Fields(msg)
		if len(parts) == 1 {
			return "Usage:\n/task list [limit]\n/task detail <task-id> [page] [pageSize]\n/task summary <task-id>", true
		}
		sub := strings.ToLower(parts[1])
		switch sub {
		case "list":
			limit := 10
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(parts[2]); err == nil && n > 0 && n <= 100 {
					limit = n
				}
			}
			return a.runTaskList("default", limit), true
		case "detail":
			if len(parts) < 3 {
				return "Usage: /task detail <task-id> [page] [pageSize]", true
			}
			page := 1
			pageSize := 20
			if len(parts) >= 4 {
				if n, err := strconv.Atoi(parts[3]); err == nil && n > 0 {
					page = n
				}
			}
			if len(parts) >= 5 {
				if n, err := strconv.Atoi(parts[4]); err == nil && n > 0 && n <= 200 {
					pageSize = n
				}
			}
			return a.runTaskDetail(parts[2], page, pageSize), true
		case "summary":
			if len(parts) < 3 {
				return "Usage: /task summary <task-id>", true
			}
			return a.runTaskSummary(parts[2]), true
		default:
			return "Unknown /task subcommand. Use: list | detail | summary", true
		}
	}

	// Match explicit run/execute patterns
	reCmd := regexp.MustCompile(`^(run|exec)\s+(.+)$`)
	cmd := ""
	if m := reCmd.FindStringSubmatch(msg); m != nil {
		cmd = strings.TrimSpace(m[2])
	}
	if strings.Contains(msg, "uname -r") {
		cmd = "uname -r"
	}
	if cmd == "" {
		return "", false
	}

	// Block dangerous commands
	danger := []string{"rm ", "rm -", "shutdown", "reboot", "mkfs", "dd ", "sudo ", "kill ", ":(){"}
	for _, d := range danger {
		if strings.Contains(cmd, d) {
			return "This command may be dangerous. Please confirm before executing.", true
		}
	}

	res, err := a.registry.CallTool("process", map[string]interface{}{"action": "start", "command": cmd})
	if err != nil {
		return fmt.Sprintf("Command execution failed: %v", err), true
	}
	// Extract sessionId
	sessionID := ""
	switch v := res.(type) {
	case tools.ProcessStartResult:
		sessionID = v.SessionID
	case *tools.ProcessStartResult:
		sessionID = v.SessionID
	case map[string]interface{}:
		if id, ok := v["sessionId"].(string); ok {
			sessionID = id
		}
	}
	if sessionID == "" {
		return "Command started but no sessionId returned.", true
	}

	// Poll command output with context + backoff
	content := a.pollCommandOutput(sessionID, 5*time.Second) // 5s deadline
	if content != "" {
		return content, true
	}
	return "Command completed but no output.", true
}

// pollCommandOutput polls command logs with exponential backoff and deadline
func (a *Agent) pollCommandOutput(sessionID string, deadline time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	getContent := func(res interface{}) string {
		switch v := res.(type) {
		case tools.ProcessLogResult:
			return strings.TrimSpace(v.Content)
		case *tools.ProcessLogResult:
			return strings.TrimSpace(v.Content)
		case map[string]interface{}:
			if c, ok := v["content"].(string); ok {
				return strings.TrimSpace(c)
			}
		}
		return ""
	}

	// Initial delay
	delay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return "" // Timeout
		default:
		}

		logRes, err := a.registry.CallTool("process", map[string]interface{}{"action": "log", "sessionId": sessionID})
		if err == nil {
			if content := getContent(logRes); content != "" {
				return content
			}
		}

		// Exponential backoff
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ""
			case <-time.After(delay):
			}
			delay = delay * 3 / 2 // 1.5x backoff
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return ""
}
