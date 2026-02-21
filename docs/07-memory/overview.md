# Memory System Overview

OCG's memory architecture for persistent context.

---

## Memory Types

### Short-term Memory

- Current conversation context
- Automatically managed
- Token-based pruning

### Long-term Memory

- Vector-based semantic memory
- Persistent across sessions
- Searchable via embeddings

---

## Storage Locations

```
/root/.openclaw/workspace/
├── MEMORY.md              # Curated long-term memories
└── memory/
    ├── 2026-02-18.md      # Daily notes
    ├── 2026-02-17.md
    └── ...
```

---

## Features

### Auto-Recall

Automatically retrieves relevant memories:

```bash
export AUTO_RECALL=true
export RECALL_THRESHOLD=0.72
```

### Semantic Search

Uses vector embeddings for similarity matching:

```bash
memory_search(query="project configuration")
```

### Compaction

Compresses old conversations to save context:

```bash
# Manual trigger
/compact

# Automatic when near limit
/context-overflow
```

---

## See Also

- [Vector Memory](vector.md)
- [Session Memory](sessions.md)
- [Memory Tools](../../05-tools/memory.md)
