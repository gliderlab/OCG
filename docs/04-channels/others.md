# Other Channels

Configure other messaging platforms.

---

## Slack

### Configuration

```bash
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_SIGNING_SECRET="..."
export SLACK_APP_TOKEN="xapp-..."
```

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "bot_token": "xoxb-...",
      "signing_secret": "...",
      "app_token": "xapp-..."
    }
  }
}
```

### Setup

1. Create app at [Slack API](https://api.slack.com/apps)
2. Enable Event Subscriptions
3. Subscribe to `message.channels`, `message.groups`
4. Install app to workspace

---

## WhatsApp

### Configuration

```bash
export WHATSAPP_PHONE_ID="..."
export WHATSAPP_ACCESS_TOKEN="..."
```

```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "phone_id": "...",
      "access_token": "..."
    }
  }
}
```

### Setup

1. Create Meta Developer account
2. Create WhatsApp API application
3. Get phone number ID
4. Generate access token

---

## IRC

### Configuration

```bash
export IRC_SERVER="irc.libera.chat"
export IRC_PORT="6667"
export IRC_NICK="ocg_bot"
export IRC_CHANNEL="#your-channel"
```

```json
{
  "channels": {
    "irc": {
      "enabled": true,
      "server": "irc.libera.chat",
      "port": 6667,
      "nick": "ocg_bot",
      "channel": "#your-channel"
    }
  }
}
```

---

## Google Chat

### Configuration

```bash
export GOOGLECHAT_BOT_TOKEN="..."
export GOOGLECHAT_SPACE_NAME="spaces/..."
```

```json
{
  "channels": {
    "googlechat": {
      "enabled": true,
      "bot_token": "...",
      "space_name": "spaces/..."
    }
  }
}
```

---

## Microsoft Teams

### Configuration

```bash
export MSTEAMS_TENANT_ID="..."
export MSTEAMS_CLIENT_ID="..."
export MSTEAMS_CLIENT_SECRET="..."
```

```json
{
  "channels": {
    "msteams": {
      "enabled": true,
      "tenant_id": "...",
      "client_id": "...",
      "client_secret": "..."
    }
  }
}
```

---

## WebChat (WebSocket)

### Configuration

```json
{
  "channels": {
    "webchat": {
      "enabled": true,
      "websocket_path": "/v1/chat/ws"
    }
  }
}
```

Connects via WebSocket for custom integrations.

---

## Chinese Platforms

### Feishu (飞书)

```bash
export FEISHU_APP_ID="..."
export FEISHU_APP_SECRET="..."
```

### Zalo

```bash
export ZALO_APP_ID="..."
export ZALO_APP_SECRET="..."
```

### LINE

```bash
export LINE_CHANNEL_TOKEN="..."
export LINE_CHANNEL_SECRET="..."
```

### Matrix

```bash
export MATRIX_HOMESERVER="https://matrix.org"
export MATRIX_USER="@ocg:matrix.org"
export MATRIX_PASSWORD="..."
```

---

## Signal

### Configuration

```bash
export SIGNAL_PHONE_NUMBER="+1234567890"
export SIGNAL_REGISTRATION_ID="..."
export SIGNAL_IDENTITY_KEY="..."
```

Note: Signal requires additional setup with signal-cli.

---

## Channel Comparison

| Channel | Protocol | Setup Difficulty | Real-time |
|---------|----------|------------------|-----------|
| Telegram | HTTP | Easy | Yes |
| Discord | WSS | Medium | Yes |
| Slack | RTM/API | Medium | Yes |
| WhatsApp | HTTP | Medium | Yes |
| IRC | TCP | Easy | Yes |
| WebChat | WebSocket | Easy | Yes |
| Signal | P2P | Hard | Yes |
| Feishu | HTTP | Medium | No |

---

## Common Issues

### Not Receiving Messages

1. Check channel is enabled in config
2. Verify authentication credentials
3. Check logs for errors

### Rate Limiting

Each channel has different rate limits. OCG handles most automatically.

### Connection Issues

```bash
# Check network connectivity
nc -zv server.com port

# Check logs
tail -f /tmp/ocg-gateway.log
```

---

## See Also

- [Channels Overview](../overview.md)
- [Telegram Setup](telegram.md)
- [Discord Setup](discord.md)
