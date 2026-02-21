// Package ircpkg provides IRC channel implementation
package ircpkg

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

type IRCChannel struct {
	host string
	port int
	nick string
	agentRPC types.AgentRPCInterface
	running bool
	stopCh  chan struct{}
}

func NewIRCChannel(host string, port int, nick string, agentRPC types.AgentRPCInterface) *IRCChannel {
	return &IRCChannel{host: host, port: port, nick: nick, agentRPC: agentRPC, stopCh: make(chan struct{})}
}

func (c *IRCChannel) ChannelInfo() types.ChannelInfo {
	return types.ChannelInfo{Name: "IRC", Type: types.ChannelIRC, Version: "1.0.0", Description: "IRC protocol integration"}
}
func (c *IRCChannel) Initialize(config map[string]interface{}) error { return nil }
func (c *IRCChannel) Start() error { log.Printf("[START] Starting IRC channel..."); c.running = true; return nil }
func (c *IRCChannel) Stop() error { c.running = false; return nil }
func (c *IRCChannel) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	return &types.SendMessageResponse{OK: true, MessageID: time.Now().Unix(), ChatID: req.ChatID, Timestamp: time.Now().Unix()}, nil
}
func (c *IRCChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ok": true}`) }
func (c *IRCChannel) HealthCheck() error { return nil }

func IRCCreateFromEnv(agentRPC types.AgentRPCInterface) (*IRCChannel, error) {
	host := os.Getenv("IRC_HOST")
	if host == "" { host = "irc.libera.chat" }
	port := 6667
	nick := os.Getenv("IRC_NICK")
	if nick == "" { nick = "openclaw" }
	return NewIRCChannel(host, port, nick, agentRPC), nil
}
