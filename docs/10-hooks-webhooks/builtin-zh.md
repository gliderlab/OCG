# 内置 Hooks

OCG 附带预配置的 Hooks。

---

## session-memory

将会话保存到记忆文件。

```bash
./bin/ocg hooks enable session-memory
```

**触发:** `message.receive`  
**操作:** 保存对话到 `memory/YYYY-MM-DD.md`

---

## command-logger

记录执行的命令。

```bash
./bin/ocg hooks enable command-logger
```

**触发:** `exec` 工具调用  
**操作:** 将命令记录到文件

---

## boot-md

启动时运行 BOOT.md。

```bash
./bin/ocg hooks enable boot-md
```

**触发:** `agent.start`  
**操作:** 执行 BOOT.md 内容

---

## bootstrap-extra-files

注入额外的 bootstrap 文件。

```bash
./bin/ocg hooks enable bootstrap-extra-files
```

**触发:** `agent.start`  
**操作:** 加载额外的 bootstrap 文件

---

## 所有内置 Hooks

| Hook | 触发 | 描述 |
|------|------|------|
| session-memory | message.receive | 保存对话 |
| command-logger | exec | 记录命令 |
| boot-md | agent.start | 运行 BOOT.md |
| bootstrap-extra-files | agent.start | 加载 bootstrap 文件 |

---

## 相关文档

- [Hooks 概览](hooks-zh.md)
