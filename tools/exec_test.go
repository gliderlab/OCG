package tools

import (
	"testing"
)

func TestExecToolName(t *testing.T) {
	tool := &ExecTool{}
	if tool.Name() != "exec" {
		t.Errorf("Expected 'exec', got '%s'", tool.Name())
	}
}

func TestExecToolDescription(t *testing.T) {
	tool := &ExecTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestExecToolParameters(t *testing.T) {
	tool := &ExecTool{}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["command"]; !ok {
		t.Error("Should have 'command' parameter")
	}
}

func TestExecToolBasic(t *testing.T) {
	tool := &ExecTool{}
	
	// Test simple echo
	args := map[string]interface{}{
		"command": "echo hello",
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestExecToolWithArgs(t *testing.T) {
	tool := &ExecTool{}
	
	// Test with arguments
	args := map[string]interface{}{
		"command": "echo",
		"args":    []string{"test", "123"},
	}
	
	_, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
}

func TestExecToolTimeout(t *testing.T) {
	tool := &ExecTool{}
	
	// Test with timeout
	args := map[string]interface{}{
		"command": "sleep 0.1",
		"timeout": 5,
	}
	
	_, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
}
