# Discord 配置

为 OCG 配置 Discord 机器人。

---

## 前置要求

1. 在 [Discord 开发者门户](https://discord.com/developers/applications) 创建应用
2. 创建机器人并获取令牌
3. 启用所需的 intents：
   - Message Content Intent
   - Presence Intent
4. 邀请机器人到服务器

---

## 配置

### 环境变量

```bash
export DISCORD_BOT_TOKEN="your-bot-token"
export DISCORD_APPLICATION_ID="your-app-id"
export DISCORD_PUBLIC_KEY="your-public-key"
```

### 配置文件

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "bot_token": "your-bot-token",
      "application_id": "your-app-id",
      "public_key": "your-public-key"
    }
  }
}
```

---

## 设置步骤

### 1. 创建应用

```bash
# 进入 Discord 开发者门户
# https://discord.com/developers/applications
#
# 1. 点击"新建应用"
# 2. 命名您的应用
# 3. 点击"机器人" → "添加机器人"
# 4. 复制机器人令牌
```

### 2. 启用 Intents

在 Discord 开发者门户中：
- 机器人 → 启用"Message Content Intent"
- 机器人 → 启用"Presence Intent"

### 3. 邀请机器人

```bash
# 生成带有权限的邀请 URL
https://discord.com/api/oauth2/authorize?client_id=YOUR_APP_ID&permissions=379968&scope=bot%20applications.commands
```

### 4. 配置 OCG

```bash
export DISCORD_BOT_TOKEN="your-actual-bot-token"
./bin/ocg restart
```

---

## 功能

### 输入指示器

显示机器人正在输入：

```go
// Discord API: POST /channels/{id}/typing
```

### 斜杠命令

| 命令 | 描述 |
|------|------|
| `/chat <消息>` | 发送消息给 AI |
| `/help` | 显示帮助 |
| `/new` | 新对话会话 |

### 消息格式

支持 Discord markdown：
- **粗体** `**text**`
- *斜体* `*text*`
- 代码 `` `code` ``
- 代码块 ```` ```code``` ````

---

## 故障排除

### 机器人离线

```bash
# 检查令牌是否正确
# 验证 intents 是否启用
# 检查机器人权限是否正确
```

### 无响应

1. 检查 Message Content Intent 是否启用
2. 验证机器人是否在服务器中
3. 检查日志：`tail -f /tmp/ocg-gateway.log`

### 权限错误

必需的机器人权限：
- 发送消息
- 嵌入链接
- 附加文件
- 读取消息历史
- 使用斜杠命令

---

## 安全性

### 机器人令牌

- 保持令牌机密
- 不要提交到 git
- 如果泄露请更换

### 速率限制

```bash
# Discord 有内置的速率限制
# OCG 自动遵守这些限制
```

---

## 相关文档

- [通道概览](overview-zh.md)
- [Discord API 文档](https://discord.com/developers/docs)
