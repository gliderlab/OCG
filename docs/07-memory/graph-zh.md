# 知识图谱 (Knowledge Graph)

结构化的实体-关系记忆存储。

---

## 概述

知识图谱 (KG) 允许以结构化的实体和关系形式存储及检索信息。它通过提供准确的事实和概念间的连接，对向量记忆形成了有力补充。

---

## 架构

- **实体 (Entities)**: 图中的节点（例如："OpenClaw-Go"、"Jacker"、"SQLite"）。
- **关系 (Relations)**: 连接实体的边（例如："Jacker" -> "works_on" -> "OpenClaw-Go"）。
- **存储**: 基于 SQLite 的 `memory_entities` 和 `memory_relations` 表。

---

## 工具支持

知识图谱通过 `memory_graph` 工具进行管理。

### 支持的动作 (Actions)

- `add_entity`: 创建或更新一个具有名称、类型和描述的节点。
- `add_relation`: 在两个现有节点之间创建带权重的边。
- `get_entity`: 检索特定节点的详细信息。
- `search_relations`: 查找与特定节点相关的所有连接。

---

## 使用示例

### 添加实体

```bash
memory_graph(action="add_entity", name="OCG", entity_type="Project", description="OpenClaw 的 Go 语言重写版")
```

### 添加关系

```bash
memory_graph(action="add_relation", source="Jacker", target="OCG", relation="leads", weight=1.0)
```

### 搜索关系

```bash
memory_graph(action="search_relations", name="Jacker")
```

---

## 混合检索 (Hybrid Retrieval)

知识图谱可以与向量记忆结合使用，为 AI 代理提供更全面的上下文。

---

## 参见

- [向量记忆 (Vector Memory)](vector-zh.md)
- [记忆工具 (Memory Tools)](../../05-tools/memory-zh.md)
