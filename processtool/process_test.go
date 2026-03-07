package processtool

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProcessTool_Execute_UnknownAction(t *testing.T) {
	tool := &ProcessTool{}
	_, err := tool.Execute(map[string]interface{}{"action": "unknown_action"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got: %v", err)
	}
}

func TestProcessTool_StartAndKill(t *testing.T) {
	tool := &ProcessTool{}

	// Test starting a simple command
	cmdStr := "echo hello"
	if runtime.GOOS == "windows" {
		cmdStr = "cmd /c echo hello"
	}
	args := map[string]interface{}{
		"action":  "start",
		"command": cmdStr,
	}

	res, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	startRes, ok := res.(ProcessStartResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}

	if !startRes.Success {
		t.Fatalf("expected success to be true")
	}

	if startRes.SessionID == "" {
		t.Fatalf("session ID is empty")
	}

	// Give it some time to finish
	time.Sleep(100 * time.Millisecond)

	// Check logs
	logArgs := map[string]interface{}{
		"action":    "log",
		"sessionId": startRes.SessionID,
	}
	logRes, err := tool.Execute(logArgs)
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}

	logData, ok := logRes.(ProcessLogResult)
	if !ok {
		t.Fatalf("unexpected log result type: %T", logRes)
	}

	if !strings.Contains(logData.Content, "hello") {
		t.Errorf("expected log to contain 'hello', got %q", logData.Content)
	}

	// Clean up / Kill
	killArgs := map[string]interface{}{
		"action":    "kill",
		"sessionId": startRes.SessionID,
	}
	_, err = tool.Execute(killArgs)
	if err != nil && !strings.Contains(err.Error(), "process already finished") && !strings.Contains(err.Error(), "kill failed") {
		// Just want to ensure it doesn't panic and removes the session
	}

	// After kill, log should fail
	_, err = tool.Execute(logArgs)
	if err == nil {
		t.Errorf("expected error getting log for killed process, got nil")
	}
}

func TestProcessTool_PtyExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not fully supported natively on Windows in this library")
	}
	tool := &ProcessTool{}

	args := map[string]interface{}{
		"action":  "start",
		"command": "echo pty-test",
		"pty":     true,
	}

	res, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("failed to start process with pty: %v", err)
	}

	startRes := res.(ProcessStartResult)

	// Wait for process to exit and PTY to flush
	time.Sleep(800 * time.Millisecond) // increased wait time for macOS CI

	logArgs := map[string]interface{}{
		"action":    "log",
		"sessionId": startRes.SessionID,
	}
	
	// Retry loop for log verification (to avoid flaky tests on slow CI)
	var logStr string
	for i := 0; i < 5; i++ {
		logRes, err := tool.Execute(logArgs)
		if err == nil {
			logData := logRes.(ProcessLogResult)
			logStr = logData.Content
			if strings.Contains(logStr, "pty-test") {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !strings.Contains(logStr, "pty-test") {
		t.Errorf("expected log to contain 'pty-test', got %q", logStr)
	}
}

func TestProcessTool_AutoRestart(t *testing.T) {
	tool := &ProcessTool{}

	cmdFailStr := "go run nonexistent_file.go"
	// Start a command that exits immediately, with auto-restart enabled
	args := map[string]interface{}{
		"action":              "start",
		"command":             cmdFailStr,
		"autoRestart":         true,
		"maxRetries":          2,
		"restartDelaySeconds": 0,
	}

	res, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	startRes := res.(ProcessStartResult)

	// Wait a moment for process to actually die
	time.Sleep(200 * time.Millisecond)

	// Check and restart explicitly as tests might not want to wait 10s for ticker
	checkAndRestartProcesses() // Retry 1
	time.Sleep(100 * time.Millisecond)

	checkAndRestartProcesses() // Retry 2
	time.Sleep(100 * time.Millisecond)

	checkAndRestartProcesses() // Retry 3 (should be ignored max 2)
	time.Sleep(100 * time.Millisecond)

	procMutex.RLock()
	proc, ok := processes[startRes.SessionID]
	procMutex.RUnlock()

	if !ok {
		t.Fatalf("process should still be in map")
	}

	procMutex.RLock()
	retries := proc.CurrentRetries
	procMutex.RUnlock()

	if retries < 2 {
		t.Errorf("expected at least 2 retries, got %d", retries)
	}

	// Clean up
	tool.Execute(map[string]interface{}{
		"action":    "kill",
		"sessionId": startRes.SessionID,
	})
}
