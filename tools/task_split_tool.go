// TaskSplitTool - explicit task splitting via /split command
package tools

import (
	"fmt"
	"log"
)

// TaskSplitTool provides explicit task splitting via command
type TaskSplitTool struct{}

func (t *TaskSplitTool) Name() string        { return "task_split" }
func (t *TaskSplitTool) Description() string { return "Split and execute complex tasks (use /split command)" }

func (t *TaskSplitTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Task description to split and execute",
			},
		},
		"required": []string{"task"},
	}
}

// Execute splits and executes a task
func (t *TaskSplitTool) Execute(args map[string]interface{}) (interface{}, error) {
	task := GetString(args, "task")
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}

	log.Printf("[TaskSplit] Tool called for: %s", task[:min(50, len(task))])

	return map[string]interface{}{
		"status":  "use_command",
		"message": "Please use /split command instead",
	}, nil
}
