# Configuration Guide

Complete guide to OCG configuration.

---

## Configuration File

OCG uses `env.config` for configuration:

```bash
/opt/openclaw-go/
├── env.config           # Main configuration file
└── bin/
    └── env.config       # Also loaded from bin/ directory
```

### File Format

```json
{
  "gateway": {
    "port": 55003,
    "ui_token": "your-token"
  },
  "embedding": {
    "model_path": "/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf",
    "port": 50000
  },
  "agent": {
    "socket_path": "/tmp/ocg-agent.sock"
  },
  "llm": {
    "provider": "openai",
    "model": "gpt-4o"
  },
  "memory": {
    "vector_provider": "hnsw",
    "index_path": "/opt/openclaw-go/vector.index"
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "your-token"
    }
  }
}
```

---

## Configuration Sections

### Gateway Configuration

```json
{
  "gateway": {
    "port": 55003,
    "ui_token": "your-secure-token",
    "cors": {
      "enabled": true,
      "origins": ["*"]
    }
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | int | 55003 | HTTP server port |
| `ui_token` | string | - | Web UI authentication token |
| `cors.enabled` | bool | true | Enable CORS |
| `cors.origins` | array | ["*"] | Allowed origins |

### Embedding Configuration

```json
{
  "embedding": {
    "model_path": "/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf",
    "port": 50000,
    "verbose": false
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model_path` | string | - | Path to embedding model |
| `port` | int | 50000 | Service port |
| `verbose` | bool | false | Verbose logging |

### Agent Configuration

```json
{
  "agent": {
    "socket_path": "/tmp/ocg-agent.sock",
    "context_tokens": 400000
  }
}
```

### LLM Configuration

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

### Memory Configuration

```json
{
  "memory": {
    "vector_provider": "hnsw",
    "index_path": "/opt/openclaw-go/vector.index",
    "auto_recall": true,
    "recall_threshold": 0.72
  }
}
```

### Channel Configuration

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "your-bot-token",
      "mode": "long_polling"
    },
    "discord": {
      "enabled": false,
      "bot_token": "your-discord-token"
    }
  }
}
```

---

## Environment Variables Override

Environment variables override config file settings:

| Variable | Config Path | Description |
|----------|-------------|-------------|
| `OCG_UI_TOKEN` | `gateway.ui_token` | UI token |
| `OPENAI_API_KEY` | - | OpenAI API key |
| `ANTHROPIC_API_KEY` | - | Anthropic API key |
| `GOOGLE_API_KEY` | - | Google API key |
| `MINIMAX_API_KEY` | - | MiniMax API key |
| `EMBEDDING_MODEL_PATH` | `embedding.model_path` | Embedding model |
| `OCG_VECTOR_INDEX` | `memory.index_path` | Vector index |

---

## Reloading Configuration

```bash
# Apply new config without restart
./bin/ocg gateway config.apply

# Or restart services
./bin/ocg restart
```

---

## See Also

- [Environment Variables](env-vars.md)
- [Port Configuration](ports.md)
- [Model Configuration](models.md)
