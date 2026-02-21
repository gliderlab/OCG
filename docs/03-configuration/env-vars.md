# Environment Variables

Complete list of OCG environment variables.

---

## Core Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OCG_DATA_DIR` | `~/.ocg` | Data directory |
| `OCG_AGENT_SOCK` | `/tmp/ocg-agent.sock` | Agent Unix socket path |
| `OCG_PID_DIR` | `/tmp/ocg` | PID file directory |
| `OCG_GATEWAY_URL` | `http://127.0.0.1:55003` | Gateway URL |
| `OCG_EMBEDDING_URL` | `http://127.0.0.1:50000` | Embedding URL |
| `OCG_UI_TOKEN` | - | **Required** - Web UI auth token |

---

## LLM Provider Variables

### OpenAI
```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

### Anthropic
```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
```

### Google Gemini
```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
```

### MiniMax
```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
export MINIMAX_BASE_URL="https://api.minimax.chat/v1"
```

### Ollama
```bash
export OLLAMA_MODEL="llama3.1"
export OLLAMA_BASE_URL="http://localhost:11434"
```

### OpenRouter
```bash
export OPENROUTER_API_KEY="..."
export OPENROUTER_MODEL="anthropic/claude-3.5-sonnet"
```

### Moonshot AI
```bash
export MOONSHOT_API_KEY="..."
export MOONSHOT_MODEL="moonshot-v1-8k"
```

### Zhipu GLM
```bash
export ZHIPU_API_KEY="..."
export ZHIPU_MODEL="glm-4"
```

### Baidu Qianfan
```bash
export QIANFAN_ACCESS_KEY="..."
export QIANFAN_MODEL="ernie-speed-8k"
```

### Vercel AI
```bash
export VERCEL_API_KEY="..."
export VERCEL_MODEL="gpt-4o"
```

---

## Embedding Variables

```bash
export EMBEDDING_MODEL_PATH="/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf"
export EMBEDDING_VERBOSE=false
export LLAMA_SERVER_BIN="/opt/openclaw-go/bin/llama-server"
export LLAMA_PORT=18000
```

---

## Memory Variables

```bash
export OCG_VECTOR_PROVIDER="hnsw"
export OCG_VECTOR_INDEX="/opt/openclaw-go/vector.index"
export OCG_AUTO_RECALL=true
export OCG_RECALL_LIMIT=5
export OCG_RECALL_MINSCORE=0.72
export OCG_KV_DIR="/opt/openclaw-go/kv"
```

---

## Channel Variables

### Telegram
```bash
export TELEGRAM_BOT_TOKEN="..."
export TELEGRAM_MODE="long_polling"  # or "webhook"
export TELEGRAM_WEBHOOK_URL="https://..."
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
# Context window (auto-detected if not set)
export OCG_CONTEXT_WINDOW=8192

# Health check
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3

# Logging
export LOG_LEVEL="info"  # debug, info, warn, error
```

---

## Browser/CDP Variables

```bash
export CDP_PORT=18800
export BROWSER_HEADLESS=false
```

---

## Variable Priority

1. Command line arguments
2. Environment variables
3. Configuration file (`env.config`)
4. Default values

---

## See Also

- [Configuration Guide](guide.md)
- [Port Configuration](ports.md)
