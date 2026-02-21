package tools

import (
	"testing"
)

func TestSessionsListToolName(t *testing.T) {
	tool := &SessionsListTool{}
	if tool.Name() != "sessions_list" {
		t.Errorf("Expected 'sessions_list', got '%s'", tool.Name())
	}
}

func TestSessionsListToolParameters(t *testing.T) {
	tool := &SessionsListTool{}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["limit"]; !ok {
		t.Error("Should have 'limit' parameter")
	}
}

func TestSessionsSendToolName(t *testing.T) {
	tool := &SessionsSendTool{}
	if tool.Name() != "sessions_send" {
		t.Errorf("Expected 'sessions_send', got '%s'", tool.Name())
	}
}

func TestSessionsSendToolParameters(t *testing.T) {
	tool := &SessionsSendTool{}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["message"]; !ok {
		t.Error("Should have 'message' parameter")
	}
}

func TestSessionsSpawnToolName(t *testing.T) {
	tool := &SessionsSpawnTool{}
	if tool.Name() != "sessions_spawn" {
		t.Errorf("Expected 'sessions_spawn', got '%s'", tool.Name())
	}
}

func TestSessionsHistoryToolName(t *testing.T) {
	tool := &SessionsHistoryTool{}
	if tool.Name() != "sessions_history" {
		t.Errorf("Expected 'sessions_history', got '%s'", tool.Name())
	}
}

func TestSessionStatusToolName(t *testing.T) {
	tool := &SessionStatusTool{}
	if tool.Name() != "session_status" {
		t.Errorf("Expected 'session_status', got '%s'", tool.Name())
	}
}

func TestAgentsListToolName(t *testing.T) {
	tool := &AgentsListTool{}
	if tool.Name() != "agents_list" {
		t.Errorf("Expected 'agents_list', got '%s'", tool.Name())
	}
}
