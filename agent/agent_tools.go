// agent_tools.go - tool execution, loop detection, and custom tool call parsing
package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/gliderlab/cogate/tools"
)

func (a *Agent) executeToolCalls(toolCalls []ToolCall) []ToolResult {
	results := make([]ToolResult, 0, len(toolCalls))

	// Tool loop detection - check before executing
	if a.toolLoopDetector != nil {
		// 先探测一次是否处于循环泥潭
		hasLoop, reason := a.toolLoopDetector.CheckLoop()
		if hasLoop {
			log.Printf("[Agent] Tool loop detected: %s", reason)

			loopResults := make([]ToolResult, 0, len(toolCalls))
			for _, call := range toolCalls {
				loopResults = append(loopResults, ToolResult{
					ID:   call.ID,
					Type: "function",
					Result: map[string]interface{}{
						"error":   "Tool loop detected: " + reason,
						"tool":    call.Function.Name,
						"success": false,
					},
				})
			}
			return loopResults
		}

		// 若无循环，批量记录本回合的全部记录卡
		for _, call := range toolCalls {
			a.toolLoopDetector.RecordCall(call.Function.Name, call.Function.Arguments)
		}
	}

	for _, call := range toolCalls {
		var result interface{}
		var err error

		if a.registry != nil {
			result, err = a.registry.CallTool(call.Function.Name, parseArgs(call.Function.Arguments))
		} else {
			err = fmt.Errorf("tool registry not initialized")
		}

		if err != nil {
			if call.Function.Name == "exec" && err.Error() == "shell features are disabled; use a simple command or enable OCG_EXEC_ALLOW_SHELL" {
				result = err.Error()
			} else {
				result = map[string]interface{}{
					"error":   err.Error(),
					"tool":    call.Function.Name,
					"success": false,
				}
			}
		} else {
			// Simplify exec output to plain text
			if call.Function.Name == "exec" {
				switch v := result.(type) {
				case tools.ExecResult:
					out := strings.TrimSpace(v.Stdout)
					if out == "" {
						out = strings.TrimSpace(v.Stderr)
					}
					if out == "" {
						out = "OK"
					}
					result = out
				case *tools.ExecResult:
					out := strings.TrimSpace(v.Stdout)
					if out == "" {
						out = strings.TrimSpace(v.Stderr)
					}
					if out == "" {
						out = "OK"
					}
					result = out
				}
			}
			result = map[string]interface{}{
				"result":  result,
				"tool":    call.Function.Name,
				"success": true,
			}
		}

		results = append(results, ToolResult{
			ID:     call.ID,
			Type:   "function",
			Result: result,
		})
	}

	// Apply tool result truncation
	results = TruncateToolResults(results, DefaultToolResultTruncationConfig)

	return results
}

func (a *Agent) handleToolCalls(messages []Message, toolCalls []ToolCall, assistantMsg *Message, depth int, callback func(string)) string {
	// Send tool execution start event
	if callback != nil && len(toolCalls) > 0 {
		callback(`[TOOL_EVENT]{"type":"tool_start","tools":[`)
		for i, tc := range toolCalls {
			if i > 0 {
				callback(",")
			}
			callback(fmt.Sprintf(`{"name":"%s","id":"%s"}`, tc.Function.Name, tc.ID))
		}
		callback(`]}`)
	}

	results := a.executeToolCalls(toolCalls)

	// Send tool result events - FIX: check actual success/failure
	if callback != nil && len(results) > 0 {
		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			// FIX-4: Correctly determine success from result content
			hasError := false
			if resultMap, ok := tr.Result.(map[string]interface{}); ok {
				if _, exists := resultMap["error"]; exists {
					hasError = true
				}
			}
			callback(fmt.Sprintf(`[TOOL_EVENT]{"type":"tool_result","tool_id":"%s","success":%t,"result":%s}`,
				toolCalls[i].ID, !hasError, string(resultBytes)))
		}
	}

	resp := ToolResponse{
		ToolResults: results,
	}
	respBytes, _ := json.Marshal(resp)

	if a.cfg.APIKey == "" {
		return string(respBytes)
	}
	if depth >= 2 {
		return summarizeToolResults(results)
	}

	newMessages := make([]Message, 0, len(messages)+2)
	newMessages = append(newMessages, messages...)

	if assistantMsg != nil {
		newMessages = append(newMessages, *assistantMsg)
	} else if len(messages) == 0 || len(messages[len(messages)-1].ToolCalls) == 0 {
		newMessages = append(newMessages, Message{Role: "assistant", ToolCalls: toolCalls})
	}

	// OpenAI-style tool messages
	for i, tr := range results {
		contentBytes, _ := json.Marshal(tr.Result)
		toolMsg := Message{Role: "tool", Content: string(contentBytes)}
		if i < len(toolCalls) {
			toolMsg.ToolCallID = toolCalls[i].ID
		} else {
			toolMsg.ToolCallID = tr.ID
		}
		newMessages = append(newMessages, toolMsg)
	}

	return a.callAPIWithDepth(newMessages, depth+1)
}

func summarizeToolResults(results []ToolResult) string {
	if len(results) == 0 {
		return "(tool) no results"
	}
	parts := make([]string, 0, len(results))
	for _, tr := range results {
		m, ok := tr.Result.(map[string]interface{})
		if !ok {
			continue
		}
		if errMsg, ok := m["error"].(string); ok && errMsg != "" {
			parts = append(parts, "tool error: "+errMsg)
			continue
		}
		if res, ok := m["result"]; ok {
			parts = append(parts, fmt.Sprintf("%v", res))
		}
	}
	if len(parts) == 0 {
		return "(tool) completed"
	}
	return strings.Join(parts, "\n")
}

func parseArgs(argsJSON string) map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		log.Printf("[WARN] Failed to parse tool args: %q -> %v", argsJSON, err)
		args = make(map[string]interface{})
	}
	return args
}

// parseCustomToolCalls parses custom tool call format from MiniMax and similar models
func parseCustomToolCalls(content string) []ToolCall {
	var toolCalls []ToolCall

	// Pre-validation: check if content looks like it has tool calls
	hasToolIndicator := strings.Contains(content, "tool_calls") ||
		strings.Contains(content, "invoke name=") ||
		strings.Contains(content, "function_call") ||
		strings.Contains(content, "=\"")

	if !hasToolIndicator && !strings.Contains(content, "[{") && !strings.Contains(content, "{\"") {
		return nil
	}

	// First, try JSON format (more robust and standard)
	jsonMatches := []string{}
	for _, match := range reJSONBlock.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && len(match[1]) > 10 {
			if !strings.Contains(match[1], "name") && !strings.Contains(match[1], "function") {
				continue
			}
			var parsed interface{}
			if err := json.Unmarshal([]byte(match[1]), &parsed); err == nil {
				jsonMatches = append(jsonMatches, match[1])
			}
		}
	}

	for _, jsonStr := range jsonMatches {
		var calls []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &calls); err != nil {
			log.Printf("[WARN] Failed to parse tool calls JSON: %v", err)
			continue
		}
		for _, call := range calls {
			name, _ := call["name"].(string)
			if name == "" {
				name, _ = call["function"].(map[string]interface{})["name"].(string)
			}
			if name == "" {
				continue
			}

			argsMap := make(map[string]interface{})
			if args, ok := call["arguments"].(map[string]interface{}); ok {
				argsMap = args
			} else if args, ok := call["function"].(map[string]interface{})["arguments"].(map[string]interface{}); ok {
				argsMap = args
			} else if argsStr, ok := call["arguments"].(string); ok {
				var argsParsed map[string]interface{}
				if err := json.Unmarshal([]byte(argsStr), &argsParsed); err != nil {
					log.Printf("[WARN] Failed to parse arguments JSON: %v", err)
					argsMap["raw"] = argsStr
				} else {
					argsMap = argsParsed
				}
			}

			argsJSON, _ := json.Marshal(argsMap)
			toolCalls = append(toolCalls, ToolCall{
				ID:   fmt.Sprintf("call_%d", len(toolCalls)),
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      mapToolName(name),
					Arguments: string(argsJSON),
				},
			})
		}
		if len(toolCalls) > 0 {
			log.Printf("[DEBUG] parseCustomToolCalls: parsed %d JSON tool calls", len(toolCalls))
			return toolCalls
		}
	}

	// Fall back to XML-like format
	// Validate content size to prevent DoS
	if len(content) > 50000 {
		log.Printf("[WARN] parseCustomToolCalls: content too large, truncating")
		content = content[:50000]
	}

	matches1 := reXMLTool1.FindAllStringSubmatch(content, -1)
	matches2 := reXMLTool2.FindAllStringSubmatch(content, -1)
	matches := append(matches1, matches2...)

	log.Printf("[DEBUG] parseCustomToolCalls: content length=%d, XML matches found=%d", len(content), len(matches))

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		toolName := m[1]
		if len(toolName) > 100 || toolName == "" {
			continue
		}
		paramsStr := m[2]

		args := make(map[string]interface{})
		paramMatches := reXMLParam.FindAllStringSubmatch(paramsStr, -1)
		for _, pm := range paramMatches {
			if len(pm) >= 3 {
				key := pm[1]
				value := strings.TrimSpace(pm[2])
				if len(key) <= 100 && len(value) <= 10000 {
					args[key] = value
				}
			}
		}

		actualToolName := mapToolName(toolName)
		argsJSON, _ := json.Marshal(args)

		toolCalls = append(toolCalls, ToolCall{
			ID:   fmt.Sprintf("call_%d", len(toolCalls)),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      actualToolName,
				Arguments: string(argsJSON),
			},
		})
	}

	if len(toolCalls) == 0 {
		toolCalls = parseSimpleToolCalls(content)
	}

	return toolCalls
}

// parseSimpleToolCalls is a fallback parser for basic tool call extraction
func parseSimpleToolCalls(content string) []ToolCall {
	var toolCalls []ToolCall

	matches := reSimple.FindAllStringSubmatch(content, -1) //nolint:govet

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		toolName := strings.TrimSpace(m[1])
		if toolName == "" || len(toolName) > 100 {
			continue
		}

		if !knownToolsSet[toolName] {
			continue
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:   fmt.Sprintf("call_simple_%d", len(toolCalls)),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      toolName,
				Arguments: "{}",
			},
		})
	}

	if len(toolCalls) > 0 {
		log.Printf("[DEBUG] parseSimpleToolCalls: extracted %d tool calls (fallback)", len(toolCalls))
	}

	return toolCalls
}

// mapToolName maps model-specific tool names to actual tool names
func mapToolName(modelToolName string) string {
	switch modelToolName {
	case "read_file":
		return "read"
	case "write_file":
		return "write"
	case "execute_command", "exec_cmd":
		return "exec"
	case "cat":
		return "read"
	default:
		return modelToolName
	}
}

type ToolResponse struct {
	ToolResults []ToolResult `json:"tool_results"`
}

var (
	knownToolsSet = map[string]bool{
		"read": true, "write": true, "edit": true, "exec": true,
		"process": true, "browser": true, "message": true, "cron": true,
		"memory_search": true, "memory_get": true, "sessions_send": true,
		"subagents": true, "image": true, "tts": true, "web_fetch": true,
	}
	reEdit1     = regexp.MustCompile(`(?i)Edit\s+([^:]+):\s*replace\s+(.+)\s+with\s+(.+)`)
	reEdit2     = regexp.MustCompile(`(?i)Edit\s+([^:]+):\s*change\s+(.+)\s+to\s+(.+)`)
	reEdit3     = regexp.MustCompile(`(?i)Replace\s+(.+)\s+with\s+(.+)\s+in\s+(.+)`)
	reEdit4     = regexp.MustCompile(`(?i)replace\s+(.+)\s+with\s+(.+)\s+in\s+([^ ]+)`)
	reJSONBlock = regexp.MustCompile(`(?s)(\[.*?\]|\{.*?\})`)
	reSimple    = regexp.MustCompile(`(?i)(?:tool_calls?|function_call|invoke)[:\s]+["']?([a-zA-Z_][a-zA-Z0-9_]*)["']?`)
	reXMLTool1  = regexp.MustCompile(`(?i)<minimax:tool_call>\s*<invoke\s+name="([^"]+)"[^>]*>(.*?)</invoke>\s*</minimax:tool_call>`)
	reXMLTool2  = regexp.MustCompile(`(?i)<minimax:tool_call>\s*<invoke\s+name="([^"]+)"[^>]*>(.*?)</invoke>\s*`)
	reXMLParam  = regexp.MustCompile(`<parameter\s+name="([^"]+)">([^<]*)</parameter>`)
)

// detectEditIntent detects natural language edit requests
func detectEditIntent(msg string) map[string]interface{} {
	if m := reEdit1.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[1]),
			"oldText": strings.TrimSpace(m[2]),
			"newText": strings.TrimSpace(m[3]),
		}
	}

	if m := reEdit2.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[1]),
			"oldText": strings.TrimSpace(m[2]),
			"newText": strings.TrimSpace(m[3]),
		}
	}

	if m := reEdit3.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[3]),
			"oldText": strings.TrimSpace(m[1]),
			"newText": strings.TrimSpace(m[2]),
		}
	}

	if m := reEdit4.FindStringSubmatch(msg); m != nil {
		return map[string]interface{}{
			"path":    strings.TrimSpace(m[3]),
			"oldText": strings.TrimSpace(m[1]),
			"newText": strings.TrimSpace(m[2]),
		}
	}

	return nil
}

// handleEdit processes edit requests
func (a *Agent) handleEdit(args map[string]interface{}) string {
	if a.registry == nil {
		return "Error: tool registry not initialized"
	}

	result, err := a.registry.CallTool("edit", args)
	if err != nil {
		return fmt.Sprintf("Edit failed: %v", err)
	}

	b, _ := json.Marshal(result)
	return fmt.Sprintf("Edit completed: %s", string(b))
}
