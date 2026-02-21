# Execution Tools

Execute shell commands and manage processes.

---

## exec

Execute shell commands.

### Usage

```bash
exec(command="ls -la", timeout=30)
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `command` | string | Shell command to execute |
| `timeout` | int | Timeout in seconds (optional) |
| `workdir` | string | Working directory (optional) |
| `env` | object | Environment variables (optional) |
| `pty` | bool | Run in pseudo-terminal (optional) |
| `elevated` | bool | Run with elevated permissions (optional) |

### Example

```bash
exec(command="ls -la /opt/openclaw-go")
exec(command="git status", workdir="/opt/openclaw-go")
exec(command="top -bn1", timeout=10)
```

### Returns

```json
{
  "stdout": "command output",
  "stderr": "error output",
  "exit_code": 0
}
```

---

## process

Manage background processes.

### List Processes

```bash
process(action="list")
```

### Execute Command

```bash
process(action="exec", command="ls -la", sessionId="mysession")
```

### Send Keys

```bash
process(action="send-keys", sessionId="mysession", keys=["enter"])
```

### Kill Process

```bash
process(action="kill", sessionId="mysession")
```

### Get Logs

```bash
process(action="log", sessionId="mysession", offset=0, limit=100)
```

---

## Working Directory

Commands run in restricted directory:

```bash
# Default: bin/work
/opt/openclaw-go/bin/work/
```

### Override

```bash
exec(command="ls", workdir="/tmp")
```

---

## Safety

### Dangerous Commands

Commands like `rm`, `sudo`, `kill` require confirmation:

```bash
# Without confirmation (will prompt)
exec(command="rm -rf /tmp/test")

# With explicit confirmation
exec(command="rm -rf /tmp/test", ask="confirm")
```

### Shell Features

Disabled by default:

```
✓ Simple commands: ls, cat, echo
✗ Pipes: cat file | grep pattern
✗ Redirection: cat > file
✗ Globbing: cat *.txt
✗ Variable expansion: $HOME
```

If shell features are needed:

```bash
exec(command="cat file | grep pattern", ask="on-miss")
```

---

## Security Modes

### deny (Default)

Only allows commands matching patterns:

```bash
exec(command="ls", security="allowlist")
```

### allowlist

Permits specified commands:

```json
{
  "security": "allowlist",
  "allowed_commands": ["ls", "cat", "echo", "git"]
}
```

### full

Allow any command (dangerous!):

```json
{
  "security": "full"
}
```

---

## Background Processes

### Start Process

```bash
process(action="exec", command="python -m http.server 8080", background=true)
```

### Manage Sessions

```bash
# List all sessions
process(action="list")

# Get process output
process(action="log", sessionId="abc123")
```

---

## See Also

- [Tools Overview](../overview.md)
- [File System Tools](filesystem.md)
