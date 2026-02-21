# Ollama

Configure Ollama for local LLM inference.

---

## Configuration

### Environment Variables

```bash
export OLLAMA_MODEL="llama3.1"
export OLLAMA_BASE_URL="http://localhost:11434"
```

### Configuration File

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

## Installation

```bash
# Install Ollama
curl -fsSL https://ollama.ai/install.sh | sh

# Start Ollama service
ollama serve

# Pull a model
ollama pull llama3.1
ollama pull qwen2.5-coder
```

---

## Available Models

| Model | Context | Description |
|-------|---------|-------------|
| llama3.1 | 131K | Meta's latest |
| qwen2.5-coder | 131K | Code optimized |
| mistral | 32K | Efficient |
| codellama | 16K | Code focused |

---

## Advantages

- **Free**: No API costs
- **Private**: All data stays local
- **Offline**: No internet required
- **Flexible**: Use any GGUF model

---

## Requirements

- Modern CPU or GPU
- 4GB+ RAM per model
- Storage for models

---

## See Also

- [Providers Overview](../overview.md)
- [Ollama Website](https://ollama.ai)
