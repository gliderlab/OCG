# OCG Architecture Documentation

> System Design and Component Architecture

## Overview

OCG (OpenClaw-Go) is a high-performance, lightweight AI agent system written in Go. It overcomes Node.js limitations with superior performance, lower resource usage, and better privacy.

## High-Level Architecture

```
┌─────────────────────────────────────────┐
│           Gateway (Port 55003)           │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐  │
│  │  Web UI │ │   WS    │ │ Channels │  │
│  └─────────┘ └─────────┘ └──────────┘  │
└────────────────┬────────────────────────┘
                 │ RPC (Unix Socket / gRPC)
                 ▼
┌─────────────────────────────────────────┐
│           Agent (LLM Engine)            │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐  │
│  │Sessions │ │ Memory  │ │  Tools   │  │
│  │         │ │(FAISS)  │ │   (17+)  │  │
│  └─────────┘ └─────────┘ └──────────┘  │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐  │
│  │  Pulse  │ │  Cron   │ │   LLM    │  │
│  │Heartbeat│ │Schedule │ │ Adapter  │  │
│  └─────────┘ └─────────┘ └──────────┘  │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│      Embedding Server (Port 50000)     │
│         (Optional, Vector Store)        │
└─────────────────────────────────────────┘
```

---

## Components

### Gateway

**Location:** `cmd/gateway/`, `gateway/`

**Responsibilities:**
- HTTP/WebSocket server
- Channel adapters (Telegram, Discord, etc.)
- Rate limiting
- Authentication
- Static file serving
- Webhook handling

**Configuration:**
```go
type GatewayConfig struct {
    Port           int
    Host           string
    Token          string
    MaxBodyChat    int64
    MaxBodyProcess int64
    MaxBodyMemory  int64
    MaxBodyCron    int64
}
```

---

### Agent

**Location:** `cmd/agent/`, `agent/`

**Responsibilities:**
- LLM communication
- Session management
- Tool execution
- Context compaction
- Memory operations
- Pulse/heartbeat handling

**Configuration:**
```go
type AgentConfig struct {
    Model           string
    Provider        string
    Temperature     float64
    MaxTokens       int
    SystemPrompt    string
}
```

---

### Embedding Server

**Location:** `cmd/embedding-server/`, `memory/`

**Responsibilities:**
- Vector embeddings
- Semantic search
- HNSW index management

**Protocol:** HTTP/REST

---

## Communication

### Unix Socket (Default)

Agent ↔ Gateway communication via Unix socket:

```
/tmp/ocg-agent.sock
```

**Protocol:** gRPC

### HTTP Fallback

When Unix socket unavailable, Gateway falls back to HTTP:

```
http://127.0.0.1:55003
```

---

## Data Flow

### 1. Chat Request

```
User → Channel → Gateway → Agent → LLM → Agent → Gateway → Channel → User
```

1. User sends message via Telegram/Discord/etc.
2. Channel adapter receives and normalizes
3. Gateway forwards to Agent via gRPC
4. Agent processes with LLM
5. If tools needed, Agent executes tools
6. Response flows back through Gateway
7. Channel adapter sends to user

### 2. Tool Execution

```
Agent → Tool Registry → Tool Execute → Result → Agent
```

1. LLM returns function call
2. Agent looks up tool in registry
3. Tool executes with provided args
4. Result returned to Agent
5. Agent continues LLM conversation

### 3. Memory Operations

```
Agent → Embedding Server → Vector Index → Results → Agent
```

---

## Storage

### SQLite

**Location:** `storage/`

**Tables:**
- `sessions` - Session metadata
- `messages` - Chat messages
- `tasks` - Task history
- `cron_jobs` - Scheduled jobs
- `rate_limits` - Rate limit config

### Vector Store (FAISS/HNSW)

**Location:** `memory/`

- Semantic search index
- Memory storage/retrieval
- Configurable index type

---

## Configuration

### Environment Variables

```bash
# Gateway
OCG_UI_TOKEN=secret-token
OCG_HOST=127.0.0.1
OCG_PORT=55003
OCG_DB_PATH=/path/to/db

# Agent
OCG_AGENT_SOCK=/tmp/ocg-agent.sock
OCG_MODEL=minimax/MiniMax-M2.1

# Embedding
EMBEDDING_SERVER_URL=http://127.0.0.1:50000
```

### Config File

```properties
# env.config
OCG_UI_TOKEN=secret
OCG_PORT=55003
default_model=minimax/MiniMax-M2.1
```

---

## Channels (18)

| Channel | Protocol | Features |
|---------|----------|----------|
| Telegram | Long Polling / Webhook | Commands, Buttons |
| Discord | Webhook | Slash Commands |
| Slack | Webhook | Buttons, Modals |
| WhatsApp | Business API | Media, Templates |
| Signal | Webhook | E2E Encryption |
| IRC | TCP | NickServ |
| Google Chat | Webhook | Cards |
| MS Teams | Webhook | Cards |
| WebChat | WebSocket | Custom UI |
| Mattermost | Webhook | Attachments |
| LINE | Webhook | Rich Menu |
| Matrix | Client-Server | E2E |
| Feishu | Webhook | Cards |
| Zalo | Webhook | Media |
| Threema | Gateway | E2E |
| Session | Webhook | E2E |
| Tox | DHT | P2P |
| iMessage | Relay | Apple |

---

## Tool System

### Registry Pattern

```go
type Registry struct {
    tools  map[string]Tool
    policy *ToolsPolicy
}
```

### Execution Flow

1. LLM returns tool call
2. Agent checks policy
3. Tool executes with args
4. Result formatted for LLM
5. LLM generates final response

---

## Security

### Default Settings

- Bind to `127.0.0.1` (localhost only)
- Token authentication required
- Rate limiting enabled
- Body size limits

### Best Practices

1. Use strong tokens
2. Enable rate limiting
3. Restrict tool access via policy
4. Use workspace-only file operations

---

## Performance

### Benchmarks

| Metric | Value |
|--------|-------|
| Startup | < 100ms |
| Memory | ~50 MB |
| Throughput | 100+ req/s |
| Latency | < 200ms (LLM aside) |

### Optimization

- Connection pooling (HTTP)
- Context compaction
- Session archiving
- Vector index caching

---

## Extension Points

### Custom Channel

Implement channel adapter:

```go
type ChannelAdapter interface {
    Initialize(config map[string]string) error
    SendMessage(target string, message string) error
    ReceiveUpdate() (*Update, error)
}
```

### Custom Provider

Implement LLM provider:

```go
type Provider interface {
    Name() string
    Initialize(config map[string]string) error
    Chat(ctx context.Context, msgs []*Message) (*Message, error)
    ChatStream(ctx context.Context, msgs []*Message) (<-chan *Message, error)
}
```

### Custom Tool

Implement tool interface:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Execute(args map[string]interface{}) (interface{}, error)
}
```

---

## CLI Commands

```bash
# Process management
ocg start/stop/status/restart

# Interactive chat
ocg agent

# Gateway management
ocg gateway config.get
ocg gateway status

# Automation
ocg hooks list/enable/disable
ocg webhook status/test/send

# Monitoring
ocg llmhealth --action status
ocg task list/status
```
