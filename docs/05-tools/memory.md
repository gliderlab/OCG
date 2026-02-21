# Memory Tools

Search, read, and store memories.

---

## memory_search

Semantic memory search using vector embeddings.

### Usage

```bash
memory_search(query="project tasks", maxResults=5)
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query |
| `maxResults` | int | Maximum results (default: 5) |
| `minScore` | float | Minimum similarity score (0.0-1.0) |

### Example

```bash
memory_search(query="OCG configuration")
memory_search(query="Telegram setup", maxResults=10, minScore=0.7)
```

### Returns

```json
{
  "results": [
    {
      "path": "MEMORY.md",
      "startLine": 10,
      "endLine": 20,
      "score": 0.92,
      "snippet": "Relevant content..."
    }
  ]
}
```

---

## memory_get

Read memory snippets from MEMORY.md or memory/*.md.

### Usage

```bash
memory_get(path="MEMORY.md")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path |
| `from` | int | Start line (optional) |
| `lines` | int | Number of lines (optional) |

### Example

```bash
memory_get(path="MEMORY.md")
memory_get(path="memory/2026-02-18.md", from=1, lines=50)
```

### Notes

- Reads from `/root/.openclaw/workspace/MEMORY.md` or `/root/.openclaw/workspace/memory/*.md`
- Returns specific lines if specified
- Used after memory_search to get full context

---

## memory_store

Store new memories.

### Usage

```bash
memory_store(content="New memory content here")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `content` | string | Memory content to store |
| `tags` | array | Tags for organization (optional) |

### Example

```bash
memory_store(content="User prefers Telegram channel for notifications")
memory_store(content="Configuration updated for production", tags=["config", "production"])
```

### Notes

- Stores to daily memory file: `memory/YYYY-MM-DD.md`
- Automatically creates memory directory if needed
- Memories are sem indexed for future search

---

## Memory Files

### Structure

```
/root/.openclaw/workspace/
├── MEMORY.md              # Long-term curated memories
└── memory/
    ├── 2026-02-18.md      # Daily notes
    ├── 2026-02-17.md
    └── ...
```

### MEMORY.md

Curated long-term memories:
- Project decisions
- Preferences and notes
- Lessons learned
- Important context

### memory/YYYY-MM-DD.md

Daily raw logs:
- What happened
- Work in progress
- Task tracking
- Temporary notes

---

## Automatic Features

### AUTO_RECALL

Automatically recalls relevant memories before LLM calls:

```bash
export AUTO_RECALL=true
export RECALL_THRESHOLD=0.72
```

### Memory Hook

Saves session memories automatically:

```bash
# Built-in hook: session-memory
# Triggered after each conversation
```

---

## See Also

- [Tools Overview](../overview.md)
- [Memory System Overview](../../07-memory/overview.md)
