# 通信机制

OCG 组件之间的通信机制。

---

## 通信模式

### 1. Agent ↔ Gateway (Unix Socket)

**路径:** `/tmp/ocg-agent.sock`

OCG 使用基于 Unix socket 的 gRPC 进行 agent-gateway 通信。

```go
// Agent 端
listener, _ := net.Listen("unix", "/tmp/ocg-agent.sock")
grpcServer := grpc.NewServer()
pb.RegisterAgentServer(grpcServer, &agentServer{})
grpcServer.Serve(listener)
```

**协议:** 使用 Protocol Buffers 的 gRPC

**消息:**
- `ChatRequest` / `ChatResponse` - 聊天消息
- `ToolCallRequest` / `ToolCallResponse` - 工具执行
- `MemoryRequest` / `MemoryResponse` - 记忆操作

### 2. Embedding ↔ Agent (HTTP)

**端点:** `http://127.0.0.1:50000`

Agent 通过 HTTP 调用 embedding 服务：
- 文本 embedding 生成
- 向量相似性搜索

```bash
# 生成 embedding
curl -X POST http://127.0.0.1:50000/embed \
  -H "Content-Type: application/json" \
  -d '{"text": "hello world"}'

# 搜索相似内容
curl -X POST http://127.0.0.1:50000/search \
  -H "Content-Type: application/json" \
  -d '{"query": "hello", "top_k": 5}'
```

### 3. Gateway ↔ 客户端 (HTTP/WebSocket)

**HTTP 端口:** 55003

**WebSocket 端点:** `/v1/chat/stream`

```javascript
// WebSocket 连接
const ws = new WebSocket('ws://localhost:55003/v1/chat/stream');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.content);
};
```

**REST 端点:**
- `POST /v1/chat/completions` - 聊天 API (OpenAI 兼容)
- `GET /health` - 健康检查
- `GET /storage/stats` - 存储统计

---

## 消息流程

### 聊天请求流程

```
客户端 → Gateway (HTTP/WS)
        ↓
Gateway → Agent (Unix Socket)
        ↓
Agent → LLM Provider (HTTP)
        ↓
LLM → Agent
        ↓
Agent → Gateway (Socket)
        ↓
Gateway → 客户端 (HTTP/WS)
```

### 记忆召回流程

```
Agent → Embedding Service (HTTP)
        ↓
Embedding → 向量索引
        ↓
Embedding → Agent (相似度分数)
        ↓
Agent → Context (添加相关记忆)
```

---

## 通道通信

每个通道有自己的通信模式：

| 通道 | 协议 | 认证 |
|------|------|------|
| Telegram | Long Polling / Webhook | Bot Token |
| Discord | Gateway (WSS) + REST | Bot Token |
| Slack | Events API + RTM | Bot Token |
| WhatsApp | Cloud API | Access Token |
| IRC | Raw TCP | 无 |
| Signal | P2P 加密 | 电话号码 |

### Telegram 示例

```bash
# Long Polling (默认 - 无需公网 URL)
export TELEGRAM_MODE="long_polling"

# Webhook (需要公网 URL)
export TELEGRAM_WEBHOOK_URL="https://your-domain.com/webhooks/telegram"
export TELEGRAM_MODE="webhook"
```

---

## 安全性

### 身份验证

**UI Token:**
- Web UI 访问必需
- 通过 `OCG_UI_TOKEN` 环境变量设置
- API 调用时通过 `X-OCG-UI-Token` 头传递

**gRPC:**
- Unix socket 有文件系统权限
- 默认: 任何人都可以连接 (仅 localhost)
- 通过 socket 权限配置

### 速率限制

```bash
# 设置速率限制
./bin/ocg ratelimit set --channel telegram --max 30 --window 60
```

---

## 相关文档

- [通道概览](04-channels/overview-zh.md)
- [API 文档](03-configuration/api-zh.md)
- [端口配置](03-configuration/ports-zh.md)
