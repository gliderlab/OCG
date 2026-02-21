// Task splitting - break long tasks into subtasks
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// SubtaskInfo represents the result of task splitting
type SubtaskInfo struct {
	Subtasks []string `json:"subtasks"`
	Reason   string   `json:"reason,omitempty"`
}

// ShouldSplitTask - kept for compatibility, always returns false now
// Task splitting is now explicit via /split command
func (a *Agent) ShouldSplitTask(message string) bool {
	return false
}

// SplitTask calls LLM to split a task into subtasks
func (a *Agent) SplitTask(message string) ([]string, error) {
	prompt := fmt.Sprintf(`Split the following task into specific subtask steps.

Requirements:
1. Each subtask should be concrete and executable
2. Subtasks should be independent steps
3. Return JSON format with subtasks array
4. Keep descriptions concise

Task:
%s

Return format:
{"subtasks": ["subtask 1", "subtask 2", ...]}`, message)

	// Call LLM to split the task
	reqBody := map[string]interface{}{
		"model": a.cfg.Model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": "You are a task splitting assistant. Return JSON only, no other content."},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2000,
		"temperature": 0.3,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %v", err)
	}

	req, err := a.client.Post(a.cfg.BaseURL+"/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer req.Body.Close()

	if req.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", req.StatusCode)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode failed: %v", err)
	}

	// Parse response
	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		// Try content for non-streaming
		content, ok := choice["message"].(map[string]interface{})["content"].(string)
		if !ok {
			return nil, fmt.Errorf("no content in response")
		}
		return parseSubtasksFromJSON(content)
	}

	content, ok := delta["content"].(string)
	if !ok {
		return nil, fmt.Errorf("no content in delta")
	}

	return parseSubtasksFromJSON(content)
}

// parseSubtasksFromJSON extracts subtasks from JSON response
func parseSubtasksFromJSON(content string) ([]string, error) {
	// Find JSON in response
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, fmt.Errorf("no JSON found")
	}

	// Find the matching closing brace
	depth := 0
	end := start
	for i, c := range content[start:] {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = start + i + 1
				break
			}
		}
	}

	jsonStr := content[start:end]
	var result SubtaskInfo
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Try to extract subtasks with regex
		return extractSubtasksManually(content)
	}

	if len(result.Subtasks) == 0 {
		return nil, fmt.Errorf("no subtasks found")
	}

	return result.Subtasks, nil
}

// extractSubtasksManually tries to extract subtasks without full JSON parsing
func extractSubtasksManually(content string) ([]string, error) {
	var subtasks []string

	// Look for numbered items or bullet points
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "1. ")
		line = strings.TrimPrefix(line, "2. ")
		line = strings.TrimPrefix(line, "3. ")
		line = strings.TrimPrefix(line, "4. ")
		line = strings.TrimPrefix(line, "5. ")

		if len(line) > 5 {
			subtasks = append(subtasks, line)
		}
	}

	if len(subtasks) == 0 {
		return nil, fmt.Errorf("could not extract subtasks")
	}

	return subtasks, nil
}

// ExecuteSubtasks executes all subtasks for a given task
func (a *Agent) ExecuteSubtasks(taskID, sessionKey string) (string, error) {
	log.Printf("[TaskSplit] Executing subtasks for task: %s", taskID)

	// Initialize task status in KV for fast access
	if a.kv != nil {
		a.kv.SetTaskStatus(taskID, "running")
	}

	var allResults []string

	for {
		// Get next pending subtask
		subtask, err := a.store.GetPendingSubtask(taskID)
		if err != nil {
			a.store.UpdateUserTaskError(taskID, fmt.Sprintf("get pending subtask failed: %v", err))
			if a.kv != nil {
				a.kv.SetTaskStatus(taskID, "failed")
			}
			return "", fmt.Errorf("get pending subtask failed: %v", err)
		}

		if subtask == nil {
			// No more pending subtasks
			break
		}

		log.Printf("[TaskSplit] Executing subtask %d: %s", subtask.IndexNum+1, subtask.Description)

		// Update in KV (fast cache)
		if a.kv != nil {
			a.kv.SetSubtaskStatus(taskID, subtask.IndexNum, "running")
		}

		// Mark subtask as started in SQLite
		err = a.store.StartSubtask(subtask.ID)
		if err != nil {
			log.Printf("[TaskSplit] Failed to start subtask: %v", err)
		}

		// Update parent task status
		err = a.store.UpdateSubtaskStatus(subtask.ID, "running", "")
		if err != nil {
			log.Printf("[TaskSplit] Failed to update subtask to running: %v", err)
		}

		// Execute the subtask as a mini LLM call
		startTime := time.Now()
		result := a.executeSubtask(subtask.Description)
		duration := time.Since(startTime)

		// Build process log
		process := fmt.Sprintf("Start: %s\nDuration: %v\nResult: %s",
			startTime.Format("2006-01-02 15:04:05"), duration, result)

		// Update subtask with process and result
		err = a.store.UpdateSubtaskStatus(subtask.ID, "completed", result)
		if err != nil {
			log.Printf("[TaskSplit] Failed to update subtask status: %v", err)
		}

		// Also update the process field
		err = a.store.UpdateSubtaskProcess(subtask.ID, process)
		if err != nil {
			log.Printf("[TaskSplit] Failed to update subtask process: %v", err)
		}

		// Update in KV (fast cache)
		if a.kv != nil {
			a.kv.SetSubtaskStatus(taskID, subtask.IndexNum, "completed")
			// Update progress
			completed := subtask.IndexNum + 1
			total, _ := a.store.GetUserTask(taskID)
			if total != nil {
				a.kv.SetTaskProgress(taskID, completed, total.Total)
			}
		}

		allResults = append(allResults, fmt.Sprintf("## Subtask %d: %s\n\n**Time:** %s\n**Duration:** %v\n\n**Result:**\n%s",
			subtask.IndexNum+1, subtask.Description,
			startTime.Format("2006-01-02 15:04:05"), duration, result))

		// Small delay between subtasks
		time.Sleep(500 * time.Millisecond)
	}

	// Combine all results
	combinedResult := strings.Join(allResults, "\n\n---\n\n")

	// Update parent task
	err := a.store.UpdateUserTaskResult(taskID, combinedResult)
	if err != nil {
		log.Printf("[TaskSplit] Failed to update task result: %v", err)
	}

	// Update KV as completed
	if a.kv != nil {
		a.kv.SetTaskStatus(taskID, "completed")
	}

	log.Printf("[TaskSplit] All subtasks completed for task: %s", taskID)
	return combinedResult, nil
}

// executeSubtask executes a single subtask
func (a *Agent) executeSubtask(description string) string {
	if a.cfg.APIKey == "" {
		return "API key not configured"
	}

	messages := []Message{
		{Role: "user", Content: description},
	}

	// Call LLM with timeout
	resultChan := make(chan string, 1)
	go func() {
		resultChan <- a.callAPI(messages)
	}()

	select {
	case result := <-resultChan:
		return result
	case <-time.After(120 * time.Second):
		return "Execution timeout"
	}
}
