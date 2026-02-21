# Environment Setup

Configure environment variables for OCG.

---

## Core Variables

### Required

```bash
# UI authentication token (required for web UI)
export OCG_UI_TOKEN="your-secure-token"
```

### Paths

```bash
# Data directory (default: ~/.ocg)
export OCG_DATA_DIR="/path/to/data"

# Agent Unix socket (default: /tmp/ocg-agent.sock)
export OCG_AGENT_SOCK="/tmp/ocg-agent.sock"

# PID directory (default: os.TempDir()/ocg)
export OCG_PID_DIR="/path/to/pid"
```

### URLs

```bash
# Gateway URL (default: http://127.0.0.1:55003)
export OCG_GATEWAY_URL="http://127.0.0.1:55003"

# Embedding URL (default: http://127.0.0.1:50000)
export OCG_EMBEDDING_URL="http://127.0.0.1:50000"
```

---

## LLM Provider Variables

### OpenAI

```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"              # Default model
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

### Anthropic

```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
export ANTHROPIC_BASE_URL="https://api.anthropic.com/v1"
```

### Google Gemini

```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
export GOOGLE_BASE_URL="https://generativelanguage.googleapis.com/v1"
```

### MiniMax

```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
export MINIMAX_BASE_URL="https://api.minimax.chat/v1"
```

### Ollama (Local)

```bash
export OLLAMA_MODEL="llama3.1"
export OLLAMA_BASE_URL="http://localhost:11434"
# No API key required
```

### Custom Provider

```bash
export CUSTOM_API_KEY="..."
export CUSTOM_BASE_URL="https://your-provider.com/v1"
```

---

## Embedding Configuration

```bash
# Embedding model path
export EMBEDDING_MODEL_PATH="/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf"

# Embedding verbose logging
export EMBEDDING_VERBOSE=false

# Llama.cpp server binary path
export LLAMA_SERVER_BIN="/opt/openclaw-go/bin/llama-server"
```

---

## Web UI

```bash
# UI Token (also via config)
export OCG_UI_TOKEN="your-token"

# Web UI static files path (if custom)
# export OCG_UI_PATH="/path/to/static"
```

---

## Memory Configuration

```ba
# Vector memory provider (hnsw or faiss)
export OCG_VECTOR_PROVIDER="hnsw"

# Vector index file path
export OCG_VECTOR_INDEX="/opt/openclaw-go/vector.index"

# Enable automatic memory recall
export AUTO_RECALL=true

# Recall threshold (0.0-1.0, lower = more recall)
export RECALL_THRESHOLD=0.72
```

---

## KV Storage (BadgerDB)

```bash
# KV storage directory (default: memory-only)
export OCG_KV_DIR="/opt/openclaw-go/kv"

# Enable TTL for KV entries
export OCG_KV_TTL_ENABLED=true
```

---

## Channel Configuration

### Telegram

```bash
# Bot token from @BotFather
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# Mode: long_polling (default, no public URL needed) or webhook
export TELEGRAM_MODE="long_polling"
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
```

### Discord

```bash
export DISCORD_BOT_TOKEN="..."
export DISCORD_APPLICATION_ID="..."
export DISCORD_PUBLIC_KEY="..."
```

### Slack

```bash
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_SIGNING_SECRET="..."
export SLACK_APP_TOKEN="xapp-..."
```

### WhatsApp

```bash
export WHATSAPP_PHONE_ID="..."
export WHATSAPP_ACCESS_TOKEN="..."
```

---

## Advanced Variables

```bash
# Context window size (auto-detected if not set)
export OCG_CONTEXT_WINDOW=8192

# Enable health check + auto-failover
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3

# Log level (debug, info, warn, error)
export LOG_LEVEL="info"
```

---

## Configuration File Priority

OCG uses configuration from multiple sources (priority order):

1. **Command line arguments** (highest priority)
2. **Environment variables**
3. **Configuration file** (`env.config`)
4. **Default values** (lowest priority)

---

## See Also

- [Configuration Guide](03-configuration/guide.md)
- [Port Configuration](03-configuration/ports.md)
- [Model Configuration](03-configuration/models.md)
