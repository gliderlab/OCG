# LLM Providers Overview

OCG supports multiple LLM providers.

---

## Supported Providers

| Provider | Environment Variable | Default Model | Status |
|----------|---------------------|---------------|--------|
| Generic | `API_KEY`, `BASE_URL`, `MODEL` | - | ✅ |
| OpenAI | `OPENAI_API_KEY` | gpt-4o | ✅ |
| Anthropic | `ANTHROPIC_API_KEY` | claude-3-5-sonnet | ✅ |
| Google Gemini | `GOOGLE_API_KEY`, `GEMINI_API_KEY` | gemini-2.0-flash | ✅ |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2.1 | ✅ |
| Ollama | `OLLAMA_BASE_URL` | llama3 | ✅ |
| OpenRouter | `OPENROUTER_API_KEY` | anthropic/claude-3.5-sonnet | ✅ |
| Moonshot AI | `MOONSHOT_API_KEY` | moonshot-v1-8k | ✅ |
| Zhipu GLM | `ZHIPU_API_KEY` | glm-4 | ✅ |
| Baidu Qianfan | `QIANFAN_ACCESS_KEY` | ernie-speed-8k | ✅ |
| Vercel AI | `VERCEL_API_TOKEN` | gpt-4o | ✅ |
| Z.AI | `ZAI_API_KEY` | default | ⚠️ |
| Custom | `CUSTOM_API_KEY` | - | ✅ |

---

## Provider Selection

### Via Generic Environment Variables

OCG supports generic environment variables that work across all providers. This is useful for one-stop configuration (e.g., when using a proxy or a unified API aggregator like OneAPI).

```bash
# Generic configuration (works for any provider)
export API_KEY="sk-..."
export BASE_URL="https://api.example.com/v1"
export MODEL="gpt-4o"
```

### Via Vendor-Specific Environment Variables

Vendor-specific variables take precedence over generic ones.

```bash
# Use OpenAI specifically
export OPENAI_API_KEY="sk-..."
export OPENAI_BASE_URL="https://api.openai.com/v1"

# Use Anthropic specifically
export ANTHROPIC_API_KEY="..."
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
