// agent_session.go - session management, task scheduling, and LLM-based compaction
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

	"github.com/gliderlab/cogate/storage"
)

// runCompact triggers manual compaction with optional instructions
func (a *Agent) runCompact(instructions string) string {
	if a.store == nil {
		return "Storage not available"
	}

	sessionKey := "default"
	messages, err := a.store.GetMessages(sessionKey, 500)
	if err != nil || len(messages) == 0 {
		return "No messages to compress"
	}

	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		meta = storage.SessionMeta{
			SessionKey: sessionKey,
		}
	}

	var summary string
	if instructions != "" {
		summary = a.buildSummaryWithInstructionsLLM(messages, instructions)
	} else {
		summary = a.buildSummaryWithInstructionsLLM(messages, "Concisely summarize the key points of the conversation")
	}

	if len(messages) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, messages[len(messages)-1].ID)
		meta.LastCompactedMessageID = messages[len(messages)-1].ID
	}
	_ = a.store.ClearMessages(sessionKey)

	if summary != "" {
		_ = a.store.AddMessage(sessionKey, "system", "[summary]\n"+summary)
	}

	meta.CompactionCount += 1
	meta.LastSummary = summary
	_ = a.store.UpsertSessionMeta(meta)

	return fmt.Sprintf("Compaction complete. Kept %d messages, summary length: %d chars", len(messages), len(summary))
}

// runNewSession starts a new session
func (a *Agent) runNewSession() string {
	if a.store == nil {
		return "Storage not available"
	}
	newKey := fmt.Sprintf("session-%d", time.Now().UnixNano())
	_ = a.store.AddMessage(newKey, "system", "New session started at "+time.Now().Format("2006-01-02 15:04:05"))
	return fmt.Sprintf("New session created: %s", newKey)
}

// runResetSession resets the current session
func (a *Agent) runResetSession() string {
	if a.store == nil {
		return "Storage not available"
	}

	sessionKey := "default"
	messages, _ := a.store.GetMessages(sessionKey, 1000)
	if len(messages) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, messages[len(messages)-1].ID)
	}
	_ = a.store.ClearMessages(sessionKey)
	_ = a.store.AddMessage(sessionKey, "system", "[session reset] "+time.Now().Format("2006-01-02 15:04:05"))

	meta, err := a.store.GetSessionMeta(sessionKey)
	if err == nil {
		meta.CompactionCount = 0
		meta.LastSummary = ""
		_ = a.store.UpsertSessionMeta(meta)
	}

	return "Session reset"
}

// executeSplitTask explicitly splits and executes a task
func (a *Agent) executeSplitTask(task string) string {
	if a.store == nil {
		return "Storage not available"
	}

	log.Printf("[TaskSplit] Splitting task: %s", task[:min(50, len(task))])

	subtasks, err := a.SplitTask(task)
	if err != nil {
		log.Printf("[TaskSplit] Failed to split task: %v", err)
		return fmt.Sprintf("Task split failed: %v", err)
	}

	if len(subtasks) == 0 {
		return "Cannot split task"
	}

	taskID, err := a.storeUserTask("default", task, subtasks)
	if err != nil {
		log.Printf("[TaskSplit] Failed to create task: %v", err)
		return fmt.Sprintf("Create task failed: %v", err)
	}

	_, err = a.ExecuteSubtasks(taskID, "default")
	if err != nil {
		log.Printf("[TaskSplit] Failed to execute subtasks: %v", err)
		return fmt.Sprintf("Execute task failed: %v", err)
	}

	return fmt.Sprintf("Task completed [OK]\nTask ID: %s\nMarker: [task_done:%s]\nUse /task detail %s to view full details.", taskID, taskID, taskID)
}

// storeUserTask stores a task in SQLite
func (a *Agent) storeUserTask(session, instructions string, subtasks []string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not available")
	}
	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	return a.store.CreateUserTask(id, session, instructions, subtasks)
}

// runTaskList lists recent tasks for a session
func (a *Agent) runTaskList(session string, limit int) string {
	if a.store == nil {
		return "Storage not available"
	}
	tasks, err := a.store.GetUserTasksBySession(session, limit)
	if err != nil {
		return fmt.Sprintf("Failed to list tasks: %v", err)
	}
	if len(tasks) == 0 {
		return "No tasks found"
	}
	var sb strings.Builder
	sb.WriteString("Recent tasks:\n")
	for _, t := range tasks {
		fmt.Fprintf(&sb, "- %s | %s | %d/%d\n", t.ID, t.Status, t.Completed, t.Total)
	}
	return sb.String()
}

// runTaskDetail returns full task details from DB by task ID
func (a *Agent) runTaskDetail(taskID string, page, pageSize int) string {
	if a.store == nil {
		return "Storage not available"
	}
	t, err := a.store.GetUserTask(taskID)
	if err != nil {
		return fmt.Sprintf("Task not found: %s", taskID)
	}
	subs, err := a.store.GetUserSubtasks(taskID)
	if err != nil {
		return fmt.Sprintf("Failed to load subtasks: %v", err)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	totalSubs := len(subs)
	start := (page - 1) * pageSize
	if start > totalSubs {
		start = totalSubs
	}
	end := start + pageSize
	if end > totalSubs {
		end = totalSubs
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Task: %s\n", t.ID)
	fmt.Fprintf(&sb, "Status: %s (%d/%d)\n", t.Status, t.Completed, t.Total)
	fmt.Fprintf(&sb, "CreatedAt: %s\n", formatUnixMilli(t.CreatedAt))
	if t.CompletedAt != nil {
		fmt.Fprintf(&sb, "CompletedAt: %s\n", formatUnixMilli(*t.CompletedAt))
		fmt.Fprintf(&sb, "DurationMs: %d\n", *t.CompletedAt-t.CreatedAt)
	}
	fmt.Fprintf(&sb, "Instructions: %s\n\n", t.Instructions)
	if t.Result != "" {
		sb.WriteString("Result:\n")
		sb.WriteString(t.Result)
		sb.WriteString("\n\n")
	}
	fmt.Fprintf(&sb, "Subtasks (page %d, size %d, total %d):\n", page, pageSize, totalSubs)
	for _, s := range subs[start:end] {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", s.IndexNum+1, s.Status, s.Description)
		if s.Result != "" {
			sb.WriteString("   result: ")
			sb.WriteString(s.Result)
			sb.WriteString("\n")
		}
	}
	if end < totalSubs {
		fmt.Fprintf(&sb, "\nMore subtasks available. Use: /task detail %s %d %d\n", taskID, page+1, pageSize)
	}
	return sb.String()
}

// runTaskSummary returns compact task status
func (a *Agent) runTaskSummary(taskID string) string {
	if a.store == nil {
		return "Storage not available"
	}
	t, err := a.store.GetUserTask(taskID)
	if err != nil {
		return fmt.Sprintf("Task not found: %s", taskID)
	}
	subs, _ := a.store.GetUserSubtasks(taskID)
	completed := 0
	for _, s := range subs {
		if s.Status == "completed" {
			completed++
		}
	}
	res := t.Result
	if len(res) > 280 {
		res = res[:280] + "..."
	}
	dur := int64(0)
	if t.CompletedAt != nil {
		dur = *t.CompletedAt - t.CreatedAt
	}
	completedAt := "-"
	if t.CompletedAt != nil {
		completedAt = formatUnixMilli(*t.CompletedAt)
	}
	return fmt.Sprintf("[task_done:%s]\nstatus=%s progress=%d/%d\ncreated_at=%s completed_at=%s duration_ms=%d\ninstructions=%s\nresult=%s\nUse /task detail %s for full process.",
		t.ID, t.Status, completed, t.Total, formatUnixMilli(t.CreatedAt), completedAt, dur, t.Instructions, res, t.ID)
}

func formatUnixMilli(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}

// FormatUnixMilliForTest exports formatUnixMilli for testing
func FormatUnixMilliForTest(ms int64) string {
	return formatUnixMilli(ms)
}

func (a *Agent) runArchiveDebug(sessionKey string) string {
	if a.store == nil {
		return "Storage not available"
	}
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		return fmt.Sprintf("Failed to load session meta: %v", err)
	}
	stats, err := a.store.GetArchiveStats(sessionKey)
	if err != nil {
		return fmt.Sprintf("Failed to load archive stats: %v", err)
	}
	return fmt.Sprintf("Archive debug (%s)\nlast_compacted_message_id=%d\narchived_count=%d\narchive_max_source_message_id=%d\ncompaction_count=%d\nlast_summary_len=%d",
		sessionKey,
		meta.LastCompactedMessageID,
		stats.ArchivedCount,
		stats.LastSourceMessage,
		meta.CompactionCount,
		len(meta.LastSummary),
	)
}

func (a *Agent) runLiveDebug(sessionKey string) string {
	now := time.Now()
	a.realtimeMu.Lock()
	activeCount := 0
	lines := make([]string, 0, len(a.realtimeSessions))
	for key, p := range a.realtimeSessions {
		if p == nil || !p.IsConnected() {
			continue
		}
		activeCount++
		last := a.realtimeLastUsed[key]
		idle := now.Sub(last).Round(time.Second)
		if sessionKey == "" || sessionKey == key {
			lines = append(lines, fmt.Sprintf("- %s connected=true last_used=%s idle=%s", key, last.Format("2006-01-02 15:04:05"), idle))
		}
	}
	a.realtimeMu.Unlock()

	metaLine := ""
	if sessionKey != "" && a.store != nil {
		if meta, err := a.store.GetSessionMeta(sessionKey); err == nil {
			metaLine = fmt.Sprintf("\nmeta: provider_type=%s realtime_last_active_at=%s", meta.ProviderType, meta.RealtimeLastActiveAt.Format("2006-01-02 15:04:05"))
		}
	}

	if len(lines) == 0 {
		if sessionKey != "" {
			return fmt.Sprintf("Live debug\nactive_connections=%d\nno active live connection for session=%s%s", activeCount, sessionKey, metaLine)
		}
		return fmt.Sprintf("Live debug\nactive_connections=%d\nno active live connections", activeCount)
	}

	return fmt.Sprintf("Live debug\nactive_connections=%d\n%s%s", activeCount, strings.Join(lines, "\n"), metaLine)
}

// buildSummaryWithInstructionsLLM builds a summary using LLM with custom instructions
func (a *Agent) buildSummaryWithInstructionsLLM(messages []storage.Message, instructions string) string {
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Summarize the following conversation history. Key points: %s\n\n", instructions)

	for _, m := range messages {
		if m.Role == "system" && strings.HasPrefix(m.Content, "[summary]") {
			continue
		}
		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, content)
	}

	summary, err := a.callLLMForSummary(sb.String())
	if err != nil {
		log.Printf("[WARN] LLM summary failed: %v, using fallback", err)
		return buildSummary(messages)
	}

	return summary
}

// callLLMForSummary makes a non-streaming LLM call to generate a summary
func (a *Agent) callLLMForSummary(prompt string) (string, error) {
	reqBody := ChatRequest{
		Model:       a.cfg.Model,
		Messages:    []Message{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   2048,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := a.cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	a.updateAnthropicRateLimit()

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return response.Choices[0].Message.Content, nil
}

// === Test Helper Functions (exported for testing) ===

// RunTestStoreUserTask exposes storeUserTask for testing
func (a *Agent) RunTestStoreUserTask(session, instructions string, subtasks []string) string {
	result, err := a.storeUserTask(session, instructions, subtasks)
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

// RunTestTaskList exposes runTaskList for testing
func (a *Agent) RunTestTaskList(session string, limit int) string {
	return a.runTaskList(session, limit)
}

// RunTestCompact exposes runCompact for testing
func (a *Agent) RunTestCompact(instructions string) string {
	return a.runCompact(instructions)
}

// RunTestNewSession exposes runNewSession for testing
func (a *Agent) RunTestNewSession() string {
	return a.runNewSession()
}

// RunTestResetSession exposes runResetSession for testing
func (a *Agent) RunTestResetSession() string {
	return a.runResetSession()
}
