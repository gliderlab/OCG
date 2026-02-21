# Process Management

Start, stop, and manage OCG processes.

---

## Start Services

```bash
./bin/ocg start
```

Starts in order: embedding → agent → gateway

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--config` | `./env.config` | Config file path |
| `--pid-dir` | `/tmp/ocg` | PID file directory |

---

## Stop Services

```bash
./bin/ocg stop
```

Graceful shutdown with escalating signals:

```
SIGTERM (3s) → SIGINT (3s) → SIGKILL
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--pid-dir` | `/tmp/ocg` | PID file directory |
| `--force` | false | Immediate SIGKILL |

---

## Restart

```bash
./bin/ocg restart
```

Equivalent to stop + start.

---

## Status

```bash
./bin/ocg status
```

Output:

```
Embedding: running (PID 1234)
Agent: running (PID 1235)
Gateway: running (PID 1236)
Health: ok
```

---

## Manual Process Control

```bash
# Start individual services
./bin/ocg-embedding &
./bin/ocg-agent &
./bin/ocg-gateway &

# Check processes
ps aux | grep ocg
```

---

## See Also

- [CLI Overview](../overview.md)
