# 向量记忆

基于 HNSW 的语义记忆存储。

---

## 概述

向量记忆使用 HNSW (Hierarchical Navigable Small World) 索引进行快速相似性搜索。

---

## 配置

```bash
export OCG_VECTOR_PROVIDER="hnsw"
export OCG_VECTOR_INDEX="/opt/openclaw-go/vector.index"
export EMBEDDING_MODEL_PATH="/opt/openclaw-go/models/embeddinggemma-300M-Q8_0.gguf"
```

---

## 组件

### Embedding 服务

- 生成文本嵌入
- 运行 llama.cpp 服务器
- 默认端口: 50000

### HNSW 索引

- 快速近似最近邻搜索
- 可配置参数
- 持久化存储

---

## 使用

### 存储记忆

```bash
memory_store(content="重要信息")
```

### 搜索记忆

```bash
memory_search(query="关于 X 的讨论内容", maxResults=5)
```

### 手动索引

```bash
# 从存储的记忆重建索引
memory_search(query="rebuild-index")
```

---

## 性能

### 索引大小

- 取决于记忆数量
- 典型: 每 1000 条记忆 1-10MB

### 搜索速度

- 10,000 条记忆 <10ms
- 次线性扩展

---

## 相关文档

- [记忆概览](overview-zh.md)
- [会话记忆](sessions-zh.md)
