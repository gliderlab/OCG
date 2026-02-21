# Communication

Communication mechanisms between OCG components.

---

## Communication Patterns

### 1. Agent ↔ Gateway (Unix Socket)

**Path:** `/tmp/ocg-agent.sock`

OCG uses gRPC over Unix socket for agent-gateway communication.

```go
// Agent side
listener, _ := net.Listen("unix", "/tmp/ocg-agent.sock")
grpcServer := grpc.NewServer()
pb.RegisterAgentServer(grpcServer, &agentServer{})
grpcServer.Serve(listener)
```

**Protocol:** gRPC with Protocol Buffers

**Messages:**
- `ChatRequest` / `ChatResponse` - Chat messages
- `ToolCallRequest` / `ToolCallResponse` - Tool execution
- `MemoryRequest` / `MemoryResponse` - Memory operations

### 2. Embedding ↔ Agent (HTTP)

**Endpoint:** `http://127.0.0.1:50000`

Agent calls embedding service via HTTP for:
- Text embedding generation
- Vector similarity search

```bash
# Generate embedding
curl -X POST http://127.0.0.1:50000/embed \
  -H "Content-Type: application/json" \
  -d '{"text": "hello world"}'

# Search similar
curl -X POST http://127.0.0.1:50000/search \
  -H "Content-Type: application/json" \
  -d '{"query": "hello", "top_k": 5}'
```

### 3. Gateway ↔ Clients (HTTP/WebSocket)

**HTTP Port:** 55003

**WebSocket Endpoint:** `/v1/chat/stream`

```javascript
// WebSocket connection
const ws = new WebSocket('ws://localhost:55003/v1/chat/stream');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.content);
};
```

**REST Endpoints:**
- `POST /v1/chat/completions` - Chat API (OpenAI-compatible)
- `GET /health` - Health check
- `GET /storage/stats` - Storage statistics

---

## Message Flow

### Chat Request Flow

```
Client → Gateway (HTTP/WS)
        ↓
Gateway → Agent (Unix Socket)
        ↓
Agent → LLM Provider (HTTP)
        ↓
LLM → Agent
        ↓
Agent → Gateway (Socket)
        ↓
Gateway → Client (HTTP/WS)
```

### Memory Recall Flow

```
Agent → Embedding Service (HTTP)
        ↓
Embedding → Vector Index
        ↓
Embedding → Agent (similarity scores)
        ↓
Agent → Context (add relevant memories)
```

---

## Channel Communication

Each channel has its own communication pattern:

| Channel | Protocol | Auth |
|---------|----------|------|
| Telegram | Long Polling / Webhook | Bot Token |
| Discord | Gateway (WSS) + REST | Bot Token |
| Slack | Events API + RTM | Bot Token |
| WhatsApp | Cloud API | Access Token |
| IRC | Raw TCP | None |
| Signal | P2P Encryption | Phonenumber |

### Telegram Example

```bash
# Long Polling (default - no public URL needed)
export TELEGRAM_MODE="long_polling"

# Webhook (requires public URL)
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
export TELEGRAM_MODE="webhook"
```

---

## Security

### Authentication

**UI Token:**
- Required for Web UI access
- Set via `OCG_UI_TOKEN` environment variable
- Passed in `X-OCG-UI-Token` header for API calls

**gRPC:**
- Unix socket has filesystem permissions
- Default: anyone can connect (localhost only)
- Configure via socket permissions

### Rate Limiting

```bash
# Set rate limits
./bin/ocg ratelimit set --channel telegram --max 30 --window 60
```

---

## See Also

- [Channels Overview](../04-channels/overview.md)
- [Configuration Guide](../03-configuration/guide.md)
- [Port Configuration](03-configuration/ports.md)
