# OCG Web API Documentation

> OpenClaw-Go (OCG) HTTP API Reference

## Base URL

```
http://localhost:55003
```

## Authentication

All protected endpoints require Bearer token authentication:

```bash
Authorization: Bearer <OCG_UI_TOKEN>
```

## Public Endpoints

### Health Check

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Gateway health status |
| GET | `/agent/health` | Agent service health |
| GET | `/embedding/health` | Embedding service health |

**Response:**
```json
{
  "status": "ok",
  "version": "v0.0.1beta13",
  "timestamp": "2026-02-21T15:00:00Z"
}
```

### Telegram Webhook

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/telegram/webhook` | Receive Telegram updates |
| POST | `/telegram/setWebhook` | Configure webhook URL |
| GET | `/telegram/status` | Get Telegram bot status |

---

## Protected Endpoints

### Chat Completion

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/chat/completions` | OpenAI-compatible chat API |

**Request:**
```json
{
  "model": "default",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": false
}
```

**Response:**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "default",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help?"
      },
      "finish_reason": "stop"
    }
  ]
}
```

**Streaming Response:**
```json
{
  "choices": [
    {
      "delta": {
        "content": "Hello"
      },
      "index": 0,
      "finish_reason": null
    }
  ]
}
```

---

### Session Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/sessions/list` | List all sessions |

**Query Parameters:**
- `activeMinutes` - Filter by active minutes
- `kinds` - Filter by session kinds (comma-separated)
- `limit` - Maximum results (default 50)

**Response:**
```json
{
  "sessions": [
    {
      "key": "telegram:123456",
      "channel": "telegram",
      "userId": "123456",
      "messageCount": 10,
      "lastActive": "2026-02-21T14:30:00Z"
    }
  ]
}
```

---

### Process Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/process/start` | Start a process |
| GET | `/process/list` | List running processes |
| GET | `/process/log` | Get process logs |
| POST | `/process/write` | Write to process stdin |
| POST | `/process/kill` | Kill a process |

**Process Start Request:**
```json
{
  "command": "ls -la",
  "workdir": "/home/user",
  "env": {"KEY": "value"},
  "pty": false
}
```

---

### Memory Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/memory/search` | Semantic search |
| GET | `/memory/get` | Get memory content |
| POST | `/memory/store` | Store memory |

**Memory Search Request:**
```json
{
  "query": "search terms",
  "limit": 5
}
```

---

### Cron Jobs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/cron/status` | Cron service status |
| GET | `/cron/list` | List all jobs |
| POST | `/cron/add` | Add new job |
| POST | `/cron/update` | Update job |
| POST | `/cron/remove` | Remove job |
| POST | `/cron/run` | Run job immediately |
| GET | `/cron/runs` | Get job run history |
| POST | `/cron/wake` | Wake cron service |

**Add Job Request:**
```json
{
  "job": {
    "id": "my-job",
    "schedule": "0 * * * *",
    "command": "echo 'hello'",
    "enabled": true
  }
}
```

---

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/hooks/wake` | Trigger wake event |
| POST | `/hooks/agent` | Run isolated agent turn |
| POST | `/hooks/<name>` | Custom webhook |

**Wake Payload:**
```json
{
  "text": "description",
  "mode": "now"
}
```

**Agent Payload:**
```json
{
  "message": "prompt",
  "name": "Name",
  "deliver": true
}
```

---

### Storage

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/storage/stats` | Get storage statistics |

---

### Internal

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/internal/pulse/trigger` | Trigger pulse event |

---

## WebSocket

### Chat WebSocket

| Method | Endpoint | Description |
|--------|----------|-------------|
| WS | `/ws/chat` | Real-time chat |

**Connect:**
```javascript
const ws = new WebSocket('ws://localhost:55003/ws/chat?token=<OCG_UI_TOKEN>');
```

**Send:**
```json
{
  "type": "message",
  "content": "Hello"
}
```

**Receive:**
```json
{
  "type": "message",
  "content": "Response"
}
```

---

## Rate Limiting

Protected endpoints use rate limiting. Configure via:

```bash
ocg ratelimit set <endpoint> <key> <maxRequests>
```

---

## Error Responses

| Status | Description |
|--------|-------------|
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 429 | Rate Limited |
| 500 | Internal Server Error |

**Error Response:**
```json
{
  "error": {
    "message": "error description",
    "type": "invalid_request_error"
  }
}
```
