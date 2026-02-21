# 配置指南

OCG 完整配置指南。

---

## 配置文件

OCG 使用 `env.config` 进行配置：

```bash
/opt/openclaw-go/
├── env.config           # 主配置文件
└── bin/
    └── env.config       # 也从 bin/ 目录加载
```

### 文件格式

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

## 配置部分

### Gateway 配置

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

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `port` | int | 55003 | HTTP 服务器端口 |
| `ui_token` | string | - | Web UI 认证令牌 |
| `cors.enabled` | bool | true | 启用 CORS |
| `cors.origins` | array | ["*"] | 允许的来源 |

### Embedding 配置

```json
{
  "embedding": {
    "model_path": "/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf",
    "port": 50000,
    "verbose": false
  }
}
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `model_path` | string | - | Embedding 模型路径 |
| `port` | int | 50000 | 服务端口 |
| `verbose` | bool | false | 详细日志 |

### Agent 配置

```json
{
  "agent": {
    "socket_path": "/tmp/ocg-agent.sock",
    "context_tokens": 400000
  }
}
```

### LLM 配置

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

### 记忆配置

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

### 通道配置

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

## 环境变量覆盖

环境变量会覆盖配置文件设置：

| 变量 | 配置路径 | 描述 |
|------|----------|------|
| `OCG_UI_TOKEN` | `gateway.ui_token` | UI 令牌 |
| `OPENAI_API_KEY` | - | OpenAI API 密钥 |
| `ANTHROPIC_API_KEY` | - | Anthropic API 密钥 |
| `GOOGLE_API_KEY` | - | Google API 密钥 |
| `MINIMAX_API_KEY` | - | MiniMax API 密钥 |
| `EMBEDDING_MODEL_PATH` | `embedding.model_path` | Embedding 模型 |
| `OCG_VECTOR_INDEX` | `memory.index_path` | 向量索引 |

---

## 重新加载配置

```bash
# 应用新配置而不重启
./bin/ocg gateway config.apply

# 或重启服务
./bin/ocg restart
```

---

## 相关文档

- [环境变量](env-vars-zh.md)
- [端口配置](ports-zh.md)
- [模型配置](models-zh.md)
