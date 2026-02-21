# Telegram Setup

Configure Telegram bot for OCG.

---

## Prerequisites

1. Create a bot via [@BotFather](https://t.me/BotFather)
2. Get your bot token
3. Start a chat with your bot

---

## Configuration

### Environment Variables

```bash
# Bot token from @BotFather
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# Mode: long_polling (default) or webhook
export TELEGRAM_MODE="long_polling"

# For webhook mode (requires public URL)
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
```

### Configuration File

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
      "mode": "long_polling"
    }
  }
}
```

---

## Long Polling vs Webhook

### Long Polling (Default)

**Advantages:**
- No public URL required
- Works on localhost/private networks
- Simpler setup

**Usage:**
```bash
export TELEGRAM_MODE="long_polling"
export TELEGRAM_BOT_TOKEN="your-token"
./bin/ocg start
```

### Webhook

**Requirements:**
- Public HTTPS URL
- SSL certificate

**Usage:**
```bash
export TELEGRAM_MODE="webhook"
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
./bin/ocg start
```

---

## Features

### Typing Indicator

Shows "typing" while AI generates response.

```go
// Implemented via sendChatAction API
// action: "typing"
```

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Start conversation |
| `/help` | Show help |
| `/new` | Create new session |
| `/reset` | Reset current session |

### Inline Buttons

Support for inline keyboard buttons:

```json
{
  "buttons": [
    [{"text": "Option 1", "callback_data": "opt1"}],
    [{"text": "Option 2", "callback_data": "opt2"}]
  ]
}
```

---

## Bot Commands Setup

To show bot commands in Telegram:

```bash
# Set commands via BotFather
/setcommands

# Add:
# start - Start conversation
# help - Show help
# new - New session
# reset - Reset session
```

---

## Troubleshooting

### Bot Not Responding

1. Check token is correct
2. Verify bot was started (/start)
3. Check logs: `tail -f /tmp/ocg-gateway.log`

### Long Polling Issues

```bash
# If long polling fails, check network
curl -s https://api.telegram.org/bot<TOKEN>/getMe
```

### Webhook Issues

```bash
# Verify webhook URL is reachable
curl -X POST https://your-domain.com/webhooks/telegram \
  -H "Content-Type: application/json" \
  -d '{"message": {"chat": {"id": 123}}}'
```

---

## Security

### Bot Token

- Keep token secret
- Don't commit to git
- Use environment variables

### Private Chats

Bot only responds to users who have started the chat.

---

## See Also

- [Channels Overview](../overview.md)
- [Telegram Integration](./telegram.md)
