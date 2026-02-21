// Gateway module - HTTP server
// Uses dependency injection for all configurable values

package gateway

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"google.golang.org/grpc"

	"github.com/gliderlab/cogate/cron"
	"github.com/gliderlab/cogate/gateway/channels"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/pkg/hooks"
	"github.com/gliderlab/cogate/pkg/hooks/bundled"
	"github.com/gliderlab/cogate/processtool"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
)

func init() {
	// Register gob types for interface{} serialization
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

// Global tiktoken tokenizer for accurate token counting
var (
	tokenizer     *tiktoken.Tiktoken
	tokenizerErr  error
	tokenizerOnce sync.Once
)

// initTokenizer initializes the tiktoken tokenizer (cl100k_base for GPT-3.5/4)
func initTokenizer() {
	tokenizerOnce.Do(func() {
		// cl100k_base is used by GPT-3.5 Turbo, GPT-4, GPT-4 Turbo
		tokenizer, tokenizerErr = tiktoken.GetEncoding("cl100k_base")
		if tokenizerErr != nil {
			log.Printf("[WARN] Failed to load tiktoken tokenizer: %v", tokenizerErr)
		} else {
			log.Printf("[OK] Tiktoken tokenizer loaded (cl100k_base)")
		}
	})
}

// writeJSON writes a JSON response with proper Content-Type header
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[WARN] Failed to encode JSON response: %v", err)
	}
}

// Gateway provides dependency injection for all gateway components
type Gateway struct {
	cfg            config.GatewayConfig
	client         *grpc.ClientConn
	server         *http.Server
	channelAdapter *channels.ChannelAdapter
	cronHandler    *cron.CronHandler
	webhookHandler *WebhookHandler // Webhook handler
	hooksRegistry  *hooks.HookRegistry  // Hooks registry
	store          interface {
		CheckRateLimit(endpoint, key string) (bool, error)
	}
	mu sync.RWMutex

	// Injected dependencies (optional)
	httpClient   HTTPClient
	idGenerator  IDGenerator
	timeProvider TimeProvider

	// WebSocket connection limiting
	wsConnCount int32
	maxWSConns  int32
	wsIPConns   map[string]int32 // Per-IP connection count

	// Config hot reload
	configFile     string
	configWatcher  interface{ Close() error }
	reloadCh       chan struct{}
}

// HTTPClient interface for dependency injection
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// IDGenerator interface for dependency injection
type IDGenerator interface {
	New() string
}

// TimeProvider interface for dependency injection
type TimeProvider interface {
	Now() time.Time
}

// Default implementations
// Optimization #3: Use HTTP transport with connection pooling
var defaultHTTPTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 100,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}

var defaultHTTPClientInstance = &http.Client{
	Transport: defaultHTTPTransport,
	Timeout:   30 * time.Second,
}

type defaultHTTPClient struct{}

func (d *defaultHTTPClient) Get(url string) (*http.Response, error) {
	return defaultHTTPClientInstance.Get(url)
}

func (d *defaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return defaultHTTPClientInstance.Do(req)
}

type defaultIDGenerator struct{}

func (d *defaultIDGenerator) New() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d-%x", time.Now().Unix(), b)
}

type defaultTimeProvider struct{}

func (d *defaultTimeProvider) Now() time.Time {
	return time.Now()
}

// New creates a new Gateway with the given configuration
func New(cfg config.GatewayConfig) *Gateway {
	g := &Gateway{
		cfg:          cfg,
		httpClient:   &defaultHTTPClient{},
		idGenerator:  &defaultIDGenerator{},
		timeProvider: &defaultTimeProvider{},
	}

	// Apply defaults
	if g.cfg.Port == 0 {
		g.cfg.Port = config.DefaultGatewayPort
	}
	if g.cfg.Host == "" {
		g.cfg.Host = "0.0.0.0"
	}
	if g.cfg.MaxBodyChat == 0 {
		g.cfg.MaxBodyChat = 2 * 1024 * 1024
	}
	if g.cfg.MaxBodyProcess == 0 {
		g.cfg.MaxBodyProcess = 512 * 1024
	}
	if g.cfg.MaxBodyMemory == 0 {
		g.cfg.MaxBodyMemory = 512 * 1024
	}
	if g.cfg.MaxBodyCron == 0 {
		g.cfg.MaxBodyCron = 256 * 1024
	}
	if g.cfg.MaxProcessLogCap == 0 {
		g.cfg.MaxProcessLogCap = 10 * 1024
	}
	if g.cfg.ReadTimeout == 0 {
		g.cfg.ReadTimeout = 120 * time.Second
	}
	if g.cfg.WriteTimeout == 0 {
		g.cfg.WriteTimeout = 180 * time.Second
	}
	if g.cfg.IdleTimeout == 0 {
		g.cfg.IdleTimeout = 300 * time.Second
	}

	// WebSocket connection limit (default 50)
	g.maxWSConns = 50

	// Per-IP WebSocket connection tracking
	g.wsIPConns = make(map[string]int32)

	// Initialize Hooks registry
	g.initHooks()

	// Initialize webhook handler if enabled
	if g.cfg.Webhook.Enabled {
		// Storage will be set later via SetStorage
		log.Printf("[Gateway] Webhook handler initialized (disabled, will be enabled when storage is set)")
	}

	return g
}

// WithHTTPClient injects a custom HTTP client
func (g *Gateway) WithHTTPClient(client HTTPClient) *Gateway {
	g.httpClient = client
	return g
}

// WithIDGenerator injects a custom ID generator
func (g *Gateway) WithIDGenerator(gen IDGenerator) *Gateway {
	g.idGenerator = gen
	return g
}

// WithTimeProvider injects a custom time provider
func (g *Gateway) WithTimeProvider(tp TimeProvider) *Gateway {
	g.timeProvider = tp
	return g
}

// Config returns the gateway configuration
func (g *Gateway) Config() config.GatewayConfig {
	return g.cfg
}

func (g *Gateway) SetClient(c *grpc.ClientConn) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.client = c
}

// SetStore sets the storage for rate limiting
func (g *Gateway) SetStore(s interface {
	CheckRateLimit(endpoint, key string) (bool, error)
}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.store = s
}

// SetWebhookStorage initializes the webhook handler with storage
func (g *Gateway) SetWebhookStorage(store *storage.Storage) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.cfg.Webhook.Enabled {
		return
	}

	// Check if token is configured
	if g.cfg.Webhook.Token == "" {
		log.Printf("[Gateway] Webhook enabled but token not configured, disabling")
		g.cfg.Webhook.Enabled = false
		return
	}

	g.webhookHandler = NewWebhookHandler(&g.cfg, g.client, store)

	// If channel adapter is available, set the deliver callback
	if g.channelAdapter != nil {
		g.webhookHandler.SetDeliverCallback(func(target, message, channel string) error {
			chType := channelTypeFromString(channel)
			if chType == "" {
				return fmt.Errorf("unknown channel: %s", channel)
			}
			
			// Parse target as chat ID (int64)
			var chatID int64
			if target != "" && target != "last" {
				parsed, err := strconv.ParseInt(target, 10, 64)
				if err != nil {
					log.Printf("[Webhook] Invalid target chat ID: %s, using 0", target)
					chatID = 0
				} else {
					chatID = parsed
				}
			}
			
			_, err := g.channelAdapter.SendMessage(chType, &channels.SendMessageRequest{
				ChatID: chatID,
				Text:   message,
			})
			return err
		})
	}

	log.Printf("[Gateway] Webhook handler enabled at %s", g.cfg.Webhook.Path)
}

// initHooks initializes the hooks registry and discovers hooks
func (g *Gateway) initHooks() {
	g.hooksRegistry = hooks.NewHookRegistry()

	// Load bundled hooks
	bundledHooks := bundled.GetAllBundledHooks()
	for _, hook := range bundledHooks {
		g.hooksRegistry.Register(hook)
		log.Printf("[Hooks] Registered bundled hook: %s %s", hook.Emoji, hook.Name)
	}

	// Discover hooks from directories
	// Priority: workspace hooks > managed hooks > bundled
	var workspaceDir, managedDir string

	// Check environment variable first
	if envDir := os.Getenv("OCG_WORKSPACE"); envDir != "" {
		workspaceDir = filepath.Join(envDir, "hooks")
	}

	// Default managed hooks directory
	managedDir = hooks.DefaultHooksDir()

	// Discover hooks if directories exist
	if workspaceDir != "" || managedDir != "" {
		discovery := hooks.NewHookDiscovery(workspaceDir, managedDir, "")
		discovered, err := discovery.Discover()
		if err != nil {
			log.Printf("[Hooks] Discovery warning: %v", err)
		}
		for _, hook := range discovered {
			g.hooksRegistry.Register(hook)
			log.Printf("[Hooks] Registered discovered hook: %s %s", hook.Emoji, hook.Name)
		}
	}

	log.Printf("[Hooks] Registry initialized with %d hooks", len(g.hooksRegistry.List()))
}

// GetHooksRegistry returns the hooks registry
func (g *Gateway) GetHooksRegistry() *hooks.HookRegistry {
	return g.hooksRegistry
}

// TriggerHook triggers a hook event
func (g *Gateway) TriggerHook(eventType hooks.EventType, action, sessionKey string, context hooks.EventContext) {
	event := hooks.NewHookEvent(eventType, action, sessionKey)
	event.Context = context
	g.hooksRegistry.Dispatch(event)
}

// validateToken checks if the request has valid authentication
func (g *Gateway) validateToken(r *http.Request) bool {
	token := strings.TrimSpace(g.cfg.UIAuthToken)
	if token == "" {
		return false
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(header) >= 7 && strings.EqualFold(header[:7], "Bearer ") {
		candidate := strings.TrimSpace(header[7:])
		if len(candidate) == len(token) && subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1 {
			return true
		}
	}

	headerToken := strings.TrimSpace(r.Header.Get("X-OCG-UI-Token"))
	if len(headerToken) == len(token) && subtle.ConstantTimeCompare([]byte(headerToken), []byte(token)) == 1 {
		return true
	}

	queryToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if len(queryToken) == len(token) && subtle.ConstantTimeCompare([]byte(queryToken), []byte(token)) == 1 {
		return true
	}

	return false
}

func (g *Gateway) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.validateToken(r) {
			if g.cfg.UIAuthToken == "" {
				http.Error(w, "unauthorized: UI token not configured on server", http.StatusUnauthorized)
			} else {
				http.Error(w, "unauthorized: invalid UI token", http.StatusUnauthorized)
			}
			return
		}
		next(w, r)
	}
}

func (g *Gateway) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if g.store == nil {
			next(w, r)
			return
		}

		key := r.Header.Get("Authorization")
		if key == "" {
			key = r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				key = host
			}
		}

		allowed, err := g.store.CheckRateLimit(r.URL.Path, key)
		if err != nil {
			log.Printf("[WARN] rate limit check failed: %v", err)
			http.Error(w, "rate limit unavailable", http.StatusServiceUnavailable)
			return
		}

		if !allowed {
			w.Header().Set("X-RateLimit-RetryAfter", "3600")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// addCORS wraps an HTTP handler with CORS headers (Fix Bug #6)
func (g *Gateway) addCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin (can be restricted if needed)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-OCG-UI-Token")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// gzipResponseWriter wraps http.ResponseWriter for gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	gw *gzip.Writer
}

func (gr *gzipResponseWriter) Write(p []byte) (int, error) {
	if gr.gw == nil {
		return gr.ResponseWriter.Write(p)
	}
	return gr.gw.Write(p)
}

// gzipHandler wraps an HTTP handler with gzip compression
type gzipHandler struct {
	h http.Handler
}

func (gh *gzipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only compress if client accepts gzip
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		gh.h.ServeHTTP(w, r)
		return
	}

	// Create gzip writer
	gw := gzip.NewWriter(w)
	defer gw.Close()

	// Set compression header
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")

	// Wrap ResponseWriter for gzip
	gh.h.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gw: gw}, r)
}

func (g *Gateway) addGzip(next http.Handler) http.Handler {
	return &gzipHandler{h: next}
}

func (g *Gateway) Start() error {
	mux := http.NewServeMux()

	// CORS middleware wrapper (Fix Bug #6)
	corsMux := g.addCORS(mux)

	// Health check - no auth required
	mux.HandleFunc("/health", g.handleHealth)

	requireAuth := g.requireAuth
	rateLimit := g.rateLimit

	// Health check endpoints (public)
	mux.HandleFunc("/agent/health", g.handleAgentHealth)
	mux.HandleFunc("/embedding/health", g.handleEmbeddingHealth)

	// API routes (protected)
	mux.HandleFunc("/v1/chat/completions", rateLimit(requireAuth(g.handleChat)))
	mux.HandleFunc("/storage/stats", requireAuth(g.handleStorageStats))
	mux.HandleFunc("/sessions/list", requireAuth(g.handleSessions))
	mux.HandleFunc("/process/start", requireAuth(g.handleProcessStart))
	mux.HandleFunc("/process/list", requireAuth(g.handleProcessList))
	mux.HandleFunc("/process/log", requireAuth(g.handleProcessLog))
	mux.HandleFunc("/process/write", requireAuth(g.handleProcessWrite))
	mux.HandleFunc("/process/kill", requireAuth(g.handleProcessKill))
	mux.HandleFunc("/memory/search", requireAuth(g.handleMemorySearch))
	mux.HandleFunc("/memory/get", requireAuth(g.handleMemoryGet))
	mux.HandleFunc("/memory/store", requireAuth(g.handleMemoryStore))

	// Cron endpoints
	mux.HandleFunc("/cron/status", requireAuth(g.handleCronStatus))
	mux.HandleFunc("/cron/list", requireAuth(g.handleCronList))
	mux.HandleFunc("/cron/add", requireAuth(g.handleCronAdd))
	mux.HandleFunc("/cron/update", requireAuth(g.handleCronUpdate))
	mux.HandleFunc("/cron/remove", requireAuth(g.handleCronRemove))
	mux.HandleFunc("/cron/run", requireAuth(g.handleCronRun))
	mux.HandleFunc("/cron/runs", requireAuth(g.handleCronRuns))
	mux.HandleFunc("/cron/wake", requireAuth(g.handleCronWake))

	// Telegram Bot webhook endpoint (public, no auth)
	mux.HandleFunc("/telegram/webhook", g.handleTelegramWebhook)

	// Telegram Bot configuration endpoints (protected)
	mux.HandleFunc("/telegram/setWebhook", requireAuth(g.handleTelegramSetWebhook))
	mux.HandleFunc("/telegram/status", requireAuth(g.handleTelegramStatus))

	// Webhook endpoints (public, uses its own token auth)
	if g.cfg.Webhook.Enabled && g.webhookHandler != nil {
		webhookPath := g.cfg.Webhook.Path
		mux.HandleFunc(webhookPath+"/wake", g.webhookHandler.HandleWake)
		mux.HandleFunc(webhookPath+"/agent", g.webhookHandler.HandleAgent)
		// Use a catch-all handler and parse name manually
		mux.HandleFunc(webhookPath+"/", g.webhookHandler.HandleCustom)
		log.Printf("[Gateway] Registered webhook routes: %s/wake, %s/agent, %s/<name>", webhookPath, webhookPath, webhookPath)
	}

	// Internal pulse trigger endpoint (called by webhook handler)
	mux.HandleFunc("/internal/pulse/trigger", g.handlePulseTrigger)

	// WebSocket endpoint for real-time chat
	mux.HandleFunc("/ws/chat", g.HandleWebSocket)

	// Static files (web chat UI) embedded in binary - MUST be last (catch-all)
	// Optimization #2: Add gzip compression for static assets
	log.Printf("Static assets: embedded (gzip enabled)")
	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For SPA: serve index.html for all non-file routes
		path := r.URL.Path
		if path == "/" {
			http.ServeFileFS(w, r, embeddedStaticFS, "static/index.html")
			return
		}
		// Try to serve static file
		filePath := "static" + path
		if _, err := embeddedStaticFS.Open(filePath); err == nil {
			http.ServeFileFS(w, r, embeddedStaticFS, filePath)
			return
		}
		// Fallback to index.html for SPA routing
		http.ServeFileFS(w, r, embeddedStaticFS, "static/index.html")
	})
	mux.Handle("/", g.addGzip(staticHandler))

	addr := fmt.Sprintf("%s:%d", g.cfg.Host, g.cfg.Port)
	g.server = &http.Server{
		Addr:         addr,
		Handler:      corsMux, // Use CORS-wrapped handler
		ReadTimeout:  g.cfg.ReadTimeout,
		WriteTimeout: g.cfg.WriteTimeout,
		IdleTimeout:  g.cfg.IdleTimeout,
	}
	log.Printf("Gateway listening on %s", addr)

	// Initialize Channel Adapter
	g.channelAdapter = channels.NewChannelAdapter(
		channels.DefaultChannelAdapterConfig(),
		&GatewayAgentRPC{client: g.client},
	)

	// Initialize Cron handler
	cronStore := g.cfg.CronJobsPath
	if cronStore == "" {
		// Use bin/data/cron/ for runtime data (not gateway/ which is source)
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		cronStore = filepath.Join(execDir, "data", "cron", "jobs.json")
	}
	g.cronHandler = cron.NewCronHandler(cronStore)
	g.cronHandler.SetSystemEventCallback(func(text string) {
		if g.client == nil {
			log.Printf("[Cron] agent not connected")
			return
		}
		_, err := (&GatewayAgentRPC{client: g.client}).Chat([]channels.Message{{Role: "system", Content: text}})
		if err != nil {
			log.Printf("[Cron] system event error: %v", err)
		}
	})
	g.cronHandler.SetAgentTurnCallback(func(message, model, thinking string) (string, error) {
		if g.client == nil {
			return "", fmt.Errorf("agent not connected")
		}
		return (&GatewayAgentRPC{client: g.client}).Chat([]channels.Message{{Role: "user", Content: message}})
	})
	g.cronHandler.SetBroadcastCallback(func(message, channel, target string) error {
		if g.channelAdapter == nil {
			return fmt.Errorf("channel adapter not initialized")
		}
		chType := channelTypeFromString(channel)
		if chType == "" {
			return fmt.Errorf("unknown channel: %s", channel)
		}
		chatID, err := strconv.ParseInt(strings.TrimSpace(target), 10, 64)
		if err != nil || chatID == 0 {
			return fmt.Errorf("invalid target chat id: %s", target)
		}
		_, err = g.channelAdapter.SendMessage(chType, &channels.SendMessageRequest{
			ChatID: chatID,
			Text:   message,
		})
		return err
	})
	// Set webhook callback for cron delivery
	g.cronHandler.SetWebhookCallback(func(url, payload string) error {
		// Perform HTTP POST to webhook URL
		resp, err := http.Post(url, "application/json", strings.NewReader(payload))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("webhook returned status: %d", resp.StatusCode)
		}
		return nil
	})
	// Set wake callback for heartbeat trigger
	g.cronHandler.SetWakeCallback(func() error {
		// Trigger a heartbeat for main session
		// This would typically enqueue a heartbeat event
		log.Printf("[Cron] Wake callback triggered")
		return nil
	})
	g.cronHandler.Start()

	// Register Telegram channel if token is provided
	telegramToken := g.cfg.TelegramToken
	if telegramToken == "" {
		telegramToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if telegramToken != "" {
		if g.client != nil {
			bot := channels.NewTelegramBot(telegramToken, &GatewayAgentRPC{client: g.client})
			if err := g.channelAdapter.RegisterChannel(bot); err != nil {
				log.Printf("[WARN] Failed to register Telegram channel: %v", err)
			} else {
				log.Printf("[BOT] Telegram channel registered")
				if err := g.channelAdapter.StartChannel(channels.ChannelTelegram); err != nil {
					log.Printf("[WARN] Failed to start Telegram channel: %v", err)
				}
			}
		} else {
			log.Printf("[WARN] Telegram Bot token found but agent not connected yet")
		}
	} else {
		log.Printf("ℹ️ No TELEGRAM_BOT_TOKEN environment variable found")
	}

	// Start config file watcher for hot reload
	g.StartConfigWatcher()

	return g.server.ListenAndServe()
}

func (g *Gateway) Stop() {
	// Stop config watcher
	if g.configWatcher != nil {
		g.configWatcher.Close()
	}
	if g.cronHandler != nil {
		g.cronHandler.Stop()
	}
	if g.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := g.server.Shutdown(ctx); err != nil {
			log.Printf("Gateway graceful shutdown failed: %v", err)
			g.server.Close()
		}
	}
}

// StartConfigWatcher starts watching config file for changes
func (g *Gateway) StartConfigWatcher() {
	if g.cfg.ReloadMode == "off" {
		log.Printf("[Config] Hot reload disabled")
		return
	}

	// Try to use fsnotify if available, otherwise poll
	g.configFile = g.cfg.EnvConfigPath
	if g.configFile == "" {
		g.configFile = filepath.Join(g.gatewayDir(), "config", "env.config")
	}

	if g.configFile == "" {
		log.Printf("[Config] No config file to watch")
		return
	}

	// Check if file exists
	if _, err := os.Stat(g.configFile); os.IsNotExist(err) {
		log.Printf("[Config] Config file does not exist: %s", g.configFile)
		return
	}

	log.Printf("[Config] Watching config file: %s (mode: %s)", g.configFile, g.cfg.ReloadMode)

	// Start file watcher (using simple polling if fsnotify not available)
	g.reloadCh = make(chan struct{}, 1)
	go g.configWatcherLoop()
}

// configWatcherLoop watches config file and triggers reload
func (g *Gateway) configWatcherLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastMod time.Time
	if fi, err := os.Stat(g.configFile); err == nil {
		lastMod = fi.ModTime()
	}

	for {
		select {
		case <-ticker.C:
			if fi, err := os.Stat(g.configFile); err == nil {
				if fi.ModTime().After(lastMod) {
					lastMod = fi.ModTime()
					g.triggerConfigReload()
				}
			}
		case <-g.reloadCh:
			// Only exit when explicitly signaled to stop
			return
		}
	}
}

// triggerConfigReload triggers a config reload
func (g *Gateway) triggerConfigReload() {
	// Don't send to reloadCh - that would stop the watcher!
	// Instead, just reload the config directly
	log.Printf("[Config] Config change detected, reloading...")

	// Read new config
	newConfig := config.ReadEnvConfig(g.configFile)
	if newConfig == nil {
		log.Printf("[Config] Failed to read new config")
		return
	}

	// Apply changes based on reload mode
	reloadMode := g.cfg.ReloadMode
	if reloadMode == "" {
		reloadMode = "hybrid"
	}

	log.Printf("[Config] Reload mode: %s", reloadMode)

	switch reloadMode {
	case "hot":
		// Hot-apply all changes
		g.applyConfigHot(newConfig)
	case "hybrid":
		// Hot-apply safe changes, log warning for restart-needed
		g.applyConfigHot(newConfig)
	case "restart":
		// Need restart for any change
		log.Printf("[Config] Config change requires restart")
	default:
		log.Printf("[Config] Unknown reload mode: %s", reloadMode)
	}
}

// applyConfigHot applies config changes without restart
func (g *Gateway) applyConfigHot(newConfig map[string]string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Fields that can be hot-reloaded
	var changes []string

	// Rate limit changes
	if newRateLimit := newConfig["RATE_LIMIT_WINDOW"]; newRateLimit != "" {
		if d, err := time.ParseDuration(newRateLimit); err == nil {
			g.cfg.RateLimitWindow = d
			changes = append(changes, "RateLimitWindow")
		}
	}

	// Process log cap
	if newLogCap := newConfig["MAX_PROCESS_LOG_CAP"]; newLogCap != "" {
		if d, err := strconv.Atoi(newLogCap); err == nil {
			g.cfg.MaxProcessLogCap = d
			changes = append(changes, "MaxProcessLogCap")
		}
	}

	if len(changes) > 0 {
		log.Printf("[Config] Hot-applied: %v", changes)
	} else {
		log.Printf("[Config] No hot-applicable changes found")
	}
}

// ReloadConfig triggers a manual config reload
func (g *Gateway) ReloadConfig() {
	g.triggerConfigReload()
}

func (g *Gateway) clientOrError() (*grpc.ClientConn, error) {
	g.mu.RLock()
	client := g.client
	g.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("agent not connected")
	}
	return client, nil
}

func (g *Gateway) handleChat(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WARN] handleChat panic recovered: %v", r)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyChat)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error", http.StatusBadRequest)
		return
	}

	if len(req.Messages) > 0 {
		last := &req.Messages[len(req.Messages)-1]
		log.Printf("Received message: role=%s len=%d stream=%v", last.Role, len(last.Content), req.Stream)
	}

	grpcClient := rpcproto.NewAgentGRPCClient(client)

	// Handle streaming request
	if req.Stream {
		ctx := r.Context()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		stream, err := grpcClient.ChatStream(ctx, &rpcproto.ChatArgs{Messages: rpcproto.ToMessagesPtr(req.Messages)})
		if err != nil {
			fmt.Fprintf(w, "data: %s\n\n", fmt.Errorf("stream error: %v", err))
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			fmt.Fprintf(w, "data: %s\n\n", fmt.Errorf("streaming not supported"))
			return
		}

		for {
			chunk, err := stream.Recv()
			if err != nil {
				break
			}
			if chunk.Done {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}
			if chunk.Content != "" {
				// Check for tool events (start with [TOOL_EVENT])
				if strings.HasPrefix(chunk.Content, "[TOOL_EVENT]") {
					// Parse and send as tool event
					eventData := strings.TrimPrefix(chunk.Content, "[TOOL_EVENT]")
					fmt.Fprintf(w, "event: tool\ndata: %s\n\n", eventData)
					flusher.Flush()
					continue
				}
				
				// Regular content - send as chat chunk
				data := map[string]interface{}{
					"id":      "chatcmpl-" + g.idGenerator.New(),
					"object":  "chat.completion.chunk",
					"created": g.timeProvider.Now().Unix(),
					"model":   req.Model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]string{
								"content": chunk.Content,
							},
							"finish_reason": "",
						},
					},
				}
				jsonData, _ := json.Marshal(data)
				fmt.Fprintf(w, "data: %s\n\n", jsonData)
				flusher.Flush()
			}
		}
		return
	}

	// Non-streaming request (original behavior)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	reply, err := grpcClient.Chat(ctx, &rpcproto.ChatArgs{Messages: rpcproto.ToMessagesPtr(req.Messages)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	promptTokens := countTokens(body)
	completionTokens := countTokens([]byte(reply.Content))
	resp := ChatResponse{
		ID:      "chatcmpl-" + g.idGenerator.New(),
		Object:  "chat.completion",
		Created: g.timeProvider.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: rpcproto.Message{
					Role:    "assistant",
					Content: reply.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, resp)
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ok"}`))
}

// handleAgentHealth checks if the agent is running
func (g *Gateway) handleAgentHealth(w http.ResponseWriter, r *http.Request) {
	// Use configured PID dir instead of hardcoded path
	pidDir := g.cfg.PidDir
	if pidDir == "" {
		pidDir = config.DefaultPidDir()
	}
	pidFile := filepath.Join(pidDir, "ocg-agent.pid")
	
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		writeJSON(w, map[string]string{"status": "offline", "reason": "no_pid_file"})
		return
	}
	
	// Parse PID and timestamp from pid file (format: "pid timestamp")
	parts := strings.Fields(string(pidData))
	if len(parts) < 1 {
		writeJSON(w, map[string]string{"status": "offline", "reason": "invalid_pid_file"})
		return
	}
	
	pid, err := strconv.Atoi(parts[0])
	if err != nil || pid <= 0 {
		writeJSON(w, map[string]string{"status": "offline", "reason": "invalid_pid"})
		return
	}
	
	// Use cross-platform process check
	if pid > 0 {
		// Check if process exists (cross-platform)
		p, err := os.FindProcess(pid)
		if err == nil {
			// On Unix, FindProcess always succeeds, we need to actually check
			// On Windows, FindProcess returns valid process even if exited
			err = p.Signal(syscall.Signal(0))
			if err == nil {
				writeJSON(w, map[string]string{"status": "online"})
				return
			}
			// For Windows, os.FindProcess doesn't reliably detect exited processes
			// But signal(0) works on Unix to check existence without actually signaling
		}
	}
	writeJSON(w, map[string]string{"status": "offline", "reason": "process_not_found"})
}

// handleEmbeddingHealth checks if the embedding service is running
func (g *Gateway) handleEmbeddingHealth(w http.ResponseWriter, r *http.Request) {
	// Use configured PID dir instead of hardcoded path
	pidDir := g.cfg.PidDir
	if pidDir == "" {
		pidDir = config.DefaultPidDir()
	}
	pidFile := filepath.Join(pidDir, "ocg-embedding.pid")
	
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		writeJSON(w, map[string]string{"status": "offline", "reason": "no_pid_file"})
		return
	}
	
	// Parse PID and timestamp from pid file
	parts := strings.Fields(string(pidData))
	if len(parts) < 1 {
		writeJSON(w, map[string]string{"status": "offline", "reason": "invalid_pid_file"})
		return
	}
	
	pid, err := strconv.Atoi(parts[0])
	if err != nil || pid <= 0 {
		writeJSON(w, map[string]string{"status": "offline", "reason": "invalid_pid"})
		return
	}
	
	// Check if process exists (cross-platform)
	if pid > 0 {
		p, err := os.FindProcess(pid)
		if err == nil {
			err = p.Signal(syscall.Signal(0))
			if err == nil {
				writeJSON(w, map[string]string{"status": "online"})
				return
			}
		}
	}
	writeJSON(w, map[string]string{"status": "offline", "reason": "process_not_found"})
}

func (g *Gateway) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	grpcClient := rpcproto.NewAgentGRPCClient(client)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	reply, err := grpcClient.Stats(ctx)
	if err != nil {
		http.Error(w, "error getting stats", http.StatusInternalServerError)
		return
	}
	stats := rpcproto.ConvertStats(reply.Stats)

	type StatsResponse struct {
		Status string         `json:"status"`
		Stats  map[string]int `json:"stats"`
	}

	writeJSON(w, StatsResponse{Status: "ok", Stats: stats})
}

func (g *Gateway) handleSessions(w http.ResponseWriter, r *http.Request) {
	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	grpcClient := rpcproto.NewAgentGRPCClient(client)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	reply, err := grpcClient.Sessions(ctx, &rpcproto.SessionsArgs{Limit: 50})
	if err != nil {
		http.Error(w, "error getting sessions", http.StatusInternalServerError)
		return
	}

	type SessionResponse struct {
		Sessions []map[string]interface{} `json:"sessions"`
		Count    int32                   `json:"count"`
	}

	sessions := make([]map[string]interface{}, 0, len(reply.Sessions))
	for _, s := range reply.Sessions {
		sessions = append(sessions, map[string]interface{}{
			"session_key":      s.SessionKey,
			"total_tokens":     s.TotalTokens,
			"compaction_count": s.CompactionCount,
			"updated_at":       s.UpdatedAt,
		})
	}

	writeJSON(w, SessionResponse{Sessions: sessions, Count: reply.Count})
}

// Utility functions
func countTokens(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	// Initialize tokenizer on first use
	initTokenizer()

	// Use tiktoken for accurate BPE token counting
	if tokenizer != nil {
		// Decode bytes to string for tokenization
		text := string(data)
		tokens := tokenizer.Encode(text, nil, nil)
		count := len(tokens)
		if count == 0 {
			return 1
		}
		return count
	}

	// Fallback: fast estimation if tokenizer fails
	asciiCount := 0
	nonAsciiCount := 0

	for _, b := range data {
		if b <= 0x7f {
			asciiCount++
		} else {
			nonAsciiCount++
		}
	}

	// Rough estimate: ASCII ~4 chars/token, non-ASCII (CJK) ~1.5 token/char
	tokenCount := asciiCount/4 + nonAsciiCount*3/2

	if tokenCount == 0 {
		tokenCount = 1
	}
	return tokenCount
}

func channelTypeFromString(s string) channels.ChannelType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(channels.ChannelTelegram):
		return channels.ChannelTelegram
	case string(channels.ChannelWhatsApp):
		return channels.ChannelWhatsApp
	case string(channels.ChannelSlack):
		return channels.ChannelSlack
	case string(channels.ChannelDiscord):
		return channels.ChannelDiscord
	case string(channels.ChannelWebChat):
		return channels.ChannelWebChat
	case string(channels.ChannelSignal):
		return channels.ChannelSignal
	case string(channels.ChannelGoogleChat):
		return channels.ChannelGoogleChat
	case string(channels.ChannelIMessage):
		return channels.ChannelIMessage
	case string(channels.ChannelMSTeams):
		return channels.ChannelMSTeams
	case string(channels.ChannelIRC):
		return channels.ChannelIRC
	case string(channels.ChannelMatrix):
		return channels.ChannelMatrix
	case string(channels.ChannelFeishu):
		return channels.ChannelFeishu
	case string(channels.ChannelZalo):
		return channels.ChannelZalo
	case string(channels.ChannelMattermost):
		return channels.ChannelMattermost
	case string(channels.ChannelThreema):
		return channels.ChannelThreema
	case string(channels.ChannelSession):
		return channels.ChannelSession
	case string(channels.ChannelTox):
		return channels.ChannelTox
	default:
		return ""
	}
}

func (g *Gateway) gatewayDir() string {
	if g.cfg.GatewayDir != "" {
		return g.cfg.GatewayDir
	}
	if env := os.Getenv("OCG_GATEWAY_DIR"); env != "" {
		return env
	}

	isValid := func(dir string) bool {
		if _, err := os.Stat(filepath.Join(dir, "static", "index.html")); err == nil {
			return true
		}
		return false
	}

	candidates := []string{
		filepath.Join(config.DefaultInstallDir(), "gateway"),
	}

	execPath, _ := os.Executable()
	if execPath != "" {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, "gateway"),
			filepath.Join(execDir, "..", "gateway"),
		)
	}

	for _, c := range candidates {
		if isValid(c) {
			return c
		}
	}

	return "gateway"
}

// Process handlers
func (g *Gateway) handleProcessStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyProcess)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req struct {
		Command string `json:"command"`
		Workdir string `json:"workdir,omitempty"`
		Env     string `json:"env,omitempty"`
		Pty     bool   `json:"pty,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	procTool := processtool.ProcessTool{}
	result, err := procTool.Execute(map[string]interface{}{
		"action":  "start",
		"command": req.Command,
		"workdir": req.Workdir,
		"env":     req.Env,
		"pty":     req.Pty,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (g *Gateway) handleProcessList(w http.ResponseWriter, r *http.Request) {
	procTool := processtool.ProcessTool{}
	result, err := procTool.Execute(map[string]interface{}{"action": "list"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (g *Gateway) handleProcessLog(w http.ResponseWriter, r *http.Request) {
	sessionId := r.URL.Query().Get("sessionId")
	if strings.TrimSpace(sessionId) == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}

	// Use strconv for proper error handling
	offset := 0
	limit := g.cfg.MaxProcessLogCap

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Enforce max limit
	if limit <= 0 || limit > g.cfg.MaxProcessLogCap {
		limit = g.cfg.MaxProcessLogCap
	}

	procTool := processtool.ProcessTool{}
	result, err := procTool.Execute(map[string]interface{}{
		"action":    "log",
		"sessionId": sessionId,
		"offset":    offset,
		"limit":     limit,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (g *Gateway) handleProcessKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionId := r.URL.Query().Get("sessionId")

	procTool := processtool.ProcessTool{}
	result, err := procTool.Execute(map[string]interface{}{
		"action":    "kill",
		"sessionId": sessionId,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (g *Gateway) handleProcessWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyProcess)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req struct {
		SessionID string `json:"sessionId"`
		Data      string `json:"data"`
		EOF       bool   `json:"eof,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}

	procTool := processtool.ProcessTool{}
	result, err := procTool.Execute(map[string]interface{}{
		"action":    "write",
		"sessionId": req.SessionID,
		"data":      req.Data,
		"eof":       req.EOF,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// Memory handlers
func (g *Gateway) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query().Get("query")
	category := r.URL.Query().Get("category")

	// Use strconv for proper error handling
	limit := 5
	minScore := float32(0.7)

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if scoreStr := r.URL.Query().Get("minScore"); scoreStr != "" {
		if parsed, err := strconv.ParseFloat(scoreStr, 64); err == nil && parsed >= 0 && parsed <= 1 {
			minScore = float32(parsed)
		}
	}
	// Enforce minScore bounds
	if minScore < 0 {
		minScore = 0
	}
	if minScore > 1 {
		minScore = 1
	}

	grpcClient := rpcproto.NewAgentGRPCClient(client)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	args := rpcproto.MemorySearchArgs{
		Query:    query,
		Category: category,
		Limit:    int32(limit),
		MinScore: float32(minScore),
	}
	reply, err := grpcClient.MemorySearch(ctx, &args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result interface{}
	if err := json.Unmarshal([]byte(reply.Result), &result); err != nil {
		log.Printf("[WARN] failed to parse memory search result: %v", err)
		result = map[string]interface{}{"error": err.Error()}
	}
	writeJSON(w, result)
}

func (g *Gateway) handleMemoryGet(w http.ResponseWriter, r *http.Request) {
	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	path := r.URL.Query().Get("path")

	grpcClient := rpcproto.NewAgentGRPCClient(client)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	args := rpcproto.MemoryGetArgs{Path: path}
	reply, err := grpcClient.MemoryGet(ctx, &args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result interface{}
	if err := json.Unmarshal([]byte(reply.Result), &result); err != nil {
		log.Printf("[WARN] failed to parse memory get result: %v", err)
		result = map[string]interface{}{"error": err.Error()}
	}
	writeJSON(w, result)
}

func (g *Gateway) handleMemoryStore(w http.ResponseWriter, r *http.Request) {
	client, err := g.clientOrError()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyMemory)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req struct {
		Text       string  `json:"text"`
		Category   string  `json:"category,omitempty"`
		Importance float64 `json:"importance,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	grpcClient := rpcproto.NewAgentGRPCClient(client)
	ctx, cancel := context.WithTimeout(r.Context(), rpcproto.DefaultGRPCTimeout())
	defer cancel()
	args := rpcproto.MemoryStoreArgs{
		Text:       req.Text,
		Category:   req.Category,
		Importance: float32(req.Importance),
	}
	reply, err := grpcClient.MemoryStore(ctx, &args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result interface{}
	if err := json.Unmarshal([]byte(reply.Result), &result); err != nil {
		log.Printf("[WARN] failed to parse memory store result: %v", err)
		result = map[string]interface{}{"error": err.Error()}
	}
	writeJSON(w, result)
}

// Cron handlers
func (g *Gateway) handleCronStatus(w http.ResponseWriter, r *http.Request) {
	if g.cronHandler == nil {
		http.Error(w, "cron not initialized", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, g.cronHandler.GetStatus())
}

func (g *Gateway) handleCronList(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}
	writeJSON(w, g.cronHandler.ListJobs())
}

// checkCronHandler helper to avoid nil pointer panics
func (g *Gateway) checkCronHandler(w http.ResponseWriter) bool {
	if g.cronHandler == nil {
		http.Error(w, "cron not initialized", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (g *Gateway) handleCronAdd(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// FIX: Add body size limit to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyCron)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	var jobData map[string]interface{}
	if v, ok := req["job"].(map[string]interface{}); ok {
		jobData = v
	} else {
		jobData = req
	}

	job, err := cron.CreateJobFromMap(jobData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := g.cronHandler.AddJob(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, job)
}

func (g *Gateway) handleCronUpdate(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// FIX: Add body size limit to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyCron)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	jobID, _ := req["jobId"].(string)
	if jobID == "" {
		jobID, _ = req["id"].(string)
	}
	patch, _ := req["patch"].(map[string]interface{})
	if jobID == "" || patch == nil {
		http.Error(w, "jobId and patch are required", http.StatusBadRequest)
		return
	}

	job, err := g.cronHandler.UpdateJob(jobID, patch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, job)
}

func (g *Gateway) handleCronRemove(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// FIX: Add body size limit to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyCron)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}
	jobID, _ := req["jobId"].(string)
	if jobID == "" {
		jobID, _ = req["id"].(string)
	}
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}
	if err := g.cronHandler.RemoveJob(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

func (g *Gateway) handleCronRun(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// FIX: Add body size limit to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, g.cfg.MaxBodyCron)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusBadRequest)
		return
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Parse error: "+err.Error(), http.StatusBadRequest)
		return
	}
	jobID, _ := req["jobId"].(string)
	if jobID == "" {
		jobID, _ = req["id"].(string)
	}
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}
	if err := g.cronHandler.RunJob(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleCronRuns returns run history for a job
func (g *Gateway) handleCronRuns(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}

	jobId := r.URL.Query().Get("jobId")
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	if jobId == "" {
		// Return all runs grouped by job
		jobs := g.cronHandler.ListJobs()
		allRuns := make(map[string][]cron.RunHistoryEntry)
		for _, job := range jobs {
			runs := g.cronHandler.GetRuns(job.ID, limit)
			if len(runs) > 0 {
				allRuns[job.ID] = runs
			}
		}
		writeJSON(w, allRuns)
		return
	}

	runs := g.cronHandler.GetRuns(jobId, limit)
	writeJSON(w, runs)
}

// handleCronWake triggers a wake event (heartbeat)
func (g *Gateway) handleCronWake(w http.ResponseWriter, r *http.Request) {
	if !g.checkCronHandler(w) {
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "now"
	}

	// Trigger wake callback if available
	if g.cronHandler != nil {
		// Wake is handled via the callback - for now just return success
		// The actual wake logic is in the cron handler
		log.Printf("[Cron] Wake triggered, mode: %s", mode)
	}

	writeJSON(w, map[string]interface{}{"ok": true, "mode": mode})
}

// GatewayAgentRPC implements channels.AgentRPCInterface
type GatewayAgentRPC struct {
	client *grpc.ClientConn
}

func (r *GatewayAgentRPC) Chat(messages []channels.Message) (string, error) {
	return r.ChatWithSession("default", messages)
}

func (r *GatewayAgentRPC) ChatWithSession(sessionKey string, messages []channels.Message) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("agent RPC client not connected")
	}

	rpcMessages := make([]rpcproto.Message, 0, len(messages))
	for _, m := range messages {
		rpcMessages = append(rpcMessages, rpcproto.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	client := rpcproto.NewAgentGRPCClient(r.client)
	args := rpcproto.ChatArgs{
		Messages:   rpcproto.ToMessagesPtr(rpcMessages),
		SessionKey: sessionKey,
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcproto.DefaultGRPCTimeout())
	defer cancel()

	reply, err := client.Chat(ctx, &args)
	if err != nil {
		return "", err
	}

	return reply.Content, nil
}

func (r *GatewayAgentRPC) GetStats() (map[string]int, error) {
	if r.client == nil {
		return nil, fmt.Errorf("agent RPC client not connected")
	}

	client := rpcproto.NewAgentGRPCClient(r.client)
	ctx, cancel := context.WithTimeout(context.Background(), rpcproto.DefaultGRPCTimeout())
	defer cancel()

	reply, err := client.Stats(ctx)
	if err != nil {
		return nil, err
	}

	return rpcproto.ConvertStats(reply.Stats), nil
}

func (r *GatewayAgentRPC) SendAudioChunk(sessionKey string, audioData []byte) error {
	if r.client == nil {
		return fmt.Errorf("agent RPC client not connected")
	}
	client := rpcproto.NewAgentGRPCClient(r.client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reply, err := client.SendAudioChunk(ctx, &rpcproto.AudioChunkArgs{
		SessionKey: sessionKey,
		AudioData:  audioData,
	})
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return fmt.Errorf("%s", reply.Error)
	}
	return nil
}

func (r *GatewayAgentRPC) EndAudioStream(sessionKey string) error {
	if r.client == nil {
		return fmt.Errorf("agent RPC client not connected")
	}
	client := rpcproto.NewAgentGRPCClient(r.client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reply, err := client.EndAudioStream(ctx, &rpcproto.AudioArgs{SessionKey: sessionKey})
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return fmt.Errorf("%s", reply.Error)
	}
	return nil
}

// ChatRequest represents OpenAI-compatible chat request
// (kept local to avoid dependency on rpcproto types)
type ChatRequest struct {
	Model    string             `json:"model"`
	Messages []rpcproto.Message `json:"messages"`
	Stream   bool               `json:"stream,omitempty"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int              `json:"index"`
	Message      rpcproto.Message `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// handlePulseTrigger triggers immediate pulse processing
// This endpoint is called by the webhook handler when a wake event with mode="now" is received
func (g *Gateway) handlePulseTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use internal RPC to trigger pulse on agent
	if g.client == nil {
		http.Error(w, "agent not connected", http.StatusServiceUnavailable)
		return
	}

	client := rpcproto.NewAgentGRPCClient(g.client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call PulseStatus to trigger pulse processing (it will check for new events)
	_, err := client.PulseStatus(ctx)
	if err != nil {
		log.Printf("[Gateway] Pulse trigger failed: %v", err)
		http.Error(w, fmt.Sprintf("pulse trigger failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
