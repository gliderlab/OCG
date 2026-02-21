# Anthropic

配置 Anthropic 作为 LLM 提供商。

---

## 配置

### 环境变量

```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
```

### 配置文件

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

## 可用模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| claude-sonnet-4 | 200K | 最佳平衡 |
| claude-haiku-4 | 200K | 快速、高效 |
| claude-opus-4 | 200K | 最高质量 |

---

## 注意

- 直接使用 Claude API
- 无需覆盖 base URL
- 非常适合长上下文

---

## 相关文档

- [Providers 概览](overview-zh.md)
