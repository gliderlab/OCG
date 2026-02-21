# Task Management

Create and manage background tasks.

---

## Create Task

```bash
./bin/ocg task create "Research AI agents"
./bin/ocg task create "Write documentation" --model gpt-4o
```

Creates task and starts execution.

---

## List Tasks

```bash
./bin/ocg task list
```

Output:

```
ID                                    Status    Created
task-abc123                           running   2026-02-19
task-def456                           done      2026-02-18
task-ghi789                           failed    2026-02-17
```

---

## Task Status

```bash
./bin/ocg task status <task_id>
```

Shows subtask progress.

---

## Retry Task

```bash
./bin/ocg task retry <task_id>
```

Retries failed task.

---

## Cancel Task

```bash
./bin/ocg task cancel <task_id>
```

Cancels running task.

---

## Delete Task

```bash
./bin/ocg task delete <task_id>
```

Removes task from history.

---

## See Also

- [CLI Overview](../overview.md)
- [Task Splitting](../../08-advanced/task-split.md)
