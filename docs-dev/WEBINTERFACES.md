# OCG Web Interfaces Documentation

> Web UI, WebSocket, and Real-time Communication

## Web UI

### Access

```
http://localhost:55003
```

**Authentication:** Token-based via `OCG_UI_TOKEN`

### Configuration

```bash
# Environment
export OCG_UI_TOKEN=your-secret-token
export OCG_HOST=127.0.0.1
export OCG_PORT=55003
```

### Features

- Interactive chat interface
- Session management
- Configuration panel
- Health monitoring
- System logs

---

## WebSocket Protocol

### Connection

```javascript
// Connect to chat WebSocket
const ws = new WebSocket(
  'ws://localhost:55003/ws/chat?token=<OCG_UI_TOKEN>'
);

// Or with session key
const ws = new WebSocket(
  'ws://localhost:55003/ws/chat?token=<OCG_UI_TOKEN>&sessionKey=telegram:123456'
);
```

### Messages

#### Send Message

```javascript
ws.send(JSON.stringify({
  type: 'message',
  content: 'Hello, AI!'
}));
```

#### Receive Response

```javascript
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  if (data.type === 'message') {
    console.log('AI:', data.content);
  } else if (data.type === 'tool_event') {
    console.log('Tool:', data.tool, data.result);
  } else if (data.type === 'typing') {
    console.log('AI is typing...');
  }
};
```

#### Tool Events

```json
{
  "type": "tool_event",
  "tool": "exec",
  "result": {
    "success": true,
    "content": "..."
  }
}
```

### Connection Lifecycle

1. Connect with token
2. Receive welcome message
3. Send messages / receive responses
4. Disconnect when done

---

## Server-Sent Events (SSE)

For streaming responses without WebSocket:

### Endpoint

```
POST /v1/chat/completions
```

### Request

```json
{
  "model": "default",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true
}
```

### Response

```
data: {"choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"choices":[{"delta":{"content":" there"},"index":0}]}

data: [DONE]
```

---

## Telegram Integration

### Webhook Setup

```bash
# Set webhook
curl -X POST https://localhost:55003/telegram/setWebhook \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://your-domain.com/telegram/webhook"}'
```

### Bot Commands

- `/new` - Start new session
- `/model <provider> <model>` - Switch model
- `/live` - Switch to live audio mode
- `/voice` - Switch to voice mode
- `/text` - Switch to text mode

---

## Discord Integration

### Configuration

```bash
export DISCORD_BOT_TOKEN=...
export DISCORD_CHANNEL_ID=...
```

### Events

- Message create → Agent chat
- Reaction add → Custom triggers
- Slash commands → Direct agent commands

---

## Multi-Channel Webhooks

### Generic Webhook

```
POST /hooks/agent
```

```json
{
  "message": "Trigger agent",
  "name": "webhook-session",
  "deliver": true
}
```

**Authentication:**
```bash
Authorization: Bearer <OCG_WEBHOOK_TOKEN>
```

---

## Static Files

### Web UI Assets

```
/                   → static/index.html
/static/*           → static/
/ui/*               → static/ui/
```

### Custom Static Content

Configure static directory:

```bash
export OCG_STATIC_DIR=/path/to/static
```

---

## CORS Configuration

Gateway adds CORS headers automatically. Configure allowed origins:

```bash
export OCG_CORS_ORIGINS=https://example.com,https://app.example.com
```

---

## Authentication Flow

### Token Authentication

1. Client sends request with `Authorization: Bearer <token>`
2. Gateway validates token against `OCG_UI_TOKEN`
3. On success: process request
4. On failure: return 401

### WebSocket Auth

```javascript
const ws = new WebSocket(
  'ws://localhost:55003/ws/chat?token=<OCG_UI_TOKEN>'
);
```

---

## Error Handling

### Connection Errors

| Error | Description |
|-------|-------------|
| 4001 | Invalid token |
| 4002 | Session not found |
| 4003 | Rate limited |

### Reconnection

```javascript
function connect() {
  ws = new WebSocket(url);
  ws.onclose = () => {
    setTimeout(connect, 1000); // Reconnect after 1s
  };
}
```

---

## JavaScript SDK (Beta)

```javascript
class OCGClient {
  constructor(token, baseUrl = 'http://localhost:55003') {
    this.token = token;
    this.baseUrl = baseUrl;
  }

  async chat(message, options = {}) {
    const response = await fetch(`${this.baseUrl}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.token}`
      },
      body: JSON.stringify({
        model: options.model || 'default',
        messages: [{role: 'user', content: message}],
        stream: options.stream || false
      })
    });
    
    if (options.stream) {
      return response.body;
    }
    return response.json();
  }

  connectWS(sessionKey = null) {
    const url = new URL(`${this.baseUrl}/ws/chat`);
    url.searchParams.set('token', this.token);
    if (sessionKey) url.searchParams.set('sessionKey', sessionKey);
    return new WebSocket(url);
  }
}

// Usage
const client = new OCGClient('my-token');
const response = await client.chat('Hello');
```
