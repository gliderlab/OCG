package tools

import (
	"testing"
)

func TestMemoryToolName(t *testing.T) {
	tool := &MemoryTool{Store: nil}
	if tool.Name() != "memory_search" {
		t.Errorf("Expected 'memory_search', got '%s'", tool.Name())
	}
}

func TestMemoryToolParameters(t *testing.T) {
	tool := &MemoryTool{Store: nil}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["query"]; !ok {
		t.Error("Should have 'query' parameter")
	}
}

func TestMemoryGetToolName(t *testing.T) {
	tool := &MemoryGetTool{Store: nil}
	if tool.Name() != "memory_get" {
		t.Errorf("Expected 'memory_get', got '%s'", tool.Name())
	}
}

func TestMemoryStoreToolName(t *testing.T) {
	tool := &MemoryStoreTool{Store: nil}
	if tool.Name() != "memory_store" {
		t.Errorf("Expected 'memory_store', got '%s'", tool.Name())
	}
}

func TestMemoryStoreToolParameters(t *testing.T) {
	tool := &MemoryStoreTool{Store: nil}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["text"]; !ok {
		t.Error("Should have 'text' parameter")
	}
}
