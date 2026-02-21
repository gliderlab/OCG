# 工具概览

OCG 提供 17+ 个内置工具用于 AI Agent 操作。

---

## 工具类别

### 文件系统
| 工具 | 描述 |
|------|------|
| `read` | 读取文件内容 |
| `write` | 创建/覆盖文件 |
| `edit` | 编辑文件内容 |
| `apply_patch` | 多文件结构化补丁 |

### 执行
| 工具 | 描述 |
|------|------|
| `exec` | 执行 shell 命令 |
| `process` | 后台进程管理 |

### Web
| 工具 | 描述 |
|------|------|
| `web_search` | AI 优化的网络搜索 |
| `web_fetch` | 获取 URL 内容 |

### 浏览器
| 工具 | 描述 |
|------|------|
| `browser` | 基于 CDP 的浏览器自动化 |

### UI
| 工具 | 描述 |
|------|------|
| `canvas` | 节点画布控制 |
| `nodes` | 配对节点管理 |

### 记忆
| 工具 | 描述 |
|------|------|
| `memory_search` | 语义记忆搜索 |
| `memory_get` | 读取记忆片段 |
| `memory_store` | 存储记忆 |

### 会话
| 工具 | 描述 |
|------|------|
| `sessions_list` | 列出会话 |
| `sessions_history` | 获取会话历史 |
| `sessions_send` | 发送到另一个会话 |
| `sessions_spawn` | 派生子 Agent |
| `session_status` | 会话状态 |
| `agents_list` | 列出可用 Agent |

### 自动化
| 工具 | 描述 |
|------|------|
| `cron` | 调度任务 |
| `message` | 发送消息 |
| `image` | 分析图像 |

---

## 工具使用

### 读取文件

```bash
read(path="/path/to/file.md")
```

### 写入文件

```bash
write(path="/path/to/file.md", content="file content")
```

### 编辑文件

```bash
edit(path="/path/to/file.md", oldText="old content", newText="new content")
```

### 执行命令

```bash
exec(command="ls -la", timeout=30)
```

### 网络搜索

```bash
web_search(query="OCG 文档")
```

### 获取网页

```bash
web_fetch(url="https://github.com/gliderlab/OCG")
```

---

## 工具执行流程

```
Agent 决定使用工具
        ↓
生成工具调用参数
        ↓
通过 Gateway 执行工具
        ↓
将结果返回给 LLM
        ↓
LLM 生成响应
```

---

## 工具限制

### 文件系统

- 工作目录: `bin/work`
- 读取大小限制: 50KB (可编辑文件)
- 写入操作: 创建、覆盖、编辑

### 执行

- Shell 功能默认禁用
- 危险命令 (rm, sudo, kill) 需要确认
- 工作目录: `bin/work`

### Web

- 超时: 15-30 秒
- 主体限制: 2-5MB

### 浏览器

- 需要 CDP 连接
- 限于 Chrome DevTools Protocol

---

## 相关文档

- [文件系统工具](filesystem-zh.md)
- [执行工具](execution-zh.md)
- [浏览器工具](browser-zh.md)
- [Web 工具](web-zh.md)
- [记忆工具](memory-zh.md)
