# 压缩

压缩使会话保持在模型上下文限制内，同时保留可恢复的历史。

---

## 概述

OCG 压缩有两个目标：

1. 保持活动上下文精简（摘要 + 最近消息）
2. 持久化旧对话以供后续检查

压缩可以运行：

- 当上下文跨过阈值时自动
- 手动通过 `/compact`

---

## 当前行为（严格增量归档）

压缩期间，OCG 现在只归档**新近被压缩的消息**。

### 保障

- 使用水位线：`session_meta.last_compacted_message_id`
- 只归档 `(last_compacted_message_id, 当前截止点]` 范围内的消息
- 跳过生成的摘要条目（`[summary]...`）不进入归档负载
- 使用归档源 ID 上的去重索引避免重复归档行

### 归档去重

`messages_archive` 包含：

- `source_message_id`
- 唯一索引 `(session_key, source_message_id)`

因此重复压缩/重试路径不会重复归档行。

---

## 流程

1. 估算 tokens 并检查压缩阈值
2. 拆分为 `old` 和 `keep`
3. 增量归档 `old`（水位线 + 去重）
4. 清除活动消息
5. 重新插入 `keep`
6. 添加 `[summary]` 系统消息
7. 更新 `session_meta` 计数器 + 水位线

---

## 命令

```bash
/compact
/compact 聚焦于决策和未解决事项
```

调试归档状态：

```bash
/debug archive
/debug archive default
```

返回水位线和归档统计以供验证。

---

## 注意

- `messages_archive` 是长期历史存储，不是活动提示上下文。
- 活动上下文保持为压缩后的 transcript + 最近回合。
- 此设计支持重复压缩（第 2 次、第 3 次...），每次只归档新的未压缩范围。
