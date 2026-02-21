# Hooks 概览

OCG 的事件驱动自动化系统。

---

## 什么是 Hooks？

Hooks 是在事件发生时自动运行的事件处理程序。

### 事件类型

| 事件 | 描述 |
|------|------|
| `agent.start` | Agent 启动 |
| `agent.stop` | Agent 停止 |
| `message.send` | 消息发送 |
| `message.receive` | 消息接收 |
| `session.create` | 新会话创建 |
| `session.end` | 会话结束 |
| `task.complete` | 任务完成 |
| `error` | 发生错误 |

---

## 使用 Hooks

### 启用/禁用

```bash
./bin/ocg hooks list
./bin/ocg hooks enable session-memory
./bin/ocg hooks disable command-logger
./bin/ocg hooks info <hook_name>
```

### 检查状态

```bash
./bin/ocg hooks status
```

---

## 配置

```json
{
  "hooks": {
    "enabled": true,
    "registry_path": "./hooks",
    "auto_discover": true
  }
}
```

---

## 相关文档

- [内置 Hooks](builtin-zh.md)
- [Webhooks](webhooks-zh.md)
