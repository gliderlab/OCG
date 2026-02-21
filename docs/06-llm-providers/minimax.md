# MiniMax

Configure MiniMax as LLM provider.

---

## Configuration

### Environment Variables

```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
export MINIMAX_BASE_URL="https://api.minimax.chat/v1"
```

### Configuration File

```json
{
  "llm": {
    "provider": "minimax",
    "model": "MiniMax-M2",
    "temperature": 0.7
  }
}
```

---

## Available Models

| Model | Context | Description |
|-------|---------|-------------|
| MiniMax-M2 | 200K | Main model |
| abab6.5s-chat | 200K | Fast version |

---

## Notes

- Excellent for Chinese language
- Competitive pricing
- Supports embeddings

---

## See Also

- [Providers Overview](../overview.md)
- [Chinese Providers](chinese.md)
