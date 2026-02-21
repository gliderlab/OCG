# 进程管理

启动、停止和管理 OCG 进程。

---

## 启动服务

```bash
./bin/ocg start
```

按顺序启动: embedding → agent → gateway

### 选项

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--config` | `./env.config` | 配置文件路径 |
| `--pid-dir` | `/tmp/ocg` | PID 文件目录 |

---

## 停止服务

```bash
./bin/ocg stop
```

使用升级信号进行优雅关闭：

```
SIGTERM (3s) → SIGINT (3s) → SIGKILL
```

### 选项

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--pid-dir` | `/tmp/ocg` | PID 文件目录 |
| `--force` | false | 立即 SIGKILL |

---

## 重启

```bash
./bin/ocg restart
```

等同于 stop + start。

---

## 状态

```bash
./bin/ocg status
```

输出:

```
Embedding: running (PID 1234)
Agent: running (PID 1235)
Gateway: running (PID 1236)
Health: ok
```

---

## 手动进程控制

```bash
# 启动单个服务
./bin/ocg-embedding &
./bin/ocg-agent &
./bin/ocg-gateway &

# 检查进程
ps aux | grep ocg
```

---

## 相关文档

- [CLI 概览](overview-zh.md)
