package tools

import (
	"testing"
)

func TestGatewayToolName(t *testing.T) {
	tool := &GatewayTool{}
	if tool.Name() != "gateway" {
		t.Errorf("Expected 'gateway', got '%s'", tool.Name())
	}
}

func TestGatewayToolDescription(t *testing.T) {
	tool := &GatewayTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestGatewayToolParameters(t *testing.T) {
	tool := &GatewayTool{}
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

func TestGatewayToolInvalidAction(t *testing.T) {
	tool := &GatewayTool{}
	
	args := map[string]interface{}{
		"action": "invalid_action",
	}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should return error for invalid action")
	}
}

func TestGatewayToolMissingAction(t *testing.T) {
	tool := &GatewayTool{}
	
	args := map[string]interface{}{}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should return error when action is missing")
	}
}
