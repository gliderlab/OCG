# 环境配置

配置 OCG 的环境变量。

---

## 核心变量

### 必需

```bash
# Web UI 认证令牌 (必需)
export OCG_UI_TOKEN="your-secure-token"
```

### 路径

```bash
# 数据目录 (默认: ~/.ocg)
export OCG_DATA_DIR="/path/to/data"

# Agent Unix socket (默认: /tmp/ocg-agent.sock)
export OCG_AGENT_SOCK="/tmp/ocg-agent.sock"

# PID 目录 (默认: os.TempDir()/ocg)
export OCG_PID_DIR="/path/to/pid"
```

### URL

```bash
# Gateway URL (默认: http://127.0.0.1:55003)
export OCG_GATEWAY_URL="http://127.0.0.1:55003"

# Embedding URL (默认: http://127.0.0.1:50000)
export OCG_EMBEDDING_URL="http://127.0.0.1:50000"
```

---

## LLM Provider 变量

### OpenAI

```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"              # 默认模型
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

### Ollama (本地)

```bash
export OLLAMA_MODEL="llama3.1"
export OLLAMA_BASE_URL="http://localhost:11434"
# 无需 API key
```

### 自定义 Provider

```bash
export CUSTOM_API_KEY="..."
export CUSTOM_BASE_URL="https://your-provider.com/v1"
```

---

## Embedding 配置

```bash
# Embedding 模型路径
export EMBEDDING_MODEL_PATH="/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf"

# Embedding 详细日志
export EMBEDDING_VERBOSE=false

# Llama.cpp server 二进制路径
export LLAMA_SERVER_BIN="/opt/openclaw-go/bin/llama-server"
```

---

## Web UI

```bash
# UI Token (也可通过配置)
export OCG_UI_TOKEN="your-token"

# Web UI 静态文件路径 (如自定义)
# export OCG_UI_PATH="/path/to/static"
```

---

## 记忆配置

```bash
# 向量记忆 provider (hnsw 或 faiss)
export OCG_VECTOR_PROVIDER="hnsw"

# 向量索引文件路径
export OCG_VECTOR_INDEX="/opt/openclaw-go/vector.index"

# 启用自动记忆召回
export AUTO_RECALL=true

# 召回阈值 (0.0-1.0，越低召回越多)
export RECALL_THRESHOLD=0.72
```

---

## KV 存储 (BadgerDB)

```bash
# KV 存储目录 (默认: 内存模式)
export OCG_KV_DIR="/opt/openclaw-go/kv"

# 启用 KV 条目 TTL
export OCG_KV_TTL_ENABLED=true
```

---

## 通道配置

### Telegram

```bash
# Bot token (从 @BotFather 获取)
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# 模式: long_polling (默认，无需公网) 或 webhook
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
export WHAPP_ACCESS_TOKEN="..."
```

---

## 高级变量

```bash
# 上下文窗口大小 (未设置则自动检测)
export OCG_CONTEXT_WINDOW=8192

# 启用健康检查 + 自动故障转移
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3

# 日志级别 (debug, info, warn, error)
export LOG_LEVEL="info"
```

---

## 配置文件优先级

OCG 使用多源配置（优先级顺序）：

1. **命令行参数** (最高优先级)
2. **环境变量**
3. **配置文件** (`env.config`)
4. **默认值** (最低优先级)

---

## 相关文档

- [配置指南](03-configuration/guide-zh.md)
- [端口配置](03-configuration/ports-zh.md)
- [模型配置](03-configuration/models-zh.md)
