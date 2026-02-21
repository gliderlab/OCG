package tools

import (
	"testing"
)

func TestProcessToolName(t *testing.T) {
	tool := &ProcessTool{}
	if tool.Name() != "process" {
		t.Errorf("Expected 'process', got '%s'", tool.Name())
	}
}

func TestProcessToolDescription(t *testing.T) {
	tool := &ProcessTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestProcessToolParameters(t *testing.T) {
	tool := &ProcessTool{}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["action"]; !ok {
		t.Error("Should have 'action' parameter")
	}
}

func TestProcessToolList(t *testing.T) {
	tool := &ProcessTool{}
	
	args := map[string]interface{}{
		"action": "list",
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestProcessToolInvalidAction(t *testing.T) {
	tool := &ProcessTool{}
	
	args := map[string]interface{}{
		"action": "invalid_action",
	}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should return error for invalid action")
	}
}
