# First Steps

Get OCG up and running in minutes.

---

## Quick Start

### 1. Configure Environment

Create or edit `env.config` in your project root:

```bash
# Required: UI authentication token
OCG_UI_TOKEN="your-secure-token-here"

# Optional: LLM Provider
OPENAI_API_KEY="sk-..."
# Or use other providers
# ANTHROPIC_API_KEY="..."
# GOOGLE_API_KEY="..."
# MINIMAX_API_KEY="..."
```

### 2. Start Services

```bash
cd /opt/openclaw-go

# Start all services (embedding → agent → gateway)
./bin/ocg start
```

**Startup sequence:**
1. Embedding service (port 50000-60000)
2. Agent service (Unix socket)
3. Gateway service (port 55003)

### 3. Access Web UI

Open your browser:

```
http://localhost:55003
```

Enter your `OCG_UI_TOKEN` to authenticate.

### 4. Start Chatting

Use the Web UI chat interface or CLI:

```bash
# Interactive CLI chat
./bin/ocg agent
```

---

## Basic Configuration

### LLM Provider Setup

**OpenAI:**
```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"
```

**Anthropic:**
```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
```

**Google Gemini:**
```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
```

**MiniMax:**
```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
```

**Ollama (Local):**
```bash
export OLLAMA_MODEL="llama3.1"
# No API key needed
```

### Telegram Setup (Optional)

```bash
# Bot token from @BotFather
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# Mode: long_polling (default) or webhook
export TELEGRAM_MODE="long_polling"
```

---

## Verify Installation

```bash
# Check service status
./bin/ocg status

# Health check
curl http://localhost:55003/health

# Storage stats (requires UI token)
curl -H "X-OCG-UI-Token: your-token" \
     http://localhost:55003/storage/stats
```

---

## Common Commands

```bash
# Start services
./bin/ocg start

# Stop services
./bin/ocg stop

# Restart
./bin/ocg restart

# Check status
./bin/ocg status

# Interactive chat
./bin/ocg agent
```

---

## Next Steps

- [Environment Setup](environment.md) - Detailed environment configuration
- [Configuration Guide](03-configuration/guide.md) - Full configuration options
- [Tools Overview](../05-tools/overview.md) - Available tools
