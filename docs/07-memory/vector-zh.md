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
export EMBEDDING_MODEL_PATH="/app/models/nomic-embed-text-v1.5.f16.gguf"
```

### 推荐模型 (Embedding Models)

OCG Embedding 服务需要 GGUF 格式的向量模型。以下是推荐的轻量级模型：

1. **Nomic Embed Text v1.5 (推荐)**
   - **特点**: 对中文支持极佳，长文本处理能力强（8k context），体积适中。
   - **下载地址**: [Hugging Face - nomic-ai/nomic-embed-text-v1.5-GGUF](https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.f16.gguf?download=true)
   - **文件**: `nomic-embed-text-v1.5.f16.gguf` (约 260MB)

2. **Gemma 300M Embedding**
   - **特点**: 极致轻量，适合资源受限环境。
   - **下载地址**: [Hugging Face - mradermaker/embedding-gemma-300M-Q8_0](https://huggingface.co/mradermaker/embedding-gemma-300M-Q8_0/resolve/main/embeddinggemma-300M-Q8_0.gguf?download=true)
   - **文件**: `embeddinggemma-300M-Q8_0.gguf` (约 300MB)

### 安装模型

在容器环境或本地部署时，请确保将模型放入 `models/` 目录：

```bash
mkdir -p models
wget -O models/nomic-embed-text-v1.5.f16.gguf https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.f16.gguf?download=true
```

---

## 混合检索 (Hybrid Search)

OCG 支持结合 FAISS 向量相似度与 SQLite FTS5 全文检索的混合检索模式。

### 配置

```bash
export HYBRID_SEARCH_ENABLED=true
export VECTOR_WEIGHT=0.7
export TEXT_WEIGHT=0.3
```

- **Vector Weight**: 语义相似度的权重（默认 0.7）
- **Text Weight**: 关键词匹配的权重（默认 0.3）

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
