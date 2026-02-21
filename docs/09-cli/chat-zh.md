# 交互式聊天

通过命令行界面使用 OCG。

---

## 启动聊天

```bash
./bin/ocg agent
```

启动交互式聊天会话。

---

## 命令

| 命令 | 描述 |
|------|------|
| `//quit` | 退出聊天 |
| `//help` | 显示帮助 |
| `//new` | 新对话 |
| `//reset` | 重置上下文 |

---

## 使用方法

```
$ ./bin/ocg agent
OCG Chat Agent
Type //quit to exit, //help for commands.

你: 你好，最近怎么样？
OCG: 我很好，谢谢！今天有什么可以帮你的吗？
你: 解释一下向量记忆
OCG: 向量记忆使用...
你: //quit
再见！
```

---

## 选项

```bash
./bin/ocg agent --model gpt-4o
./bin/ocg agent --system "你是一个有用的助手"
```

---

## 会话行为

- 维护对话上下文
- 支持所有 OCG 工具
- 与所有配置的通道配合工作

---

## 相关文档

- [CLI 概览](overview-zh.md)
