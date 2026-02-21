# KV 引擎 (BadgerDB)

嵌入式高性能键值存储。

---

## 概述

BadgerDB 为 OCG 提供快速、嵌入式的 KV 存储。

### 用例

- 任务状态缓存
- Token 追踪
- 会话元数据
- 速率限制计数器

---

## 配置

```bash
# 内存模式 (默认)
export OCG_KV_DIR=""

# 持久化模式
export OCG_KV_DIR="/opt/openclaw-go/kv"

# 启用 TTL
export OCG_KV_TTL_ENABLED=true
```

或通过配置：

```json
{
  "kv": {
    "enabled": true,
    "dir": "/opt/openclaw-go/kv",
    "ttl_enabled": true,
    "ttl": 86400  // 24 小时
  }
}
```

---

## 功能

### 内存模式

- 最快的性能
- 重启后数据丢失
- 不使用磁盘空间

### 持久化模式

- 数据在重启后保留
- 使用磁盘存储
- 稍慢

### TTL 支持

- 自动键过期
- 可按密钥或全局配置
- 适用于缓存

---

## 使用

```bash
# 检查 KV 状态
ocg kv status

# 查看密钥
ocg kv list

# 清除过期数据
ocg kv clean
```

---

## 相关文档

- [高级功能概览](overview-zh.md)
