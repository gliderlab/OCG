# OCG LLM Providers Documentation

> Supported LLM Providers and Configuration

## Overview

OCG supports 13 LLM providers with unified OpenAI-compatible API interface.

## Provider List

| Provider | Environment Variable | Default Model |
|----------|---------------------|---------------|
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

## Provider Configuration

### OpenAI

```bash
# Environment
export OPENAI_API_KEY=sk-...
export OPENAI_BASE_URL=https://api.openai.com/v1  # optional

# Config
openai.model=gpt-4o
```

**Models:**
- gpt-4o, gpt-4o-mini
- gpt-4-turbo, gpt-4-turbo-preview
- gpt-3.5-turbo

---

### Anthropic

```bash
# Environment
export ANTHROPIC_API_KEY=sk-ant-...

# Config
anthropic.model=claude-3.5-sonnet
anthropic.max_tokens=4096
```

**Models:**
- claude-3.5-sonnet
- claude-3-opus
- claude-3-sonnet
- claude-3-haiku

---

### Google Gemini

```bash
# Environment
export GOOGLE_API_KEY=AI...

# Config
google.model=gemini-2.0-flash
google.base_url=https://generativelanguage.googleapis.com/v1beta
```

**Models:**
- gemini-2.0-flash
- gemini-2.0-flash-exp
- gemini-1.5-pro
- gemini-1.5-flash

**Realtime Audio:**
```bash
# Enable Gemini Live Realtime
google.realtime=true
google.realtime_voice=verse
```

---

### MiniMax

```bash
# Environment
export MINIMAX_API_KEY=...

# Config
minimax.model=MiChat-4
minimax.group_id=...
minimax.user_id=...
```

**Models:**
- MiniMax-M2.1
- MiniMax-M2
- MiniMax-Text-01

---

### Ollama

```bash
# Environment
# No API key needed

# Config
ollama.model=llama3
ollama.base_url=http://localhost:11434
```

**Models:**
- llama3, llama3.1, llama3.2
- mistral
- codellama
- phi3

---

### OpenRouter

```bash
# Environment
export OPENROUTER_API_KEY=sk-or-...

# Config
openrouter.model=anthropic/claude-3.5-sonnet
openrouter.base_url=https://openrouter.ai/api/v1
```

**Models:**
- anthropic/claude-3.5-sonnet
- openai/gpt-4o
- google/gemini-pro

---

### Moonshot (月之暗面)

```bash
# Environment
export MOONSHOT_API_KEY=...

# Config
moonshot.model=moonshot-v1-8k
moonshot.base_url=https://api.moonshot.cn/v1
```

**Models:**
- moonshot-v1-8k
- moonshot-v1-32k
- moonshot-v1-128k

---

### GLM (智谱)

```bash
# Environment
export ZHIPU_API_KEY=...

# Config
glm.model=glm-4
glm.base_url=https://open.bigmodel.cn/api/paas/v4
```

**Models:**
- glm-4
- glm-4-flash
- glm-4-plus
- glm-3-turbo

---

### Qianfan (百度千帆)

```bash
# Environment
export QIANFAN_ACCESS_KEY=...
export QIANFAN_SECRET_KEY=...

# Config
qianfan.model=ernie-speed-8k
qianfan.base_url=https://qianfan.baidubce.com/v2
```

**Models:**
- ernie-speed-8k
- ernie-speed-128k
- ernie-bot-4
- ernie-bot-turbo

---

### Bedrock (AWS)

```bash
# Environment
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1

# Config
bedrock.model=anthropic.claude-3-sonnet-20240229-v1:0
bedrock.region=us-east-1
```

**Models:**
- anthropic.claude-3-sonnet
- anthropic.claude-3-haiku
- amazon.titan-text-express

---

### Vercel

```bash
# Environment
export VERCEL_API_KEY=...

# Config
vercel.model=gpt-4o
vercel.base_url=https://api.vercel.ai/v1
```

**Models:**
- gpt-4o
- gpt-4o-mini
- claude-3.5-sonnet

---

### Z.AI

```bash
# Environment
export ZAI_API_KEY=...

# Config
zai.model=default
zai.base_url=https://api.z-ai.cloud/v1
```

---

### Custom

```bash
# Environment
export CUSTOM_API_KEY=...
export CUSTOM_BASE_URL=https://your-api.com/v1

# Config
custom.model=custom-model
custom.base_url=https://your-api.com/v1
```

---

## Provider Interface

```go
// Provider defines LLM provider interface
type Provider interface {
    Name() string
    Initialize(config map[string]string) error
    Chat(ctx context.Context, msgs []*Message) (*Message, error)
    ChatStream(ctx context.Context, msgs []*Message) (<-chan *Message, error)
    Embeddings(ctx context.Context, texts []string) ([]float64, error)
}
```

---

## Model Selection

### Via Configuration

```properties
# env.config
default_model=minimax/MiniMax-M2.1
```

### Via API Request

```json
{
  "model": "openai/gpt-4o",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

### Via Command

```
/model openai gpt-4o
/model anthropic claude-3.5-sonnet
```

---

## Health Check & Failover

OCG provides LLM health monitoring:

```bash
# Check health status
ocg llmhealth --action status

# Start health monitoring
ocg llmhealth --action start

# Manual failover
ocg llmhealth --action failover --provider openai

# View failover events
ocg llmhealth --action events
```

**Configuration:**
```bash
LLM_HEALTH_CHECK=1
LLM_HEALTH_INTERVAL=1h
LLM_HEALTH_FAILURE_THRESHOLD=3
```

---

## Context Window

| Provider | Max Tokens |
|----------|------------|
| GPT-4o | 128K |
| Claude 3.5 | 200K |
| Gemini 2.0 | 1M |
| MiniMax-M2.1 | 100K |
| GLM-4 | 128K |
| Moonshot | 128K |

---

## Function Calling

All providers support OpenAI-style function calling:

```json
{
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "weather",
        "description": "Get weather",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          },
          "required": ["location"]
        }
      }
    }
  ]
}
```
