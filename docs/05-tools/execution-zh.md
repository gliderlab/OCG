# 执行工具

执行 shell 命令和管理进程。

---

## exec

执行 shell 命令。

### 使用

```bash
exec(command="ls -la", timeout=30)
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `command` | string | 要执行的 shell 命令 |
| `timeout` | int | 超时秒数 (可选) |
| `workdir` | string | 工作目录 (可选) |
| `env` | object | 环境变量 (可选) |
| `pty` | bool | 在伪终端中运行 (可选) |
| `elevated` | bool | 使用提升权限运行 (可选) |

### 示例

```bash
exec(command="ls -la /opt/openclaw-go")
exec(command="git status", workdir="/opt/openclaw-go")
exec(command="top -bn1", timeout=10)
```

### 返回

```json
{
  "stdout": "命令输出",
  "stderr": "错误输出",
  "exit_code": 0
}
```

---

## process

管理后台进程。

### 列出进程

```bash
process(action="list")
```

### 执行命令

```bash
process(action="exec", command="ls -la", sessionId="mysession")
```

### 发送按键

```bash
process(action="send-keys", sessionId="mysession", keys=["enter"])
```

### 终止进程

```bash
process(action="kill", sessionId="mysession")
```

### 获取日志

```bash
process(action="log", sessionId="mysession", offset=0, limit=100)
```

---

## 工作目录

命令在受限目录中运行：

```bash
# 默认: bin/work
/opt/openclaw-go/bin/work/
```

### 覆盖

```bash
exec(command="ls", workdir="/tmp")
```

---

## 安全性

### 危险命令

类似 `rm`、`sudo`、`kill` 的命令需要确认：

```bash
# 无确认 (会提示)
exec(command="rm -rf /tmp/test")

# 明确确认
exec(command="rm -rf /tmp/test", ask="confirm")
```

### Shell 功能

默认禁用：

```
✓ 简单命令: ls, cat, echo
✗ 管道: cat file | grep pattern
✗ 重定向: cat > file
✗ 通配符: cat *.txt
✗ 变量展开: $HOME
```

如果需要 shell 功能：

```bash
exec(command="cat file | grep pattern", ask="on-miss")
```

---

## 安全模式

### deny (默认)

仅允许匹配模式的命令：

```bash
exec(command="ls", security="allowlist")
```

### allowlist

允许指定命令：

```json
{
  "security": "allowlist",
  "allowed_commands": ["ls", "cat", "echo", "git"]
}
```

### full

允许任何命令 (危险！)：

```json
{
  "security": "full"
}
```

---

## 后台进程

### 启动进程

```bash
process(action="exec", command="python -m http.server 8080", background=true)
```

### 管理会话

```bash
# 列出所有会话
process(action="list")

# 获取进程输出
process(action="log", sessionId="abc123")
```

---

## 相关文档

- [工具概览](overview-zh.md)
- [文件系统工具](filesystem-zh.md)
