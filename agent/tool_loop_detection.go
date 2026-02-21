package agent

import (
	"fmt"
	"strings"
	"time"
)

// ToolLoopDetectionConfig Tool loop detection config
type ToolLoopDetectionConfig struct {
	MaxCalls     int           // Maximum call count
	TimeWindow   time.Duration // Time window
	SameToolLimit int          // Maximum consecutive calls for same tool
}

// DefaultToolLoopDetectionConfig default config
var DefaultToolLoopDetectionConfig = ToolLoopDetectionConfig{
	MaxCalls:       20,
	TimeWindow:    5 * time.Minute,
	SameToolLimit: 10,
}

// ToolCallRecord Tool call record
type ToolCallRecord struct {
	ToolName string
	CallTime time.Time
	Args     string
}

// ToolLoopDetector Tool loop detector
type ToolLoopDetector struct {
	config     ToolLoopDetectionConfig
	calls      []ToolCallRecord
	lastTool   string
	consecutiveCount int
}

// NewToolLoopDetector Create new loop detector
func NewToolLoopDetector(cfg ToolLoopDetectionConfig) *ToolLoopDetector {
	if cfg.MaxCalls == 0 {
		cfg = DefaultToolLoopDetectionConfig
	}
	return &ToolLoopDetector{
		config:   cfg,
		calls:    make([]ToolCallRecord, 0),
		lastTool: "",
	}
}

// RecordCall Record tool call
func (d *ToolLoopDetector) RecordCall(toolName string, args string) {
	now := time.Now()
	
	// Cleanup old records
	d.calls = d.cleanupOldCalls(now)
	
	// Record new call
	d.calls = append(d.calls, ToolCallRecord{
		ToolName: toolName,
		CallTime: now,
		Args:     args,
	})
	
	// Update consecutive count
	if toolName == d.lastTool {
		d.consecutiveCount++
	} else {
		d.consecutiveCount = 1
	}
	d.lastTool = toolName
}

// CheckLoop Check for loop
func (d *ToolLoopDetector) CheckLoop() (bool, string) {
	now := time.Now()
	d.calls = d.cleanupOldCalls(now)
	
	// Check total call count
	if len(d.calls) >= d.config.MaxCalls {
		return true, fmt.Sprintf("Tool call count exceeds limit (>= %d)", d.config.MaxCalls)
	}
	
	// Check same tool consecutive calls
	if d.consecutiveCount >= d.config.SameToolLimit {
		return true, fmt.Sprintf("Tool '%s' consecutive calls exceed limit (>= %d)", 
			d.lastTool, d.config.SameToolLimit)
	}
	
	// Check duplicate pattern (same tool+similar args)
	if len(d.calls) >= 3 {
		last3 := d.calls[len(d.calls)-3:]
		if last3[0].ToolName == last3[2].ToolName && 
		   last3[0].Args == last3[2].Args &&
		   last3[1].ToolName == last3[0].ToolName {
			return true, fmt.Sprintf("Detected duplicate tool call pattern: %s", last3[0].ToolName)
		}
	}
	
	return false, ""
}

// cleanupOldCalls Cleanup old records
func (d *ToolLoopDetector) cleanupOldCalls(now time.Time) []ToolCallRecord {
	cutoff := now.Add(-d.config.TimeWindow)
	result := make([]ToolCallRecord, 0)
	for _, call := range d.calls {
		if call.CallTime.After(cutoff) {
			result = append(result, call)
		}
	}
	return result
}

// GetStats Get stats
func (d *ToolLoopDetector) GetStats() string {
	now := time.Now()
	d.calls = d.cleanupOldCalls(now)
	
	var sb strings.Builder
	sb.WriteString("Tool call stats:\n")
	fmt.Fprintf(&sb, "  Total calls: %d\n", len(d.calls))
	fmt.Fprintf(&sb, "  Time window: %v\n", d.config.TimeWindow)
	fmt.Fprintf(&sb, "  Last tool: %s\n", d.lastTool)
	fmt.Fprintf(&sb, "  Consecutive count: %d\n", d.consecutiveCount)
	
	return sb.String()
}
