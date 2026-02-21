# System Overview

High-level architecture of OCG-Go (OCG).

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     OCG Gateway (55003)                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ │
│  │  Web UI  │  │ REST API │  │ WebSocket│  │   Channels   │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘ │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   OCG Agent (Unix Socket)                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │    Pulse    │  │  Context    │  │  Tools Registry     │  │
│  │  Heartbeat  │  │  Manager    │  │  (17+ tools)        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐                           │
│  │    Tasks    │  │   Memory    │                           │
│  │  Scheduler  │  │  (HNSW/FAISS)│                          │
│  └─────────────┘  └─────────────┘                           │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              OCG Embedding (50000-60000)                     │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │            llama.cpp Server                             │ │
│  │  - Embedding generation                                 │ │
│  │  - Vector similarity search                             │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

Channels:
┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐
│ Telegram  │ │  Discord  │ │   Slack   │ │ WhatsApp  │
├───────────┤ ├───────────┤ ├───────────┤ ├───────────┤
│  Signal   │ │   IRC     │ │ GoogleChat│ │  MS Teams │
├───────────┤ ├───────────┤ ├───────────┤ ├───────────┤
│ Mattermost│ │   LINE    │ │  Matrix   │ │   Feishu  │
└───────────┘ └───────────┘ └───────────┘ └───────────┘
```

---

## Core Components

### 1. Gateway
**Port:** 55003 (HTTP/WebSocket)

The Gateway is the main entry point:
- **Web UI**: Browser-based chat interface
- **REST API**: `/v1/chat/completions` (OpenAI-compatible)
- **WebSocket**: Real-time chat streaming
- **Channels**: Multi-platform messaging integration

### 2. Agent
**Connection:** Unix Socket (`/tmp/ocg-agent.sock`)

The Agent handles core logic:
- **Pulse**: Event loop and heartbeat system
- **Context**: Message history and token management
- **Tools**: 17+ built-in tools (file I/O, exec, browser, etc.)
- **Tasks**: Task scheduling and splitting
- **Memory**: Vector memory with HNSW/FAISS

### 3. Embedding
**Ports:** 50000-60000 (HTTP)

Embedding service provides:
- **Text Embedding**: Generate vectors for semantic search
- **Vector Search**: Find similar memories
- **Local Model**: Uses llama.cpp for offline embedding

---

## Data Flow

### Chat Request
```
User → Gateway (55003) → Agent (Socket) → LLM → Response → User
          │                              │
          ├── Vector Search ────────────┤
          └── Memory Recall ────────────┘
```

### Memory Storage
```
User Message → Embedding Service → Vector Index
                               → SQLite (full text)
```

---

## Process Model

OCG uses a multi-process architecture for:
- **Isolation**: Crashes in one service don't affect others
- **Scalability**: Services can be scaled independently
- **Simplicity**: Single binary, no complex dependencies

**Startup Order:**
1. Embedding (waits for llama.cpp)
2. Agent (waits for embedding health)
3. Gateway (waits for agent socket)

---

## See Also

- [Process Model](processes.md)
- [Communication](communication.md)
- [Port Configuration](03-configuration/ports.md)
