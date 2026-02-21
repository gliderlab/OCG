# 中国提供商

适用于 OCG 的中国 LLM 提供商。

---

## Moonshot AI (月之暗面)

### 配置

```bash
export MOONSHOT_API_KEY="..."
export MOONSHOT_MODEL="moonshot-v1-8k"
```

### 模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| moonshot-v1-8k | 8K | 基础上下文 |
| moonshot-v1-32K | 32K | 扩展上下文 |
| moonshot-v1-128K | 128K | 长上下文 |

---

## 智谱 GLM (Zhipu)

### 配置

```bash
export ZHIPU_API_KEY="..."
export ZHIPU_MODEL="glm-4"
```

### 模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| glm-4 | 128K | 主要模型 |
| glm-4v | 128K | 视觉能力 |
| chatglm-turbo | 16K | 快速版本 |

---

## 百度千帆 (Baidu Qianfan)

### 配置

```bash
export QIANFAN_ACCESS_KEY="..."
export QIANFAN_SECRET_KEY="..."
export QIANFAN_MODEL="ernie-speed-8k"
```

### 模型

| 模型 | 上下文 | 描述 |
|------|--------|------|
| ernie-speed-8k | 8K | 快速、性价比高 |
| ernie-bot-4 | 8K | 完整版本 |
| ernie-tiny-8k | 8K | 轻量级 |

---

## Vercel AI SDK

### 配置

```bash
export VERCEL_API_KEY="..."
export VERCEL_MODEL="gpt-4o"
```

使用 Vercel 的 AI 网关进行统一访问。

---

## 提供商选择

对于中文语言任务：

```bash
# 最佳质量
export MOONSHOT_API_KEY="..."
export MOONSHOT_MODEL="moonshot-v1-128K"

# 最佳性价比
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"

# 最多功能
export ZHIPU_API_KEY="..."
export ZHIPU_MODEL="glm-4"
```

---

## 相关文档

- [Providers 概览](overview-zh.md)
- [MiniMax](minimax-zh.md)
