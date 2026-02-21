# 上下文裁剪

自动从上下文中删除旧的工具结果。

---

## 概述

旧的工具结果消耗上下文空间。裁剪在保留重要信息的同时删除它们。

---

## 配置

```bash
export CONTEXT_PRUNE_DAYS=30
export CONTEXT_PRUNE_TOOL_RESULTS=true
```

或通过配置：

```json
{
  "pruning": {
    "enabled": true,
    "prune_tool_results_days": 30,
    "preserve_user_messages": true,
    "preserve_summaries": true
  }
}
```

---

## 被裁剪的内容

| 类型 | 保留 | 30天后裁剪 |
|------|------|-----------|
| 工具结果 | ❌ | 是 |
| 中间步骤 | ❌ | 是 |
| 用户消息 | ✅ | 否 |
| 最终摘要 | ✅ | 否 |

---

## 自动触发

裁剪自动发生：

1. 在压缩之前
2. 当上下文接近限制时
3. 执行 `/compact` 命令时

---

## 手动触发

```bash
/prune           # 裁剪旧的工具结果
/prune all       # 清除所有缓存
/prune status    # 检查裁剪状态
```

---

## 相关文档

- [压缩](../../07-memory/compaction-zh.md)
- [记忆概览](../../07-memory/overview-zh.md)
