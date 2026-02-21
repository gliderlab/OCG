package tools

import (
	"testing"
)

func TestWebSearchToolName(t *testing.T) {
	tool := &WebSearchTool{}
	if tool.Name() != "web_search" {
		t.Errorf("Expected 'web_search', got '%s'", tool.Name())
	}
}

func TestWebSearchToolParameters(t *testing.T) {
	tool := &WebSearchTool{}
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

func TestWebFetchToolName(t *testing.T) {
	tool := &WebFetchTool{}
	if tool.Name() != "web_fetch" {
		t.Errorf("Expected 'web_fetch', got '%s'", tool.Name())
	}
}

func TestWebFetchToolParameters(t *testing.T) {
	tool := &WebFetchTool{}
	params := tool.Parameters()
	
	if params == nil {
		t.Fatal("Parameters should not be nil")
	}
	
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	
	if _, ok := props["url"]; !ok {
		t.Error("Should have 'url' parameter")
	}
}

func TestWebFetchToolBasic(t *testing.T) {
	tool := &WebFetchTool{}
	
	args := map[string]interface{}{
		"url": "https://example.com",
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestWebFetchToolMaxChars(t *testing.T) {
	tool := &WebFetchTool{}
	
	args := map[string]interface{}{
		"url":      "https://example.com",
		"maxChars": 100,
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	_ = result
}

func TestWebFetchToolInvalidURL(t *testing.T) {
	tool := &WebFetchTool{}
	
	args := map[string]interface{}{
		"url": "not_a_valid_url",
	}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should fail for invalid URL")
	}
}

func TestWebFetchToolMissingURL(t *testing.T) {
	tool := &WebFetchTool{}
	
	args := map[string]interface{}{}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should fail when URL is missing")
	}
}
