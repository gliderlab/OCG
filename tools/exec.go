// Exec Tool - run shell commands
package tools

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
)

type ExecTool struct{}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute shell commands with timeout control and error handling."
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to execute",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default 30, max 300)",
				"default":     30,
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory (default: current)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecTool) Execute(args map[string]interface{}) (interface{}, error) {
	command := GetString(args, "command")
	timeout := GetInt(args, "timeout")
	workdir := GetString(args, "workdir")

	// Default to binary dir's work subdir if not specified
	workspaceDir := ""
	if exePath, exeErr := os.Executable(); exeErr == nil {
		binDir := filepath.Dir(exePath)
		workspaceDir = filepath.Join(binDir, "work")
	}
	if workspaceDir == "" {
		workspaceDir = config.DefaultWorkspaceDir()
	}
	if workdir == "" {
		workdir = workspaceDir
	}
	// Ensure workdir is within workspace
	resolvedWorkdir, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		resolvedWorkdir = workdir
	}
	absWorkdir, pathErr := filepath.Abs(resolvedWorkdir)
	if pathErr != nil {
		return nil, &ExecError{Message: "invalid workdir"}
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspaceDir)
	if err != nil {
		resolvedWorkspace = workspaceDir
	}
	absWorkspace, _ := filepath.Abs(resolvedWorkspace)
	if !strings.HasPrefix(absWorkdir+string(os.PathSeparator), absWorkspace+string(os.PathSeparator)) && absWorkdir != absWorkspace {
		return nil, &ExecError{Message: "workdir must be within bin/work"}
	}
	workdir = absWorkdir

	if command == "" {
		return nil, &ExecError{Message: "command is required"}
	}

	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		return nil, &ExecError{Message: "timeout cannot exceed 300 seconds"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	allowShell := strings.ToLower(strings.TrimSpace(os.Getenv("OCG_EXEC_ALLOW_SHELL"))) == "true"
	allowlistEnv := strings.TrimSpace(os.Getenv("OCG_EXEC_ALLOWLIST"))
	allowlist := map[string]struct{}{}
	if allowlistEnv != "" {
		for _, item := range strings.Split(allowlistEnv, ",") {
			name := strings.TrimSpace(item)
			if name != "" {
				allowlist[name] = struct{}{}
			}
		}
	}

	// Parse command - try to use shell-less execution for safety
	// If command contains shell metacharacters, use shell only when explicitly allowed
	hasShellChar := strings.ContainsAny(command, "|;&$<>`")
	var cmd *exec.Cmd
	if hasShellChar {
		if !allowShell {
			return nil, &ExecError{Message: "shell features are disabled; use a simple command or enable OCG_EXEC_ALLOW_SHELL"}
		}
		if len(allowlist) > 0 {
			parts := strings.Fields(command)
			if len(parts) == 0 {
				return nil, &ExecError{Message: "empty command"}
			}
			if _, ok := allowlist[parts[0]]; !ok {
				return nil, &ExecError{Message: "command not in allowlist"}
			}
		}
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", command)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
		}
	} else {
		// Parse command without shell for simple commands (safer)
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return nil, &ExecError{Message: "empty command"}
		}
		if len(allowlist) > 0 {
			if _, ok := allowlist[parts[0]]; !ok {
				return nil, &ExecError{Message: "command not in allowlist"}
			}
		}
		if len(parts) == 1 {
			cmd = exec.CommandContext(ctx, parts[0])
		} else {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		}
	}

	// Set working directory
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	result := ExecResult{
		Command:  command,
		Timeout:  timeout,
		Workdir:  workdir,
		Success:  runErr == nil,
		ExitCode: -1,
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	result.Stdout = Truncate(stdout.String(), 10000)
	result.Stderr = Truncate(stderr.String(), 2000)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, &ExecError{
			Message:  "command timed out",
			Metadata: map[string]interface{}{"command": command, "timeout": timeout},
		}
	}

	if runErr != nil {
		result.Error = runErr.Error()
	}

	return result, nil
}

type ExecResult struct {
	Command  string `json:"command"`
	Timeout  int    `json:"timeout"`
	Workdir  string `json:"workdir,omitempty"`
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
}

type ExecError struct {
	Message  string
	Metadata map[string]interface{}
}

func (e *ExecError) Error() string {
	return e.Message
}
