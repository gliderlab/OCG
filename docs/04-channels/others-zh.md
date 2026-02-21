# 其他通道

配置其他消息平台。

---

## Slack

### 配置

```bash
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_SIGNING_SECRET="..."
export SLACK_APP_TOKEN="xapp-..."
```

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "bot_token": "xoxb-...",
      "signing_secret": "...",
      "app_token": "xapp-..."
    }
  }
}
```

### 设置

1. 在 [Slack API](https://api.slack.com/apps) 创建应用
2. 启用 Event Subscriptions
3. 订阅 `message.channels`, `message.groups`
4. 安装应用到工作区

---

## WhatsApp

### 配置

```bash
export WHATSAPP_PHONE_ID="..."
export WHATSAPP_ACCESS_TOKEN="..."
```

```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "phone_id": "...",
      "access_token": "..."
    }
  }
}
```

### 设置

1. 创建 Meta 开发者账户
2. 创建 WhatsApp API 应用
3. 获取电话号码 ID
4. 生成访问令牌

---

## IRC

### 配置

```bash
export IRC_SERVER="irc.libera.chat"
export IRC_PORT="6667"
export IRC_NICK="ocg_bot"
export IRC_CHANNEL="#your-channel"
```

```json
{
  "channels": {
    "irc": {
      "enabled": true,
      "server": "irc.libera.chat",
      "port": 6667,
      "nick": "ocg_bot",
      "channel": "#your-channel"
    }
  }
}
```

---

## Google Chat

### 配置

```bash
export GOOGLECHAT_BOT_TOKEN="..."
export GOOGLECHAT_SPACE_NAME="spaces/..."
```

```json
{
  "channels": {
    "googlechat": {
      "enabled": true,
      "bot_token": "...",
      "space_name": "spaces/..."
    }
  }
}
```

---

## Microsoft Teams

### 配置

```bash
export MSTEAMS_TENANT_ID="..."
export MSTEAMS_CLIENT_ID="..."
export MSTEAMS_CLIENT_SECRET="..."
```

```json
{
  "channels": {
    "msteams": {
      "enabled": true,
      "tenant_id": "...",
      "client_id": "...",
      "client_secret": "..."
    }
  }
}
```

---

## WebChat (WebSocket)

### 配置

```json
{
  "channels": {
    "webchat": {
      "enabled": true,
      "websocket_path": "/v1/chat/ws"
    }
  }
}
```

通过 WebSocket 连接进行自定义集成。

---

## 中国平台

### 飞书

```bash
export FEISHU_APP_ID="..."
export FEISHU_APP_SECRET="..."
```

### Zalo

```bash
export ZALO_APP_ID="..."
export ZALO_APP_SECRET="..."
```

### LINE

```bash
export LINE_CHANNEL_TOKEN="..."
export LINE_CHANNEL_SECRET="..."
```

### Matrix

```bash
export MATRIX_HOMESERVER="https://matrix.org"
export MATRIX_USER="@ocg:matrix.org"
export MATRIX_PASSWORD="..."
```

---

## Signal

### 配置

```bash
export SIGNAL_PHONE_NUMBER="+1234567890"
export SIGNAL_REGISTRATION_ID="..."
export SIGNAL_IDENTITY_KEY="..."
```

注意：Signal 需要使用 signal-cli 进行额外设置。

---

## 通道对比

| 通道 | 协议 | 设置难度 | 实时 |
|------|------|----------|------|
| Telegram | HTTP | 简单 | 是 |
| Discord | WSS | 中等 | 是 |
| Slack | RTM/API | 中等 | 是 |
| WhatsApp | HTTP | 中等 | 是 |
| IRC | TCP | 简单 | 是 |
| WebChat | WebSocket | 简单 | 是 |
| Signal | P2P | 困难 | 是 |
| 飞书 | HTTP | 中等 | 否 |

---

## 常见问题

### 未收到消息

1. 检查通道是否在配置中启用
2. 验证身份凭证
3. 检查日志中的错误

### 速率限制

每个通道有不同的速率限制。OCG 自动处理大多数情况。

### 连接问题

```bash
# 检查网络连接
nc -zv server.com port

# 检查日志
tail -f /tmp/ocg-gateway.log
```

---

## 相关文档

- [通道概览](overview-zh.md)
- [Telegram 配置](telegram-zh.md)
- [Discord 配置](discord-zh.md)
