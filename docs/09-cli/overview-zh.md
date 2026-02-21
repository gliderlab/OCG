# CLI 概览

OCG 命令行界面参考。

---

## 命令

### 进程管理

```bash
./bin/ocg start     # 启动所有服务
./bin/ocg stop      # 停止所有服务
./bin/ocg restart   # 重启服务
./bin/ocg status    # 检查状态
./bin/ocg version   # 显示版本
```

### 交互式聊天

```bash
./bin/ocg agent     # 启动交互式聊天
```

### 任务

```bash
./bin/ocg task create "描述"  # 创建任务
./bin/ocg task list                  # 列出任务
./bin/ocg task status <id>           # 任务状态
./bin/ocg task retry <id>            # 重试任务
```

### 速率限制

```bash
./bin/ocg ratelimit set --channel telegram --max 30 --window 60
./bin/ocg ratelimit list
./bin/ocg ratelimit check --channel telegram
```

### LLM 健康

```bash
ocg llmhealth --action status        # 检查健康
ocg llmhealth --action failover      # 手动故障转移
ocg llmhealth --action events        # 查看事件
```

### Gateway 管理

```bash
./bin/ocg gateway restart           # 重启 gateway
./bin/ocg gateway config.get        # 获取配置
./bin/ocg gateway config.patch      # 补丁配置
./bin/ocg gateway update.run        # 运行更新
```

---

## 选项

```bash
./bin/ocg start --config ./env.config --pid-dir /tmp/ocg
./bin/ocg stop --pid-dir /tmp/ocg
```

---

## 相关文档

- [进程管理](process-zh.md)
- [交互式聊天](chat-zh.md)
- [任务管理](tasks-zh.md)
