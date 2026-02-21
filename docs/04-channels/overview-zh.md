# 通道概览

OCG 支持的消息平台。

---

## 支持的通道

OCG 支持 18 个消息通道：

| 通道 | 协议 | 状态 |
|------|------|------|
| Telegram | Long Polling / Webhook | ✅ |
| Discord | Gateway (WSS) | ✅ |
| Slack | Events API + RTM | ✅ |
| WhatsApp | Cloud API | ✅ |
| Signal | P2P | ✅ |
| IRC | Raw TCP | ✅ |
| Google Chat | HTTP | ✅ |
| Microsoft Teams | HTTP | ✅ |
| WebChat | WebSocket | ✅ |
| Mattermost | HTTP | ✅ |
| LINE | HTTP | ✅ |
| Matrix | HTTP | ✅ |
| 飞书 | HTTP | ✅ |
| Zalo | HTTP | ✅ |
| Threema | HTTP | ✅ |
| Session | P2P | ✅ |
| Tox | P2P | ✅ |
| iMessage | P2P | ✅ |

---

## 通道架构

```
Gateway (55003)
    │
    ├── telegram/      # Telegram Bot API
    ├── discord/       # Discord Gateway + REST
    ├── slack/         # Slack Events + RTM
    ├── whatsapp/      # WhatsApp Cloud API
    ├── signal/        # Signal P2P
    ├── irc/           # IRC RFC 1459
    ├── googlechat/    # Google Chat API
    ├── msteams/       # Microsoft Teams API
    ├── webchat/       # WebSocket 聊天
    └── ...
```

---

## 启用通道

### 通过环境变量

```bash
# Telegram
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_MODE="long_polling"  # 或 "webhook"

# Discord
export DISCORD_BOT_TOKEN="your-token"

# Slack
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_SIGNING_SECRET="..."
```

### 通过配置文件

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "your-token"
    },
    "discord": {
      "enabled": true,
      "bot_token": "your-token"
    }
  }
}
```

---

## 通用功能

### 输入指示器

AI 生成响应时显示"正在输入"：

- **Telegram**: `sendChatAction` API
- **Discord**: `channels/{id}/typing`
- **Slack**: `users.typing`

### 会话上下文

每个用户/通道有独立的对话历史：

- 会话密钥格式：`telegram_{chat_id}`, `discord_{channel_id}`
- 自动加载历史（最近 100 条消息）
- 按通道隔离上下文

---

## 速率限制

```bash
# 设置每个通道的速率限制
./bin/ocg ratelimit set --channel telegram --max 30 --window 60

# 列出所有限制
./bin/ocg ratelimit list

# 检查特定通道
./bin/ocg ratelimit check --channel telegram
```

---

## 相关文档

- [Telegram 配置](telegram-zh.md)
- [Discord 配置](discord-zh.md)
- [其他通道](others-zh.md)
