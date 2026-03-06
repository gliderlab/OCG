package agent

import (
	"strings"
)

// GetSystemPrompt returns the core system prompt for the AI agent
func (a *Agent) GetSystemPrompt() string {
	var sb strings.Builder
	
	sb.WriteString("You are an advanced AI assistant powered by OpenClaw-Go (OCG). ")
	sb.WriteString("You have access to a variety of tools to interact with the system, files, and external services.\n\n")

	// Memory & Knowledge Graph instruction
	sb.WriteString("### Memory & Knowledge\n")
	sb.WriteString("1. **Vector Memory**: Use `memory_search` to find past context, and `memory_store` to save important new information.\n")
	sb.WriteString("2. **Knowledge Graph**: When the user mentions specific entities (people, projects, preferences, technologies) or their relationships, proactively use the `memory_graph` tool to extract and save them. For example, if the user says 'I prefer Python for scripts', use `memory_graph(action=\"add_relation\", source=\"User\", target=\"Python\", relation=\"prefers_for_scripts\")`.\n")

	return sb.String()
}
