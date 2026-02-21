# CLI Overview

OCG command-line interface reference.

---

## Commands

### Process Management

```bash
./bin/ocg start     # Start all services
./bin/ocg stop      # Stop all services
./bin/ocg restart   # Restart services
./bin/ocg status    # Check status
./bin/ocg version   # Show version
```

### Interactive Chat

```bash
./bin/ocg agent     # Start interactive chat
```

### Tasks

```bash
./bin/ocg task create "description"  # Create task
./bin/ocg task list                  # List tasks
./bin/ocg task status <id>           # Task status
./bin/ocg task retry <id>            # Retry task
```

### Rate Limits

```bash
./bin/ocg ratelimit set --channel telegram --max 30 --window 60
./bin/ocg ratelimit list
./bin/ocg ratelimit check --channel telegram
```

### LLM Health

```bash
ocg llmhealth --action status        # Check health
ocg llmhealth --action failover      # Manual failover
ocg llmhealth --action events        # View events
```

### Gateway Management

```bash
./bin/ocg gateway restart           # Restart gateway
./bin/ocg gateway config.get        # Get config
./bin/ocg gateway config.patch      # Patch config
./bin/ocg gateway update.run        # Run updates
```

---

## Options

```bash
./bin/ocg start --config ./env.config --pid-dir /tmp/ocg
./bin/ocg stop --pid-dir /tmp/ocg
```

---

## See Also

- [Process Management](process.md)
- [Interactive Chat](chat.md)
- [Task Management](tasks.md)
