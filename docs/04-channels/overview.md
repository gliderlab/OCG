# Channel Overview

Messaging platforms supported by OCG.

---

## Supported Channels

OCG supports 18 messaging channels:

| Channel | Protocol | Status |
|---------|----------|--------|
| Telegram | Long Polling / Webhook | ✅ |
| Discord | Gateway (WSS) | ✅ |
| Slack | Events API + RTM | ✅ |
| WhatsApp | Cloud API | ✅ |
| Signal | P2P | ✅ |
| IRC | Raw TCP | ✅ |
| Google Chat | HTTP | ✅ |
| Microsoft Teams | HTTP | ✅ |
| WebChat | WebSocket | ✅ |
| Mattermost | HTTP | ✅ |
| LINE | HTTP | ✅ |
| Matrix | HTTP | ✅ |
| Feishu (飞书) | HTTP | ✅ |
| Zalo | HTTP | ✅ |
| Threema | HTTP | ✅ |
| Session | P2P | ✅ |
| Tox | P2P | ✅ |
| iMessage | P2P | ✅ |

---

## Channel Architecture

```
Gateway (55003)
    │
    ├── telegram/      # Telegram Bot API
    ├── discord/       # Discord Gateway + REST
    ├── slack/         # Slack Events + RTM
    ├── whatsapp/     # WhatsApp Cloud API
    ├── signal/       # Signal P2P
    ├── irc/          # IRC RFC 1459
    ├── googlechat/   # Google Chat API
    ├── msteams/      # Microsoft Teams API
    ├── webchat/      # WebSocket chat
    └── ...
```

---

## Enabling Channels

### Via Environment Variables

```bash
# Telegram
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_MODE="long_polling"  # or "webhook"

# Discord
export DISCORD_BOT_TOKEN="your-token"

# Slack
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_SIGNING_SECRET="..."
```

### Via Configuration File

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "your-token"
    },
    "discord": {
      "enabled": true,
      "bot_token": "your-token"
    }
  }
}
```

---

## Common Features

### Typing Indicator

Shows "typing" while AI is generating response:

- **Telegram**: `sendChatAction` API
- **Discord**: `channels/{id}/typing`
- **Slack**: `users.typing`

### Session Context

Each user/channel has independent conversation history:

- Session key format: `telegram_{chat_id}`, `discord_{channel_id}`
- Automatic history loading (last 100 messages)
- Per-channel context isolation

---

## Rate Limiting

```bash
# Set rate limits per channel
./bin/ocg ratelimit set --channel telegram --max 30 --window 60

# List all limits
./bin/ocg ratelimit list

# Check specific channel
./bin/ocg ratelimit check --channel telegram
```

---

## See Also

- [Telegram Setup](telegram.md)
- [Discord Setup](discord.md)
- [Other Channels](others.md)
