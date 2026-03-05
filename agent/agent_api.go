// agent_api.go - LLM API call layer: non-streaming HTTP requests and simple response mode
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *Agent) callAPI(messages []Message) string {
	return a.callAPIWithDepth(messages, 0)
}

func (a *Agent) callAPIWithDepth(messages []Message, depth int) string {
	reqBody := ChatRequest{
		Model:       a.cfg.Model,
		Messages:    messages,
		Temperature: a.cfg.Temperature,
		MaxTokens:   a.cfg.MaxTokens,
	}
	if len(a.systemTools) == 0 {
		a.refreshToolSpecs()
	}

	reqBody.Tools = a.systemTools

	body, _ := json.Marshal(reqBody)
	url := a.cfg.BaseURL + "/chat/completions"

	// For tool result processing (depth > 0), use shorter timeout
	ctx := context.Background()
	if depth > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		log.Printf("[FAST] depth=%d: using 30s timeout", depth)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("request build error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Sprintf("API error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("read error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	a.updateAnthropicRateLimit()

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return fmt.Sprintf("parse error: %v", err)
	}

	// handle tool call chain if returned (standard format)
	if len(chatResp.Choices) > 0 && len(chatResp.Choices[0].Message.ToolCalls) > 0 {
		validCalls := make([]ToolCall, 0)
		for _, tc := range chatResp.Choices[0].Message.ToolCalls {
			if tc.Function.Name != "" && tc.Function.Arguments != "" {
				validCalls = append(validCalls, tc)
			}
		}
		if len(validCalls) > 0 {
			assistantMsg := chatResp.Choices[0].Message
			return a.handleToolCalls(messages, validCalls, &assistantMsg, depth, nil)
		}
	}

	// handle custom tool call format (MiniMax, etc.)
	if len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content

		toolCalls := parseCustomToolCalls(content)
		if len(toolCalls) > 0 {
			assistantMsg := Message{Role: "assistant", Content: content, ToolCalls: toolCalls}
			return a.handleToolCalls(messages, toolCalls, &assistantMsg, depth, nil)
		}

		return content
	}

	return "no response"
}

func (a *Agent) simpleResponse(messages []Message) string {
	var userMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}

	input := strings.TrimSpace(strings.ToLower(userMsg))
	response := ""

	switch {
	case strings.Contains(input, "hello") || strings.Contains(input, "hi"):
		if a.cfg.Greeting != "" {
			response = a.cfg.Greeting
		} else {
			response = "Hello! I am assistant.\n\nAvailable tools:\n- exec: run commands\n- read: read files\n- write: write files"
		}
	case strings.Contains(input, "time"):
		if a.timeProvider != nil {
			response = a.timeProvider.Now().Format("2006-01-02 15:04:05")
		} else {
			response = time.Now().Format("2006-01-02 15:04:05")
		}
	case strings.Contains(input, "stat"):
		if a.store != nil {
			stats, _ := a.store.Stats()
			response = fmt.Sprintf("Storage stats:\n- messages: %d\n- memories: %d\n- files: %d", stats["messages"], stats["memories"], stats["files"])
		} else {
			response = "Storage not available"
		}
	case strings.Contains(input, "tools"):
		if a.registry != nil {
			toolList := a.registry.List()
			response = "Available tools:\n- " + strings.Join(toolList, "\n- ")
		} else {
			response = "tools not initialized"
		}
	case strings.Contains(input, "help") || strings.Contains(input, "aid"):
		response = "OCG-Go\n\nCommands:\n- hello - greeting\n- time - time\n- stat - stats\n- tools - list tools\n- help - help"
	default:
		response = "I received: " + userMsg
	}

	return response
}
