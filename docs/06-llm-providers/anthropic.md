# Anthropic

Configure Anthropic as LLM provider.

---

## Configuration

### Environment Variables

```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
```

### Configuration File

```json
{
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.7
  }
}
```

---

## Available Models

| Model | Context | Description |
|-------|---------|-------------|
| claude-sonnet-4 | 200K | Best balance |
| claude-haiku-4 | 200K | Fast, efficient |
| claude-opus-4 | 200K | Highest quality |

---

## Notes

- Uses Claude API directly
- No base URL override needed
- Excellent for long contexts

---

## See Also

- [Providers Overview](../overview.md)
