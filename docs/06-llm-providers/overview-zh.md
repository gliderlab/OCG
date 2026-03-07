# LLM Providers 概览

OCG 支持多个 LLM 提供商。

---

## 支持的提供商

| 提供商 | 环境变量 | 默认模型 | 状态 |
|--------|----------|----------|------|
| 通用 (Generic) | `API_KEY`, `BASE_URL`, `MODEL` | - | ✅ |
| OpenAI | `OPENAI_API_KEY` | gpt-4o | ✅ |
| Anthropic | `ANTHROPIC_API_KEY` | claude-3-5-sonnet | ✅ |
| Google Gemini | `GOOGLE_API_KEY`, `GEMINI_API_KEY` | gemini-2.0-flash | ✅ |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2.1 | ✅ |
| Ollama | `OLLAMA_BASE_URL` | llama3 | ✅ |
| OpenRouter | `OPENROUTER_API_KEY` | anthropic/claude-3.5-sonnet | ✅ |
| Moonshot AI | `MOONSHOT_API_KEY` | moonshot-v1-8k | ✅ |
| 智谱 GLM | `ZHIPU_API_KEY` | glm-4 | ✅ |
| 百度千帆 | `QIANFAN_ACCESS_KEY` | ernie-speed-8k | ✅ |
| Vercel AI | `VERCEL_API_TOKEN` | gpt-4o | ✅ |
| Z.AI | `ZAI_API_KEY` | default | ⚠️ |
| Custom | `CUSTOM_API_KEY` | - | ✅ |

---

## 选择提供商

### 通过通用环境变量

OCG 支持可在所有提供商之间通用的环境变量。这对于统一配置（例如使用代理或 OneAPI 等聚合 API 时）非常有用。

```bash
# 通用配置 (适用于任何提供商)
export API_KEY="sk-..."
export BASE_URL="https://api.example.com/v1"
export MODEL="gpt-4o"
```

### 通过厂商专用环境变量

厂商专用变量的优先级高于通用变量。

```bash
# 特别指定 OpenAI
export OPENAI_API_KEY="sk-..."
export OPENAI_BASE_URL="https://api.openai.com/v1"

# 特别指定 Anthropic
export ANTHROPIC_API_KEY="..."
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
