# OpenAI

配置 OpenAI 作为 LLM 提供商。

---

## 配置

### 环境变量

```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

### 配置文件

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

## 可用模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| gpt-4o | 128K | 整体最佳 |
| gpt-4o-mini | 128K | 成本效益高 |
| gpt-4-turbo | 128K | 旧版高端 |
| gpt-3.5-turbo | 16K | 经济选项 |

---

## 流式传输

OCG 支持流式响应：

```bash
# 通过 WebSocket 启用流式传输
ws://localhost:55003/v1/chat/stream
```

---

## API 兼容性

OCG 的 OpenAI 兼容 API：

```bash
# 聊天补全
curl -X POST http://localhost:55003/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_UI_TOKEN" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

---

## 相关文档

- [Providers 概览](overview-zh.md)
- [环境变量](../03-configuration/env-vars-zh.md)
