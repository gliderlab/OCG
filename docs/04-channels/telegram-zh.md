# Telegram 配置

为 OCG 配置 Telegram 机器人。

---

## 前置要求

1. 通过 [@BotFather](https://t.me/BotFather) 创建机器人
2. 获取机器人令牌
3. 与机器人开始聊天

---

## 配置

### 环境变量

```bash
# 从 @BotFather 获取的机器人令牌
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# 模式：long_polling (默认) 或 webhook
export TELEGRAM_MODE="long_polling"

# Webhook 模式（需要公网 URL）
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
```

### 配置文件

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
      "mode": "long_polling"
    }
  }
}
```

---

## Long Polling vs Webhook

### Long Polling（默认）

**优势：**
- 无需公网 URL
- 可在本地/私有网络运行
- 设置更简单

**用法：**
```bash
export TELEGRAM_MODE="long_polling"
export TELEGRAM_BOT_TOKEN="your-token"
./bin/ocg start
```

### Webhook

**要求：**
- 公共 HTTPS URL
- SSL 证书

**用法：**
```bash
export TELEGRAM_MODE="webhook"
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
./bin/ocg start
```

---

## 功能

### 输入指示器

AI 生成响应时显示"正在输入"。

```go
// 通过 sendChatAction API 实现
// action: "typing"
```

### 命令

| 命令 | 描述 |
|------|------|
| `/start` | 开始对话 |
| `/help` | 显示帮助 |
| `/new` | 创建新会话 |
| `/reset` | 重置当前会话 |

### 内联按钮

支持内联键盘按钮：

```json
{
  "buttons": [
    [{"text": "选项 1", "callback_data": "opt1"}],
    [{"text": "选项 2", "callback_data": "opt2"}]
  ]
}
```

---

## 机器人命令设置

在 Telegram 中显示机器人命令：

```bash
# 通过 BotFather 设置命令
/setcommands

# 添加：
# start - 开始对话
# help - 显示帮助
# new - 新会话
# reset - 重置会话
```

---

## 故障排除

### 机器人无响应

1. 检查令牌是否正确
2. 验证机器人已启动 (/start)
3. 检查日志：`tail -f /tmp/ocg-gateway.log`

### Long Polling 问题

```bash
# 如果 long polling 失败，检查网络
curl -s https://api.telegram.org/bot<TOKEN>/getMe
```

### Webhook 问题

```bash
# 验证 webhook URL 可访问
curl -X POST https://your-domain.com/webhooks/telegram \
  -H "Content-Type: application/json" \
  -d '{"message": {"chat": {"id": 123}}}'
```

---

## 安全性

### 机器人令牌

- 保持令牌机密
- 不要提交到 git
- 使用环境变量

### 私聊

机器人只会响应已启动聊天的用户。

---

## 相关文档

- [通道概览](overview-zh.md)
- [Telegram 集成](./telegram.md)
