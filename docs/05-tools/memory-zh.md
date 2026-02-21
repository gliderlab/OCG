# 记忆工具

搜索、读取和存储记忆。

---

## memory_search

使用向量嵌入进行语义记忆搜索。

### 使用

```bash
memory_search(query="项目任务", maxResults=5)
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `query` | string | 搜索查询 |
| `maxResults` | int | 最大结果数 (默认: 5) |
| `minScore` | float | 最小相似度分数 (0.0-1.0) |

### 示例

```bash
memory_search(query="OCG 配置")
memory_search(query="Telegram 设置", maxResults=10, minScore=0.7)
```

### 返回

```json
{
  "results": [
    {
      "path": "MEMORY.md",
      "startLine": 10,
      "endLine": 20,
      "score": 0.92,
      "snippet": "相关内容..."
    }
  ]
}
```

---

## memory_get

从 MEMORY.md 或 memory/*.md 读取记忆片段。

### 使用

```bash
memory_get(path="MEMORY.md")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `path` | string | 文件路径 |
| `from` | int | 起始行 (可选) |
| `lines` | int | 行数 (可选) |

### 示例

```bash
memory_get(path="MEMORY.md")
memory_get(path="memory/2026-02-18.md", from=1, lines=50)
```

### 注意

- 从 `/root/.openclaw/workspace/MEMORY.md` 或 `/root/.openclaw/workspace/memory/*.md` 读取
- 如指定则返回特定行
- 在 memory_search 后获取完整上下文

---

## memory_store

存储新记忆。

### 使用

```bash
memory_store(content="新记忆内容")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `content` | string | 要存储的记忆内容 |
| `tags` | array | 组织标签 (可选) |

### 示例

```bash
memory_store(content="用户偏好使用 Telegram 通道接收通知")
memory_store(content="生产环境配置已更新", tags=["config", "production"])
```

### 注意

- 存储到每日记忆文件: `memory/YYYY-MM-DD.md`
- 必要时自动创建 memory 目录
- 记忆被语义索引以供将来搜索

---

## 记忆文件

### 结构

```
/root/.openclaw/workspace/
├── MEMORY.md              # 长期精选记忆
└── memory/
    ├── 2026-02-18.md      # 每日笔记
    ├── 2026-02-17.md
    └── ...
```

### MEMORY.md

长期精选记忆：
- 项目决策
- 偏好和笔记
- 经验教训
- 重要上下文

### memory/YYYY-MM-DD.md

每日原始日志：
- 发生了什么
- 进行中的工作
- 任务跟踪
- 临时笔记

---

## 自动功能

### AUTO_RECALL

在 LLM 调用前自动召回相关记忆：

```bash
export AUTO_RECALL=true
export RECALL_THRESHOLD=0.72
```

### 记忆钩子

自动保存会话记忆：

```bash
# 内置钩子: session-memory
# 在每次对话后触发
```

---

## 相关文档

- [工具概览](overview-zh.md)
- [记忆系统概览](../../07-memory/overview-zh.md)
