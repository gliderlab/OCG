// Apply Patch Tool - multi-file structured patch execution
//
// Patch format:
// *** Begin Patch
// *** Add File: path/to/file.txt
// +line 1
// +line 2
// *** Update File: src/app.ts
// @@
// -old line
// +new line
// *** Delete File: obsolete.txt
// *** Move to: newname.txt
// *** End of File
// *** End Patch

package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ApplyPatchTool struct{}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Description() string {
	return "Apply structured multi-file patches. Supports Add/Update/Delete/Move operations."
}

func (t *ApplyPatchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"input": map[string]interface{}{
				"type":        "string",
				"description": "Full patch content with *** Begin Patch and *** End Patch markers",
			},
		},
		"required": []string{"input"},
	}
}

func (t *ApplyPatchTool) Execute(args map[string]interface{}) (interface{}, error) {
	input := GetString(args, "input")
	if input == "" {
		return nil, &ApplyPatchError{Message: "input is required"}
	}

	// Parse and execute the patch
	results, err := parseAndApplyPatch(input)
	if err != nil {
		return nil, &ApplyPatchError{Message: err.Error()}
	}

	return PatchResult{
		Applied:  len(results),
		Results:  results,
		Summary:  buildSummary(results),
	}, nil
}

// parseAndApplyPatch parses the patch format and applies all operations
func parseAndApplyPatch(patch string) ([]PatchOpResult, error) {
	results := []PatchOpResult{}

	// Normalize line endings
	patch = strings.ReplaceAll(patch, "\r\n", "\n")

	lines := strings.Split(patch, "\n")
	state := "seeking_begin" // seeking_begin, seeking_op, in_add, in_update, in_delete
	var currentFile string
	var currentLines []string
	var oldLines []string
	var newLines []string
	var inHunk bool

	flushUpdate := func() error {
		if currentFile != "" && len(oldLines) > 0 || len(newLines) > 0 {
			res, err := applyUpdate(currentFile, oldLines, newLines)
			if err != nil {
				return err
			}
			results = append(results, res)
			currentFile = ""
			oldLines = nil
			newLines = nil
			inHunk = false
		}
		return nil
	}

	for i, line := range lines {
		switch state {
		case "seeking_begin":
			if strings.HasPrefix(line, "*** Begin Patch") {
				state = "seeking_op"
			}

		case "seeking_op":
			if strings.HasPrefix(line, "*** Add File:") {
				// Flush any pending update
				if err := flushUpdate(); err != nil {
					return nil, err
				}
				currentFile = strings.TrimPrefix(line, "*** Add File:")
				currentFile = strings.TrimSpace(currentFile)
				state = "in_add"
				currentLines = nil
			} else if strings.HasPrefix(line, "*** Update File:") {
				if err := flushUpdate(); err != nil {
					return nil, err
				}
				currentFile = strings.TrimPrefix(line, "*** Update File:")
				currentFile = strings.TrimSpace(currentFile)
				state = "in_update"
				oldLines = nil
				newLines = nil
				inHunk = false
			} else if strings.HasPrefix(line, "*** Delete File:") {
				if err := flushUpdate(); err != nil {
					return nil, err
				}
				currentFile = strings.TrimPrefix(line, "*** Delete File:")
				currentFile = strings.TrimSpace(currentFile)
				res, err := applyDelete(currentFile)
				if err != nil {
					return nil, err
				}
				results = append(results, res)
				currentFile = ""
				state = "seeking_op"
			} else if strings.HasPrefix(line, "*** End Patch") {
				if err := flushUpdate(); err != nil {
					return nil, err
				}
				state = "done"
			}

		case "in_add":
			if strings.HasPrefix(line, "***") {
				// New operation, apply current add first
				res, err := applyAdd(currentFile, currentLines)
				if err != nil {
					return nil, err
				}
				results = append(results, res)

				// Process new operation
				if strings.HasPrefix(line, "*** Add File:") {
					currentFile = strings.TrimPrefix(line, "*** Add File:")
					currentFile = strings.TrimSpace(currentFile)
					currentLines = nil
					state = "in_add"
				} else if strings.HasPrefix(line, "*** Update File:") {
					currentFile = strings.TrimPrefix(line, "*** Update File:")
					currentFile = strings.TrimSpace(currentFile)
					oldLines = nil
					newLines = nil
					inHunk = false
					state = "in_update"
				} else if strings.HasPrefix(line, "*** Delete File:") {
					currentFile = strings.TrimPrefix(line, "*** Delete File:")
					currentFile = strings.TrimSpace(currentFile)
					res, err := applyDelete(currentFile)
					if err != nil {
						return nil, err
					}
					results = append(results, res)
					currentFile = ""
					state = "seeking_op"
				} else if strings.HasPrefix(line, "*** End Patch") {
					state = "done"
				}
			} else if strings.HasPrefix(line, "+") {
				currentLines = append(currentLines, strings.TrimPrefix(line, "+"))
			} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") {
				// Non-prefixed line in add mode - treat as content too
				currentLines = append(currentLines, line)
			}

		case "in_update":
			if strings.HasPrefix(line, "@@") {
				inHunk = true
				continue
			}
			if strings.HasPrefix(line, "*** End of File") {
				// EOF marker - remaining newLines go at end
				continue
			}
			if strings.HasPrefix(line, "*** Move to:") && inHunk {
				// Move within update - apply current first then handle move
				res, err := applyUpdate(currentFile, oldLines, newLines)
				if err != nil {
					return nil, err
				}
				results = append(results, res)

				newPath := strings.TrimPrefix(line, "*** Move to:")
				newPath = strings.TrimSpace(newPath)
				res, err = applyMove(currentFile, newPath)
				if err != nil {
					return nil, err
				}
				results = append(results, res)
				currentFile = ""
				oldLines = nil
				newLines = nil
				inHunk = false
				state = "seeking_op"
				continue
			}
			if strings.HasPrefix(line, "***") {
				// End of current update
				res, err := applyUpdate(currentFile, oldLines, newLines)
				if err != nil {
					return nil, err
				}
				results = append(results, res)

				if strings.HasPrefix(line, "*** Add File:") {
					currentFile = strings.TrimPrefix(line, "*** Add File:")
					currentFile = strings.TrimSpace(currentFile)
					currentLines = nil
					state = "in_add"
				} else if strings.HasPrefix(line, "*** Update File:") {
					currentFile = strings.TrimPrefix(line, "*** Update File:")
					currentFile = strings.TrimSpace(currentFile)
					oldLines = nil
					newLines = nil
					inHunk = false
					state = "in_update"
				} else if strings.HasPrefix(line, "*** Delete File:") {
					currentFile = strings.TrimPrefix(line, "*** Delete File:")
					currentFile = strings.TrimSpace(currentFile)
					res, err := applyDelete(currentFile)
					if err != nil {
						return nil, err
					}
					results = append(results, res)
					currentFile = ""
					state = "seeking_op"
				} else if strings.HasPrefix(line, "*** End Patch") {
					state = "done"
				}
				continue
			}

			if inHunk {
				if strings.HasPrefix(line, "-") {
					oldLines = append(oldLines, strings.TrimPrefix(line, "-"))
				} else if strings.HasPrefix(line, "+") {
					newLines = append(newLines, strings.TrimPrefix(line, "+"))
				} else if strings.TrimSpace(line) != "" {
					// Context line - add to both
					oldLines = append(oldLines, strings.TrimPrefix(line, " "))
					newLines = append(newLines, strings.TrimPrefix(line, " "))
				}
			} else {
				// Before @@, lines starting with space are context
				if strings.HasPrefix(line, " ") || strings.TrimSpace(line) == "" {
					// Could be context before hunk
					ctxLine := strings.TrimPrefix(line, " ")
					oldLines = append(oldLines, ctxLine)
					newLines = append(newLines, ctxLine)
				}
			}
		}

		// Safety check
		if i > 10000 {
			return nil, fmt.Errorf("patch too large (max 10000 lines)")
		}
	}

	// Handle any remaining content
	if state == "in_add" && currentFile != "" {
		res, err := applyAdd(currentFile, currentLines)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	} else if state == "in_update" && currentFile != "" {
		res, err := applyUpdate(currentFile, oldLines, newLines)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}

	return results, nil
}

func applyAdd(path string, lines []string) (PatchOpResult, error) {
	absPath, err := IsPathAllowed(path)
	if err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	// Check if file already exists
	if _, err := os.Stat(absPath); err == nil {
		return PatchOpResult{Path: absPath, Status: "skipped", Message: "file already exists"}, nil
	}

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: "cannot create directory: " + err.Error()}, nil
	}

	// Write file
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	return PatchOpResult{Path: absPath, Status: "added", Message: fmt.Sprintf("added %d lines", len(lines))}, nil
}

func applyUpdate(path string, oldLines, newLines []string) (PatchOpResult, error) {
	absPath, err := IsPathAllowed(path)
	if err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: "read failed: " + err.Error()}, nil
	}
	original := string(content)
	modified := original

	// Simple replace: find oldLines as a block and replace with newLines
	if len(oldLines) > 0 {
		oldBlock := strings.Join(oldLines, "\n")
		newBlock := strings.Join(newLines, "\n")

		if strings.Contains(modified, oldBlock) {
			modified = strings.Replace(modified, oldBlock, newBlock, 1)
		} else {
			// Try line by line replacement for more flexibility
			modified = applyPartialUpdate(original, oldLines, newLines)
		}
	} else if len(newLines) > 0 {
		// No old lines = append to end
		modified = original + "\n" + strings.Join(newLines, "\n")
	}

	if modified == original {
		return PatchOpResult{Path: absPath, Status: "skipped", Message: "no changes applied"}, nil
	}

	if err := os.WriteFile(absPath, []byte(modified), 0644); err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	return PatchOpResult{Path: absPath, Status: "updated", Message: fmt.Sprintf("updated %d -> %d lines", len(oldLines), len(newLines))}, nil
}

// applyPartialUpdate tries to apply changes more flexibly
func applyPartialUpdate(content string, oldLines, newLines []string) string {
	// Try to find each old line and replace contextually
	modified := content
	for i, oldLine := range oldLines {
		if oldLine = strings.TrimSpace(oldLine); oldLine == "" {
			continue
		}
		// Find this line in the content
		lines := strings.Split(modified, "\n")
		found := false
		for j, l := range lines {
			if strings.Contains(l, oldLine) {
				// Replace with corresponding new line if exists
				if i < len(newLines) {
					lines[j] = newLines[i]
				}
				found = true
				break
			}
		}
		if found {
			modified = strings.Join(lines, "\n")
		}
	}
	return modified
}

func applyDelete(path string) (PatchOpResult, error) {
	absPath, err := IsPathAllowed(path)
	if err != nil {
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	if err := os.Remove(absPath); err != nil {
		if os.IsNotExist(err) {
			return PatchOpResult{Path: absPath, Status: "skipped", Message: "file does not exist"}, nil
		}
		return PatchOpResult{Path: path, Status: "error", Message: err.Error()}, nil
	}

	return PatchOpResult{Path: absPath, Status: "deleted", Message: "file deleted"}, nil
}

func applyMove(oldPath, newPath string) (PatchOpResult, error) {
	absOldPath, err := IsPathAllowed(oldPath)
	if err != nil {
		return PatchOpResult{Path: oldPath, Status: "error", Message: err.Error()}, nil
	}

	absNewPath, err := IsPathAllowed(newPath)
	if err != nil {
		return PatchOpResult{Path: newPath, Status: "error", Message: err.Error()}, nil
	}

	// Ensure target directory exists
	dir := filepath.Dir(absNewPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return PatchOpResult{Path: oldPath, Status: "error", Message: "cannot create directory: " + err.Error()}, nil
	}

	if err := os.Rename(absOldPath, absNewPath); err != nil {
		return PatchOpResult{Path: oldPath, Status: "error", Message: err.Error()}, nil
	}

	return PatchOpResult{Path: absOldPath, Status: "moved", Message: "moved to " + absNewPath}, nil
}

func buildSummary(results []PatchOpResult) string {
	var summary []string
	counts := map[string]int{
		"added": 0, "updated": 0, "deleted": 0, "moved": 0, "skipped": 0, "error": 0,
	}
	for _, r := range results {
		counts[r.Status]++
	}
	for status, count := range counts {
		if count > 0 {
			summary = append(summary, fmt.Sprintf("%d %s", count, status))
		}
	}
	return strings.Join(summary, ", ")
}

// Types

type PatchResult struct {
	Applied int              `json:"applied"`
	Results []PatchOpResult  `json:"results"`
	Summary string           `json:"summary"`
}

type PatchOpResult struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // added, updated, deleted, moved, skipped, error
	Message string `json:"message"`
}

type ApplyPatchError struct {
	Message string
}

func (e *ApplyPatchError) Error() string {
	return e.Message
}

// Ensure scanner is used
var _ = bufio.Scanner{}
