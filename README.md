# [English](README.md) | [中文](README-zh.md)

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/SQLite-3.x-003B57?style=for-the-badge&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
</p>

---

# OCG (OpenClaw-Go) 🦀

> **The Next-Gen Local AI Agent System**  
> *Fast, Private, and Extensible*  
> *A Go reimplementation of OpenClaw*

---

## 📖 Introduction

**OCG (OpenClaw-Go)** is a high-performance, lightweight AI agent system rewritten in Go from OpenClaw. It overcomes the limitations of the original Node.js-based OpenClaw with superior performance, lower resource usage, and fundamental privacy.

OCG runs entirely locally, integrating **Vector Memory**, **Task Scheduling**, and **Multi-Channel Communication** into 4 independent binaries.

---

## ✨ Why OCG?

| Feature | OCG | Node.js Agents |
|---------|-----|----------------|
| **Startup** | **< 100ms** ⚡ | 5-10s |
| **Memory** | **~50 MB** 🪶 | 200-500 MB+ |
| **Deployment** | 4 Binaries (All-in-One or Distributed) | Complex Dependencies |
| **Storage** | SQLite + FAISS (Local) | Mongo/Postgres (Remote) |
| **Architecture** | Multi-Process RPC | Monolithic |

---

## 🚀 Key Features

- ⚡ **Blazing Fast** - Startup in <1s, minimal footprint
- 🔒 **Privacy First** - All data stays local
- 🧠 **Vector Memory** - Semantic search with HNSW
- 🔌 **Universal Gateway** - WebSocket, HTTP REST, Telegram
- 🛠️ **17+ Tools** - File I/O, Shell, Process, Browser, Cron
- 💓 **Pulse System** - Heartbeat event loop
- ⏰ **Smart Scheduling** - Cron-based automation
- 🎣 **Hooks & Webhooks** - Event-driven automation
- 🌐 **13 LLM Providers** - OpenAI, Anthropic, Google, MiniMax, Ollama, and more
- 📱 **Telegram Long Polling** - No public URL required (default)
- ✍️ **Typing Indicator** - Shows "typing" while AI responds
- 💭 **Session Context** - Each user/channel has independent conversation history
- 🧹 **Strict Incremental Compaction Archive** - Watermark-based archive, dedupe, summary-skip
- ✅ **Task Marker Context Strategy** - Main chat keeps `[task_done:task-...]`, details in DB
- 🎙️ **Google Native-Audio Realtime** - PCM uplink, WAV output, function-calling callbacks
- 🔄 **Tool Enhancements** - Loop detection, result truncation, thinking mode
- 🛡️ **Security** - Default bind to 127.0.0.1, token authentication
- 🎤 **Live Voice Routing** - Telegram voice → Live audio (no transcription), voice+text → HTTP
- 📡 **Realtime Session Management** - Per-session WebSocket, idle cleanup, provider_type tracking
- 🔀 **Modality Switching** - Commands: `/live`, `/text`, `/voice`, `/audio`, `/http`
- ⏮️ **Live Fallback** - Automatic fallback to HTTP LLM on live errors
- 🔒 **Concurrency Safety** - Per-session mutex for simultaneous live requests

---

## ⚡ Quick Start

### Prerequisites

- Go 1.22+
- GCC (for SQLite/CGO)

```bash
# Linux dependencies
sudo apt-get install -y libgomp1 libblas3 liblapack3 libopenblas0 libgfortran5
```

### Build

```bash
git clone https://github.com/gliderlab/OCG.git
cd OCG
make
```

OCG consists of 4 independent binaries:
- `ocg` - Main CLI entry point (lightweight)
- `ocg-gateway` - Gateway service (message routing)
- `ocg-agent` - AI core (LLM + tools)
- `ocg-embedding` - Vector service

All-in-One by default (all services on one machine). Production: can deploy Gateway/Agent/Embedding separately.

### Run

```bash
# Configure environment
export OCG_UI_TOKEN="your-token"

# Start services
./bin/ocg start
```

### Access

- **Web UI**: http://localhost:55003
- **API**: http://localhost:55003/v1/chat/completions

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────┐
│           Gateway (Port 55003)           │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │  Web UI │ │   WS    │ │ Channels │ │
│  └─────────┘ └─────────┘ └──────────┘ │
└────────────────┬───────────────────────┘
                 │ RPC (Unix Socket)
                 ▼
┌─────────────────────────────────────────┐
│           Agent (LLM Engine)            │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │Sessions │ │ Memory  │ │  Tools   │ │
│  │         │ │(FAISS)  │ │   (17+)  │ │
│  └─────────┘ └─────────┘ └──────────┘ │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │  Pulse  │ │  Cron   │ │   LLM    │ │
│  │Heartbeat│ │Schedule │ │ Adapter  │ │
│  └─────────┘ └─────────┘ └──────────┘ │
└─────────────────────────────────────────┘
```

---

## 🛠️ Tools

| Tool | Description |
|------|-------------|
| `exec` | Safe shell execution |
| `read` | Read files |
| `write` | Write files |
| `edit` | Smart file editing |
| `apply_patch` | Multi-file patches |
| `process` | Process management |
| `browser` | CDP browser control |
| `image` | Vision model analysis |
| `memory` | Vector memory search/store |
| `message` | Multi-channel messaging |
| `cron` | Task scheduling |
| `sessions` | Session management |
| `webhooks` | HTTP triggers |

---

## 🔌 LLM Providers (13)

| Provider | Env Variable | Default Model |
|----------|-------------|---------------|
| OpenAI | `OPENAI_API_KEY` | gpt-4o |
| Anthropic | `ANTHROPIC_API_KEY` | claude-3.5-sonnet |
| Google | `GOOGLE_API_KEY` | gemini-2.0-flash |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2.1 |
| Ollama | - | llama3 |
| OpenRouter | `OPENROUTER_API_KEY` | claude-3.5-sonnet |
| Moonshot | `MOONSHOT_API_KEY` | moonshot-v1-8k |
| GLM | `ZHIPU_API_KEY` | glm-4 |
| Qianfan | `QIANFAN_ACCESS_KEY` | ernie-speed-8k |
| Bedrock | `AWS_ACCESS_KEY_ID` | claude-3-sonnet |
| Vercel | `VERCEL_API_KEY` | gpt-4o |
| Z.AI | `ZAI_API_KEY` | default |
| Custom | `CUSTOM_API_KEY` | custom |

---

## 💬 Channels (18)

Telegram, Discord, Slack, WhatsApp, Signal, IRC, Google Chat, MS Teams, WebChat, Mattermost, LINE, Matrix, Feishu, Zalo, Threema, Session, Tox, iMessage

### Telegram Features
- **Long Polling** (default) - No public URL needed
- **Session Context** - Each chat has independent history
- **Typing Indicator** - Shows while AI is responding

---

## 🆕 Recent Additions (2026-02-19)

### Session/Task Context Controls

- `/task list [limit]`
- `/task summary <task-id>`
- `/task detail <task-id> [page] [pageSize]`
- Marker auto-resolve: `[task_done:task-...]` (supports multiple markers in one message)
- `/debug archive [session]` for compaction watermark/archive validation

### Compaction Reliability

- `session_meta.last_compacted_message_id` watermark
- `messages_archive.source_message_id` + unique index for dedupe
- archive path skips `[summary]` system entries

### Google Realtime

- Full `RealtimeProvider` callback surface (audio/text/tools/transcription/VAD/usage/session events)
- Function calling with tool parameter schema conversion
- Native-audio workflow with finalized WAV callback

## ⚙️ CLI Commands

```bash
# Process management
ocg start/stop/status/restart

# Interactive chat
ocg agent

# Gateway management
ocg gateway [config.get|config.apply|config.patch|status]

# Automation
ocg hooks [list|enable|disable|info]
ocg webhook [status|test|send]

# Monitoring
ocg llmhealth [--action status|start|stop|failover|events|reset|test]
ocg task [list|status]
```

---

## 📁 Project Structure

```
OCG/
├── cmd/
│   ├── ocg/           # Main entry (CLI)
│   ├── gateway/       # HTTP/WebSocket server
│   ├── agent/         # LLM agent
│   └── embedding-server/ # Vector embeddings
├── pkg/
│   ├── llm/           # LLM adapters
│   ├── memory/        # Vector store
│   ├── hooks/         # Event hooks
│   └── config/        # Configuration
├── gateway/
│   ├── static/        # Web UI
│   └── channels/      # Channel adapters
└── docs/             # Documentation
```

---

## 📄 License

MIT © Glider Labs
