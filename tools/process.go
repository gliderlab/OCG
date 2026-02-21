// Process Tool - manage running processes with optional PTY
package tools

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlab/cogate/pkg/config"
)

// MaxLogBufferSize limits the output buffer to prevent memory exhaustion
const MaxLogBufferSize = 10 * 1024 * 1024 // 10MB

type ProcessInfo struct {
	ID        string
	Cmd       *exec.Cmd
	Buffer    *bytes.Buffer
	Pty       *os.File
	StdinPipe io.WriteCloser
	Mutex     *sync.Mutex
	CreatedAt time.Time
}

type lockedWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

var (
	processes   = make(map[string]*ProcessInfo)
	procMutex   sync.RWMutex
	exitedProcs = make(map[string]int) // sessionId -> exitCode, kept briefly for query
)

type ProcessTool struct{}

func (t *ProcessTool) Name() string {
	return "process"
}

func (t *ProcessTool) Description() string {
	return "Manage processes: start (PTY supported), list, tail logs, write stdin, kill."
}

func (t *ProcessTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: start, list, log, write, kill",
			},
			"sessionId": map[string]interface{}{
				"type":        "string",
				"description": "Process session ID",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to execute (required for start)",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory",
			},
			"env": map[string]interface{}{
				"type":        "string",
				"description": "Environment variables (newline separated)",
			},
			"pty": map[string]interface{}{
				"type":        "boolean",
				"description": "Use PTY (interactive terminal)",
				"default":     false,
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Log start offset",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Log length limit",
			},
			"data": map[string]interface{}{
				"type":        "string",
				"description": "Data to write to stdin",
			},
			"eof": map[string]interface{}{
				"type":        "boolean",
				"description": "Close stdin after write",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ProcessTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := GetString(args, "action")

	switch action {
	case "start":
		return t.start(args)
	case "list":
		return t.list()
	case "log":
		return t.log(args)
	case "write":
		return t.write(args)
	case "kill":
		return t.kill(args)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func resolveProcessWorkdir(workdir string) (string, error) {
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

	resolvedWorkdir, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		resolvedWorkdir = workdir
	}
	absWorkdir, pathErr := filepath.Abs(resolvedWorkdir)
	if pathErr != nil {
		return "", fmt.Errorf("invalid workdir")
	}

	resolvedWorkspace, err := filepath.EvalSymlinks(workspaceDir)
	if err != nil {
		resolvedWorkspace = workspaceDir
	}
	absWorkspace, _ := filepath.Abs(resolvedWorkspace)

	if !strings.HasPrefix(absWorkdir+string(os.PathSeparator), absWorkspace+string(os.PathSeparator)) && absWorkdir != absWorkspace {
		return "", fmt.Errorf("workdir must be within bin/work")
	}
	return absWorkdir, nil
}

func (t *ProcessTool) start(args map[string]interface{}) (interface{}, error) {
	command := GetString(args, "command")
	workdir := GetString(args, "workdir")
	envList := GetString(args, "env")
	usePty := GetBool(args, "pty")

	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Parse command - support quoted arguments
	var cmd *exec.Cmd
	parts := parseCommandArgs(command)
	if len(parts) > 1 {
		cmd = exec.Command(parts[0], parts[1:]...)
	} else {
		cmd = exec.Command(command)
	}

	// Working directory (restricted)
	resolvedWorkdir, err := resolveProcessWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	cmd.Dir = resolvedWorkdir

	// Environment variables
	if envList != "" {
		envs := strings.Split(envList, "\n")
		envs = append(envs, "PATH=/usr/local/bin:/usr/bin:/bin")
		cmd.Env = envs
	}

	var (
		buf       bytes.Buffer
		bufMu     = &sync.Mutex{}
		stdinPipe io.WriteCloser
		ptyFile   *os.File
	)

	if usePty {
		var err error
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to start PTY: %v", err)
		}
	} else {
		// Non-PTY mode
		locked := &lockedWriter{buf: &buf, mu: bufMu}
		cmd.Stdout = locked
		cmd.Stderr = locked
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start: %v", err)
		}
	}

	// Create sessionId
	sessionId := fmt.Sprintf("proc_%d", time.Now().UnixNano())

	procMutex.Lock()
	processes[sessionId] = &ProcessInfo{
		ID:        sessionId,
		Cmd:       cmd,
		Buffer:    &buf,
		Pty:       ptyFile,
		StdinPipe: stdinPipe,
		Mutex:     bufMu,
		CreatedAt: time.Now(),
	}
	procMutex.Unlock()

	log.Printf("[OK] process started: %s (PID: %d, PTY: %v)", sessionId, cmd.Process.Pid, usePty)

	// Read PTY output asynchronously
	if usePty {
		go func() {
			readBuf := make([]byte, 1024)
			for {
				n, err := ptyFile.Read(readBuf)
				if err != nil {
					break
				}
				procMutex.Lock()
				p, ok := processes[sessionId]
				if ok {
					p.Mutex.Lock()
					// Check buffer size before writing to prevent memory exhaustion
					if p.Buffer.Len() < MaxLogBufferSize {
						p.Buffer.Write(readBuf[:n])
					}
					p.Mutex.Unlock()
				}
				procMutex.Unlock()
			}
		}()
	}

	// Wait asynchronously for completion
	go func() {
		cmd.Wait()
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		procMutex.Lock()
		if _, ok := processes[sessionId]; ok {
			log.Printf("[END] process exited: %s (exit code: %d)", sessionId, exitCode)
			// Track exit code briefly for status queries
			exitedProcs[sessionId] = exitCode
			// Schedule cleanup after 5 minutes
			go func() {
				time.Sleep(5 * time.Minute)
				procMutex.Lock()
				delete(processes, sessionId)
				delete(exitedProcs, sessionId)
				procMutex.Unlock()
			}()
		}
		procMutex.Unlock()
	}()

	return ProcessStartResult{
		SessionID: sessionId,
		PID:       cmd.Process.Pid,
		Command:   command,
		Pty:       usePty,
		Success:   true,
	}, nil
}

func (t *ProcessTool) list() (interface{}, error) {
	procMutex.Lock()
	defer procMutex.Unlock()

	items := make([]map[string]interface{}, 0)
	for id, p := range processes {
		var status string
		if p.Cmd.ProcessState == nil {
			status = "running"
		} else if p.Cmd.ProcessState.Exited() {
			status = "exited"
		} else {
			status = "running"
		}

		items = append(items, map[string]interface{}{
			"sessionId": id,
			"pid":       p.Cmd.Process.Pid,
			"status":    status,
			"pty":       p.Pty != nil,
			"createdAt": p.CreatedAt.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"processes": items,
		"count":     len(items),
	}, nil
}

func (t *ProcessTool) log(args map[string]interface{}) (interface{}, error) {
	sessionId := GetString(args, "sessionId")
	offset := GetInt(args, "offset")
	limit := GetInt(args, "limit")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	procMutex.Lock()
	p, ok := processes[sessionId]
	procMutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	p.Mutex.Lock()
	content := p.Buffer.String()
	p.Mutex.Unlock()

	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}

	output := content[offset:]
	if limit > 0 && limit < len(output) {
		output = output[:limit]
	}

	maxLen := 8000
	if len(output) > maxLen {
		output = output[:maxLen]
	}

	return ProcessLogResult{
		SessionID: sessionId,
		Offset:    offset,
		Content:   output,
		Truncated: len(output) < len(content[offset:]),
	}, nil
}

func (t *ProcessTool) write(args map[string]interface{}) (interface{}, error) {
	sessionId := GetString(args, "sessionId")
	data := GetString(args, "data")
	eof := GetBool(args, "eof")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}
	// Allow empty data when using EOF

	procMutex.Lock()
	p, ok := processes[sessionId]
	procMutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	var n int
	var err error

	if p.Pty != nil {
		n, err = p.Pty.Write([]byte(data))
	} else if p.StdinPipe != nil {
		n, err = p.StdinPipe.Write([]byte(data))
	} else {
		return nil, fmt.Errorf("stdin not available")
	}

	if err != nil {
		return nil, fmt.Errorf("write failed: %v", err)
	}

	if eof {
		if p.Pty != nil {
			p.Pty.Close()
			p.Pty = nil
		}
		if p.StdinPipe != nil {
			p.StdinPipe.Close()
			p.StdinPipe = nil
		}
	}

	return map[string]interface{}{
		"sessionId": sessionId,
		"written":   n,
		"eof":       eof,
	}, nil
}

func (t *ProcessTool) kill(args map[string]interface{}) (interface{}, error) {
	sessionId := GetString(args, "sessionId")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	procMutex.Lock()
	p, ok := processes[sessionId]
	procMutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	// Close PTY/stdin
	if p.Pty != nil {
		p.Pty.Close()
	}
	if p.StdinPipe != nil {
		p.StdinPipe.Close()
	}

	if err := p.Cmd.Process.Kill(); err != nil {
		return nil, fmt.Errorf("failed to kill: %v", err)
	}

	procMutex.Lock()
	delete(processes, sessionId)
	procMutex.Unlock()

	return map[string]interface{}{
		"sessionId": sessionId,
		"killed":    true,
	}, nil
}

type ProcessStartResult struct {
	SessionID string `json:"sessionId"`
	PID       int    `json:"pid"`
	Command   string `json:"command"`
	Pty       bool   `json:"pty,omitempty"`
	Success   bool   `json:"success"`
}

type ProcessLogResult struct {
	SessionID string `json:"sessionId"`
	Offset    int    `json:"offset"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// parseCommandArgs parses a command string, supporting quoted arguments
// Example: `ls -la "my file.txt"` -> ["ls", "-la", "my file.txt"]
func parseCommandArgs(command string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range command {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
				quoteChar = rune(0)
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			}
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	if len(args) == 0 {
		return []string{command}
	}

	return args
}
