# Task Splitting

Task splitting executes complex work in subtasks while keeping main-session context small.

---

## Overview

Use `/split <task>` to:

1. Split a complex task into subtasks
2. Execute subtasks
3. Persist full process/results in SQLite
4. Return a lightweight completion marker to chat

Main-session result is now marker-based:

```text
Task completed ✅
Task ID: task-...
Marker: [task_done:task-...]
```

This avoids flooding active context with long execution logs.

---

## Commands

### Split and run

```bash
/split Build a weekly KPI summary from logs and incidents
```

### Query tasks

```bash
/task list [limit]
/task summary <task-id>
/task detail <task-id> [page] [pageSize]
```

### Marker shortcut

If a message contains one or more markers like:

```text
[task_done:task-1739999999999]
```

OCG auto-resolves them into task summaries.

---

## Storage Model

Task data is persisted in SQLite tables:

- `user_tasks`
- `user_subtasks`

So full details survive compaction/reset of normal chat context.

---

## Why this design

### Pros

- Main context stays compact
- Full details always recoverable from DB
- Works well with repeated compaction cycles

### Tradeoff

- Requires explicit lookup (`/task summary` or `/task detail`) for deep history

---

## Pagination

`/task detail` supports pagination for very large tasks:

```bash
/task detail task-1739999999999 1 20
/task detail task-1739999999999 2 20
```

---

## Time fields

Task summary/detail now render local readable timestamps:

- `created_at`
- `completed_at`
- `duration_ms`
