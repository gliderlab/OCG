# Context Pruning

Automatically remove old tool results from context.

---

## Overview

Old tool results consume context space. Pruning removes them while preserving important information.

---

## Configuration

```bash
export CONTEXT_PRUNE_DAYS=30
export CONTEXT_PRUNE_TOOL_RESULTS=true
```

Or via config:

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

## What Gets Pruned

| Type | Preserved | Pruned After |
|------|-----------|--------------|
| Tool results | ❌ | 30 days |
| Intermediate steps | ❌ | 7 days |
| User messages | ✅ | Never |
| Final summaries | ✅ | Never |

---

## Automatic Trigger

Pruning happens automatically:

1. Before compaction
2. When context is near limit
3. On `/compact` command

---

## Manual Trigger

```bash
/prune           # Prune old tool results
/prune all       # Clear all cache
/prune status    # Check prune status
```

---

## See Also

- [Compaction](../../07-memory/compaction.md)
- [Memory Overview](../../07-memory/overview.md)
