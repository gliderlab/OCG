# Chinese Providers

Chinese LLM providers for OCG.

---

## Moonshot AI (月之暗面)

### Configuration

```bash
export MOONSHOT_API_KEY="..."
export MOONSHOT_MODEL="moonshot-v1-8k"
```

### Models

| Model | Context | Description |
|-------|---------|-------------|
| moonshot-v1-8k | 8K | Base context |
| moonshot-v1-32K | 32K | Extended context |
| moonshot-v1-128K | 128K | Long context |

---

## Zhipu GLM (智谱)

### Configuration

```bash
export ZHIPU_API_KEY="..."
export ZHIPU_MODEL="glm-4"
```

### Models

| Model | Context | Description |
|-------|---------|-------------|
| glm-4 | 128K | Main model |
| glm-4v | 128K | Vision capable |
| chatglm-turbo | 16K | Fast version |

---

## Baidu Qianfan (百度千帆)

### Configuration

```bash
export QIANFAN_ACCESS_KEY="..."
export QIANFAN_SECRET_KEY="..."
export QIANFAN_MODEL="ernie-speed-8k"
```

### Models

| Model | Context | Description |
|-------|---------|-------------|
| ernie-speed-8k | 8K | Fast, cost-effective |
| ernie-bot-4 | 8K | Full version |
| ernie-tiny-8k | 8K | Lightweight |

---

## Vercel AI SDK

### Configuration

```bash
export VERCEL_API_KEY="..."
export VERCEL_MODEL="gpt-4o"
```

Uses Vercel's AI gateway for unified access.

---

## Provider Selection

For Chinese language tasks:

```bash
# Best quality
export MOONSHOT_API_KEY="..."
export MOONSHOT_MODEL="moonshot-v1-128K"

# Best value
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"

# Most features
export ZHIPU_API_KEY="..."
export ZHIPU_MODEL="glm-4"
```

---

## See Also

- [Providers Overview](../overview.md)
- [MiniMax](minimax.md)
