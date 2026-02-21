// Package msteams provides Microsoft Teams channel implementation
package msteams

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

type MSTeamsChannel struct {
	appID       string
	appPassword string
	tenantID    string
	client      *http.Client
	agentRPC    types.AgentRPCInterface
	running     bool
	stopCh      chan struct{}
}

func NewMSTeamsChannel(appID, appPassword, tenantID string, agentRPC types.AgentRPCInterface) *MSTeamsChannel {
	return &MSTeamsChannel{
		appID: appID, appPassword: appPassword, tenantID: tenantID,
		client: &http.Client{Timeout: 30 * time.Second}, agentRPC: agentRPC, stopCh: make(chan struct{}),
	}
}

func (c *MSTeamsChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{Name: "Microsoft Teams", Type: types.ChannelMSTeams, Version: "1.0.0", Description: "Microsoft Teams Bot Framework integration"}
}
func (c *MSTeamsChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *MSTeamsChannel) Start() error { log.Printf("[START] Starting Microsoft Teams channel..."); c.running = true; return nil }
func (c *MSTeamsChannel) Stop() error { c.running = false; return nil }
func (c *MSTeamsChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}
func (c *MSTeamsChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ok": true}`) }
func (c *MSTeamsChannel) HealthCheck() error { return nil }

func MSTeamsCreateFromEnv(agentRPC types.AgentRPCInterface) (*MSTeamsChannel, error) {
	appID := os.Getenv("MSTEAMS_APP_ID")
	if appID == "" { return nil, fmt.Errorf("MSTEAMS_APP_ID not set") }
	return NewMSTeamsChannel(appID, os.Getenv("MSTEAMS_APP_PASSWORD"), os.Getenv("MSTEAMS_TENANT_ID"), agentRPC), nil
}
