# File System Tools

Read, write, and edit files.

---

## read

Read file contents.

### Usage

```bash
read(path="/path/to/file")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path (relative or absolute) |
| `offset` | int | Line number to start (1-indexed, optional) |
| `limit` | int | Maximum lines (optional) |

### Example

```bash
read(path="/opt/openclaw-go/README.md")
read(path="/opt/openclaw-go/README.md", offset=1, limit=50)
```

### Returns

```
File contents as text string.
Image files are sent as attachments.
```

### Limitations

- Maximum: 2000 lines or 50KB (whichever first)
- Use offset/limit for large files

---

## write

Create or overwrite files.

### Usage

```bash
write(path="/path/to/file", content="file content")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path |
| `content` | string | Content to write |

### Example

```bash
write(path="/tmp/test.md", content="# Hello World\n\nThis is a test.")
```

### Notes

- Creates parent directories if needed
- Overwrites existing files

---

## edit

Edit file contents by replacing exact text.

### Usage

```bash
edit(path="/path/to/file", oldText="old content", newText="new content")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | File path |
| `oldText` | string | Text to find (must match exactly) |
| `newText` | string | Replacement text |

### Example

```bash
edit(path="/tmp/test.md", oldText="old content", newText="new content")
```

### Notes

- oldText must match exactly (including whitespace)
- Only first occurrence is replaced
- Use for precise, surgical edits

---

## apply_patch

Multi-file structured patches.

### Usage

```bash
apply_patch(patch=[...])
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `patch` | array | Array of patch operations |

### Patch Operations

```json
{
  "patch": [
    {
      "op": "add",
      "path": "/new/file.md",
      "content": "# New File"
    },
    {
      "op": "update",
      "path": "/existing/file.md",
      "oldText": "old text",
      "newText": "new text"
    },
    {
      "op": "delete",
      "path": "/file/to/delete.md"
    },
    {
      "op": "move",
      "from": "/old/path.md",
      "to": "/new/path.md"
    }
  ]
}
```

### Operations

| Op | Description |
|----|-------------|
| `add` | Create new file |
| `update` | Modify existing file |
| `delete` | Remove file |
| `move` | Move/rename file |

---

## Working Directory

All file operations are restricted to:

```bash
# Linux/macOS
/opt/openclaw-go/bin/work/

# Windows
%TEMP%\ocg-work\
```

Files outside this directory require explicit path or confirmation.

---

## Safety

### Read Operations

- Files are truncated to 2000 lines or 50KB
- Binary files may not display properly

### Write Operations

- Create parent directories automatically
- Overwrite without warning

### Dangerous Operations

- Files in system directories require confirmation
- `rm` operations are logged

---

## See Also

- [Tools Overview](../overview.md)
- [Execution Tools](execution.md)
