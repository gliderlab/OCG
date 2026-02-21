# Health Check & Failover

Monitor LLM health and automatically failover.

---

## Overview

Health monitoring ensures reliable LLM service by:
- Periodic health checks
- Automatic failover on failure
- Manual recovery options

---

## Configuration

```bash
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

```json
{
  "health": {
    "enabled": true,
    "interval": "1h",
    "failure_threshold": 3,
    "auto_failover": true
  }
}
```

---

## How It Works

### Health Check

1. Sends test prompt to LLM
2. Measures response time
3. Checks for errors
4. Records result

### Failure Detection

- Response timeout
- API errors
- Consecutive failures > threshold

### Failover Process

1. Detect failure
2. Switch to backup provider
3. Notify user (optional)
4. Continue operation

---

## Commands

```bash
# Check health status
ocg llmhealth --action status

# Manual failover
ocg llmhealth --action failover --provider anthropic

# Reset to primary
ocg llmhealth --action reset

# View events
ocg llmhealth --action events

# Test specific provider
ocg llmhealth --action test --provider openai
```

---

## Providers with Fallback

| Primary | Fallback |
|---------|----------|
| OpenAI | Anthropic |
| Anthropic | Google Gemini |
| Google | OpenAI |
| MiniMax | Ollama (if available) |

---

## See Also

- [Advanced Overview](../overview.md)
- [LLM Providers Overview](../../06-llm-providers/overview.md)
