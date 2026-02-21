# Discord Setup

Configure Discord bot for OCG.

---

## Prerequisites

1. Create application at [Discord Developer Portal](https://discord.com/developers/applications)
2. Create bot and get token
3. Enable required intents:
   - Message Content Intent
   - Presence Intent
4. Invite bot to server

---

## Configuration

### Environment Variables

```bash
export DISCORD_BOT_TOKEN="your-bot-token"
export DISCORD_APPLICATION_ID="your-app-id"
export DISCORD_PUBLIC_KEY="your-public-key"
```

### Configuration File

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "bot_token": "your-bot-token",
      "application_id": "your-app-id",
      "public_key": "your-public-key"
    }
  }
}
```

---

## Setup Steps

### 1. Create Application

```bash
# Go to Discord Developer Portal
# https://discord.com/developers/applications
#
# 1. Click "New Application"
# 2. Name your application
# 3. Click "Bot" → "Add Bot"
# 4. Copy bot token
```

### 2. Enable Intents

In Discord Developer Portal:
- Bot → Enable "Message Content Intent"
- Bot → Enable "Presence Intent"

### 3. Invite Bot

```bash
# Generate invite URL with permissions
https://discord.com/api/oauth2/authorize?client_id=YOUR_APP_ID&permissions=379968&scope=bot%20applications.commands
```

### 4. Configure OCG

```bash
export DISCORD_BOT_TOKEN="your-actual-bot-token"
./bin/ocg restart
```

---

## Features

### Typing Indicator

Shows bot is typing:

```go
// Discord API: POST /channels/{id}/typing
```

### Slash Commands

| Command | Description |
|---------|-------------|
| `/chat <message>` | Send message to AI |
| `/help` | Show help |
| `/new` | New conversation session |

### Message Formatting

Supports Discord markdown:
- **Bold** `**text**`
- *Italic* `*text*`
- Code `` `code` ``
- Code blocks ```` ```code``` ````

---

## Troubleshooting

### Bot Offline

```bash
# Check token is correct
# Verify intents are enabled
# Check bot has proper permissions
```

### Not Responding

1. Check Message Content Intent is enabled
2. Verify bot is in the server
3. Check logs: `tail -f /tmp/ocg-gateway.log`

### Permission Errors

Required bot permissions:
- Send Messages
- Embed Links
- Attach Files
- Read Message History
- Use Slash Commands

---

## Security

### Bot Token

- Keep token secret
- Don't commit to git
- Rotate if compromised

### Rate Limiting

```bash
# Discord has built-in rate limits
# OCG respects these automatically
```

---

## See Also

- [Channels Overview](../overview.md)
- [Discord API Docs](https://discord.com/developers/docs)
