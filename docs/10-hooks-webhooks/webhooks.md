# Webhooks

HTTP endpoints for external integrations.

---

## Overview

Webhooks allow external services to trigger OCG events.

---

## Configuration

```bash
export WEBHOOK_ENABLED=true
export WEBHOOK_TOKEN="your-secret-token"
export WEBHOOK_PATH="/hooks"
```

```json
{
  "webhook": {
    "enabled": true,
    "token": "your-secret-token",
    "path": "/hooks",
    "rate_limit": 100
  }
}
```

---

## Endpoints

### Wake Event

```
POST /hooks/wake
```

Triggers system wake event.

### Agent Event

```
POST /hooks/agent
```

Runs isolated agent turn.

### Custom Mapping

```
POST /hooks/:name
```

Maps to custom handler.

---

## Authentication

Include token in header:

```bash
curl -X POST http://localhost:55003/hooks/wake \
  -H "Authorization: Bearer your-secret-token" \
  -H "X-OCG-Token: your-ui-token"
```

---

## See Also

- [Hooks Overview](hooks.md)
