# LLM Providers 概览

OCG 支持多个 LLM 提供商。

---

## 支持的提供商

| 提供商 | 环境变量 | 默认模型 | 状态 |
|--------|----------|----------|------|
| OpenAI | `OPENAI_API_KEY` | gpt-4o | ✅ |
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4 | ✅ |
| Google Gemini | `GOOGLE_API_KEY` | gemini-2.5-flash | ✅ |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2 | ✅ |
| Ollama | - | llama3.1 | ✅ |
| OpenRouter | `OPENROUTER_API_KEY` | claude-3.5-sonnet | ✅ |
| Moonshot AI | `MOONSHOT_API_KEY` | moonshot-v1-8k | ✅ |
| 智谱 GLM | `ZHIPU_API_KEY` | glm-4 | ✅ |
| 百度千帆 | `QIANFAN_ACCESS_KEY` | ernie-speed-8k | ✅ |
| Vercel AI | `VERCEL_API_KEY` | gpt-4o | ✅ |
| Z.AI | `ZAI_API_KEY` | default | ⚠️ |
| Custom | `CUSTOM_API_KEY` | - | ✅ |

---

## 选择提供商

### 通过环境变量

```bash
# 使用 OpenAI
export OPENAI_API_KEY="sk-..."

# 使用 Anthropic
export ANTHROPIC_API_KEY="..."

# 使用 Ollama (本地，无需 key)
export OLLAMA_MODEL="llama3.1"
```

### 通过配置文件

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o"
  }
}
```

---

## 上下文窗口检测

OCG 自动检测上下文窗口大小：

```go
// 优先级: API 查询 → 已知模型 → 配置 → 默认值 (8192)
```

| 模型 | 上下文窗口 |
|------|-----------|
| gpt-4o | 128,000 |
| claude-sonnet-4 | 200,000 |
| gemini-2.5-flash | 1,000,000 |
| MiniMax-M2 | 200,000 |
| llama3.1 | 131,072 |

---

## 健康检查和故障转移

启用健康监控：

```bash
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

命令：

```bash
# 检查状态
ocg llmhealth --action status

# 手动故障转移
ocg llmhealth --action failover --provider anthropic

# 查看事件
ocg llmhealth --action events
```

---

## 提供商对比

| 提供商 | 优势 | 劣势 |
|--------|------|------|
| OpenAI | 可靠、文档完善 | 成本 |
| Anthropic | 长上下文、高质量 | 昂贵 |
| Google Gemini | 快速、大上下文 | 区域可用性 |
| MiniMax | 中文友好 | 语言有限 |
| Ollama | 免费、本地、私密 | 需要硬件 |

---

## 相关文档

- [OpenAI](openai-zh.md)
- [Anthropic](anthropic-zh.md)
- [Google Gemini](google-zh.md)
- [MiniMax](minimax-zh.md)
- [Ollama](ollama-zh.md)
- [中国提供商](chinese-zh.md)
