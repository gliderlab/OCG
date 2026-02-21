// Process Tool - process management tool (built into Gateway)
package processtool

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/shlex"
)

type ProcessInfo struct {
	ID              string
	Cmd             *exec.Cmd
	Buffer          *bytes.Buffer
	BufferMu        sync.RWMutex
	Pty             *os.File
	StdinPipe       io.WriteCloser
	CreatedAt       time.Time
	AutoRestart     bool
	MaxRetries      int
	CurrentRetries  int
	RestartDelay    time.Duration
	LastRestartTime time.Time
	HealthCheckFn   func() bool
	Command         string
	WorkDir         string
	Env             string
	UsePty          bool
}

var (
	processes = make(map[string]*ProcessInfo)
	procMutex sync.RWMutex
	// Max buffer size per process (10MB)
	maxBufferSize = 10 * 1024 * 1024
	// Cleanup interval
	cleanupInterval = 5 * time.Minute
	// Process monitoring
	monitorInterval = 10 * time.Second
	monitorEnabled  = true
	monitorMutex    sync.RWMutex
)

// init starts the background cleanup goroutine
func init() {
	go cleanupProcesses()
	go monitorProcesses()
}

// SetMonitorInterval sets the process monitoring interval
func SetMonitorInterval(interval time.Duration) {
	monitorMutex.Lock()
	defer monitorMutex.Unlock()
	monitorInterval = interval
}

// DisableMonitoring disables process monitoring
func DisableMonitoring() {
	monitorMutex.Lock()
	defer monitorMutex.Unlock()
	monitorEnabled = false
}

// EnableMonitoring enables process monitoring
func EnableMonitoring() {
	monitorMutex.Lock()
	defer monitorMutex.Unlock()
	monitorEnabled = true
}

// IsMonitoringEnabled returns if monitoring is enabled
func IsMonitoringEnabled() bool {
	monitorMutex.RLock()
	defer monitorMutex.RUnlock()
	return monitorEnabled
}

// cleanupProcesses removes dead processes from the map every few minutes
func cleanupProcesses() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		// First collect keys to delete (avoid holding lock during iteration)
		var toDelete []string
		procMutex.RLock()
		for id, proc := range processes {
			if proc.Cmd != nil && proc.Cmd.ProcessState != nil && proc.Cmd.ProcessState.Exited() {
				toDelete = append(toDelete, id)
			}
		}
		procMutex.RUnlock()

		// Then delete outside the read lock
		if len(toDelete) > 0 {
			procMutex.Lock()
			for _, id := range toDelete {
				delete(processes, id)
			}
			procMutex.Unlock()
		}
	}
}

// monitorProcesses checks for crashed processes and restarts them if auto-restart is enabled
func monitorProcesses() {
	monitorMutex.RLock()
	interval := monitorInterval
	enabled := monitorEnabled
	monitorMutex.RUnlock()

	if !enabled {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		monitorMutex.RLock()
		enabled = monitorEnabled
		monitorMutex.RUnlock()

		if !enabled {
			continue
		}

		// Collect processes to restart
		var toRestart []struct {
			id     string
			proc   *ProcessInfo
			reason string
		}

		procMutex.RLock()
		for id, proc := range processes {
			// Check if process is dead
			if proc.Cmd == nil || proc.Cmd.ProcessState == nil {
				continue
			}

			exited := proc.Cmd.ProcessState.Exited()

			// If process has exited and auto-restart is enabled
			// NOTE: Don't modify CurrentRetries under read lock!
			if exited && proc.AutoRestart {
				// Collect for later processing (outside lock)
				toRestart = append(toRestart, struct {
					id     string
					proc   *ProcessInfo
					reason string
				}{id, proc, "process exited"})
			}
		}
		procMutex.RUnlock()

		// Process restarts outside the lock
		for _, r := range toRestart {
			// Use write lock for modifying CurrentRetries
			procMutex.Lock()
			if proc, ok := processes[r.id]; ok {
				proc.CurrentRetries++
				currentRetries := proc.CurrentRetries
				maxRetries := proc.MaxRetries
				procMutex.Unlock()

				if currentRetries <= maxRetries {
					log.Printf("[RELOAD] Auto-restarting process %s (reason: %s, retry %d/%d)",
						r.id, r.reason, currentRetries, maxRetries)

					// Wait a bit before restarting
					if r.proc.RestartDelay > 0 {
						time.Sleep(r.proc.RestartDelay)
					}

					// Restart the process
					_, err := restartProcess(r.proc)
					if err != nil {
						log.Printf("[ERROR] Failed to restart process %s: %v", r.id, err)
					} else {
						log.Printf("[OK] Process %s restarted successfully", r.id)
					}
				} else {
					log.Printf("[WARN] Process %s exceeded max retries (%d), not restarting", r.id, maxRetries)
				}
			} else {
				procMutex.Unlock()
			}
		}
	}
}

// restartProcess restarts a process with the same configuration
func restartProcess(proc *ProcessInfo) (map[string]interface{}, error) {
	command := proc.Command
	if command == "" {
		return nil, fmt.Errorf("cannot restart: command not saved")
	}

	// Parse command
	var cmd *exec.Cmd
	if strings.Contains(command, " ") {
		parts, err := shlex.Split(command)
		if err != nil {
			return nil, fmt.Errorf("failed to parse command: %v", err)
		}
		if len(parts) > 1 {
			cmd = exec.Command(parts[0], parts[1:]...)
		} else {
			cmd = exec.Command(parts[0])
		}
	} else {
		cmd = exec.Command(command)
	}

	// Working directory
	if proc.WorkDir != "" {
		cmd.Dir = proc.WorkDir
	}

	// Environment variables
	if proc.Env != "" {
		envs := strings.Split(proc.Env, "\n")
		envs = append(envs, "PATH=/usr/local/bin:/usr/bin:/bin")
		cmd.Env = envs
	}

	var buf bytes.Buffer
	var stdinPipe io.WriteCloser
	var ptyFile *os.File
	var err error

	if proc.UsePty {
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			return nil, fmt.Errorf("PTY start failed: %v", err)
		}
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("start failed: %v", err)
		}
	}

	// Generate new sessionId
	newSessionId := fmt.Sprintf("proc_%d", time.Now().UnixNano())

	procMutex.Lock()
	processes[newSessionId] = &ProcessInfo{
		ID:             newSessionId,
		Cmd:            cmd,
		Buffer:         &buf,
		Pty:            ptyFile,
		StdinPipe:      stdinPipe,
		CreatedAt:      time.Now(),
		AutoRestart:    proc.AutoRestart,
		MaxRetries:     proc.MaxRetries,
		CurrentRetries: 0,
		RestartDelay:   proc.RestartDelay,
		Command:        proc.Command,
		WorkDir:        proc.WorkDir,
		Env:            proc.Env,
		UsePty:         proc.UsePty,
	}
	procMutex.Unlock()

	// Wait for process in background
	go func() {
		cmd.Wait()
	}()

	return map[string]interface{}{
		"sessionId":   newSessionId,
		"pid":         cmd.Process.Pid,
		"autoRestart": proc.AutoRestart,
		"maxRetries":  proc.MaxRetries,
	}, nil
}

type ProcessTool struct{}

func (t *ProcessTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := getString(args, "action")

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
	case "restart":
		return t.restart(args)
	case "autostart":
		return t.setAutoRestart(args)
	case "monitor":
		return t.monitorStatus(args)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ProcessTool) start(args map[string]interface{}) (interface{}, error) {
	command := getString(args, "command")
	workdir := getString(args, "workdir")
	envList := getString(args, "env")
	usePty := getBool(args, "pty")

	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Parse command
	var cmd *exec.Cmd
	if strings.Contains(command, " ") {
		parts, err := shlex.Split(command)
		if err != nil {
			return nil, fmt.Errorf("failed to parse command: %v", err)
		}
		if len(parts) > 1 {
			cmd = exec.Command(parts[0], parts[1:]...)
		} else {
			cmd = exec.Command(parts[0])
		}
	} else {
		cmd = exec.Command(command)
	}

	// Working directory
	if workdir != "" {
		cmd.Dir = workdir
	}

	// Environment variables
	if envList != "" {
		envs := strings.Split(envList, "\n")
		envs = append(envs, "PATH=/usr/local/bin:/usr/bin:/bin")
		cmd.Env = envs
	}

	var (
		buf       bytes.Buffer
		stdinPipe io.WriteCloser
		ptyFile   *os.File
		err       error
	)

	if usePty {
		// PTY mode: pty.Start already started the process
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			return nil, fmt.Errorf("PTY start failed: %v", err)
		}
	} else {
		// Non-PTY mode
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("start failed: %v", err)
		}
	}

	// Generate sessionId
	sessionId := fmt.Sprintf("proc_%d", time.Now().UnixNano())

	// Auto-restart settings
	autoRestart := getBool(args, "autoRestart")
	maxRetries := getInt(args, "maxRetries")
	restartDelay := getInt(args, "restartDelaySeconds")

	procMutex.Lock()
	processes[sessionId] = &ProcessInfo{
		ID:             sessionId,
		Cmd:            cmd,
		Buffer:         &buf,
		Pty:            ptyFile,
		StdinPipe:      stdinPipe,
		CreatedAt:      time.Now(),
		AutoRestart:    autoRestart,
		MaxRetries:     maxRetries,
		CurrentRetries: 0,
		RestartDelay:   time.Duration(restartDelay) * time.Second,
		Command:        command,
		WorkDir:        workdir,
		Env:            envList,
		UsePty:         usePty,
	}
	procMutex.Unlock()

	log.Printf("[OK] Process started: %s (PID: %d, PTY: %v)", sessionId, cmd.Process.Pid, usePty)

	// Fix Bug #4: Use sync.Once and ensure all PTY data is read before process exits
	if usePty {
		var readDone sync.Once
		closeChan := make(chan struct{})

		// Goroutine to read PTY output
		go func() {
			readBuf := make([]byte, 1024)
			for {
				n, err := ptyFile.Read(readBuf)
				if err != nil {
					// Signal that reading is done
					readDone.Do(func() {
						close(closeChan)
					})
					break
				}
				procMutex.Lock()
				p, ok := processes[sessionId]
				if ok {
					p.BufferMu.Lock()
					// Check buffer size and truncate if needed
					if p.Buffer.Len()+n > maxBufferSize {
						// Truncate oldest content
						overflow := p.Buffer.Len() + n - maxBufferSize
						if overflow > 0 {
							content := p.Buffer.String()
							p.Buffer.Reset()
							p.Buffer.WriteString(content[overflow:])
						}
					}
					p.Buffer.Write(readBuf[:n])
					p.BufferMu.Unlock()
				}
				procMutex.Unlock()
			}
		}()

		// Wait for process and then ensure all data is flushed
		go func() {
			cmd.Wait()
			// Wait a bit for remaining PTY data
			time.Sleep(100 * time.Millisecond)
			// Flush any remaining data in PTY
			readBuf := make([]byte, 1024)
			for {
				n, _ := ptyFile.Read(readBuf)
				if n == 0 {
					break
				}
				procMutex.Lock()
				p, ok := processes[sessionId]
				if ok {
					p.BufferMu.Lock()
					if p.Buffer.Len()+n <= maxBufferSize {
						p.Buffer.Write(readBuf[:n])
					}
					p.BufferMu.Unlock()
				}
				procMutex.Unlock()
			}
			// Mark reading as complete
			readDone.Do(func() {
				close(closeChan)
			})
			log.Printf("[END] Process ended: %s (exit code: %d)", sessionId, cmd.ProcessState.ExitCode())
		}()
	} else {
		// Non-PTY mode: just wait for process
		go func() {
			cmd.Wait()
			procMutex.Lock()
			if _, ok := processes[sessionId]; ok {
				log.Printf("[END] Process ended: %s (exit code: %d)", sessionId, cmd.ProcessState.ExitCode())
			}
			procMutex.Unlock()
		}()
	}

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
	sessionId := getString(args, "sessionId")
	offset := getInt(args, "offset")
	limit := getInt(args, "limit")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	procMutex.Lock()
	p, ok := processes[sessionId]
	procMutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	p.BufferMu.RLock()
	content := p.Buffer.String()
	p.BufferMu.RUnlock()

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

	return ProcessLogResult{
		SessionID: sessionId,
		Offset:    offset,
		Content:   output,
		Truncated: len(output) < len(content[offset:]),
	}, nil
}

func (t *ProcessTool) write(args map[string]interface{}) (interface{}, error) {
	sessionId := getString(args, "sessionId")
	data := getString(args, "data")
	eof := getBool(args, "eof")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}
	if data == "" && !eof {
		return nil, fmt.Errorf("data is required")
	}

	procMutex.Lock()
	p, ok := processes[sessionId]
	procMutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	var n int
	var err error

	if data != "" {
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
	sessionId := getString(args, "sessionId")

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
		return nil, fmt.Errorf("kill failed: %v", err)
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

func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(args map[string]interface{}, key string) int {
	if v, ok := args[key]; ok {
		switch f := v.(type) {
		case float64:
			return int(f)
		case int:
			return f
		case string:
			var i int
			fmt.Sscanf(f, "%d", &i)
			return i
		}
	}
	return 0
}

func getBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// restart restarts a running or stopped process
func (t *ProcessTool) restart(args map[string]interface{}) (interface{}, error) {
	sessionId := getString(args, "sessionId")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	procMutex.RLock()
	proc, ok := processes[sessionId]
	procMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	// Kill the existing process
	if proc.Cmd != nil && proc.Cmd.Process != nil {
		proc.Cmd.Process.Kill()
	}

	// Wait for process to exit
	if proc.Cmd != nil {
		proc.Cmd.Wait()
	}

	// Restart
	restartInfo, err := restartProcess(proc)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": "Process restarted successfully",
		"oldId":   sessionId,
		"newInfo": restartInfo,
	}, nil
}

// setAutoRestart configures auto-restart for a process
func (t *ProcessTool) setAutoRestart(args map[string]interface{}) (interface{}, error) {
	sessionId := getString(args, "sessionId")
	enable := getBool(args, "enable")
	maxRetries := getInt(args, "maxRetries")
	restartDelay := getInt(args, "restartDelaySeconds")

	if sessionId == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	procMutex.Lock()
	defer procMutex.Unlock()

	proc, ok := processes[sessionId]
	if !ok {
		return nil, fmt.Errorf("process not found: %s", sessionId)
	}

	proc.AutoRestart = enable
	proc.MaxRetries = maxRetries
	proc.RestartDelay = time.Duration(restartDelay) * time.Second
	if !enable {
		proc.CurrentRetries = 0
	}

	return map[string]interface{}{
		"sessionId":    sessionId,
		"autoRestart":  enable,
		"maxRetries":   maxRetries,
		"restartDelay": restartDelay,
	}, nil
}

// monitorStatus returns monitoring status
func (t *ProcessTool) monitorStatus(args map[string]interface{}) (interface{}, error) {
	showAll := getBool(args, "all")

	procMutex.RLock()
	defer procMutex.RUnlock()

	monitorMutex.RLock()
	enabled := monitorEnabled
	interval := monitorInterval
	monitorMutex.RUnlock()

	items := make([]map[string]interface{}, 0)
	for id, p := range processes {
		if !showAll && !p.AutoRestart {
			continue
		}

		var status string
		if p.Cmd == nil || p.Cmd.ProcessState == nil {
			status = "running"
		} else if p.Cmd.ProcessState.Exited() {
			status = "exited"
		} else {
			status = "running"
		}

		items = append(items, map[string]interface{}{
			"sessionId":      id,
			"pid":            p.Cmd.Process.Pid,
			"status":         status,
			"autoRestart":    p.AutoRestart,
			"maxRetries":     p.MaxRetries,
			"currentRetries": p.CurrentRetries,
			"restartDelay":   p.RestartDelay.Seconds(),
		})
	}

	return map[string]interface{}{
		"monitoring":   enabled,
		"interval":     interval.Seconds(),
		"processes":    items,
		"processCount": len(items),
	}, nil
}
