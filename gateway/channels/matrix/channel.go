// Package matrix provides Matrix channel implementation
package matrix

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// MatrixChannel implements the types.ChannelLoader interface
type MatrixChannel struct {
	homeserver  string
	userID      string
	accessToken string
	deviceID    string
	client      *http.Client
	agentRPC    types.AgentRPCInterface
	running     bool
	stopCh      chan struct{}
}

// NewMatrixChannel creates a new Matrix channel
func NewMatrixChannel(homeserver, userID, accessToken, deviceID string, agentRPC types.AgentRPCInterface) *MatrixChannel {
	return &MatrixChannel{
		homeserver:  strings.TrimRight(homeserver, "/"),
		userID:      userID,
		accessToken: accessToken,
		deviceID:    deviceID,
		client:      &http.Client{Timeout: 30 * time.Second},
		agentRPC:    agentRPC,
		stopCh:      make(chan struct{}),
	}
}

func (c *MatrixChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Matrix",
		Type:        types.ChannelMatrix,
		Version:     "1.0.0",
		Description: "Matrix open protocol for real-time communication",
		Capabilities: []string{
			"text",
			"media",
			"rooms",
			"webhooks",
			"encryption",
		},
	}
}

func (c *MatrixChannel) Initialize(config map[string]interface{}) error {
	if c.homeserver == "" {
		c.homeserver = "https://matrix.org"
	}
	return nil
}

func (c *MatrixChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Matrix channel: %s", c.homeserver)

	// Verify connection
	resp, err := c.client.Get(c.homeserver + "/_matrix/client/versions")
	if err != nil {
		return fmt.Errorf("failed to connect to Matrix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("matrix API error: %d", resp.StatusCode)
	}

	c.running = true
	log.Printf("[OK] Matrix channel connected")
	return nil
}

func (c *MatrixChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Matrix channel stopped")
	return nil
}

func (c *MatrixChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 4000 {
		text = text[:4000]
	}

	// Matrix room ID format: !roomID:homeserver
	roomID := fmt.Sprintf("!%d:%s", req.ChatID, c.homeserver)

	// Create message event
	txnID := fmt.Sprintf("msg_%d", time.Now().UnixMilli())
	event := map[string]interface{}{
		"type":           "m.room.message",
		"content":        map[string]string{"msgtype": "m.text", "body": text},
		"room_id":        roomID,
		"sender":         c.userID,
		"transaction_id": txnID,
	}

	payloadBytes, _ := json.Marshal(event)
	matrixURL := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message/%s",
		c.homeserver, url.PathEscape(roomID), txnID)

	httpReq, _ := http.NewRequest("PUT", matrixURL, strings.NewReader(string(payloadBytes)))
	httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("matrix API error: %d", resp.StatusCode)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *MatrixChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Matrix webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *MatrixChannel) HealthCheck() error {
	resp, err := c.client.Get(c.homeserver + "/_matrix/client/versions")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("matrix unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// MatrixCreateFromEnv creates Matrix channel from environment
func MatrixCreateFromEnv(agentRPC types.AgentRPCInterface) (*MatrixChannel, error) {
	homeserver := os.Getenv("MATRIX_HOMESERVER")
	userID := os.Getenv("MATRIX_USER_ID")
	accessToken := os.Getenv("MATRIX_ACCESS_TOKEN")
	deviceID := os.Getenv("MATRIX_DEVICE_ID")

	if accessToken == "" {
		return nil, fmt.Errorf("MATRIX_ACCESS_TOKEN not set")
	}

	channel := NewMatrixChannel(homeserver, userID, accessToken, deviceID, agentRPC)
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
