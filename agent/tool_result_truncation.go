package agent

import (
	"fmt"
	"strings"
)

// ToolResultTruncationConfig Configure tool result truncation
type ToolResultTruncationConfig struct {
	MaxBytes       int  // Maximum bytes
	MaxLines       int  // Maximum lines
	TruncateBefore bool // true=truncate front, false=truncate back
}

// DefaultToolResultTruncationConfig default config
var DefaultToolResultTruncationConfig = ToolResultTruncationConfig{
	MaxBytes:       15000, // 15KB
	MaxLines:       500,
	TruncateBefore: false,
}

// TruncateToolResult Truncate oversized tool results
func TruncateToolResult(result map[string]interface{}, cfg ToolResultTruncationConfig) map[string]interface{} {
	if cfg.MaxBytes == 0 {
		cfg = DefaultToolResultTruncationConfig
	}

	// Get content field
	content, ok := result["content"].(string)
	if !ok {
		return result
	}

	// Check if truncation needed
	needsTruncation := len(content) > cfg.MaxBytes
	if cfg.MaxLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > cfg.MaxLines {
			needsTruncation = true
		}
	}

	if !needsTruncation {
		return result
	}

	// Copy original result
	truncated := make(map[string]interface{})
	for k, v := range result {
		truncated[k] = v
	}

	// Truncate content
	var truncatedContent string
	if cfg.TruncateBefore {
		// Keep front portion
		truncatedContent = content
		if len(truncatedContent) > cfg.MaxBytes {
			truncatedContent = truncatedContent[:cfg.MaxBytes] + "\n[...truncated]"
		}
		if cfg.MaxLines > 0 {
			lines := strings.Split(truncatedContent, "\n")
			if len(lines) > cfg.MaxLines {
				lines = lines[:cfg.MaxLines]
				truncatedContent = strings.Join(lines, "\n") + "\n[...truncated]"
			}
		}
	} else {
		// Keep front and back, truncate middle
		truncatedContent = content
		if len(truncatedContent) > cfg.MaxBytes {
			half := cfg.MaxBytes / 2
			truncatedContent = content[:half] + "\n[... " + 
				fmt.Sprintf("%d", len(content)-cfg.MaxBytes) + " bytes truncated ...]\n" + content[len(content)-half:]
		}
	}

	truncated["content"] = truncatedContent
	truncated["truncated"] = true
	truncated["original_size"] = len(content)
	truncated["truncated_size"] = len(truncatedContent)

	return truncated
}

// TruncateToolResults Batch truncate tool results
func TruncateToolResults(results []ToolResult, cfg ToolResultTruncationConfig) []ToolResult {
	if cfg.MaxBytes == 0 {
		cfg = DefaultToolResultTruncationConfig
	}

	for i := range results {
		if results[i].Result == nil {
			continue
		}
		if resultMap, ok := results[i].Result.(map[string]interface{}); ok {
			results[i].Result = TruncateToolResult(resultMap, cfg)
		}
	}
	return results
}
