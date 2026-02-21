# Channel Adapter

Multi-channel support for OCG-Go.

## Implemented Channels

| Channel | Directory | Status |
|---------|-----------|--------|
| Telegram | `telegram/` | ✅ Implemented |
| Discord | `discord/` | ✅ Implemented |
| Slack | `slack/` | ✅ Implemented |
| WhatsApp | `whatsapp/` | ✅ Implemented |
| Signal | `signal/` | ✅ Implemented |
| IRC | `irc/` | ✅ Implemented |
| Google Chat | `googlechat/` | ✅ Implemented |
| Microsoft Teams | `msteams/` | ✅ Implemented |
| WebChat | `webchat/` | ✅ Implemented |

## Architecture

```
gateway/channels/
├── types/
│   └── types.go       # Shared interfaces (ChannelLoader, types)
├── channel_adapter.go  # Channel adapter
├── bot.go            # Telegram wrapper (backward compatible)
├── telegram/         # Telegram implementation
├── discord/
├── slack/
├── whatsapp/
├── signal/
├── irc/
├── googlechat/
├── msteams/
└── webchat/
```

## Interface

Each channel implements `types.ChannelLoader`:

```go
type ChannelLoader interface {
    ChannelInfo() ChannelInfo
    Initialize(config map[string]interface{}) error
    Start() error
    Stop() error
    SendMessage(req *SendMessageRequest) (*SendMessageResponse, error)
    HandleWebhook(w http.ResponseWriter, r *http.Request)
    HealthCheck() error
}
```
