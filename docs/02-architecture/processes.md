# Process Model

OCG process architecture and lifecycle management.

---

## Multi-Process Architecture

OCG runs 3 processes:

| Process | Type | Connection | Port |
|---------|------|------------|------|
| **ocg-embedding** | Service | HTTP | 50000-60000 |
| **ocg-agent** | Service | Unix Socket | N/A |
| **ocg-gateway** | Service + CLI | HTTP | 55003 |

### Embedding Service

Responsible for:
- Running llama.cpp server
- Generating text embeddings
- Vector similarity search

**Health Check:**
```bash
curl http://127.0.0.1:50000/health
```

### Agent Service

Core AI logic:
- Message processing
- Tool execution
- Context management
- Memory operations

**Socket Path:** `/tmp/ocg-agent.sock`

### Gateway Service

Main entry point:
- Web UI serving
- API endpoints
- WebSocket connections
- Channel integrations

---

## Lifecycle Management

### Startup Sequence

```bash
./bin/ocg start
```

1. **Kill existing** - Stop any running OCG processes
2. **Start embedding** - Launch llama.cpp server
3. **Wait for health** - Embedding responds to `/health`
4. **Start agent** - Launch agent service
5. **Wait for socket** - Agent creates Unix socket
6. **Start gateway** - Launch gateway service
7. **Wait for health** - Gateway responds to `/health`
8. **Exit** - Process manager exits (services run in background)

### Shutdown Sequence

```bash
./bin/ocg stop
```

Uses escalating signals:

```
SIGTERM (3s) → SIGINT (3s) → SIGKILL
```

Order:
1. **Stop gateway** - Graceful shutdown first
2. **Stop agent** - Then agent
3. **Stop embedding** - Finally embedding

---

## Process Files

### PID Files

Stored in `/tmp/ocg/` (or custom via `--pid-dir`):

```
/tmp/ocg/
├── ocg-embedding.pid
├── ocg-agent.pid
└── ocg-gateway.pid
```

### Checking Status

```bash
./bin/ocg status

# Output:
# Embedding: running (PID 1234)
# Agent: running (PID 1235)
# Gateway: running (PID 1236)
# Health: ok
```

---

## Process Options

```bash
./bin/ocg start [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--config` | `./env.config` | Config file path |
| `--pid-dir` | `/tmp/ocg` | PID file directory |

```bash
./bin/ocg stop [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--pid-dir` | `/tmp/ocg` | PID file directory |

---

## Manual Process Control

```bash
# Start embedding
./bin/ocg-embedding &

# Start agent (after embedding is ready)
./bin/ocg-agent &

# Start gateway (after agent socket exists)
./bin/ocg-gateway &

# Check processes
ps aux | grep ocg
```

---

## Troubleshooting

### Process Won't Start

```bash
# Check logs
cat /tmp/ocg-embedding.log
cat /tmp/ocg-agent.log
cat /tmp/ocg-gateway.log

# Check ports
lsof -i :55003
lsof -i :50000
```

### Socket Not Created

```bash
# Check agent logs
tail -f /tmp/ocg-agent.log

# Verify embedding is running
curl http://127.0.0.1:50000/health
```

---

## See Also

- [CLI Overview](../09-cli/overview.md)
- [Process Management](../09-cli/process.md)
