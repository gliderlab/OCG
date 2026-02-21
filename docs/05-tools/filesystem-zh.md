# 文件系统工具

读取、写入和编辑文件。

---

## read

读取文件内容。

### 使用

```bash
read(path="/path/to/file")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `path` | string | 文件路径（相对或绝对） |
| `offset` | int | 起始行号 (1-indexed，可选) |
| `limit` | int | 最大行数 (可选) |

### 示例

```bash
read(path="/opt/openclaw-go/README.md")
read(path="/opt/openclaw-go/README.md", offset=1, limit=50)
```

### 返回

```
文件内容文本字符串。
图像文件作为附件发送。
```

### 限制

- 最大: 2000 行或 50KB (以先到者为准)
- 大文件使用 offset/limit

---

## write

创建或覆盖文件。

### 使用

```bash
write(path="/path/to/file", content="file content")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `path` | string | 文件路径 |
| `content` | string | 要写入的内容 |

### 示例

```bash
write(path="/tmp/test.md", content="# 你好世界\n\n这是一个测试。")
```

### 注意

- 必要时创建父目录
- 覆盖现有文件

---

## edit

通过替换精确文本来编辑文件。

### 使用

```bash
edit(path="/path/to/file", oldText="old content", newText="new content")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `path` | string | 文件路径 |
| `oldText` | string | 要查找的文本（必须完全匹配） |
| `newText` | string | 替换文本 |

### 示例

```bash
edit(path="/tmp/test.md", oldText="旧内容", newText="新内容")
```

### 注意

- oldText 必须完全匹配（包括空白）
- 只替换第一个匹配项
- 用于精确、细微的编辑

---

## apply_patch

多文件结构化补丁。

### 使用

```bash
apply_patch(patch=[...])
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `patch` | array | 补丁操作数组 |

### 补丁操作

```json
{
  "patch": [
    {
      "op": "add",
      "path": "/new/file.md",
      "content": "# 新文件"
    },
    {
      "op": "update",
      "path": "/existing/file.md",
      "oldText": "旧文本",
      "newText": "新文本"
    },
    {
      "op": "delete",
      "path": "/file/to/delete.md"
    },
    {
      "op": "move",
      "from": "/old/path.md",
      "to": "/new/path.md"
    }
  ]
}
```

### 操作

| 操作 | 描述 |
|------|------|
| `add` | 创建新文件 |
| `update` | 修改现有文件 |
| `delete` | 删除文件 |
| `move` | 移动/重命名文件 |

---

## 工作目录

所有文件操作限制在：

```bash
# Linux/macOS
/opt/openclaw-go/bin/work/

# Windows
%TEMP%\ocg-work\
```

此目录外的文件需要明确路径或确认。

---

## 安全性

### 读取操作

- 文件截断为 2000 行或 50KB
- 二进制文件可能无法正常显示

### 写入操作

- 自动创建父目录
- 不经警告直接覆盖

### 危险操作

- 系统目录中的文件需要确认
- `rm` 操作被记录

---

## 相关文档

- [工具概览](overview-zh.md)
- [执行工具](execution-zh.md)
