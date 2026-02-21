# 记忆系统概览

OCG 用于持久化上下文的记忆架构。

---

## 记忆类型

### 短期记忆

- 当前对话上下文
- 自动管理
- 基于 token 的裁剪

### 长期记忆

- 基于向量的语义记忆
- 跨会话持久化
- 通过嵌入搜索

---

## 存储位置

```
/root/.openclaw/workspace/
├── MEMORY.md              # 精选的长期记忆
└── memory/
    ├── 2026-02-18.md      # 每日笔记
    ├── 2026-02-17.md
    └── ...
```

---

## 功能

### 自动召回

自动检索相关记忆：

```bash
export AUTO_RECALL=true
export RECALL_THRESHOLD=0.72
```

### 语义搜索

使用向量嵌入进行相似性匹配：

```bash
memory_search(query="项目配置")
```

### 压缩

压缩旧对话以节省上下文：

```bash
# 手动触发
/compact

# 接近限制时自动
/context-overflow
```

---

## 相关文档

- [向量记忆](vector-zh.md)
- [会话记忆](sessions-zh.md)
- [记忆工具](../../05-tools/memory-zh.md)
