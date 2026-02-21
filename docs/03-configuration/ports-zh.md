# 端口配置

OCG 端口分配和配置。

---

## 默认端口

| 服务 | 端口 | 范围 | 协议 |
|------|------|------|------|
| **Gateway** | 55003 | - | HTTP/WebSocket |
| **Embedding** | 50000 | 50000-60000 | HTTP |
| **Llama.cpp** | 18000 | 18000-19000 | HTTP |

---

## 端口范围

### Embedding 服务

**默认:** 50000  
**范围:** 50000-60000

用于 embedding 生成和向量搜索。

```bash
# 配置特定端口
export EMBEDDING_PORT=50000

# 或通过配置
{
  "embedding": {
    "port": 50000
  }
}
```

### Llama.cpp 服务器

**默认:** 18000  
**范围:** 18000-19000

用于本地 LLM 推理。

```bash
# 配置端口
export LLAMA_PORT=18000
```

### Gateway 端口

**默认:** 55003

```bash
# 配置
export OCG_PORT=55003

# 或通过配置
{
  "gateway": {
    "port": 55003
  }
}
```

---

## 配置函数

OCG 使用集中化端口配置：

```go
// pkg/config/defaults.go

const (
    DefaultGatewayPort       = 55003
    DefaultEmbeddingPortMin  = 50000
    DefaultEmbeddingPortMax  = 60000
    DefaultLlamaPortMin      = 18000
    DefaultLlamaPortMax      = 19000
    DefaultCDPPort           = 18800
)

func DefaultEmbeddingPort() int {
    return DefaultEmbeddingPortMin  // 50000
}
```

---

## 动态端口分配

当默认端口被占用时，OCG 尝试下一个可用端口：

```bash
# 如果 50000 被占用，尝试 50001、50002 等
```

检查已用端口：

```bash
# Linux
netstat -tuln | grep -E ':(55003|50000|18000)'

# macOS
lsof -i -P -n | grep -E ':(55003|50000|18000)'
```

---

## 端口安全

### 仅本地主机

默认情况下所有服务绑定到 `127.0.0.1` 以保证安全。

```bash
# 默认 - 仅 localhost
http://127.0.0.1:55003

# 暴露到网络 (不推荐)
export OCG_HOST="0.0.0.0"
```

### 防火墙

如果需要暴露端口，使用防火墙：

```bash
# UFW (Ubuntu)
sudo ufw allow 55003/tcp

# iptables
sudo iptables -A INPUT -p tcp --dport 55003 -j ACCEPT
```

---

## 配置优先级

1. 命令行参数
2. 环境变量
3. 配置文件
4. 默认值

---

## 相关文档

- [配置指南](guide-zh.md)
- [环境变量](env-vars-zh.md)
