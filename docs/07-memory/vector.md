# Vector Memory

HNSW-based semantic memory storage.

---

## Overview

Vector memory uses HNSW (Hierarchical Navigable Small World) index for fast similarity search.

---

## Configuration

```bash
export OCG_VECTOR_PROVIDER="hnsw"
export OCG_VECTOR_INDEX="/opt/openclaw-go/vector.index"
export EMBEDDING_MODEL_PATH="/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf"
```

---

## Hybrid Search

OCG supports hybrid search combining FAISS vector similarity with SQLite FTS5 full-text search.

### Configuration

```bash
export HYBRID_SEARCH_ENABLED=true
export VECTOR_WEIGHT=0.7
export TEXT_WEIGHT=0.3
```

- **Vector Weight**: Importance of semantic similarity (default 0.7)
- **Text Weight**: Importance of keyword matching (default 0.3)

---

## Components

### Embedding Service

- Generates text embeddings
- Runs llama.cpp server
- Default port: 50000

### HNSW Index

- Fast approximate nearest neighbor search
- Configurable parameters
- Persistent storage

---

## Usage

### Store Memory

```bash
memory_store(content="Important information here")
```

### Search Memory

```bash
memory_search(query="What was discussed about X?", maxResults=5)
```

### Manual Index

```bash
# Rebuild index from stored memories
memory_search(query="rebuild-index")
```

---

## Performance

### Index Size

- Depends on number of memories
- Typical: 1-10MB per 1000 memories

### Search Speed

- <10ms for 10,000 memories
- Sub-linear scaling

---

## See Also

- [Memory Overview](../overview.md)
- [Session Memory](sessions.md)
