package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadToolName(t *testing.T) {
	tool := &ReadTool{}
	if tool.Name() != "read" {
		t.Errorf("Expected 'read', got '%s'", tool.Name())
	}
}

func TestReadToolBasic(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	
	content := "Hello, World!"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tool := &ReadTool{}
	args := map[string]interface{}{
		"path": tmpFile,
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestReadToolNotFound(t *testing.T) {
	tool := &ReadTool{}
	args := map[string]interface{}{
		"path": "/nonexistent/file.txt",
	}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should return error for non-existent file")
	}
}

func TestWriteToolName(t *testing.T) {
	tool := &WriteTool{}
	if tool.Name() != "write" {
		t.Errorf("Expected 'write', got '%s'", tool.Name())
	}
}

func TestWriteToolBasic(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	
	tool := &WriteTool{}
	args := map[string]interface{}{
		"path":    tmpFile,
		"content": "Test content",
	}
	
	result, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	// Verify file was created
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Errorf("Failed to read created file: %v", err)
	}
	
	if string(data) != "Test content" {
		t.Errorf("Expected 'Test content', got '%s'", string(data))
	}
	
	_ = result // Silence unused warning
}

func TestWriteToolAppend(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	
	// Create initial file
	os.WriteFile(tmpFile, []byte("Line 1\n"), 0644)
	
	tool := &WriteTool{}
	args := map[string]interface{}{
		"path":    tmpFile,
		"content": "Line 2",
		"append":  true,
	}
	
	_, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	data, _ := os.ReadFile(tmpFile)
	if string(data) != "Line 1\nLine 2" {
		t.Errorf("Expected appended content, got '%s'", string(data))
	}
}

func TestEditToolName(t *testing.T) {
	tool := &EditTool{}
	if tool.Name() != "edit" {
		t.Errorf("Expected 'edit', got '%s'", tool.Name())
	}
}

func TestEditToolBasic(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	
	content := "line 1\nline 2\nline 3"
	os.WriteFile(tmpFile, []byte(content), 0644)
	
	tool := &EditTool{}
	args := map[string]interface{}{
		"path":      tmpFile,
		"oldText":   "line 2",
		"newText":   "line 2 modified",
	}
	
	_, err := tool.Execute(args)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	
	data, _ := os.ReadFile(tmpFile)
	expected := "line 1\nline 2 modified\nline 3"
	if string(data) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(data))
	}
}

func TestEditToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	
	content := "line 1\nline 2"
	os.WriteFile(tmpFile, []byte(content), 0644)
	
	tool := &EditTool{}
	args := map[string]interface{}{
		"path":      tmpFile,
		"oldText":   "nonexistent",
		"newText":   "replacement",
	}
	
	_, err := tool.Execute(args)
	if err == nil {
		t.Error("Should return error when oldText not found")
	}
}
