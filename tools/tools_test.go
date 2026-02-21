package tools

import (
	"testing"
)

func TestToolRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test empty registry
	if len(registry.tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(registry.tools))
	}

	// Register a test tool
	registry.Register(&ExecTool{})

	// Test count after registration
	if len(registry.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(registry.tools))
	}

	// Test Get
	tool, ok := registry.Get("exec")
	if !ok {
		t.Error("Expected to find 'exec' tool")
	}
	if tool == nil {
		t.Error("Tool should not be nil")
	}

	// Test Get with non-existent tool
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Should not find non-existent tool")
	}
}

func TestToolRegistryList(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&ExecTool{})
	registry.Register(&ReadTool{})
	registry.Register(&WriteTool{})

	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
}

func TestToolRegistryGetToolSpecs(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&ExecTool{})

	specs := registry.GetToolSpecs()
	if len(specs) != 1 {
		t.Errorf("Expected 1 tool spec, got %d", len(specs))
	}
}

func TestGetString(t *testing.T) {
	// Test with existing key
	args := map[string]interface{}{"name": "test"}
	result := GetString(args, "name")
	if result != "test" {
		t.Errorf("Expected 'test', got '%s'", result)
	}

	// Test with missing key
	result = GetString(args, "missing")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}

	// Test with wrong type
	args = map[string]interface{}{"name": 123}
	result = GetString(args, "name")
	if result != "" {
		t.Errorf("Expected empty string for wrong type, got '%s'", result)
	}
}

func TestGetInt(t *testing.T) {
	// Test with existing key
	args := map[string]interface{}{"count": 42}
	result := GetInt(args, "count")
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test with missing key
	result = GetInt(args, "missing")
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}

	// Test with float
	args = map[string]interface{}{"count": 42.5}
	result = GetInt(args, "count")
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test with wrong type
	args = map[string]interface{}{"count": "string"}
	result = GetInt(args, "count")
	if result != 0 {
		t.Errorf("Expected 0 for wrong type, got %d", result)
	}
}

func TestGetBool(t *testing.T) {
	// Test true
	args := map[string]interface{}{"enabled": true}
	result := GetBool(args, "enabled")
	if !result {
		t.Error("Expected true")
	}

	// Test false
	args = map[string]interface{}{"enabled": false}
	result = GetBool(args, "enabled")
	if result {
		t.Error("Expected false")
	}

	// Test missing
	result = GetBool(args, "missing")
	if result {
		t.Error("Expected false for missing key")
	}

	// Note: GetBool only handles bool type, not string "true"/"false"
	// Test string "true" - GetBool doesn't convert strings
	args = map[string]interface{}{"enabled": "true"}
	result = GetBool(args, "enabled")
	if result {
		t.Error("Expected false for string 'true' (not supported)")
	}
}

func TestGetFloat64(t *testing.T) {
	// Test with existing key
	args := map[string]interface{}{"value": 3.14}
	result := GetFloat64(args, "value")
	if result != 3.14 {
		t.Errorf("Expected 3.14, got %f", result)
	}

	// Test with missing key
	result = GetFloat64(args, "missing")
	if result != 0 {
		t.Errorf("Expected 0, got %f", result)
	}

	// Test with int (should convert)
	args = map[string]interface{}{"value": 10}
	result = GetFloat64(args, "value")
	if result != 10 {
		t.Errorf("Expected 10, got %f", result)
	}
}
