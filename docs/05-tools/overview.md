# Tools Overview

OCG provides 17+ built-in tools for AI agent operations.

---

## Tool Categories

### File System
| Tool | Description |
|------|-------------|
| `read` | Read file contents |
| `write` | Create/overwrite files |
| `edit` | Edit file contents |
| `apply_patch` | Multi-file structured patches |

### Execution
| Tool | Description |
|------|-------------|
| `exec` | Execute shell commands |
| `process` | Background process management |

### Web
| Tool | Description |
|------|-------------|
| `web_search` | AI-optimized web search |
| `web_fetch` | Fetch URL content |

### Browser
| Tool | Description |
|------|-------------|
| `browser` | CDP-based browser automation |

### UI
| Tool | Description |
|------|-------------|
| `canvas` | Node canvas control |
| `nodes` | Paired node management |

### Memory
| Tool | Description |
|------|-------------|
| `memory_search` | Semantic memory search |
| `memory_get` | Read memory snippets |
| `memory_store` | Store memories |

### Sessions
| Tool | Description |
|------|-------------|
| `sessions_list` | List sessions |
| `sessions_history` | Fetch session history |
| `sessions_send` | Send to another session |
| `sessions_spawn` | Spawn sub-agent |
| `session_status` | Session status |
| `agents_list` | List available agents |

### Automation
| Tool | Description |
|------|-------------|
| `cron` | Schedule jobs |
| `message` | Send messages |
| `image` | Analyze images |

---

## Tool Usage

### Read File

```bash
read(path="/path/to/file.md")
```

### Write File

```bash
write(path="/path/to/file.md", content="file content")
```

### Edit File

```bash
edit(path="/path/to/file.md", oldText="old content", newText="new content")
```

### Execute Command

```bash
exec(command="ls -la", timeout=30)
```

### Web Search

```bash
web_search(query="OCG documentation")
```

### Web Fetch

```bash
web_fetch(url="https://github.com/gliderlab/OCG")
```

---

## Tool Execution Flow

```
Agent decides to use tool
        ↓
Generate tool call arguments
        ↓
Execute tool via Gateway
        ↓
Return result to LLM
        ↓
LLM generates response
```

---

## Tool Limitations

### File System

- Working directory: `bin/work`
- Read size limit: 50KB (editable files)
- Write operations: Create, overwrite, edit

### Execution

- Shell features disabled by default
- Dangerous commands (rm, sudo, kill) require confirmation
- Working directory: `bin/work`

### Web

- Timeout: 15-30 seconds
- Body limit: 2-5MB

### Browser

- Requires CDP connection
- Limited to Chrome DevTools Protocol

---

## See Also

- [File System Tools](filesystem.md)
- [Execution Tools](execution.md)
- [Browser Tool](browser.md)
- [Web Tools](web.md)
- [Memory Tools](memory.md)
