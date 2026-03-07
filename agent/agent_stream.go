// agent_stream.go - ChatStream: SSE streaming with tool call support
// FIX-5: commands in ChatStream now accept sessionKey for proper session routing
package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ChatStream sends chat messages and streams the response via callback.
// sessionKey defaults to "default" for backward compat; use ChatStreamWithSession for named sessions.
func (a *Agent) ChatStream(messages []Message, callback func(string)) {
	a.chatStreamInternal("default", messages, callback)
}

// ChatStreamWithSession streams with an explicit session key (fixes hard-coded "default").
func (a *Agent) ChatStreamWithSession(sessionKey string, messages []Message, callback func(string)) {
	a.chatStreamInternal(sessionKey, messages, callback)
}

func (a *Agent) chatStreamInternal(sessionKey string, messages []Message, callback func(string)) {
	a.mu.RLock()
	apiKey := a.cfg.APIKey
	baseURL := a.cfg.BaseURL
	model := a.cfg.Model
	temperature := a.cfg.Temperature
	maxTokens := a.cfg.MaxTokens

	// Apply configuration groups if provider is set
	if a.cfg.Provider != "" && a.cfg.Groups != nil {
		if group, ok := a.cfg.Groups[a.cfg.Provider]; ok {
			if group.APIKey != "" {
				apiKey = group.APIKey
			}
			if group.BaseURL != "" {
				baseURL = group.BaseURL
			}
			if group.Model != "" {
				model = group.Model
			}
		}
	}
	a.mu.RUnlock()

	lastMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastMsg = messages[i].Content
			break
		}
	}

	// FIX-5: use sessionKey instead of hard-coded "default"
	if out, ok := a.runCommandIfRequested(lastMsg); ok {
		callback(out)
		if a.store != nil && lastMsg != "" {
			a.storeMessage(sessionKey, "user", lastMsg)
			a.storeMessage(sessionKey, "assistant", out)
		}
		return
	}

	// Handle /split command (explicit task splitting)
	if strings.HasPrefix(lastMsg, "/split ") || lastMsg == "/split" {
		taskMsg := strings.TrimPrefix(lastMsg, "/split ")
		if taskMsg == "/split" || taskMsg == "" {
			callback("Usage: /split <task>\nExample: /split summarize today's meeting notes")
		} else {
			result := a.executeSplitTask(taskMsg)
			callback(result)
		}
		return
	}

	if apiKey == "" {
		response := a.simpleResponse(messages)
		callback(response)
		if a.store != nil && lastMsg != "" {
			a.storeMessage(sessionKey, "user", lastMsg)
			a.storeMessage(sessionKey, "assistant", response)
		}
		return
	}

	if a.store != nil && lastMsg != "" {
		// Shared preprocessing: auto memory capture + flush + compaction
		a.preprocessChat(sessionKey, lastMsg, messages)
	}

	// Overflow handling
	if a.store != nil {
		// FIX-5: pass sessionKey instead of "default"
		messages = a.handleContextOverflow(sessionKey, messages)
	}

	// Use streaming HTTP request
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      true,
	}
	if len(a.systemTools) == 0 {
		a.refreshToolSpecs()
	}
	reqBody.Tools = a.systemTools

	body, _ := json.Marshal(reqBody)
	url := baseURL + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		callback(fmt.Sprintf("request build error: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.client.Do(req)
	if err != nil {
		callback(fmt.Sprintf("API error: %v", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		callback(fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody)))
		return
	}

	// Update rate limiter for Anthropic
	a.updateAnthropicRateLimit()

	// Read SSE stream and check for tool calls
	reader := bufio.NewReader(resp.Body)
	var contentBuilder strings.Builder
	var toolCalls []ToolCall
	tcArgBuilders := make(map[int]*strings.Builder) // index -> arguments builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							// Check for tool calls (incremental merge by index)
							if tcRaw, ok := delta["tool_calls"].([]interface{}); ok && len(tcRaw) > 0 {
								for _, tcItem := range tcRaw {
									tcMap, ok := tcItem.(map[string]interface{})
									if !ok {
										continue
									}
									idxFloat, _ := tcMap["index"].(float64)
									idx := int(idxFloat)

									if idx >= len(toolCalls) {
										tcData, _ := json.Marshal(tcMap)
										var tcCall ToolCall
										_ = json.Unmarshal(tcData, &tcCall)
										toolCalls = append(toolCalls, tcCall)
										tcArgBuilders[idx] = &strings.Builder{}
										tcArgBuilders[idx].WriteString(tcCall.Function.Arguments)
									} else {
										if fnMap, ok := tcMap["function"].(map[string]interface{}); ok {
											if argChunk, ok := fnMap["arguments"].(string); ok {
												if b, exists := tcArgBuilders[idx]; exists {
													b.WriteString(argChunk)
												}
											}
										}
									}
								}
							}
							// Send content
							if content, ok := delta["content"].(string); ok {
								contentBuilder.WriteString(content)
								callback(content)
							}
						}
					}
				}
			}
		}
	}

	// Finalize tool call arguments from builders
	for idx, b := range tcArgBuilders {
		if idx < len(toolCalls) {
			toolCalls[idx].Function.Arguments = b.String()
		}
	}

	// Handle tool calls if any
	if len(toolCalls) > 0 && a.registry != nil {
		validCalls := make([]ToolCall, 0, len(toolCalls))
		for _, tc := range toolCalls {
			args := strings.TrimSpace(tc.Function.Arguments)
			if args == "" {
				args = "{}"
			}
			if !json.Valid([]byte(args)) {
				continue
			}
			tc.Function.Arguments = args
			if tc.Function.Name != "" {
				validCalls = append(validCalls, tc)
			}
		}
		if len(validCalls) == 0 {
			// Fallback to non-streaming flow if tool calls are invalid
			reply := a.Chat(messages)
			if reply != "" {
				callback(reply)
			}
			return
		}
		toolCalls = validCalls

		// Send tool execution start event
		log.Printf("[TOOL] Sending tool_start event, toolCalls=%d", len(toolCalls))
		callback(`[TOOL_EVENT]{"type":"tool_start","tools":[` +
			strings.Join(func() []string {
				var names []string
				for _, tc := range toolCalls {
					names = append(names, fmt.Sprintf(`{"name":"%s","id":"%s"}`, tc.Function.Name, tc.ID))
				}
				return names
			}(), ",") + `]}`)

		// Execute tool calls
		results := a.executeToolCalls(toolCalls)

		// FIX-4: Send tool result event with actual success/failure
		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			hasError := false
			if resultMap, ok := tr.Result.(map[string]interface{}); ok {
				if _, exists := resultMap["error"]; exists {
					hasError = true
				}
			}
			log.Printf("[TOOL] Sending tool_result event for %s", toolCalls[i].ID)
			callback(fmt.Sprintf(`[TOOL_EVENT]{"type":"tool_result","tool_id":"%s","success":%t,"result":%s}`,
				toolCalls[i].ID, !hasError, string(resultBytes)))
		}

		// Build tool result messages
		newMessages := make([]Message, 0, len(messages)+len(results)+1)
		newMessages = append(newMessages, messages...)
		newMessages = append(newMessages, Message{Role: "assistant", Content: contentBuilder.String(), ToolCalls: toolCalls})

		for i, tr := range results {
			resultBytes, _ := json.Marshal(tr.Result)
			toolMsg := Message{
				Role:       "tool",
				Content:    string(resultBytes),
				ToolCallID: toolCalls[i].ID,
			}
			newMessages = append(newMessages, toolMsg)
		}

		// Recurse with sessionKey
		a.ChatStreamWithSession(sessionKey, newMessages, callback)
		return
	}

	// No tool calls: store final assistant reply
	if a.store != nil && lastMsg != "" {
		reply := strings.TrimSpace(contentBuilder.String())
		if reply != "" {
			// FIX-5: use sessionKey instead of "default"
			a.storeMessage(sessionKey, "user", lastMsg)
			a.storeMessage(sessionKey, "assistant", reply)
		}
	}
}
