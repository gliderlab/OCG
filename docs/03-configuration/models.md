# Model Configuration

Configure LLM models and context windows.

---

## Default Context Windows

OCG automatically detects context window size:

| Provider | Model | Context |
|----------|-------|---------|
| OpenAI | gpt-4o | 128,000 |
| OpenAI | gpt-4o-mini | 128,000 |
| Anthropic | claude-sonnet-4 | 200,000 |
| Anthropic | claude-haiku-4 | 200,000 |
| Google | gemini-2.5-flash | 1,000,000 |
| MiniMax | MiniMax-M2 | 200,000 |
| Ollama | llama3.1 | 131,072 |

---

## Manual Configuration

```bash
# Override auto-detection
export OCG_CONTEXT_WINDOW=8192
```

Or in config file:

```json
{
  "llm": {
    "model": "gpt-4o",
    "context_window": 128000
  }
}
```

---

## Model Settings

### Temperature

```json
{
  "llm": {
    "temperature": 0.7
  }
}
```

**Range:** 0.0 - 2.0  
**Default:** 0.7

- **Lower** (0.0-0.3): More focused, deterministic
- **Balanced** (0.7): Standard creative
- **Higher** (1.0+): More random, creative

### Max Tokens

```json
{
  "llm": {
    "max_tokens": 4000
  }
}
```

Limits response length.

---

## Provider Configuration

### OpenAI

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o",
    "temperature": 0.7,
    "max_tokens": 4000,
    "api_key": "sk-..."
  }
}
```

### Anthropic

```json
{
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.7,
    "max_tokens": 4000
  }
}
```

### MiniMax

```json
{
  "llm": {
    "provider": "minimax",
    "model": "MiniMax-M2",
    "temperature": 0.7,
    "max_tokens": 4000
  }
}
```

### Ollama (Local)

```json
{
  "llm": {
    "provider": "ollama",
    "model": "llama3.1",
    "base_url": "http://localhost:11434"
  }
}
```

---

## Health Check & Failover

```bash
# Enable health monitoring
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

When enabled, OCG will:
1. Periodically check LLM availability
2. Switch to backup model on failure
3. Log health events

```bash
# Check health status
ocg llmhealth --action status

# Manual failover
ocg llmhealth --action failover --provider anthropic

# View events
ocg llmhealth --action events
```

---

## See Also

- [LLM Providers Overview](../../06-llm-providers/overview.md)
- [Environment Variables](env-vars.md)
