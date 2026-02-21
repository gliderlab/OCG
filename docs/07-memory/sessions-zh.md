# 会话记忆

每个用户、每个通道的对话历史。

---

## 概述

每个用户/通道有独立的对话上下文。

### 会话密钥格式

| 通道 | 格式 | 示例 |
|------|------|------|
| Telegram | `telegram_{chat_id}` | telegram_123456789 |
| Discord | `discord_{channel_id}` | discord_987654321 |
| Slack | `slack_{channel_id}` | slack_C00123456 |

---

## 功能

### 历史加载

- 自动加载最近 100 条消息
- 跨会话保留上下文
- 每个通道独立上下文

### 命令

| 命令 | 描述 |
|------|------|
| `/new` | 创建新会话 |
| `/reset` | 重置当前会话 |
| `/compact` | 压缩对话 |

---

## 配置

```json
{
  "sessions": {
    "max_messages": 100,
    "dm_scope": "all"  // 或 "private", "group"
  }
}
```

---

## 持久化

会话存储在 SQLite 中：

```bash
# 查看会话
./bin/ocg sessions list

# 查看历史
./bin/ocg sessions history <session_key>
```

---

## 相关文档

- [记忆概览](overview-zh.md)
- [压缩](compaction-zh.md)
