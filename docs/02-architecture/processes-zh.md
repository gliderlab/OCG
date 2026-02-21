# 进程模型

OCG 进程架构和生命周期管理。

---

## 多进程架构

OCG 运行 3 个进程：

| 进程 | 类型 | 连接 | 端口 |
|------|------|------|------|
| **ocg-embedding** | 服务 | HTTP | 50000-60000 |
| **ocg-agent** | 服务 | Unix Socket | N/A |
| **ocg-gateway** | 服务 + CLI | HTTP | 55003 |

### Embedding 服务

负责：
- 运行 llama.cpp 服务器
- 生成文本 embedding
- 向量相似性搜索

**健康检查:**
```bash
curl http://127.0.0.1:50000/health
```

### Agent 服务

核心 AI 逻辑：
- 消息处理
- 工具执行
- 上下文管理
- 记忆操作

**Socket 路径:** `/tmp/ocg-agent.sock`

### Gateway 服务

主要入口点：
- Web UI 服务
- API 端点
- WebSocket 连接
- 通道集成

---

## 生命周期管理

### 启动顺序

```bash
./bin/ocg start
```

1. **终止现有** - 停止任何运行的 OCG 进程
2. **启动 embedding** - 启动 llama.cpp 服务器
3. **等待健康** - Embedding 响应 `/health`
4. **启动 agent** - 启动 agent 服务
5. **等待 socket** - Agent 创建 Unix socket
6. **启动 gateway** - 启动 gateway 服务
7. **等待健康** - Gateway 响应 `/health`
8. **退出** - 进程管理器退出 (服务在后台运行)

### 关闭顺序

```bash
./bin/ocg stop
```

使用升级信号：

```
SIGTERM (3s) → SIGINT (3s) → SIGKILL
```

顺序:
1. **停止 gateway** - 首先优雅关闭
2. **停止 agent** - 然后是 agent
3. **停止 embedding** - 最后是 embedding

---

## 进程文件

### PID 文件

存储在 `/tmp/ocg/` (或通过 `--pid-dir` 自定义):

```
/tmp/ocg/
├── ocg-embedding.pid
├── ocg-agent.pid
└── ocg-gateway.pid
```

### 检查状态

```bash
./bin/ocg status

# 输出:
# Embedding: running (PID 1234)
# Agent: running (PID 1235)
# Gateway: running (PID 1236)
# Health: ok
```

---

## 进程选项

```bash
./bin/ocg start [选项]
```

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--config` | `./env.config` | 配置文件路径 |
| `--pid-dir` | `/tmp/ocg` | PID 文件目录 |

```bash
./bin/ocg stop [选项]
```

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--pid-dir` | `/tmp/ocg` | PID 文件目录 |

---

## 手动进程控制

```bash
# 启动 embedding
./bin/ocg-embedding &

# 启动 agent (在 embedding 就绪后)
./bin/ocg-agent &

# 启动 gateway (在 agent socket 存在后)
./bin/ocg-gateway &

# 检查进程
ps aux | grep ocg
```

---

## 故障排除

### 进程无法启动

```bash
# 检查日志
cat /tmp/ocg-embedding.log
cat /tmp/ocg-agent.log
cat /tmp/ocg-gateway.log

# 检查端口
lsof -i :55003
lsof -i :50000
```

### Socket 未创建

```bash
# 检查 agent 日志
tail -f /tmp/ocg-agent.log

# 验证 embedding 是否运行
curl http://127.0.0.1:50000/health
```

---

## 相关文档

- [CLI 概览](../09-cli/overview-zh.md)
- [进程管理](../09-cli/process-zh.md)
