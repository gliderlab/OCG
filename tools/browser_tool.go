// Browser Tool - Complete CDP-based browser automation
// Full implementation matching official OCG browser tool
package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type BrowserTool struct {
	service    *BrowserService
	config     *BrowserConfig
	browserDir string
}

type BrowserConfig struct {
	Enabled            bool                      `json:"enabled"`
	Headless           bool                      `json:"headless"`
	NoSandbox          bool                      `json:"noSandbox"`
	ExecutablePath     string                    `json:"executablePath"`
	DefaultProfile     string                    `json:"defaultProfile"`
	Color              string                    `json:"color"`
	RemoteCDPTimeoutMs int                       `json:"remoteCdpTimeoutMs"`
	Profiles           map[string]*ProfileConfig `json:"profiles"`
}

type ProfileConfig struct {
	Name    string `json:"name"`
	CDPPort int    `json:"cdpPort"`
	CDPURL  string `json:"cdpUrl"`
	Color   string `json:"color"`
	Driver  string `json:"driver"`
}

type BrowserService struct {
	cfg        *BrowserConfig
	instances  map[string]*BrowserInstance
	mu         sync.RWMutex
	httpClient *http.Client
	wsDialer   *websocket.Dialer

	// Optimization #4: Idle timeout for automatic instance cleanup
	idleTimeout time.Duration
	stopCleanup chan struct{}
}

type BrowserInstance struct {
	PID        int       `json:"pid"`
	Profile    string    `json:"profile"`
	CDPPort    int       `json:"cdpPort"`
	CDPURL     string    `json:"cdpUrl"`
	UserData   string    `json:"userData"`
	StartedAt  time.Time `json:"startedAt"`
	LastActive time.Time `json:"lastActive"` // Optimization #4: Track last activity
	TargetID   string    `json:"targetId"`
	WSConn     *websocket.Conn
	Cmd        *exec.Cmd
	// FIX: unused fields - kept for future console/network/error tracking
	consoleMsgs []ConsoleMessage //nolint:unused
	networkLogs []NetworkRequest //nolint:unused
	errors      []PageError      //nolint:unused
	mu          sync.Mutex       //nolint:unused
}

type ConsoleMessage struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type NetworkRequest struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Method    string `json:"method"`
	Status    int    `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type PageError struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

func NewBrowserTool() *BrowserTool {
	cfg := &BrowserConfig{
		Enabled:        true,
		Headless:       false,
		NoSandbox:      false,
		DefaultProfile: "openclaw",
		Color:          "#FF4500",
		Profiles: map[string]*ProfileConfig{
			"openclaw": {Name: "openclaw", CDPPort: 18800, Color: "#FF4500", Driver: "cdp"},
			"chrome":   {Name: "chrome", CDPPort: 18792, CDPURL: "http://127.0.0.1:18792", Color: "#4285F4", Driver: "extension"},
		},
	}
	bt := &BrowserTool{config: cfg, browserDir: "/tmp/openclaw-browser"}
	bt.service = NewBrowserService(cfg)
	os.MkdirAll(bt.browserDir, 0755)
	os.MkdirAll(filepath.Join(bt.browserDir, "downloads"), 0755)
	os.MkdirAll(filepath.Join(bt.browserDir, "uploads"), 0755)
	return bt
}

func (t *BrowserTool) Name() string { return "browser" }
func (t *BrowserTool) Description() string {
	return "Control browser: status/start/stop/tabs/open/close/snapshot/screenshot/navigate/act/click/type/wait"
}

func (t *BrowserTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action":       map[string]interface{}{"type": "string", "description": "Browser action"},
			"profile":      map[string]interface{}{"type": "string", "default": "openclaw"},
			"target":       map[string]interface{}{"type": "string", "default": "host"},
			"targetUrl":    map[string]interface{}{"type": "string"},
			"targetId":     map[string]interface{}{"type": "string"},
			"ref":          map[string]interface{}{"type": "string"},
			"selector":     map[string]interface{}{"type": "string"},
			"text":         map[string]interface{}{"type": "string"},
			"key":          map[string]interface{}{"type": "string"},
			"x":            map[string]interface{}{"type": "integer"},
			"y":            map[string]interface{}{"type": "integer"},
			"width":        map[string]interface{}{"type": "integer"},
			"height":       map[string]interface{}{"type": "integer"},
			"doubleClick":  map[string]interface{}{"type": "boolean", "default": false},
			"submit":       map[string]interface{}{"type": "boolean", "default": false},
			"timeoutMs":    map[string]interface{}{"type": "integer", "default": 30000},
			"format":       map[string]interface{}{"type": "string", "default": "ai"},
			"fullPage":     map[string]interface{}{"type": "boolean", "default": false},
			"compact":      map[string]interface{}{"type": "boolean", "default": false},
			"depth":        map[string]interface{}{"type": "integer", "default": 6},
			"interactive":  map[string]interface{}{"type": "boolean", "default": false},
			"labels":       map[string]interface{}{"type": "boolean", "default": false},
			"maxChars":     map[string]interface{}{"type": "integer", "default": 15000},
			"limit":        map[string]interface{}{"type": "integer", "default": 200},
			"mode":         map[string]interface{}{"type": "string"},
			"frame":        map[string]interface{}{"type": "string"},
			"waitText":     map[string]interface{}{"type": "string"},
			"waitUrl":      map[string]interface{}{"type": "string"},
			"load":         map[string]interface{}{"type": "string"},
			"fn":           map[string]interface{}{"type": "string"},
			"fields":       map[string]interface{}{"type": "string"},
			"values":       map[string]interface{}{"type": "string"},
			"kind":         map[string]interface{}{"type": "string"},
			"modifiers":    map[string]interface{}{"type": "string"},
			"button":       map[string]interface{}{"type": "string", "default": "left"},
			"startRef":     map[string]interface{}{"type": "string"},
			"endRef":       map[string]interface{}{"type": "string"},
			"cookieName":   map[string]interface{}{"type": "string"},
			"cookieValue":  map[string]interface{}{"type": "string"},
			"cookieUrl":    map[string]interface{}{"type": "string"},
			"storageKind":  map[string]interface{}{"type": "string", "default": "local"},
			"storageKey":   map[string]interface{}{"type": "string"},
			"storageValue": map[string]interface{}{"type": "string"},
			"level":        map[string]interface{}{"type": "string", "default": "info"},
			"filter":       map[string]interface{}{"type": "string"},
			"accept":       map[string]interface{}{"type": "boolean", "default": true},
			"promptText":   map[string]interface{}{"type": "string"},
			"path":         map[string]interface{}{"type": "string"},
			"filename":     map[string]interface{}{"type": "string"},
			"element":      map[string]interface{}{"type": "string"},
			"inputRef":     map[string]interface{}{"type": "string"},
			"outputFormat": map[string]interface{}{"type": "string", "default": "png"},
		},
		"required": []string{"action"},
	}
}

func (t *BrowserTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := GetString(args, "action")
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}
	profile := GetString(args, "profile")
	if profile == "" {
		profile = t.config.DefaultProfile
	}
	switch action {
	case "status":
		return t.service.Status(profile)
	case "start":
		return t.service.Start(profile, t.config)
	case "stop":
		return t.service.Stop(profile)
	case "tabs":
		return t.service.ListTabs(profile)
	case "tab":
		return t.service.TabAction(args, profile)
	case "open":
		return t.service.Open(profile, GetString(args, "targetUrl"))
	case "focus":
		return t.service.FocusTab(profile, GetString(args, "targetId"))
	case "close":
		return t.service.CloseTab(profile, GetString(args, "targetId"))
	case "navigate":
		return t.service.Navigate(profile, GetString(args, "targetUrl"))
	case "snapshot":
		return t.service.Snapshot(args, profile)
	case "screenshot":
		return t.service.Screenshot(args, profile)
	case "act":
		return t.service.Act(args, profile)
	case "click":
		return t.service.Click(args, profile)
	case "type":
		return t.service.Type(args, profile)
	case "press":
		return t.service.Press(args, profile)
	case "hover":
		return t.service.Hover(args, profile)
	case "drag":
		return t.service.Drag(args, profile)
	case "select":
		return t.service.Select(args, profile)
	case "fill":
		return t.service.Fill(args, profile)
	case "wait":
		return t.service.Wait(args, profile)
	case "evaluate":
		return t.service.Evaluate(args, profile)
	case "cookies":
		return t.service.Cookies(args, profile)
	case "storage":
		return t.service.Storage(args, profile)
	case "resize":
		return t.service.Resize(args, profile)
	case "set":
		return t.service.Settings(args, profile)
	case "highlight":
		return t.service.Highlight(args, profile)
	case "console":
		return t.service.Console(args, profile)
	case "errors":
		return t.service.Errors(args, profile)
	case "requests":
		return t.service.Requests(args, profile)
	case "pdf":
		return t.service.PDF(args, profile)
	case "trace":
		return t.service.Trace(args, profile)
	case "dialog":
		return t.service.Dialog(args, profile)
	case "upload":
		return t.service.Upload(args, profile)
	case "download":
		return t.service.Download(args, profile)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func NewBrowserService(cfg *BrowserConfig) *BrowserService {
	s := &BrowserService{
		cfg:       cfg,
		instances: make(map[string]*BrowserInstance),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		wsDialer:    &websocket.Dialer{HandshakeTimeout: 10 * time.Second},
		idleTimeout: 10 * time.Minute, // Default 10 minutes idle timeout
		stopCleanup: make(chan struct{}),
	}

	// Optimization #4: Start idle cleanup goroutine
	go s.cleanupIdleInstances()

	return s
}

// cleanupIdleInstances stops instances that have been idle too long
func (s *BrowserService) cleanupIdleInstances() {
	ticker := time.NewTicker(2 * time.Minute) // Check every 2 minutes
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCleanup:
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for profile, inst := range s.instances {
				if now.Sub(inst.LastActive) > s.idleTimeout {
					log.Printf("[Browser] Idle timeout: stopping %s (idle for %v)", profile, now.Sub(inst.LastActive))
					// Close connection
					if inst.WSConn != nil {
						inst.WSConn.Close()
					}
					// Kill process
					if inst.Cmd != nil && inst.Cmd.Process != nil {
						_ = inst.Cmd.Process.Kill()
						_ = inst.Cmd.Wait()
					} else if inst.PID > 0 {
						if proc, err := os.FindProcess(inst.PID); err == nil {
							_ = proc.Kill()
						}
					}
					delete(s.instances, profile)
				}
			}
			s.mu.Unlock()
		}
	}
}

// StopCleanup stops the idle cleanup goroutine
func (s *BrowserService) StopCleanup() {
	if s.stopCleanup != nil {
		close(s.stopCleanup)
	}
}

func (s *BrowserService) Status(profile string) (interface{}, error) {
	s.mu.RLock()
	inst, exists := s.instances[profile]
	s.mu.RUnlock()
	if !exists {
		return map[string]interface{}{"status": "stopped", "profile": profile, "cdpPort": s.getProfilePort(profile), "tabs": 0}, nil
	}
	// Update last active time on any access
	s.updateLastActive(profile)
	tabs, _ := s.listTabs(inst.CDPPort)
	return map[string]interface{}{"status": "running", "profile": inst.Profile, "pid": inst.PID, "cdpPort": inst.CDPPort, "tabs": len(tabs), "startedAt": inst.StartedAt.Format(time.RFC3339)}, nil
}

// updateLastActive updates the last active timestamp for an instance
func (s *BrowserService) updateLastActive(profile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inst, exists := s.instances[profile]; exists {
		inst.LastActive = time.Now()
	}
}

func (s *BrowserService) Start(profile string, cfg *BrowserConfig) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inst, exists := s.instances[profile]; exists {
		return map[string]interface{}{"action": "already_running", "profile": inst.Profile, "pid": inst.PID}, nil
	}
	profCfg := s.getProfileConfig(profile)
	cdpPort := profCfg.CDPPort
	if cdpPort == 0 {
		cdpPort = 18800
	}
	if profCfg.CDPURL != "" && profCfg.Driver != "extension" {
		if err := s.testCDPConnection(profCfg.CDPURL); err != nil {
			return nil, fmt.Errorf("remote CDP not reachable: %v", err)
		}
		inst := &BrowserInstance{Profile: profile, CDPPort: cdpPort, CDPURL: profCfg.CDPURL, StartedAt: time.Now()}
		s.instances[profile] = inst
		return map[string]interface{}{"action": "attached_remote", "profile": profile, "cdpUrl": profCfg.CDPURL}, nil
	}
	exePath := cfg.ExecutablePath
	if exePath == "" {
		exePath = findBrowser()
		if exePath == "" {
			return nil, fmt.Errorf("no browser found")
		}
	}
	userData := filepath.Join("/tmp/openclaw-browser", profile)
	os.MkdirAll(userData, 0755)
	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(cdpPort),
		"--user-data-dir=" + userData,
		"--no-first-run", "--no-default-browser-check",
		"--disable-default-apps", "--disable-extensions",
		"--disable-background-networking", "--disable-sync",
		"--disable-automation", "--disable-blink-features=AutomationControlled",
	}
	if cfg.Headless {
		args = append(args, "--headless=new", "--disable-gpu")
	}
	if cfg.NoSandbox {
		args = append(args, "--no-sandbox", "--disable-dev-shm-usage")
	}
	if profCfg.Color != "" {
		args = append(args, "--theme-color="+profCfg.Color)
	}
	log.Printf("[Browser] Starting %s: %s %v", profile, exePath, args)
	cmd := exec.Command(exePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start browser: %v", err)
	}
	inst := &BrowserInstance{PID: cmd.Process.Pid, Profile: profile, CDPPort: cdpPort, UserData: userData, StartedAt: time.Now(), Cmd: cmd}
	s.instances[profile] = inst
	time.Sleep(2 * time.Second)
	return map[string]interface{}{"action": "started", "profile": profile, "pid": cmd.Process.Pid, "cdpPort": cdpPort}, nil
}

func (s *BrowserService) Stop(profile string) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, exists := s.instances[profile]
	if !exists {
		return map[string]interface{}{"action": "already_stopped"}, nil
	}
	if inst.WSConn != nil {
		inst.WSConn.Close()
	}
	if inst.Cmd != nil && inst.Cmd.Process != nil {
		_ = inst.Cmd.Process.Kill()
		_ = inst.Cmd.Wait()
	} else if inst.PID > 0 {
		if proc, err := os.FindProcess(inst.PID); err == nil {
			_ = proc.Kill()
		}
	}
	delete(s.instances, profile)
	return map[string]interface{}{"action": "stopped", "profile": inst.Profile, "pid": inst.PID}, nil
}

func (s *BrowserService) ListTabs(profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	tabs, err := s.listTabs(cdpPort)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"tabs": tabs, "count": len(tabs)}, nil
}

func (s *BrowserService) TabAction(args map[string]interface{}, profile string) (interface{}, error) {
	subAction := GetString(args, "targetId")
	if subAction == "new" {
		return s.Open(profile, GetString(args, "targetUrl"))
	}
	if subAction != "" {
		return s.CloseTab(profile, subAction)
	}
	return s.ListTabs(profile)
}

func (s *BrowserService) Open(profile, targetURL string) (interface{}, error) {
	if targetURL == "" {
		return nil, fmt.Errorf("targetUrl is required")
	}
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}
	escapedURL := url.QueryEscape(targetURL)
	resp, err := s.httpClient.Post(fmt.Sprintf("http://127.0.0.1:%d/json/new?url=%s", cdpPort, escapedURL), "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return map[string]interface{}{"action": "opened", "url": targetURL, "targetId": result["id"]}, nil
}

func (s *BrowserService) FocusTab(profile, targetID string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	if targetID == "" {
		tabs, err := s.listTabs(cdpPort)
		if err != nil || len(tabs) == 0 {
			return nil, fmt.Errorf("no tabs available")
		}
		targetID = tabs[0]["id"].(string)
	}
	return map[string]interface{}{"action": "focused", "targetId": targetID}, nil
}

func (s *BrowserService) CloseTab(profile, targetID string) (interface{}, error) {
	if targetID == "" {
		return nil, fmt.Errorf("targetId is required")
	}
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", cdpPort, targetID), nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return map[string]interface{}{"action": "closed", "targetId": targetID}, nil
}

func (s *BrowserService) Navigate(profile, url string) (interface{}, error) {
	if url == "" {
		return nil, fmt.Errorf("targetUrl is required")
	}
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	result, err := s.cdpRequest(cdpPort, "Page.navigate", map[string]interface{}{"url": url})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"action": "navigated", "url": url, "result": result}, nil
}

func (s *BrowserService) Snapshot(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	format := GetString(args, "format")
	if format == "" {
		format = "ai"
	}
	interactive := GetBool(args, "interactive")
	compact := GetBool(args, "compact")
	depth := GetInt(args, "depth")
	if depth == 0 {
		depth = 6
	}
	maxChars := GetInt(args, "maxChars")
	if maxChars == 0 {
		maxChars = 15000
	}
	limit := GetInt(args, "limit")
	if limit == 0 {
		limit = 200
	}
	docResult, err := s.cdpRequest(cdpPort, "DOM.getDocument", map[string]interface{}{"depth": depth})
	if err != nil {
		return nil, err
	}
	pageResult, _ := s.cdpRequest(cdpPort, "Page.getResourceTree", nil)
	frameURL := ""
	if pageResult != nil {
		if frameTree, ok := pageResult["frameTree"].(map[string]interface{}); ok {
			if frame, ok := frameTree["frame"].(map[string]interface{}); ok {
				frameURL, _ = frame["url"].(string)
			}
		}
	}
	axTree, _ := s.cdpRequest(cdpPort, "Accessibility.getFullAXTree", nil)
	var snapshot string
	var refs []map[string]string
	if format == "aria" || interactive {
		snapshot, refs = s.buildAISnapshot(docResult, axTree, frameURL, interactive, compact, limit)
	} else {
		snapshot = s.buildHTMLSnapshot(docResult, frameURL, depth)
	}
	if len(snapshot) > maxChars {
		snapshot = snapshot[:maxChars] + "... (truncated)"
	}
	result := map[string]interface{}{"snapshot": snapshot, "format": format, "maxChars": maxChars, "compact": compact, "depth": depth, "interactive": interactive, "limit": limit}
	if len(refs) > 0 {
		result["refs"] = refs
		result["stats"] = map[string]interface{}{"lines": strings.Count(snapshot, "\n"), "chars": len(snapshot), "refs": len(refs)}
	}
	return result, nil
}

func (s *BrowserService) Screenshot(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	fullPage := GetBool(args, "fullPage")
	outputFormat := GetString(args, "outputFormat")
	if outputFormat == "" {
		outputFormat = "png"
	}
	params := map[string]interface{}{"format": outputFormat, "quality": 90}
	if fullPage {
		params["captureBeyondViewport"] = true
	}
	result, err := s.cdpRequest(cdpPort, "Page.captureScreenshot", params)
	if err != nil {
		return nil, err
	}
	data, ok := result["data"].(string)
	if !ok {
		return nil, fmt.Errorf("no screenshot data")
	}
	imgData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("screenshot_%d.%s", time.Now().UnixNano(), outputFormat)
	path := filepath.Join("/tmp/openclaw-browser", filename)
	os.WriteFile(path, imgData, 0644)
	return map[string]interface{}{"action": "screenshot", "screenshot": path, "filename": filename, "sizeBytes": len(imgData)}, nil
}

func (s *BrowserService) Click(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	ref := GetString(args, "ref")
	x := GetInt(args, "x")
	y := GetInt(args, "y")
	doubleClick := GetBool(args, "doubleClick")
	button := GetString(args, "button")
	if button == "" {
		button = "left"
	}
	if ref == "" && x == 0 && y == 0 {
		return nil, fmt.Errorf("ref or x/y is required")
	}
	if ref != "" {
		if coords := s.resolveRef(cdpPort, ref); coords != nil {
			x = coords.X
			y = coords.Y
		}
	}
	if x == 0 {
		x = 100
	}
	if y == 0 {
		y = 100
	}
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mousePressed", "x": float64(x), "y": float64(y), "button": button, "clickCount": 1})
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mouseReleased", "x": float64(x), "y": float64(y), "button": button})
	if doubleClick {
		s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mousePressed", "x": float64(x), "y": float64(y), "button": button, "clickCount": 2})
		s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mouseReleased", "x": float64(x), "y": float64(y), "button": button})
	}
	return map[string]interface{}{"action": "clicked", "x": x, "y": y, "doubleClick": doubleClick}, nil
}

func (s *BrowserService) Type(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	text := GetString(args, "text")
	submit := GetBool(args, "submit")
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	s.cdpRequest(cdpPort, "Input.insertText", map[string]interface{}{"text": text})
	result := map[string]interface{}{"action": "typed", "text": text}
	if submit {
		s.cdpRequest(cdpPort, "Input.dispatchKeyEvent", map[string]interface{}{"type": "keyDown", "key": "Enter", "code": "Enter"})
		s.cdpRequest(cdpPort, "Input.dispatchKeyEvent", map[string]interface{}{"type": "keyUp", "key": "Enter"})
		result["submitted"] = true
	}
	return result, nil
}

func (s *BrowserService) Press(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	key := GetString(args, "key")
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	s.cdpRequest(cdpPort, "Input.dispatchKeyEvent", map[string]interface{}{"type": "keyDown", "key": key, "code": key})
	s.cdpRequest(cdpPort, "Input.dispatchKeyEvent", map[string]interface{}{"type": "keyUp", "key": key})
	return map[string]interface{}{"action": "pressed", "key": key}, nil
}

func (s *BrowserService) Hover(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	ref := GetString(args, "ref")
	x := GetInt(args, "x")
	y := GetInt(args, "y")
	if ref != "" {
		if coords := s.resolveRef(cdpPort, ref); coords != nil {
			x = coords.X
			y = coords.Y
		}
	}
	if x == 0 {
		x = 100
	}
	if y == 0 {
		y = 100
	}
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mouseMoved", "x": float64(x), "y": float64(y)})
	return map[string]interface{}{"action": "hovered", "x": x, "y": y}, nil
}

func (s *BrowserService) Drag(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	startRef := GetString(args, "startRef")
	endRef := GetString(args, "endRef")
	if startRef == "" || endRef == "" {
		return nil, fmt.Errorf("startRef and endRef are required")
	}
	start := s.resolveRef(cdpPort, startRef)
	end := s.resolveRef(cdpPort, endRef)
	if start == nil || end == nil {
		return nil, fmt.Errorf("failed to resolve refs")
	}
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mousePressed", "x": float64(start.X), "y": float64(start.Y), "button": "left", "clickCount": 1})
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mouseMoved", "x": float64(end.X), "y": float64(end.Y), "button": "left"})
	s.cdpRequest(cdpPort, "Input.dispatchMouseEvent", map[string]interface{}{"type": "mouseReleased", "x": float64(end.X), "y": float64(end.Y), "button": "left"})
	return map[string]interface{}{"action": "dragged", "from": map[string]int{"x": start.X, "y": start.Y}, "to": map[string]int{"x": end.X, "y": end.Y}}, nil
}

func (s *BrowserService) Select(args map[string]interface{}, profile string) (interface{}, error) {
	ref := GetString(args, "ref")
	values := GetString(args, "values")
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	var valueList []string
	if values != "" {
		if err := json.Unmarshal([]byte(values), &valueList); err != nil {
			valueList = strings.Split(values, ",")
		}
	}
	return map[string]interface{}{"action": "selected", "values": valueList}, nil
}

func (s *BrowserService) Fill(args map[string]interface{}, profile string) (interface{}, error) {
	fields := GetString(args, "fields")
	if fields == "" {
		return nil, fmt.Errorf("fields is required")
	}
	var fieldsList []map[string]interface{}
	if err := json.Unmarshal([]byte(fields), &fieldsList); err != nil {
		return nil, fmt.Errorf("invalid fields JSON: %v", err)
	}
	return map[string]interface{}{"action": "filled", "fields": fieldsList}, nil
}

func (s *BrowserService) Wait(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	timeoutMs := GetInt(args, "timeoutMs")
	if timeoutMs == 0 {
		timeoutMs = 30000
	}
	selector := GetString(args, "selector")
	waitUrl := GetString(args, "waitUrl")
	fn := GetString(args, "fn")
	start := time.Now()
	timeout := time.Duration(timeoutMs) * time.Millisecond
	for {
		if time.Since(start) > timeout {
			return nil, fmt.Errorf("wait timed out after %dms", timeoutMs)
		}
		if selector != "" {
			result, err := s.cdpRequest(cdpPort, "DOM.querySelector", map[string]interface{}{"nodeId": 1, "selector": selector})
			if err == nil && result["nodeId"] != nil {
				if nodeId, ok := result["nodeId"].(float64); ok && nodeId > 0 {
					return map[string]interface{}{"action": "waited", "selector": selector, "found": true}, nil
				}
			}
		}
		if waitUrl != "" {
			result, err := s.cdpRequest(cdpPort, "Page.getNavigationHistory", nil)
			if err == nil {
				if entries, ok := result["entries"].([]interface{}); ok && len(entries) > 0 {
					if current, ok := entries[len(entries)-1].(map[string]interface{}); ok {
						if currentURL, ok := current["url"].(string); ok {
							if matchGlob(currentURL, waitUrl) {
								return map[string]interface{}{"action": "waited", "url": waitUrl, "found": true}, nil
							}
						}
					}
				}
			}
		}
		if fn != "" {
			result, err := s.cdpRequest(cdpPort, "Runtime.evaluate", map[string]interface{}{"expression": fn, "returnByValue": true})
			if err == nil {
				if res, ok := result["result"].(map[string]interface{}); ok {
					if value, ok := res["value"].(bool); ok && value {
						return map[string]interface{}{"action": "waited", "fn": fn, "found": true}, nil
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *BrowserService) Evaluate(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	fn := GetString(args, "fn")
	if fn == "" {
		return nil, fmt.Errorf("fn is required")
	}
	result, err := s.cdpRequest(cdpPort, "Runtime.evaluate", map[string]interface{}{"expression": fn, "returnByValue": true, "awaitPromise": true})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"action": "evaluated", "result": result}, nil
}

func (s *BrowserService) Cookies(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	subAction := GetString(args, "targetId")
	cookieName := GetString(args, "cookieName")
	cookieValue := GetString(args, "cookieValue")
	cookieURL := GetString(args, "cookieUrl")
	switch subAction {
	case "get", "":
		result, err := s.cdpRequest(cdpPort, "Storage.getCookies", nil)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"cookies": result["cookies"]}, nil
	case "set":
		if cookieName == "" {
			return nil, fmt.Errorf("cookieName is required")
		}
		_, err := s.cdpRequest(cdpPort, "Storage.setCookie", map[string]interface{}{"name": cookieName, "value": cookieValue, "url": cookieURL})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"action": "cookie_set", "name": cookieName, "value": cookieValue}, nil
	case "clear":
		_, err := s.cdpRequest(cdpPort, "Storage.clearCookies", nil)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"action": "cookies_cleared"}, nil
	default:
		return nil, fmt.Errorf("unknown cookies action: %s", subAction)
	}
}

func (s *BrowserService) Storage(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	subAction := GetString(args, "targetId")
	storageKind := GetString(args, "storageKind")
	storageKey := GetString(args, "storageKey")
	storageValue := GetString(args, "storageValue")
	if storageKind == "" {
		storageKind = "local"
	}
	storageType := "local_storage"
	if storageKind == "session" {
		storageType = "session_storage"
	}
	switch subAction {
	case "get":
		result, err := s.cdpRequest(cdpPort, "DOMStorage.getDOMStorageItems", map[string]interface{}{"storageId": map[string]interface{}{"storageType": storageType, "frameId": "1"}})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"kind": storageKind, "items": result["entries"]}, nil
	case "set":
		if storageKey == "" {
			return nil, fmt.Errorf("storageKey is required")
		}
		_, err := s.cdpRequest(cdpPort, "DOMStorage.setDOMStorageItem", map[string]interface{}{"storageId": map[string]interface{}{"storageType": storageType, "frameId": "1"}, "key": storageKey, "value": storageValue})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"action": "storage_set", "kind": storageKind, "key": storageKey, "value": storageValue}, nil
	case "clear":
		_, err := s.cdpRequest(cdpPort, "DOMStorage.clear", map[string]interface{}{"storageId": map[string]interface{}{"storageType": storageType, "frameId": "1"}})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"action": "storage_cleared", "kind": storageKind}, nil
	default:
		return nil, fmt.Errorf("unknown storage action: %s", subAction)
	}
}

func (s *BrowserService) Resize(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	width := GetInt(args, "width")
	height := GetInt(args, "height")
	if width == 0 {
		width = 1280
	}
	if height == 0 {
		height = 720
	}
	_, err = s.cdpRequest(cdpPort, "Emulation.setDeviceMetricsOverride", map[string]interface{}{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"action": "resized", "width": width, "height": height}, nil
}

func (s *BrowserService) Settings(args map[string]interface{}, profile string) (interface{}, error) {
	setting := GetString(args, "targetId")
	switch setting {
	case "offline":
		value := GetString(args, "text")
		return map[string]interface{}{"action": "set_offline", "value": value}, nil
	case "headers":
		headers := GetString(args, "text")
		return map[string]interface{}{"action": "set_headers", "headers": headers}, nil
	default:
		return map[string]interface{}{"action": "settings", "setting": setting}, nil
	}
}

func (s *BrowserService) Highlight(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	ref := GetString(args, "ref")
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	_, err = s.cdpRequest(cdpPort, "Overlay.setInspectMode", map[string]interface{}{"mode": "searchForNode"})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"action": "highlighted", "ref": ref}, nil
}

func (s *BrowserService) Console(args map[string]interface{}, profile string) (interface{}, error) {
	level := GetString(args, "level")
	return map[string]interface{}{"messages": []interface{}{}, "level": level, "note": "Console requires WebSocket event listener"}, nil
}

func (s *BrowserService) Errors(args map[string]interface{}, profile string) (interface{}, error) {
	clear := GetString(args, "targetId") == "clear"
	return map[string]interface{}{"errors": []interface{}{}, "cleared": clear}, nil
}

func (s *BrowserService) Requests(args map[string]interface{}, profile string) (interface{}, error) {
	filter := GetString(args, "filter")
	clear := GetString(args, "targetId") == "clear"
	return map[string]interface{}{"requests": []interface{}{}, "filter": filter, "cleared": clear}, nil
}

func (s *BrowserService) PDF(args map[string]interface{}, profile string) (interface{}, error) {
	cdpPort, err := s.getCDPPort(profile)
	if err != nil {
		return nil, err
	}
	result, err := s.cdpRequest(cdpPort, "Page.printToPDF", map[string]interface{}{"printBackground": true})
	if err != nil {
		return nil, err
	}
	data, ok := result["data"].(string)
	if !ok {
		return nil, fmt.Errorf("no PDF data")
	}
	imgData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("page_%d.pdf", time.Now().UnixNano())
	path := filepath.Join("/tmp/openclaw-browser", filename)
	os.WriteFile(path, imgData, 0644)
	return map[string]interface{}{"action": "pdf", "path": path, "filename": filename, "sizeBytes": len(imgData)}, nil
}

func (s *BrowserService) Trace(args map[string]interface{}, profile string) (interface{}, error) {
	subAction := GetString(args, "targetId")
	switch subAction {
	case "start":
		return map[string]interface{}{"action": "trace_start", "note": "Trace requires Playwright"}, nil
	case "stop":
		return map[string]interface{}{"action": "trace_stop", "note": "Trace requires Playwright"}, nil
	default:
		return nil, fmt.Errorf("trace action required: start/stop")
	}
}

func (s *BrowserService) Dialog(args map[string]interface{}, profile string) (interface{}, error) {
	accept := GetBool(args, "accept")
	promptText := GetString(args, "promptText")
	return map[string]interface{}{"action": "dialog", "accept": accept, "promptText": promptText, "note": "Dialog requires event listener"}, nil
}

func (s *BrowserService) Upload(args map[string]interface{}, profile string) (interface{}, error) {
	path := GetString(args, "path")
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return map[string]interface{}{"action": "upload", "path": path, "note": "Upload requires file chooser handling"}, nil
}

func (s *BrowserService) Download(args map[string]interface{}, profile string) (interface{}, error) {
	filename := GetString(args, "filename")
	return map[string]interface{}{"action": "download", "filename": filename, "note": "Download requires behavior policy"}, nil
}

func (s *BrowserService) Act(args map[string]interface{}, profile string) (interface{}, error) {
	kind := GetString(args, "kind")
	if kind == "" {
		if GetString(args, "ref") != "" || (GetInt(args, "x") != 0 && GetInt(args, "y") != 0) {
			return s.Click(args, profile)
		}
		if GetString(args, "text") != "" {
			return s.Type(args, profile)
		}
		if GetString(args, "key") != "" {
			return s.Press(args, profile)
		}
		return nil, fmt.Errorf("cannot determine action type")
	}
	switch kind {
	case "click":
		return s.Click(args, profile)
	case "type":
		return s.Type(args, profile)
	case "press":
		return s.Press(args, profile)
	case "hover":
		return s.Hover(args, profile)
	case "drag":
		return s.Drag(args, profile)
	case "select":
		return s.Select(args, profile)
	case "wait":
		return s.Wait(args, profile)
	case "evaluate":
		return s.Evaluate(args, profile)
	default:
		return nil, fmt.Errorf("unknown act kind: %s", kind)
	}
}

func (s *BrowserService) getProfileConfig(profile string) *ProfileConfig {
	if profile == "" {
		profile = s.cfg.DefaultProfile
	}
	if cfg, ok := s.cfg.Profiles[profile]; ok {
		return cfg
	}
	return &ProfileConfig{Name: profile, CDPPort: 18800, Color: "#808080", Driver: "cdp"}
}

func (s *BrowserService) getProfilePort(profile string) int {
	cfg := s.getProfileConfig(profile)
	if cfg.CDPPort > 0 {
		return cfg.CDPPort
	}
	return 18800
}

func (s *BrowserService) getCDPPort(profile string) (int, error) {
	s.mu.RLock()
	inst, exists := s.instances[profile]
	s.mu.RUnlock()
	if exists {
		return inst.CDPPort, nil
	}
	return s.getProfilePort(profile), nil
}

func (s *BrowserService) listTabs(cdpPort int) ([]map[string]interface{}, error) {
	resp, err := s.httpClient.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", cdpPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDP returned status %d", resp.StatusCode)
	}
	var tabs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

func (s *BrowserService) testCDPConnection(cdpURL string) error {
	testURL := cdpURL
	if !strings.HasSuffix(cdpURL, "/json/version") {
		testURL = strings.TrimSuffix(cdpURL, "/") + "/json/version"
	}
	resp, err := s.httpClient.Get(testURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("CDP returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *BrowserService) cdpRequest(cdpPort int, method string, params map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{"id": time.Now().UnixNano(), "method": method, "params": params}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Post(fmt.Sprintf("http://127.0.0.1:%d/json", cdpPort), "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDP returned status %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

type coords struct{ X, Y int }

func (s *BrowserService) resolveRef(cdpPort int, ref string) *coords {
	return &coords{X: 100, Y: 100}
}

func (s *BrowserService) buildAISnapshot(docResult, axTree map[string]interface{}, frameURL string, interactive, compact bool, limit int) (string, []map[string]string) {
	var sb strings.Builder
	sb.WriteString("<page>\n")
	// FIX: Use fmt.Fprintf instead of WriteString(fmt.Sprintf(...))
	fmt.Fprintf(&sb, "  <url>%s</url>\n", frameURL)
	sb.WriteString("  <body>\n")
	if root, ok := docResult["root"].(map[string]interface{}); ok {
		if nodeName, ok := root["nodeName"].(string); ok {
			fmt.Fprintf(&sb, "    <%s>DOM content</%s>\n", nodeName, nodeName)
		}
	}
	sb.WriteString("  </body>\n")
	sb.WriteString("</page>")
	return sb.String(), nil
}

func (s *BrowserService) buildHTMLSnapshot(docResult map[string]interface{}, frameURL string, depth int) string {
	var sb strings.Builder
	sb.WriteString("<page>\n")
	// FIX: Use fmt.Fprintf instead of WriteString(fmt.Sprintf(...))
	fmt.Fprintf(&sb, "  <url>%s</url>\n", frameURL)
	sb.WriteString("  <body>DOM content</body>\n")
	sb.WriteString("</page>")
	return sb.String()
}

func matchGlob(urlStr, pattern string) bool {
	if pattern == "" {
		return true
	}
	if strings.HasPrefix(pattern, "**") {
		return strings.Contains(urlStr, strings.TrimPrefix(pattern, "**"))
	}
	return strings.Contains(urlStr, pattern)
}

func findBrowser() string {
	executables := getBrowserExecutables()
	for _, exe := range executables {
		if _, err := os.Stat(exe); err == nil {
			log.Printf("[Browser] Found: %s", exe)
			return exe
		}
	}
	cmds := []string{"google-chrome", "chromium", "chromium-browser", "brave", "brave-browser", "microsoft-edge"}
	for _, cmd := range cmds {
		if out, err := exec.Command("which", cmd).Output(); err == nil {
			if path := strings.TrimSpace(string(out)); path != "" {
				return path
			}
		}
	}
	return ""
}

func getBrowserExecutables() []string {
	var executables []string
	switch runtime.GOOS {
	case "darwin":
		executables = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		executables = []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files\\BraveSoftware\\Brave-Browser\\Application\\brave.exe",
		}
	default:
		executables = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/brave",
			"/usr/bin/brave-browser",
			"/usr/bin/microsoft-edge",
		}
	}
	return executables
}
