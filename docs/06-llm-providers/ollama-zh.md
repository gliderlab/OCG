# Ollama

配置 Ollama 用于本地 LLM 推理。

---

## 配置

### 环境变量

```bash
export OLLAMA_MODEL="llama3.1"
export OLLAMA_BASE_URL="http://localhost:11434"
```

### 配置文件

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

## 安装

```bash
# 安装 Ollama
curl -fsSL https://ollama.ai/install.sh | sh

# 启动 Ollama 服务
ollama serve

# 拉取模型
ollama pull llama3.1
ollama pull qwen2.5-coder
```

---

## 可用模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| llama3.1 | 131K | Meta 最新 |
| qwen2.5-coder | 131K | 代码优化 |
| mistral | 32K | 高效 |
| codellama | 16K | 代码专注 |

---

## 优势

- **免费**: 无 API 成本
- **私密**: 所有数据留在本地
- **离线**: 无需互联网
- **灵活**: 使用任何 GGUF 模型

---

## 要求

- 现代 CPU 或 GPU
- 每个模型 4GB+ RAM
- 模型存储空间

---

## 相关文档

- [Providers 概览](overview-zh.md)
- [Ollama 网站](https://ollama.ai)
