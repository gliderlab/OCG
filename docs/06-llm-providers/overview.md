# LLM Providers Overview

OCG supports multiple LLM providers.

---

## Supported Providers

| Provider | Environment Variable | Default Model | Status |
|----------|---------------------|---------------|--------|
| OpenAI | `OPENAI_API_KEY` | gpt-4o | ✅ |
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4 | ✅ |
| Google Gemini | `GOOGLE_API_KEY` | gemini-2.5-flash | ✅ |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2 | ✅ |
| Ollama | - | llama3.1 | ✅ |
| OpenRouter | `OPENROUTER_API_KEY` | claude-3.5-sonnet | ✅ |
| Moonshot AI | `MOONSHOT_API_KEY` | moonshot-v1-8k | ✅ |
| Zhipu GLM | `ZHIPU_API_KEY` | glm-4 | ✅ |
| Baidu Qianfan | `QIANFAN_ACCESS_KEY` | ernie-speed-8k | ✅ |
| Vercel AI | `VERCEL_API_KEY` | gpt-4o | ✅ |
| Z.AI | `ZAI_API_KEY` | default | ⚠️ |
| Custom | `CUSTOM_API_KEY` | - | ✅ |

---

## Provider Selection

### Via Environment Variable

```bash
# Use OpenAI
export OPENAI_API_KEY="sk-..."

# Use Anthropic
export ANTHROPIC_API_KEY="..."

# Use Ollama (local, no key)
export OLLAMA_MODEL="llama3.1"
```

### Via Configuration File

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o"
  }
}
```

---

## Context Window Detection

OCG automatically detects context window size:

```go
// Priority: API Query → Known Models → Config → Default (8192)
```

| Model | Context Window |
|-------|---------------|
| gpt-4o | 128,000 |
| claude-sonnet-4 | 200,000 |
| gemini-2.5-flash | 1,000,000 |
| MiniMax-M2 | 200,000 |
| llama3.1 | 131,072 |

---

## Health Check & Failover

Enable health monitoring:

```bash
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

Commands:

```bash
# Check status
ocg llmhealth --action status

# Manual failover
ocg llmhealth --action failover --provider anthropic

# View events
ocg llmhealth --action events
```

---

## Provider Comparison

| Provider | Strengths | weaknesses |
|----------|-----------|------------|
| OpenAI | Reliable, well-documented | Cost |
| Anthropic | Long context, high quality | Expensive |
| Google Gemini | Fast, large context | Regional availability |
| MiniMax | Good for Chinese | Limited languages |
| Ollama | Free, local, private | Requires hardware |

---

## See Also

- [OpenAI](openai.md)
- [Anthropic](anthropic.md)
- [Google Gemini](google.md)
- [MiniMax](minimax.md)
- [Ollama](ollama.md)
- [Chinese Providers](chinese.md)
