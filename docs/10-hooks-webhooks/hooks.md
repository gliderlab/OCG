# Hooks Overview

Event-driven automation system for OCG.

---

## What are Hooks?

Hooks are event handlers that run automatically when events occur.

### Event Types

| Event | Description |
|-------|-------------|
| `agent.start` | Agent started |
| `agent.stop` | Agent stopped |
| `message.send` | Message sent |
| `message.receive` | Message received |
| `session.create` | New session created |
| `session.end` | Session ended |
| `task.complete` | Task completed |
| `error` | Error occurred |

---

## Using Hooks

### Enable/Disable

```bash
./bin/ocg hooks list
./bin/ocg hooks enable session-memory
./bin/ocg hooks disable command-logger
./bin/ocg hooks info <hook_name>
```

### Check Status

```bash
./bin/ocg hooks status
```

---

## Configuration

```json
{
  "hooks": {
    "enabled": true,
    "registry_path": "./hooks",
    "auto_discover": true
  }
}
```

---

## See Also

- [Built-in Hooks](builtin.md)
- [Webhooks](webhooks.md)
