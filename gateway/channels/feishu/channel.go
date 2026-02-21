// Package feishu provides Feishu (Lark) channel implementation
package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// FeishuChannel implements the types.ChannelLoader interface
type FeishuChannel struct {
	appID     string
	appSecret string
	tenantKey string
	token     string
	client    *http.Client
	agentRPC  types.AgentRPCInterface
	running   bool
	stopCh    chan struct{}
}

// NewFeishuChannel creates a new Feishu channel
func NewFeishuChannel(appID, appSecret, tenantKey string, agentRPC types.AgentRPCInterface) *FeishuChannel {
	return &FeishuChannel{
		appID:     appID,
		appSecret: appSecret,
		tenantKey: tenantKey,
		client:    &http.Client{Timeout: 30 * time.Second},
		agentRPC:  agentRPC,
		stopCh:    make(chan struct{}),
	}
}

func (c *FeishuChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{
		Name:        "Feishu",
		Type:        types.ChannelFeishu,
		Version:     "1.0.0",
		Description: "Feishu (Lark) enterprise messaging integration",
		Capabilities: []string{
			"text",
			"media",
			"cards",
			"buttons",
			"webhooks",
			"threads",
		},
	}
}

func (c *FeishuChannel) Initialize(config map[string]interface{}) error {
	return nil
}

func (c *FeishuChannel) Start() error {
	if c.running {
		return nil
	}
	log.Printf("[START] Starting Feishu channel...")

	// Get tenant access token
	tokenURL := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	payload := map[string]interface{}{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	payloadBytes, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", tokenURL, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect to Feishu: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)

	if token, ok := result["tenant_access_token"].(string); ok {
		c.token = token
	} else {
		return fmt.Errorf("failed to get Feishu token: %s", string(body))
	}

	c.running = true
	log.Printf("[OK] Feishu channel connected")
	return nil
}

func (c *FeishuChannel) Stop() error {
	if !c.running {
		return nil
	}
	close(c.stopCh)
	c.running = false
	log.Printf("🛑 Feishu channel stopped")
	return nil
}

func (c *FeishuChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 4000 {
		text = text[:4000]
	}

	// Feishu uses chat_id for group chats, open_id for users
	msgType := "text"
	content := map[string]interface{}{
		"text": text,
	}

	// Handle rich text with buttons
	if len(req.Buttons) > 0 {
		msgType = "interactive"
		elements := make([]map[string]interface{}, 0)
		for _, row := range req.Buttons {
			actions := make([]map[string]interface{}, 0)
			for _, btn := range row {
				actions = append(actions, map[string]interface{}{
					"tag":  "button",
					"text": btn.Text,
					"url":  "https://open.feishu.cn",
					"type": "primary",
				})
			}
			elements = append(elements, map[string]interface{}{
				"tag":     "action",
				"actions": actions,
			})
		}
		card := map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": "Message",
				},
			},
			"elements": elements,
		}
		contentBytes, _ := json.Marshal(card)
		content = map[string]interface{}{
			"type": "template",
			"data": map[string]interface{}{
				"raw": string(contentBytes),
			},
		}
	}

	payload := map[string]interface{}{
		"receive_id": fmt.Sprintf("%d", req.ChatID),
		"msg_type":   msgType,
		"content":    content,
	}

	payloadBytes, _ := json.Marshal(payload)

	// Determine endpoint based on chat type
	url := "https://open.feishu.cn/open-apis/im/v1/messages"
	if strings.HasPrefix(fmt.Sprintf("%d", req.ChatID), "oc_") {
		url += "?receive_id_type=open_id"
	} else {
		url += "?receive_id_type=chat_id"
	}

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &types.SendMessageResponse{OK: false, Error: string(body)}, fmt.Errorf("feishu API error: %d", resp.StatusCode)
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: time.Now().Unix(),
		ChatID:    req.ChatID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *FeishuChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	types.LimitBody(w, r)
	body, _ := io.ReadAll(r.Body)
	log.Printf("Feishu webhook: %s", string(body[:min(200, len(body))]))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (c *FeishuChannel) HealthCheck() error {
	if c.token == "" {
		return fmt.Errorf("feishu not authenticated")
	}
	return nil
}

// FeishuCreateFromEnv creates Feishu channel from environment
func FeishuCreateFromEnv(agentRPC types.AgentRPCInterface) (*FeishuChannel, error) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	tenantKey := os.Getenv("FEISHU_TENANT_KEY")

	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("FEISHU_APP_ID and FEISHU_APP_SECRET must be set")
	}

	channel := NewFeishuChannel(appID, appSecret, tenantKey, agentRPC)
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
