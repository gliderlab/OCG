# 模型配置

配置 LLM 模型和上下文窗口。

---

## 默认上下文窗口

OCG 自动检测上下文窗口大小：

| Provider | 模型 | 上下文 |
|----------|------|--------|
| OpenAI | gpt-4o | 128,000 |
| OpenAI | gpt-4o-mini | 128,000 |
| Anthropic | claude-sonnet-4 | 200,000 |
| Anthropic | claude-haiku-4 | 200,000 |
| Google | gemini-2.5-flash | 1,000,000 |
| MiniMax | MiniMax-M2 | 200,000 |
| Ollama | llama3.1 | 131,072 |

---

## 手动配置

```bash
# 覆盖自动检测
export OCG_CONTEXT_WINDOW=8192
```

或在配置文件中：

```json
{
  "llm": {
    "model": "gpt-4o",
    "context_window": 128000
  }
}
```

---

## 模型设置

### Temperature

```json
{
  "llm": {
    "temperature": 0.7
  }
}
```

**范围:** 0.0 - 2.0  
**默认值:** 0.7

- **较低** (0.0-0.3): 更专注、确定性
- **平衡** (0.7): 标准创意
- **较高** (1.0+): 更随机、创意

### Max Tokens

```json
{
  "llm": {
    "max_tokens": 4000
  }
}
```

限制响应长度。

---

## Provider 配置

### OpenAI

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o",
    "temperature": 0.7,
    "max_tokens": 4000,
    "api_key": "sk-..."
  }
}
```

### Anthropic

```json
{
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.7,
    "max_tokens": 4000
  }
}
```

### MiniMax

```json
{
  "llm": {
    "provider": "minimax",
    "model": "MiniMax-M2",
    "temperature": 0.7,
    "max_tokens": 4000
  }
}
```

### Ollama (本地)

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

## 健康检查和故障转移

```bash
# 启用健康监控
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

启用后，OCG 将：
1. 定期检查 LLM 可用性
2. 故障时切换到备用模型
3. 记录健康事件

```bash
# 检查健康状态
ocg llmhealth --action status

# 手动故障转移
ocg llmhealth --action failover --provider anthropic

# 查看事件
ocg llmhealth --action events
```

---

## 相关文档

- [LLM Providers 概览](../../06-llm-providers/overview-zh.md)
- [环境变量](env-vars-zh.md)
