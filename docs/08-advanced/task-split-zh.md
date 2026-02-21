# 任务拆分

任务拆分将复杂工作分解为子任务，同时保持主会话上下文精简。

---

## 概述

使用 `/split <任务>` 可以：

1. 将复杂任务拆分为子任务
2. 执行子任务
3. 将完整过程/结果持久化到 SQLite
4. 向聊天返回轻量级完成 marker

主会话结果现在基于 marker：

```text
任务已完成 ✅
任务 ID: task-...
Marker: [task_done:task-...]
```

这避免了用长执行日志淹没活动上下文。

---

## 命令

### 拆分并运行

```bash
/split 从日志和事件构建每周 KPI 汇总
```

### 查询任务

```bash
/task list [limit]
/task summary <task-id>
/task detail <task-id> [page] [pageSize]
```

### Marker 快捷方式

如果消息包含一个或多个 marker，如：

```text
[task_done:task-1739999999999]
```

OCG 会自动将它们解析为任务摘要。

---

## 存储模型

任务数据持久化在 SQLite 表中：

- `user_tasks`
- `user_subtasks`

因此完整详情在普通聊天上下文压缩/重置后仍可恢复。

---

## 为什么这样设计

### 优点

-保持精简
- 完整详情始终可从 主上下文 DB 恢复
- 与重复压缩周期配合良好

### 权衡

- 需要显式查询（`/task summary` 或 `/task detail`）才能获取深层历史

---

## 分页

`/task detail` 支持非常大型任务的分页：

```bash
/task detail task-1739999999999 1 20
/task detail task-1739999999999 2 20
```

---

## 时间字段

任务摘要/详情现在渲染本地可读时间戳：

- `created_at`
- `completed_at`
- `duration_ms`
