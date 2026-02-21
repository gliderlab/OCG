# MiniMax

配置 MiniMax 作为 LLM 提供商。

---

## 配置

### 环境变量

```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
export MINIMAX_BASE_URL="https://api.minimax.chat/v1"
```

### 配置文件

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

## 可用模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| MiniMax-M2 | 200K | 主要模型 |
| abab6.5s-chat | 200K | 快速版本 |

---

## 注意

- 中文表现优秀
- 有竞争力的价格
- 支持 embeddings

---

## 相关文档

- [Providers 概览](overview-zh.md)
- [中国提供商](chinese-zh.md)
