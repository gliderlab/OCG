# 任务管理

创建和管理后台任务。

---

## 创建任务

```bash
./bin/ocg task create "研究 AI agent"
./bin/ocg task create "编写文档" --model gpt-4o
```

创建任务并开始执行。

---

## 列出任务

```bash
./bin/ocg task list
```

输出:

```
ID                                    状态      创建时间
task-abc123                           运行中    2026-02-19
task-def456                           已完成    2026-02-18
task-ghi789                           失败      2026-02-17
```

---

## 任务状态

```bash
./bin/ocg task status <task_id>
```

显示子任务进度。

---

## 重试任务

```bash
./bin/ocg task retry <task_id>
```

重试失败的任务。

---

## 取消任务

```bash
./bin/ocg task cancel <task_id>
```

取消运行中的任务。

---

## 删除任务

```bash
./bin/ocg task delete <task_id>
```

从历史记录中删除任务。

---

## 相关文档

- [CLI 概览](overview-zh.md)
- [任务拆分](../../08-advanced/task-split-zh.md)
