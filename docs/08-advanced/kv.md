# KV Engine (BadgerDB)

Embedded high-performance key-value storage.

---

## Overview

BadgerDB provides fast, embedded KV storage for OCG.

### Use Cases

- Task state caching
- Tokens tracking
- Session metadata
- Rate limiting counters

---

## Configuration

```bash
# Memory-only mode (default)
export OCG_KV_DIR=""

# Persistent mode
export OCG_KV_DIR="/opt/openclaw-go/kv"

# Enable TTL
export OCG_KV_TTL_ENABLED=true
```

Or via config:

```json
{
  "kv": {
    "enabled": true,
    "dir": "/opt/openclaw-go/kv",
    "ttl_enabled": true,
    "ttl": 86400  // 24 hours
  }
}
```

---

## Features

### Memory Mode

- Fastest performance
- Data lost on restart
- No disk space usage

### Persistent Mode

- Data survives restart
- Uses disk storage
- Slightly slower

### TTL Support

- Automatic key expiration
- Configurable per-key or global
- Useful for caching

---

## Usage

```bash
# Check KV status
ocg kv status

# View keys
ocg kv list

# Clear expired
ocg kv clean
```

---

## See Also

- [Advanced Overview](../overview.md)
