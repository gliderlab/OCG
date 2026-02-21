# Installation Guide

**OCG-Go (OCG)** - Fast, lightweight AI agent system in Go.

---

## Prerequisites

### System Requirements

- **OS**: Linux (Ubuntu/Debian recommended), macOS, Windows (WSL2)
- **Go**: 1.22 or higher
- **GCC**: Required for SQLite CGO bindings
- **Memory**: Minimum 512MB RAM (1GB+ recommended)
- **Storage**: ~500MB for binary + models

### Linux Dependencies

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y \
    golang-go \
    gcc \
    libgomp1 \
    libblas3 \
    liblapack3 \
    libopenblas0 \
    libgfortran5 \
    git \
    make

# Arch Linux
sudo pacman -S go gcc openblas lapack git make

# Fedora/RHEL
sudo dnf install -y golang gcc openblas lapack git make
```

### macOS Dependencies

```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install Go (if not using Homebrew)
brew install go
```

---

## Installation Methods

### Method 1: Build from Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/gliderlab/OCG.git
cd cogate

# Build all components
make build-all

# Or build specific components
make build-gateway
make build-agent
make build-embedding
```

**Build outputs:**
- `bin/ocg` - Main CLI and process manager
- `bin/ocg-gateway` - Gateway service (HTTP/WebSocket)
- `bin/ocg-agent` - Agent service (RPC)
- `bin/ocg-embedding` - Embedding service
- `bin/llama-server` - llama.cpp server for embeddings

### Method 2: Pre-built Binary

Check [Releases](https://github.com/gliderlab/OCG/releases) for pre-built binaries.

```bash
# Download latest release
wget https://github.com/gliderlab/OCG/releases/latest/download/ocg-linux-amd64.tar.gz
tar -xzf ocg-linux-amd64.tar.gz
sudo mv ocg /usr/local/bin/
```

---

## Verification

```bash
# Check version
./bin/ocg version

# Check system status
./bin/ocg status

# Health check (after start)
curl http://localhost:55003/health
```

---

## Next Steps

- [First Steps](first-steps.md) - Get started with OCG
- [Environment Setup](environment.md) - Configure environment variables

---

## Troubleshooting

### CGO Build Errors

```bash
# Install missing libraries
sudo apt-get install -y libgomp1 libblas3 liblapack3
```

### Port Already in Use

```bash
# Check what's using the port
lsof -i :55003

# Kill the process
kill -9 <PID>
```

### Permission Denied

```bash
# Make binary executable
chmod +x bin/ocg
```
