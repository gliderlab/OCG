# Port Configuration

OCG port allocation and configuration.

---

## Default Ports

| Service | Port | Range | Protocol |
|---------|------|-------|----------|
| **Gateway** | 55003 | - | HTTP/WebSocket |
| **Embedding** | 50000 | 50000-60000 | HTTP |
| **Llama.cpp** | 18000 | 18000-19000 | HTTP |

---

## Port Ranges

### Embedding Service

**Default:** 50000  
**Range:** 50000-60000

Used for embedding generation and vector search.

```bash
# Configure specific port
export EMBEDDING_PORT=50000

# Or via config
{
  "embedding": {
    "port": 50000
  }
}
```

### Llama.cpp Server

**Default:** 18000  
**Range:** 18000-19000

Used for local LLM inference.

```bash
# Configure port
export LLAMA_PORT=18000
```

### Gateway Port

**Default:** 55003

```bash
# Configure via environment variable
export OCG_PORT=55003

# Or via config
{
  "gateway": {
    "port": 55003
  }
}
```

---

## Configuration Functions

OCG uses centralized port configuration:

```go
// pkg/config/defaults.go

const (
    DefaultGatewayPort       = 55003
    DefaultEmbeddingPortMin  = 50000
    DefaultEmbeddingPortMax  = 60000
    DefaultLlamaPortMin      = 18000
    DefaultLlamaPortMax      = 19000
    DefaultCDPPort           = 18800
)

func DefaultEmbeddingPort() int {
    return DefaultEmbeddingPortMin  // 50000
}
```

---

## Dynamic Port Allocation

When default port is in use, OCG tries next available port:

```bash
# If 50000 is busy, tries 50001, 50002, etc.
```

Check used ports:

```bash
# Linux
netstat -tuln | grep -E ':(55003|50000|18000)'

# macOS
lsof -i -P -n | grep -E ':(55003|50000|18000)'
```

---

## Port Security

### Localhost Only

All services bind to `127.0.0.1` by default for security.

```bash
# Default - localhost only
http://127.0.0.1:55003

# Expose to network (not recommended)
export OCG_HOST="0.0.0.0"
```

### Firewall

If exposing ports, use firewall:

```bash
# UFW (Ubuntu)
sudo ufw allow 55003/tcp

# iptables
sudo iptables -A INPUT -p tcp --dport 55003 -j ACCEPT
```

---

## Configuration Priority

1. Command line arguments
2. Environment variables
3. Configuration file
4. Default values

---

## See Also

- [Configuration Guide](guide.md)
- [Environment Variables](env-vars.md)
