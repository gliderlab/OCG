# Webhooks

用于外部集成的 HTTP 端点。

---

## 概述

Webhooks 允许外部服务触发 OCG 事件。

---

## 配置

```bash
export WEBHOOK_ENABLED=true
export WEBHOOK_TOKEN="your-secret-token"
export WEBHOOK_PATH="/hooks"
```

```json
{
  "webhook": {
    "enabled": true,
    "token": "your-secret-token",
    "path": "/hooks",
    "rate_limit": 100
  }
}
```

---

## 端点

### Wake 事件

```
POST /hooks/wake
```

触发系统唤醒事件。

### Agent 事件

```
POST /hooks/agent
```

运行孤立的 agent turn。

### 自定义映射

```
POST /hooks/:name
```

映射到自定义处理程序。

---

## 身份验证

在头中包含令牌：

```bash
curl -X POST http://localhost:55003/hooks/wake \
  -H "Authorization: Bearer your-secret-token" \
  -H "X-OCG-Token: your-ui-token"
```

---

## 相关文档

- [Hooks 概览](hooks-zh.md)
