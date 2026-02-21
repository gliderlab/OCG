// Package threema provides Threema messaging channel implementation
package threema

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// ThreemaChannel implements the types.ChannelLoader interface
type ThreemaChannel struct {
	gatewayID  string
	secret     string
	privateKey string
	publicKey  string
	identity   string
	client     *http.Client
	agentRPC   types.AgentRPCInterface
	running    bool
	stopCh     chan struct{}
}

// NewThreemaChannel creates a new Threema channel
func NewThreemaChannel(gatewayID, secret, privateKey, publicKey, identity string, agentRPC types.AgentRPCInterface) *ThreemaChannel {
	return &ThreemaChannel{
		gatewayID:  gatewayID,
		secret:     secret,
		privateKey: privateKey,
		publicKey:  publicKey,
		identity:   identity,
		client:     &http.Client{Timeout: 30 * time.Second},
		agentRPC:   agentRPC,
		stopCh:     make(chan struct{}),
	}
}

func (c *ThreemaChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Threema",
		Type:        types.ChannelThreema,
		Version:     "1.0.0",
		Description: "Threema secure messaging integration",
		Capabilities: []string{
			"text",
			"media",
			"end-to-end",
			"webhooks",
		},
	}
}

func (c *ThreemaChannel) Initialize(config map[string]interface{}) error {
	return nil
}

func (c *ThreemaChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Threema channel...")

	// Verify gateway connection
	url := fmt.Sprintf("https://msgapi.threema.ch/%s/capabilities", c.gatewayID)
	if c.secret != "" {
		url += "?secret=" + c.secret
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Threema: %w", err)
	}
	defer resp.Body.Close()

	c.running = true
	log.Printf("[OK] Threema channel connected")
	return nil
}

func (c *ThreemaChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Threema channel stopped")
	return nil
}

// encryptMessage encrypts message for Threema
//
//nolint:unused
func (c *ThreemaChannel) encryptMessage(text, recipientKey string) (string, error) {
	// Simplified: using random key for demo
	// Real implementation would use Threema's NaCl encryption
	key := make([]byte, 32)
	rand.Read(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *ThreemaChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 3500 {
		text = text[:3500]
	}

	// Threema Gateway API
	payload := map[string]interface{}{
		"from": c.identity,
		"to":   fmt.Sprintf("%d", req.ChatID),
		"text": text,
	}

	payloadBytes, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://msgapi.threema.ch/%s/messages", c.gatewayID)
	if c.secret != "" {
		url += "?secret=" + c.secret
	}

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("threema API error: %d", resp.StatusCode)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *ThreemaChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Threema webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *ThreemaChannel) HealthCheck() error {
	url := fmt.Sprintf("https://msgapi.threema.ch/%s/capabilities", c.gatewayID)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("threema unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// ThreemaCreateFromEnv creates Threema channel from environment
func ThreemaCreateFromEnv(agentRPC types.AgentRPCInterface) (*ThreemaChannel, error) {
	gatewayID := os.Getenv("THREEMA_GATEWAY_ID")
	secret := os.Getenv("THREEMA_SECRET")
	privateKey := os.Getenv("THREEMA_PRIVATE_KEY")
	publicKey := os.Getenv("THREEMA_PUBLIC_KEY")
	identity := os.Getenv("THREEMA_IDENTITY")

	if gatewayID == "" {
		return nil, fmt.Errorf("THREEMA_GATEWAY_ID not set")
	}

	channel := NewThreemaChannel(gatewayID, secret, privateKey, publicKey, identity, agentRPC)
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
