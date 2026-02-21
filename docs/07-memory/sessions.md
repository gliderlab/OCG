# Session Memory

Per-user, per-channel conversation history.

---

## Overview

Each user/channel has independent conversation context.

### Session Key Format

| Channel | Format | Example |
|---------|--------|---------|
| Telegram | `telegram_{chat_id}` | telegram_123456789 |
| Discord | `discord_{channel_id}` | discord_987654321 |
| Slack | `slack_{channel_id}` | slack_C00123456 |

---

## Features

### History Loading

- Automatic loading of last 100 messages
- Context preserved across sessions
- Independent contexts per channel

### Commands

| Command | Description |
|---------|-------------|
| `/new` | Create new session |
| `/reset` | Reset current session |
| `/compact` | Compress conversation |

---

## Configuration

```json
{
  "sessions": {
    "max_messages": 100,
    "dm_scope": "all"  // or "private", "group"
  }
}
```

---

## Persistence

Sessions are stored in SQLite:

```bash
# View sessions
./bin/ocg sessions list

# View history
./bin/ocg sessions history <session_key>
```

---

## See Also

- [Memory Overview](../overview.md)
- [Compaction](compaction.md)
