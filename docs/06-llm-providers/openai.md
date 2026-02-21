# OpenAI

Configure OpenAI as LLM provider.

---

## Configuration

### Environment Variables

```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

### Configuration File

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o",
    "temperature": 0.7,
    "max_tokens": 4000
  }
}
```

---

## Available Models

| Model | Context | Description |
|-------|---------|-------------|
| gpt-4o | 128K | Best overall |
| gpt-4o-mini | 128K | Cost-effective |
| gpt-4-turbo | 128K | Legacy high-end |
| gpt-3.5-turbo | 16K | Budget option |

---

## Streaming

OCG supports streaming responses:

```bash
# Enable streaming via WebSocket
ws://localhost:55003/v1/chat/stream
```

---

## API Compatibility

OCG's OpenAI-compatible API:

```bash
# Chat completions
curl -X POST http://localhost:55003/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_UI_TOKEN" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## See Also

- [Providers Overview](../overview.md)
- [Environment Variables](../03-configuration/env-vars.md)
