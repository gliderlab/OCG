# 快速入门

在几分钟内让 OCG 运行起来。

---

## 快速开始

### 1. 配置环境

在项目根目录创建或编辑 `env.config`：

```bash
# 必需：UI 认证令牌
OCG_UI_TOKEN="your-secure-token-here"

# 可选：LLM Provider
OPENAI_API_KEY="sk-..."
# 或使用其他提供商
# ANTHROPIC_API_KEY="..."
# GOOGLE_API_KEY="..."
# MINIMAX_API_KEY="..."
```

### 2. 启动服务

```bash
cd /opt/openclaw-go

# 启动所有服务 (embedding → agent → gateway)
./bin/ocg start
```

**启动顺序:**
1. Embedding 服务 (端口 50000-60000)
2. Agent 服务 (Unix socket)
3. Gateway 服务 (端口 55003)

### 3. 访问 Web UI

打开浏览器：

```
http://localhost:55003
```

输入您的 `OCG_UI_TOKEN` 进行身份验证。

### 4. 开始聊天

使用 Web UI 聊天界面或 CLI：

```bash
# 交互式 CLI 聊天
./bin/ocg agent
```

---

## 基本配置

### LLM Provider 设置

**OpenAI:**
```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_MODEL="gpt-4o"
```

**Anthropic:**
```bash
export ANTHROPIC_API_KEY="..."
export ANTHROPIC_MODEL="claude-sonnet-4-20250514"
```

**Google Gemini:**
```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
```

**MiniMax:**
```bash
export MINIMAX_API_KEY="..."
export MINIMAX_MODEL="MiniMax-M2"
```

**Ollama (本地):**
```bash
export OLLAMA_MODEL="llama3.1"
# 无需 API key
```

### Telegram 设置 (可选)

```bash
# 从 @BotFather 获取的 Bot token
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# 模式：long_polling (默认) 或 webhook
export TELEGRAM_MODE="long_polling"
```

---

## 验证安装

```bash
# 检查服务状态
./bin/ocg status

# 健康检查
curl http://localhost:55003/health

# 存储统计 (需要 UI token)
curl -H "X-OCG-UI-Token: your-token" \
     http://localhost:55003/storage/stats
```

---

## 常用命令

```bash
# 启动服务
./bin/ocg start

# 停止服务
./bin/ocg stop

# 重启
./bin/ocg restart

# 检查状态
./bin/ocg status

# 交互式聊天
./bin/ocg agent
```

---

## 下一步

- [环境配置](environment-zh.md) - 详细的环境配置
- [配置指南](03-configuration/guide-zh.md) - 完整配置选项
- [工具概览](../05-tools/overview-zh.md) - 可用工具
