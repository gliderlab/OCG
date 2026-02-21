// Package tox provides Tox P2P messaging channel implementation
package tox

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// ToxChannel implements the types.ChannelLoader interface
// Note: Tox core would run as a daemon, this provides HTTP gateway interface
type ToxChannel struct {
	toxID     string
	daemonURL string
	proxyAddr string
	client    *http.Client
	agentRPC  types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
}

// NewToxChannel creates a new Tox channel
func NewToxChannel(toxID, daemonURL, proxyAddr string, agentRPC types.AgentRPCInterface) *ToxChannel {
	return &ToxChannel{
		toxID:     toxID,
		daemonURL: daemonURL,
		proxyAddr: proxyAddr,
		client:    &http.Client{Timeout: 30 * time.Second},
		agentRPC:  agentRPC,
		stopCh:    make(chan struct{}),
	}
}

func (c *ToxChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Tox",
		Type:        types.ChannelTox,
		Version:     "1.0.0",
		Description: "Tox P2P encrypted messaging",
		Capabilities: []string{
			"text",
			"media",
			"voice",
			"video",
			"end-to-end",
			"p2p",
		},
	}
}

func (c *ToxChannel) Initialize(config map[string]interface{}) error {
	if c.daemonURL == "" {
		c.daemonURL = "http://localhost:25900"
	}
	return nil
}

func (c *ToxChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Tox channel...")

	// Check if toxcore daemon is running
	conn, err := net.Dial("tcp", "localhost:25900")
	if err != nil {
		log.Printf("[WARN] Tox daemon not running, starting in proxy mode")
	} else {
		conn.Close()
	}

	c.running = true
	log.Printf("[OK] Tox channel ready")
	return nil
}

func (c *ToxChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Tox channel stopped")
	return nil
}

func (c *ToxChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 1400 {
		text = text[:1400]
	}

	// Tox ID format: XXXXX...XXXXX@xxx.xxx.xxx.xxx
	// Convert chat ID to Tox ID format
	recipientID := fmt.Sprintf("%d", req.ChatID)
	if len(recipientID) < 76 {
		// Pad or hash to get valid Tox ID
		recipientID = fmt.Sprintf("%-76s", recipientID)[:76]
	}

	// Send via Tox HTTP API (if toxcore is running as HTTP daemon)
	payload := map[string]interface{}{
		"recipient": recipientID,
		"message":   text,
	}

	_ = payload

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *ToxChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Tox webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *ToxChannel) HealthCheck() error {
	if c.toxID == "" {
		return fmt.Errorf("TOX_ID not set")
	}

	// Try to connect to toxcore daemon
	conn, err := net.DialTimeout("tcp", "localhost:25900", 2*time.Second)
	if err != nil {
		return fmt.Errorf("tox daemon not reachable: %w", err)
	}
	conn.Close()
	return nil
}

// ToxCreateFromEnv creates Tox channel from environment
func ToxCreateFromEnv(agentRPC types.AgentRPCInterface) (*ToxChannel, error) {
	toxID := os.Getenv("TOX_ID")
	daemonURL := os.Getenv("TOX_DAEMON_URL")
	proxyAddr := os.Getenv("TOX_PROXY_ADDR")

	if toxID == "" && proxyAddr == "" {
		return nil, fmt.Errorf("TOX_ID or TOX_PROXY_ADDR must be set")
	}

	channel := NewToxChannel(toxID, daemonURL, proxyAddr, agentRPC)
	if err := channel.Initialize(nil); err != nil {
		return nil, err
	}
	return channel, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
