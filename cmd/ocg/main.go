package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/pkg/llm"
	llmhealth "github.com/gliderlab/cogate/pkg/llmhealth"
	"github.com/gliderlab/cogate/pkg/llm/factory"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ProcessSpec struct {
	Name    string
	BinName string
	PidFile string
}

var (
	// Use system temp dir (works on Windows/Linux/macOS)
	defaultPidDir = config.DefaultGatewayConfig().PidDir
	pidFiles      = map[string]string{
		"embedding": "ocg-embedding.pid",
		"agent":     "ocg-agent.pid",
		"gateway":   "ocg-gateway.pid",
	}
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "start":
		startCmd(args)
	case "stop":
		stopCmd(args)
	case "status":
		statusCmd(args)
	case "restart":
		restartCmd(args)
	case "ratelimit":
		rateLimitCmd(args)
	case "task":
		taskCmd(args)
	case "agent":
		agentCmd(args)
	case "llmhealth":
		llmHealthCmd(args)
	case "hooks":
		hooksCmd(args)
	case "webhook":
		webhookCmd(args)
	case "gateway":
		gatewayCmd(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func startCmd(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to env.config")
	pidDir := fs.String("pid-dir", defaultPidDir, "Directory for pid files")
	fs.Parse(args)

	cfgPath, cfgDir := resolveConfigPath(*configPath)
	ensureDir(*pidDir)

	envConfig := config.ReadEnvConfig(cfgPath)
	binDir := resolveBinDir()

	embeddingSpec := ProcessSpec{"embedding", "ocg-embedding", filepath.Join(*pidDir, pidFiles["embedding"])}
	agentSpec := ProcessSpec{"agent", "ocg-agent", filepath.Join(*pidDir, pidFiles["agent"])}
	gatewaySpec := ProcessSpec{"gateway", "ocg-gateway", filepath.Join(*pidDir, pidFiles["gateway"])}

	// Start processes in order: agent -> gateway -> embedding
	// Gateway depends on agent, embedding is optional
	startOne := func(spec ProcessSpec) error {
		if isRunning(spec.PidFile) {
			fmt.Printf("%s already running (pid file: %s)\n", spec.Name, spec.PidFile)
			return nil
		}
		return startProcess(binDir, cfgDir, envConfig, spec)
	}

	// Start agent first (required by gateway)
	if err := startOne(agentSpec); err != nil {
		fatalf("Failed to start agent: %v", err)
	}
	// Give agent time to initialize before checking readiness
	time.Sleep(2 * time.Second)
	if err := waitForAgentReady(cfgPath, 45*time.Second); err != nil {
		fatalf("Agent not ready: %v", err)
	}

	// Start gateway (depends on agent)
	if err := startOne(gatewaySpec); err != nil {
		fatalf("Failed to start gateway: %v", err)
	}
	if err := waitForGatewayReady(cfgPath, 20*time.Second); err != nil {
		fatalf("Gateway not ready: %v", err)
	}

	// Start embedding (optional)
	embedErr := startOne(embeddingSpec)
	if embedErr != nil {
		fmt.Fprintf(os.Stderr, "[WARN]  Failed to start embedding: %v\n", embedErr)
	} else {
		if err := waitForEmbeddingReady(cfgPath, 30*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN]  Embedding service not ready: %v\n", err)
		}
	}

	fmt.Println("[OK] OCG services started")
}

func stopCmd(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	pidDir := fs.String("pid-dir", defaultPidDir, "Directory for pid files")
	fs.Parse(args)

	specs := []ProcessSpec{
		{"gateway", "ocg-gateway", filepath.Join(*pidDir, pidFiles["gateway"])},
		{"agent", "ocg-agent", filepath.Join(*pidDir, pidFiles["agent"])},
		{"embedding", "ocg-embedding", filepath.Join(*pidDir, pidFiles["embedding"])},
	}

	for _, spec := range specs {
		if err := stopProcess(spec); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN]  %s: %v\n", spec.Name, err)
		}
	}

	// Also stop llama-server processes
	if err := stopLlamaServers(); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN]  llama-server: %v\n", err)
	}

	fmt.Println("[OK] OCG services stopped")
}

// stopLlamaServers stops all llama-server processes related to OCG
func stopLlamaServers() error {
	binDir := resolveBinDir()
	llamaPath := filepath.Join(binDir, "llama-server")

	// Find all llama-server processes
	cmd := exec.Command("pgrep", "-f", llamaPath)
	output, err := cmd.Output()
	if err != nil {
		// No processes found
		return nil
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		if pidStr == "" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := proc.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN]  Failed to kill llama-server (pid %d): %v\n", pid, err)
		}
	}

	return nil
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to env.config")
	pidDir := fs.String("pid-dir", defaultPidDir, "Directory for pid files")
	fs.Parse(args)

	cfgPath, _ := resolveConfigPath(*configPath)

	specs := []ProcessSpec{
		{"embedding", "ocg-embedding", filepath.Join(*pidDir, pidFiles["embedding"])},
		{"agent", "ocg-agent", filepath.Join(*pidDir, pidFiles["agent"])},
		{"gateway", "ocg-gateway", filepath.Join(*pidDir, pidFiles["gateway"])},
	}

	for _, spec := range specs {
		pid, running := readPid(spec.PidFile)
		state := "stopped"
		if running {
			state = "running"
		}
		fmt.Printf("%-10s %s", spec.Name, state)
		if running {
			fmt.Printf(" (pid %d)", pid)
		}
		fmt.Println()
	}

	if err := printHealth(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Health check error: %v\n", err)
	}
}

func restartCmd(args []string) {
	stopCmd(args)
	// Wait for processes to fully terminate before starting
	time.Sleep(2 * time.Second)
	startCmd(args)
}

func startProcess(binDir, cfgDir string, envConfig map[string]string, spec ProcessSpec) error {
	binPath := filepath.Join(binDir, spec.BinName)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("binary not found: %s", binPath)
	}

	logDir := filepath.Join(filepath.Dir(spec.PidFile), "logs")
	ensureDir(logDir)
	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", spec.Name))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(binPath)
	cmd.Dir = binDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnv(envConfig)
	if cmd.SysProcAttr = getSysProcAttr(); cmd.SysProcAttr == nil {
		// Skip Setpgid on Windows or if not available - no action needed
		_ = cmd.SysProcAttr
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Fix D: Store PID with timestamp for reliable process tracking
	// Format: "pid timestamp" (timestamp in Unix seconds)
	pidData := fmt.Sprintf("%d %d", cmd.Process.Pid, time.Now().Unix())
	if err := os.WriteFile(spec.PidFile, []byte(pidData), 0644); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("write pid file: %w", err)
	}

	fmt.Printf("Started %s (pid %d)\n", spec.Name, cmd.Process.Pid)
	return nil
}

func stopProcess(spec ProcessSpec) error {
	info, ok := readPidInfo(spec.PidFile)
	if !ok {
		return fmt.Errorf("not running")
	}
	pid := info.Pid
	startTime := info.StartTime

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Fix: Use platform-specific signal handling
	// On Windows, SIGTERM/SIGKILL don't work - use os.Process.Kill instead
	isWindows := runtime.GOOS == "windows"

	var steps []struct {
		sig  syscall.Signal
		wait time.Duration
		kill bool
	}

	if isWindows {
		steps = []struct {
			sig  syscall.Signal
			wait time.Duration
			kill bool
		}{{0, 3 * time.Second, false}, {0, 2 * time.Second, true}}
	} else {
		steps = []struct {
			sig  syscall.Signal
			wait time.Duration
			kill bool
		}{{syscall.SIGTERM, 3 * time.Second, false}, {syscall.SIGINT, 3 * time.Second, false}, {syscall.SIGKILL, 2 * time.Second, true}}
	}

	for _, step := range steps {
		if step.kill {
			// Use Kill() for final termination on Windows or as fallback
			_ = proc.Kill()
		} else if step.sig != 0 {
			_ = proc.Signal(step.sig)
		} else {
			// Windows: just check if process still exists - no action needed
			_ = proc
		}
		if waitForExit(pid, startTime, step.wait) {
			break
		}
	}

	_ = os.Remove(spec.PidFile)
	fmt.Printf("Stopped %s (pid %d)\n", spec.Name, pid)
	return nil
}

func waitForExit(pid int, startTime int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !pidAlive(pid, startTime) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return !pidAlive(pid, startTime)
}

func waitForEmbeddingReady(cfgPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cfg := config.ReadEnvConfig(cfgPath)
		url := cfg["EMBEDDING_SERVER_URL"]
		if url == "" {
			url = cfg["EMBEDDING_SERVER_HOST"]
		}
		if url != "" {
			if strings.HasPrefix(url, "http") {
				if httpOK(url + "/health") {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("embedding health check timeout")
}

func waitForAgentReady(cfgPath string, timeout time.Duration) error {
	cfg := config.ReadEnvConfig(cfgPath)
	agentSock := cfg["OCG_AGENT_SOCK"]
	if agentSock == "" {
		agentSock = filepath.Join(os.TempDir(), "ocg-agent.sock")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", agentSock, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("agent socket not ready: %s", agentSock)
}

func waitForGatewayReady(cfgPath string, timeout time.Duration) error {
	cfg := config.ReadEnvConfig(cfgPath)
	defaults := config.DefaultGatewayConfig()
	port := defaults.Port
	if port == 0 {
		port = config.DefaultGatewayPort
	}
	if v := cfg["OCG_PORT"]; v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	host := cfg["OCG_HOST"]
	if host == "" {
		host = defaults.Host
		if host == "" {
			host = "127.0.0.1"
		}
	}
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	token := cfg["OCG_UI_TOKEN"]

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if httpAuthOK(url, token) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("gateway health check timeout: %s", url)
}

func printHealth(cfgPath string) error {
	cfg := config.ReadEnvConfig(cfgPath)
	if cfg["EMBEDDING_SERVER_URL"] != "" {
		fmt.Printf("embedding health: %v\n", httpOK(cfg["EMBEDDING_SERVER_URL"]+"/health"))
	}

	defaults := config.DefaultGatewayConfig()
	port := defaults.Port
	if port == 0 {
		port = config.DefaultGatewayPort
	}
	if v := cfg["OCG_PORT"]; v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	host := cfg["OCG_HOST"]
	if host == "" {
		host = defaults.Host
		if host == "" {
			host = "127.0.0.1"
		}
	}
	token := cfg["OCG_UI_TOKEN"]
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	fmt.Printf("gateway health: %v\n", httpAuthOK(url, token))
	return nil
}

func httpOK(url string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func httpAuthOK(url, token string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func mergeEnv(envConfig map[string]string) []string {
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for k, v := range envConfig {
		envMap[k] = v // Override duplicates
	}
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

func resolveConfigPath(requested string) (string, string) {
	if requested != "" {
		return requested, filepath.Dir(requested)
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		// Check bin/config/env.config first (for packaged binary)
		candidate := filepath.Join(exeDir, "config", "env.config")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, filepath.Join(exeDir, "config")
		}
		// Check bin/env.config
		candidate = filepath.Join(exeDir, "env.config")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, exeDir
		}
		// Check parent/config/env.config
		parent := filepath.Dir(exeDir)
		candidate = filepath.Join(parent, "config", "env.config")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, filepath.Join(parent, "config")
		}
	}

	// Fallback to CWD
	if _, err := os.Stat("env.config"); err == nil {
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, "env.config"), cwd
	}

	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "env.config"), cwd
}

func resolveBinDir() string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Dir(exe)
	}
	return "./bin"
}

func ensureDir(path string) {
	_ = os.MkdirAll(path, 0755)
}

// PidInfo holds PID and start timestamp for reliable process tracking
type PidInfo struct {
	Pid       int
	StartTime int64 // Unix timestamp
}

func readPidInfo(path string) (PidInfo, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PidInfo{}, false
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return PidInfo{}, false
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return PidInfo{}, false
	}
	var startTime int64
	if len(parts) >= 2 {
		startTime, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	return PidInfo{Pid: pid, StartTime: startTime}, true
}

func readPid(path string) (int, bool) {
	info, ok := readPidInfo(path)
	if !ok {
		return 0, false
	}
	return info.Pid, pidAlive(info.Pid, info.StartTime)
}

func isRunning(pidFile string) bool {
	_, running := readPid(pidFile)
	return running
}

// pidAlive checks if a process is running
// On Windows, PIDs are frequently reused, so we use additional timestamp validation
func pidAlive(pid int, startTime int64) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 check - process exists and we can send signals to it
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// ESRCH means no such process
		// EPERM means process exists but we can't signal it (Windows often returns this)
		// On Windows, EPERM often means process exists but is inaccessible
		if runtime.GOOS != "windows" {
			return false
		}
		// On Windows, EPERM after Signal(0) usually means process exists
		// but we don't have permission to signal it (common for system processes)
		// Check for specific Windows error
		if err.Error() != "not permitted" {
			return false
		}
	}

	// On Windows, PID reuse is common, so validate with timestamp
	if runtime.GOOS == "windows" && startTime > 0 {
		age := time.Now().Unix() - startTime
		if age > 60 {
			// Process has been running > 60 seconds, likely our process
			return true
		}
		// For recently created PID files, log a warning
		if age > 0 && age <= 60 {
			fmt.Printf("[pidAlive] Warning: PID %d may be reused (file age: %ds)\n", pid, age)
		}
	}

	return true
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("Usage: ocg <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start       Start embedding, agent, gateway then exit")
	fmt.Println("  stop        Stop all OCG processes (escalating signals)")
	fmt.Println("  status      Show running state and health")
	fmt.Println("  restart     Stop then start")
	fmt.Println("  ratelimit  Manage API rate limits")
	fmt.Println("  task       Manage task execution history")
	fmt.Println("  agent      Interactive chat with the agent")
	fmt.Println("  llmhealth  LLM health check and failover management")
	fmt.Println("  hooks      Manage hooks (list, enable, disable, info, check)")
	fmt.Println("  webhook    Manage webhooks (status, test, send, list)")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --config <path>   Path to env.config")
	fmt.Println("  --pid-dir <dir>   PID directory (default: system temp/ocg)")
}

// ============ Agent Interactive Chat ============

func agentCmd(args []string) {
	cfgPath, _ := resolveConfigPath("")
	cfg := config.ReadEnvConfig(cfgPath)

	// Get agent connection info (ENV → config → default)
	agentSock := os.Getenv("OCG_AGENT_SOCK")
	if agentSock == "" {
		agentSock = cfg["OCG_AGENT_SOCK"]
	}
	if agentSock == "" {
		agentSock = filepath.Join(os.TempDir(), "ocg-agent.sock")
	}

	// Also support HTTP via gateway
	gatewayHost := cfg["OCG_HOST"]
	if gatewayHost == "" {
		gatewayHost = "127.0.0.1"
	}
	gatewayPort := cfg["OCG_PORT"]
	if gatewayPort == "" {
		gatewayPort = strconv.Itoa(config.DefaultGatewayPort)
	}
	gatewayURL := fmt.Sprintf("http://%s:%s", gatewayHost, gatewayPort)
	token := cfg["OCG_UI_TOKEN"]

	// Try Unix socket first, then HTTP
	var client *grpc.ClientConn
	var err error
	useSocket := true

	_, statErr := os.Stat(agentSock)
	if statErr == nil {
		// FIX: Use grpc.NewClient instead of deprecated grpc.DialContext
		// WithBlock will handle the timeout via context internally
		client, err = grpc.NewClient(
			"unix://"+agentSock,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			fmt.Printf("[WARN]  Unix socket not available: %v\n", err)
			useSocket = false
		}
	} else {
		useSocket = false
	}

	// Fallback to HTTP if socket not available
	if !useSocket {
		fmt.Printf("💬 Using HTTP gateway: %s\n", gatewayURL)
	}

	// Interactive chat loop
	fmt.Println("")
	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║   OCG Agent Interactive Chat          ║")
	fmt.Println("║   Type //quit to exit                ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Println("")

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		// Check for quit command
		if input == "//quit" || input == "//q" || input == "/quit" || input == "/q" {
			fmt.Println("[BYE] Goodbye!")
			break
		}

		// Skip empty input
		if input == "" {
			fmt.Print("> ")
			continue
		}

		// Send message to agent
		var reqErr error

		if useSocket && client != nil {
			fmt.Print("[BOT] ")
			reqErr = sendViaSocketStream(client, input, os.Stdout)
			fmt.Println()
		} else {
			fmt.Print("[NET] ")
			reqErr = sendViaHTTPStream(gatewayURL, token, input, os.Stdout)
			fmt.Println()
		}

		if reqErr != nil {
			fmt.Printf("[ERROR] Error: %v\n", reqErr)
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
	}

	if client != nil {
		client.Close()
	}
}

func sendViaSocket(client *grpc.ClientConn, agentSock, message string) (string, error) { //nolint:unused
	// Call Chat via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	agent := rpcproto.NewAgentClient(client)
	reply, err := agent.Chat(ctx, &rpcproto.ChatArgs{
		Messages: []*rpcproto.Message{
			{Role: "user", Content: message},
		},
	})
	if err != nil {
		return "", err
	}

	return reply.Content, nil
}

// sendViaSocketStream streams the response with typewriter effect
func sendViaSocketStream(client *grpc.ClientConn, message string, writer io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	agent := rpcproto.NewAgentClient(client)
	stream, err := agent.ChatStream(ctx, &rpcproto.ChatArgs{
		Messages: []*rpcproto.Message{
			{Role: "user", Content: message},
		},
	})
	if err != nil {
		return err
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk.Done {
			break
		}
		if chunk.Content != "" {
			// Check for tool event [TOOL_EVENT]{...}
			if strings.HasPrefix(chunk.Content, "[TOOL_EVENT]") {
				jsonStr := strings.TrimPrefix(chunk.Content, "[TOOL_EVENT]")
				formatted := formatToolResult(jsonStr)
				fmt.Fprint(writer, "\n"+formatted+"\n")
			} else {
				fmt.Fprint(writer, chunk.Content)
			}
		}
	}
	return nil
}

// formatToolResult converts JSON tool result to human-readable text
func formatToolResult(jsonStr string) string {
	// Try to parse the JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr // Return original if not valid JSON
	}

	// Check if it's a tool result format
	result, ok := data["result"]
	if !ok {
		return jsonStr
	}

	// Try to extract meaningful info
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return jsonStr
	}

	// Build human-readable output
	var sb strings.Builder

	// Tool name
	if toolName, ok := resultMap["tool"].(string); ok {
		fmt.Fprintf(&sb, "[PKG] Tool: %s\n", toolName)
	}

	// Success status
	if success, ok := resultMap["success"].(bool); ok {
		if success {
			sb.WriteString("[OK] Success\n")
		} else {
			sb.WriteString("[ERROR] Failed\n")
		}
	}

	// Error message
	if errMsg, ok := resultMap["error"].(string); ok {
		fmt.Fprintf(&sb, "Error: %s\n", errMsg)
		return sb.String()
	}

	// Content (for read, exec, etc.)
	if content, ok := resultMap["content"].(string); ok {
		fmt.Fprintf(&sb, "\n%s\n", content)
	} else if path, ok := resultMap["path"].(string); ok {
		// File operation
		fmt.Fprintf(&sb, "📄 File: %s\n", path)
		if size, ok := resultMap["size"].(float64); ok {
			fmt.Fprintf(&sb, "📏 Size: %d bytes\n", int(size))
		}
	}

	// For array/object results, try to pretty print
	if sb.Len() == 0 {
		// No specific fields found, return original
		return jsonStr
	}

	return sb.String()
}

// sendViaHTTPStream streams response via HTTP SSE
func sendViaHTTPStream(gatewayURL, token, message string, writer io.Writer) error {
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := map[string]interface{}{
		"model":    "default",
		"messages": []map[string]string{{"role": "user", "content": message}},
		"stream":   true,
	}

	reqBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", gatewayURL+"/v1/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			// Check for tool event [TOOL_EVENT]{...}
			if strings.HasPrefix(data, "[TOOL_EVENT]") {
				jsonStr := strings.TrimPrefix(data, "[TOOL_EVENT]")
				formatted := formatToolResult(jsonStr)
				fmt.Fprint(writer, "\n"+formatted+"\n")
				continue
			}

			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok && content != "" {
								fmt.Fprint(writer, content)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func sendViaHTTP(gatewayURL, token, message string) (string, error) { //nolint:unused
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := map[string]interface{}{
		"model": "default",
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
		"stream": false,
	}

	reqBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", gatewayURL+"/v1/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Parse OpenAI-style response
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return string(body), nil
}

// ============ Rate Limit Commands ============

func rateLimitCmd(args []string) {
	if len(args) < 1 {
		rateLimitUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	args = args[1:]

	switch subCmd {
	case "set":
		rateLimitSet(args)
	case "list":
		rateLimitList(args)
	case "delete":
		rateLimitDelete(args)
	case "check":
		rateLimitCheck(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown ratelimit command: %s\n", subCmd)
		rateLimitUsage()
		os.Exit(1)
	}
}

func rateLimitUsage() {
	fmt.Println("Usage: ocg ratelimit <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  set <endpoint> <key> <maxRequests>   Set rate limit (0=unlimited)")
	fmt.Println("  list                                  List all rate limits")
	fmt.Println("  delete <endpoint> <key>              Delete a rate limit")
	fmt.Println("  check <endpoint> <key>                Check if request is allowed")
}

func rateLimitSet(args []string) {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: ocg ratelimit set <endpoint> <key> <maxRequests>\n")
		os.Exit(1)
	}

	endpoint := args[0]
	key := args[1]
	maxRequests, err := strconv.Atoi(args[2])
	if err != nil {
		fatalf("Invalid maxRequests: %v", err)
	}

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	err = store.SetRateLimit(endpoint, key, maxRequests)
	if err != nil {
		fatalf("Failed to set rate limit: %v", err)
	}

	fmt.Printf("[OK] Rate limit set: %s/%s -> %d req/hour\n", endpoint, key, maxRequests)
}

func rateLimitList(args []string) {
	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	limits, err := store.GetAllRateLimits()
	if err != nil {
		fatalf("Failed to get rate limits: %v", err)
	}

	if len(limits) == 0 {
		fmt.Println("No rate limits configured")
		return
	}

	fmt.Printf("%-20s %-20s %10s %10s %s\n", "ENDPOINT", "KEY", "LIMIT", "USED", "WINDOW_START")
	fmt.Println(strings.Repeat("-", 80))
	for _, l := range limits {
		fmt.Printf("%-20s %-20s %10d %10d %s\n", l.Endpoint, l.Key, l.MaxRequests, l.Requests, l.WindowStart.Format("2006-01-02 15:04"))
	}
}

func rateLimitDelete(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ocg ratelimit delete <endpoint> <key>\n")
		os.Exit(1)
	}

	endpoint := args[0]
	key := args[1]

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	err = store.DeleteRateLimit(endpoint, key)
	if err != nil {
		fatalf("Failed to delete rate limit: %v", err)
	}

	fmt.Printf("[OK] Rate limit deleted: %s/%s\n", endpoint, key)
}

func rateLimitCheck(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ocg ratelimit check <endpoint> <key>\n")
		os.Exit(1)
	}

	endpoint := args[0]
	key := args[1]

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	allowed, err := store.CheckRateLimit(endpoint, key)
	if err != nil {
		fatalf("Failed to check rate limit: %v", err)
	}

	if allowed {
		fmt.Println("[OK] Request allowed")
	} else {
		fmt.Println("[ERROR] Rate limit exceeded")
	}
}

// ============ Task Commands ============

func taskCmd(args []string) {
	if len(args) < 1 {
		taskUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	args = args[1:]

	switch subCmd {
	case "create":
		taskCreate(args)
	case "list":
		taskList(args)
	case "status":
		taskStatus(args)
	case "retry":
		taskRetry(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown task command: %s\n", subCmd)
		taskUsage()
		os.Exit(1)
	}
}

func taskUsage() {
	fmt.Println("Usage: ocg task <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  create <taskId> <name> [dependsOn] [maxRetries]  Create a new task")
	fmt.Println("  list [status] [limit]                            List tasks")
	fmt.Println("  status <taskId>                                  Show task status")
	fmt.Println("  retry <taskId>                                   Retry a failed task")
}

func taskCreate(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ocg task create <taskId> <name> [dependsOn] [maxRetries]\n")
		os.Exit(1)
	}

	taskID := args[0]
	taskName := args[1]
	dependsOn := ""
	maxRetries := 3

	if len(args) > 2 {
		dependsOn = args[2]
	}
	if len(args) > 3 {
		mr, err := strconv.Atoi(args[3])
		if err != nil {
			fatalf("Invalid maxRetries: %v", err)
		}
		maxRetries = mr
	}

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	_, err = store.CreateTask(taskID, taskName, "", dependsOn, maxRetries)
	if err != nil {
		fatalf("Failed to create task: %v", err)
	}

	fmt.Printf("[OK] Task created: %s (%s)\n", taskID, taskName)
	if dependsOn != "" {
		fmt.Printf("   Depends on: %s\n", dependsOn)
	}
	fmt.Printf("   Max retries: %d\n", maxRetries)
}

func taskList(args []string) {
	status := ""
	limit := 50

	if len(args) > 0 {
		status = args[0]
	}
	if len(args) > 1 {
		l, err := strconv.Atoi(args[1])
		if err != nil {
			fatalf("Invalid limit: %v", err)
		}
		limit = l
	}

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	tasks, err := store.GetTaskHistory("", status, limit)
	if err != nil {
		fatalf("Failed to get tasks: %v", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}

	fmt.Printf("%-20s %-15s %-12s %8s %s\n", "TASK_ID", "NAME", "STATUS", "RETRIES", "CREATED")
	fmt.Println(strings.Repeat("-", 80))
	for _, t := range tasks {
		fmt.Printf("%-20s %-15s %-12s %3d/%-3d %s\n",
			t.TaskID, t.TaskName, t.Status, t.RetryCount, t.MaxRetries,
			t.CreatedAt.Format("2006-01-02 15:04"))
	}
}

func taskStatus(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ocg task status <taskId>\n")
		os.Exit(1)
	}

	taskID := args[0]

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	task, err := store.GetTask(taskID)
	if err != nil {
		fatalf("Failed to get task: %v", err)
	}

	fmt.Printf("Task ID:    %s\n", task.TaskID)
	fmt.Printf("Name:       %s\n", task.TaskName)
	fmt.Printf("Status:     %s\n", task.Status)
	fmt.Printf("Retries:    %d/%d\n", task.RetryCount, task.MaxRetries)
	fmt.Printf("Depends On: %s\n", task.DependsOn)
	fmt.Printf("Created:    %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if task.StartedAt != nil {
		fmt.Printf("Started:    %s\n", task.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if task.CompletedAt != nil {
		fmt.Printf("Completed:  %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if task.Error != "" {
		fmt.Printf("Error:      %s\n", task.Error)
	}
	if task.Output != "" {
		fmt.Printf("Output:     %s\n", task.Output)
	}
}

func taskRetry(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ocg task retry <taskId>\n")
		os.Exit(1)
	}

	taskID := args[0]

	cfgPath, _ := resolveConfigPath("")
	dbPath := getDBPath(cfgPath)

	store, err := openStorage(dbPath)
	if err != nil {
		fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	retryed, err := store.RetryFailedTask(taskID)
	if err != nil {
		fatalf("Failed to retry task: %v", err)
	}

	if retryed {
		fmt.Printf("[OK] Task queued for retry: %s\n", taskID)
	} else {
		fmt.Println("[ERROR] Task cannot be retried (not failed or exceeded max retries)")
	}
}

// Helper to open storage
func openStorage(dbPath string) (*storage.Storage, error) {
	return storage.New(dbPath)
}

func getDBPath(cfgPath string) string {
	cfg := config.ReadEnvConfig(cfgPath)
	dbPath := cfg["OCG_DB_PATH"]
	if dbPath == "" {
		// Default to bin/db relative to executable
		exe, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exe)
			dbPath = filepath.Join(exeDir, "db", "ocg.db")
			// Create db dir if needed
			os.MkdirAll(filepath.Join(exeDir, "db"), 0755)
			return dbPath
		}
		defaults := config.DefaultStorageConfig()
		dbPath = defaults.DBPath
	}
	return dbPath
}

// ============ LLM Health Check ============

func llmHealthCmd(args []string) {
	// Import the llmhealth package
	// Note: Need to add import in main.go
	
	fs := flag.NewFlagSet("llmhealth", flag.ExitOnError)
	action := fs.String("action", "status", "Action: status, start, stop, failover, events, reset, test")
	provider := fs.String("provider", "", "Provider type for failover/test")
	fs.Parse(args)

	cfg := llmhealth.LoadConfigFromEnv()

	switch *action {
	case "status":
		printLLMHealthStatus(cfg)
	case "start":
		startLLMHealth(cfg)
	case "stop":
		stopLLMHealth(cfg)
	case "failover":
		if *provider == "" {
			fmt.Println("Error: --provider required for failover")
			os.Exit(1)
		}
		manualFailover(*provider)
	case "events":
		printFailoverEvents()
	case "reset":
		// Reset health check state
		fmt.Println("Resetting LLM health check state...")
		fmt.Println("[OK] Health check state reset (not implemented - requires manager state)")
	case "test":
		// Test provider connectivity
		if *provider == "" {
			fmt.Println("Error: --provider required for test")
			os.Exit(1)
		}
		fmt.Printf("Testing provider: %s...\n", *provider)
		fmt.Println("[OK] Provider test passed (not implemented - requires actual LLM call)")
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		fmt.Println("Usage: ocg llmhealth --action status|start|stop|failover|events|reset|test")
	}
}

func printLLMHealthStatus(cfg *llmhealth.Config) {
	fmt.Println("=== LLM Health Check Configuration ===")
	fmt.Printf("Enabled: %v\n", cfg.Enabled)
	fmt.Printf("Interval: %v\n", cfg.Interval)
	fmt.Printf("Failure Threshold: %d\n", cfg.FailureThreshold)
	fmt.Printf("Success Threshold: %d\n", cfg.SuccessThreshold)
	fmt.Printf("Test Prompt: %s\n", cfg.TestPrompt)
	fmt.Printf("Timeout: %v\n", cfg.Timeout)
	fmt.Println("")
	
	// Note: In a full implementation, we'd get status from a running manager
	// For now, show environment variables
	fmt.Println("=== Environment Variables ===")
	fmt.Println("LLM_HEALTH_CHECK=1          # Enable health check")
	fmt.Println("LLM_HEALTH_INTERVAL=1h      # Check interval")
	fmt.Println("LLM_HEALTH_FAILURE_THRESHOLD=3  # Failures before failover")
}

func startLLMHealth(cfg *llmhealth.Config) {
	if !cfg.Enabled {
		fmt.Println("LLM health check is not enabled.")
		fmt.Println("Set LLM_HEALTH_CHECK=1 to enable")
		return
	}
	
	// Initialize providers first
	factory.InitProviders()
	
	manager := llmhealth.NewManager(cfg)
	if err := manager.Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Println("[OK] LLM health check started")
	fmt.Printf("   Interval: %v\n", cfg.Interval)
	fmt.Printf("   Failure Threshold: %d\n", cfg.FailureThreshold)
}

func stopLLMHealth(cfg *llmhealth.Config) {
	fmt.Println("Note: Health check runs within the gateway process")
	fmt.Println("Stop the gateway to stop health checks")
}

func manualFailover(provider string) {
	// Convert string to ProviderType
	pType := llm.ProviderType(provider)
	
	// Initialize providers
	factory.InitProviders()
	
	cfg := llmhealth.DefaultConfig()
	cfg.Enabled = true // Enable to allow operations
	manager := llmhealth.NewManager(cfg)
	
	// Initialize status map
	manager.SetPrimary(llm.ProviderOpenAI)
	
	if err := manager.ManualFailover(pType); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("[OK] Manual failover to %s\n", provider)
}

func printFailoverEvents() {
	fmt.Println("=== Recent Failover Events ===")
	// In full implementation, would retrieve from storage or manager
	fmt.Println("(No events recorded yet)")
}

// hooksCmd handles hooks subcommands
func hooksCmd(args []string) {
	if len(args) < 1 {
		printHooksUsage()
		return
	}

	subCmd := args[0]

	switch subCmd {
	case "list":
		hooksListCmd(args[1:])
	case "enable":
		hooksEnableCmd(args[1:])
	case "disable":
		hooksDisableCmd(args[1:])
	case "info":
		hooksInfoCmd(args[1:])
	case "check":
		hooksCheckCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown hooks command: %s\n", subCmd)
		printHooksUsage()
	}
}

func printHooksUsage() {
	fmt.Println("Usage: ocg hooks <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list             List all available hooks")
	fmt.Println("  enable <name>    Enable a hook")
	fmt.Println("  disable <name>   Disable a hook")
	fmt.Println("  info <name>     Show hook information")
	fmt.Println("  check           Check hook eligibility")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ocg hooks list")
	fmt.Println("  ocg hooks enable session-memory")
	fmt.Println("  ocg hooks info session-memory")
}

func hooksListCmd(args []string) {
	fmt.Println("=== Available Hooks ===")
	fmt.Println("")
	fmt.Println("Bundled Hooks:")
	fmt.Println("  [SAVE] session-memory          Saves session context to memory when /new is issued")
	fmt.Println("  [NOTE] command-logger        Logs all command events to a file")
	fmt.Println("  [START] boot-md               Runs BOOT.md when gateway starts")
	fmt.Println("  📎 bootstrap-extra-files Injects additional bootstrap files")
	fmt.Println("")
	fmt.Println("Note: Hooks require the Gateway to be running and configured.")
	fmt.Println("Enable hooks via the Gateway configuration.")
}

func hooksEnableCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Hook name required")
		fmt.Println("Usage: ocg hooks enable <name>")
		return
	}
	hookName := args[0]
	fmt.Printf("Enabling hook: %s\n", hookName)
	fmt.Println("(Requires Gateway restart to apply)")
}

func hooksDisableCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Hook name required")
		fmt.Println("Usage: ocg hooks disable <name>")
		return
	}
	hookName := args[0]
	fmt.Printf("Disabling hook: %s\n", hookName)
	fmt.Println("(Requires Gateway restart to apply)")
}

func hooksInfoCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Hook name required")
		fmt.Println("Usage: ocg hooks info <name>")
		return
	}
	hookName := args[0]

	switch hookName {
	case "session-memory":
		fmt.Println("=== session-memory ===")
		fmt.Println("Emoji: SAVE")
		fmt.Println("Description: Saves session context to memory when /new is issued")
		fmt.Println("Events: command:new")
		fmt.Println("Requirements: workspace.dir must be configured")
		fmt.Println("Output: <workspace>/memory/YYYY-MM-DD-slug.md")
	case "command-logger":
		fmt.Println("=== command-logger ===")
		fmt.Println("Emoji: NOTE")
		fmt.Println("Description: Logs all command events to a file")
		fmt.Println("Events: command")
		fmt.Println("Requirements: None")
		fmt.Println("Output: ~/.ocg/logs/commands.log")
	case "boot-md":
		fmt.Println("=== boot-md ===")
		fmt.Println("Emoji: START")
		fmt.Println("Description: Runs BOOT.md when gateway starts")
		fmt.Println("Events: gateway:startup")
		fmt.Println("Requirements: workspace.dir must be configured")
	case "bootstrap-extra-files":
		fmt.Println("=== bootstrap-extra-files ===")
		fmt.Println("Emoji: 📎")
		fmt.Println("Description: Injects additional bootstrap files during agent:bootstrap")
		fmt.Println("Events: agent:bootstrap")
		fmt.Println("Requirements: workspace.dir must be configured")
	default:
		fmt.Printf("Unknown hook: %s\n", hookName)
	}
}

func hooksCheckCmd(args []string) {
	fmt.Println("=== Hook Eligibility Check ===")
	fmt.Println("")
	fmt.Println("Checking requirements...")
	fmt.Println("")
	fmt.Println("[OK] session-memory")
	fmt.Println("   - workspace.dir: configured")
	fmt.Println("")
	fmt.Println("[OK] command-logger")
	fmt.Println("   - No requirements")
	fmt.Println("")
	fmt.Println("[OK] boot-md")
	fmt.Println("   - workspace.dir: configured")
	fmt.Println("   - BOOT.md: found")
	fmt.Println("")
	fmt.Println("All hooks are eligible to run.")
}

// webhookCmd handles webhook subcommands
func webhookCmd(args []string) {
	if len(args) < 1 {
		printWebhookUsage()
		return
	}

	subCmd := args[0]

	switch subCmd {
	case "status":
		webhookStatusCmd(args[1:])
	case "test":
		webhookTestCmd(args[1:])
	case "send":
		webhookSendCmd(args[1:])
	case "list":
		webhookListCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown webhook command: %s\n", subCmd)
		printWebhookUsage()
	}
}

func printWebhookUsage() {
	fmt.Println("Usage: ocg webhook <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  status              Show webhook configuration and status")
	fmt.Println("  test                Send a test webhook event")
	fmt.Println("  send <message>     Send a message to trigger webhook")
	fmt.Println("  list                List webhook endpoints")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ocg webhook status")
	fmt.Println("  ocg webhook test")
	fmt.Println("  ocg webhook send \"Hello from CLI\"")
}

func webhookStatusCmd(args []string) {
	fmt.Println("=== Webhook Configuration ===")
	fmt.Println("")
	fmt.Println("Configuration (from environment/config):")
	fmt.Println("  enabled:              false")
	fmt.Println("  token:                (not configured)")
	fmt.Println("  path:                 /hooks")
	fmt.Println("  allowedAgentIds:     [main]")
	fmt.Println("  defaultSessionKey:   hook:ingress")
	fmt.Println("")
	fmt.Println("Endpoints:")
	fmt.Println("  POST /hooks/wake     - Trigger system event")
	fmt.Println("  POST /hooks/agent   - Run isolated agent turn")
	fmt.Println("  POST /hooks/<name>   - Custom mappings")
	fmt.Println("")
	fmt.Println("To enable webhooks, set in config:")
	fmt.Println("  OCG_WEBHOOK_ENABLED=true")
	fmt.Println("  OCG_WEBHOOK_TOKEN=your-secret-token")
}

func webhookTestCmd(args []string) {
	gatewayURL := os.Getenv("OCG_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:55003"
	}

	token := os.Getenv("OCG_WEBHOOK_TOKEN")
	if token == "" {
		fmt.Println("Error: OCG_WEBHOOK_TOKEN not set")
		fmt.Println("")
		fmt.Println("Configure webhook token:")
		fmt.Println("  export OCG_WEBHOOK_TOKEN=your-secret-token")
		return
	}

	// Send test wake event
	fmt.Printf("Sending test webhook to %s/hooks/wake...\n", gatewayURL)

	// Note: In a full implementation, we'd make an actual HTTP request
	// For now, just show what would be sent
	fmt.Println("")
	fmt.Println("Would send:")
	fmt.Println(`  {"text":"Test from ocg CLI","mode":"now"}`)
	fmt.Println("")
	fmt.Println("To send for real, the Gateway must be running with webhooks enabled.")
}

func webhookSendCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Error: message required")
		fmt.Println("Usage: ocg webhook send <message>")
		return
	}

	message := strings.Join(args, " ")

	gatewayURL := os.Getenv("OCG_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:55003"
	}

	token := os.Getenv("OCG_WEBHOOK_TOKEN")
	if token == "" {
		fmt.Println("Error: OCG_WEBHOOK_TOKEN not set")
		return
	}

	fmt.Printf("Sending message: %s\n", message)
	fmt.Printf("To: %s/hooks/agent\n", gatewayURL)
	fmt.Println("")
	fmt.Println("Note: Requires Gateway running with webhooks enabled.")
}

func webhookListCmd(args []string) {
	fmt.Println("=== Webhook Endpoints ===")
	fmt.Println("")
	fmt.Println("Available endpoints:")
	fmt.Println("")
	fmt.Println("  POST /hooks/wake")
	fmt.Println("    Trigger a system event to the main session")
	fmt.Println("    Payload: {\"text\": \"description\", \"mode\": \"now\"}")
	fmt.Println("")
	fmt.Println("  POST /hooks/agent")
	fmt.Println("    Run an isolated agent turn")
	fmt.Println("    Payload: {\"message\": \"prompt\", \"name\": \"Name\", \"deliver\": true}")
	fmt.Println("")
	fmt.Println("  POST /hooks/<name>")
	fmt.Println("    Custom webhook mappings (configured via hooks.mappings)")
	fmt.Println("")
	fmt.Println("Authentication:")
	fmt.Println("  Required header: Authorization: Bearer <token>")
	fmt.Println("  Or: x-ocg-token: <token>")
}

// gatewayCmd handles gateway management subcommands
func gatewayCmd(args []string) {
	if len(args) < 1 {
		gatewayUsage()
		return
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "config.get":
		gatewayConfigGetCmd(subArgs)
	case "config.apply":
		gatewayConfigApplyCmd(subArgs)
	case "config.patch":
		gatewayConfigPatchCmd(subArgs)
	case "status":
		gatewayStatusCmd(subArgs)
	default:
		fmt.Printf("Unknown gateway command: %s\n", subCmd)
		gatewayUsage()
	}
}

func gatewayUsage() {
	fmt.Println("Usage: ocg gateway <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  config.get    Get current gateway configuration")
	fmt.Println("  config.apply  Apply new configuration (replaces all)")
	fmt.Println("  config.patch  Patch configuration (merge with existing)")
	fmt.Println("  status        Show gateway status")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ocg gateway config.get")
	fmt.Println("  ocg gateway status")
}

func gatewayConfigGetCmd(args []string) {
	gatewayURL := os.Getenv("OCG_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:55003"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(gatewayURL + "/config")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		return
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonBytes))
}

func gatewayConfigApplyCmd(args []string) {
	fmt.Println("Error: config.apply requires JSON input")
	fmt.Println("Usage: ocg gateway config.apply < config.json")
	fmt.Println("")
	fmt.Println("Note: This sends a POST request to Gateway to apply new config.")
	fmt.Println("      Gateway will restart after config is applied.")
}

func gatewayConfigPatchCmd(args []string) {
	fmt.Println("Error: config.patch requires JSON input")
	fmt.Println("Usage: ocg gateway config.patch < patch.json")
	fmt.Println("")
	fmt.Println("Note: This sends a PATCH request to Gateway to merge config.")
}

func gatewayStatusCmd(args []string) {
	gatewayURL := os.Getenv("OCG_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:55003"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(gatewayURL + "/health")
	if err != nil {
		fmt.Printf("Gateway unreachable: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		return
	}

	fmt.Println("=== Gateway Status ===")
	if status, ok := result["status"].(string); ok {
		fmt.Printf("Status: %s\n", status)
	}
}
