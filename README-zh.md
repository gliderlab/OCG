# [English](README.md) | [中文](README-zh.md)

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/SQLite-3.x-003B57?style=for-the-badge&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
</p>

---

# OCG (OpenClaw-Go) 🦀

> **下一代本地 AI Agent 系统**  
> *快速、私有、可扩展*  
> *OpenClaw 的 Go 重实现*

---

## 📖 简介

**OCG (OpenClaw-Go)** 是一个用 Go 重写的高性能、轻量级 AI Agent 系统，基于 OpenClaw 重构。它克服了原始 Node.js OpenClaw 的局限性，提供更优的性能、更低的资源消耗和更好的隐私保护。

OCG 完全本地运行，将**向量记忆**、**任务调度**和**多通道通信**集成到 4 个独立的二进制文件中。

---

## ✨ 为什么选择 OCG？

| 特性 | OCG | Node.js Agent |
|------|-----|---------------|
| **启动时间** | **< 100ms** ⚡ | 5-10秒 |
| **内存占用** | **~50 MB** 🪶 | 200-500 MB+ |
| **部署方式** | 4 二进制（All-in-One 或分布式） | 复杂依赖 |
| **存储** | SQLite + FAISS (本地) | Mongo/Postgres (远程) |
| **架构** | 多进程 RPC | 单体架构 |

---

## 🚀 核心功能

- ⚡ **极速启动** - 启动时间 <1s，资源占用低
- 🔒 **隐私优先** - 所有数据本地存储
- 🧠 **向量记忆** - HNSW 语义搜索
- 🔌 **通用网关** - WebSocket、HTTP REST、Telegram
- 🛠️ **17+ 工具** - 文件 I/O、Shell、进程、浏览器、Cron
- 💓 **Pulse 系统** - 心跳事件循环
- ⏰ **智能调度** - Cron 定时任务
- 🎣 **Hooks & Webhooks** - 事件驱动自动化
- 🌐 **13 个 LLM Provider** - OpenAI、Anthropic、Google、MiniMax、Ollama 等
- 📱 **Telegram Long Polling** - 无需公网 URL（默认）
- ✍️ **输入指示器** - AI 响应时显示"正在输入"
- 💭 **会话上下文** - 每个用户/通道独立对话历史
- 🧹 **严格增量压缩归档** - 水位线、去重、跳过摘要
- ✅ **任务 Marker 上下文策略** - 主聊天保留 `[task_done:task-...]`，详情存 DB
- 🎙️ **Google 原生音频 Realtime** - PCM 上行、WAV 输出、函数调用回调
- 🔄 **工具增强** - 循环检测、结果截断、thinking 模式
- 🛡️ **安全** - 默认绑定 127.0.0.1、Token 认证
- 🎤 **实时语音路由** - Telegram 语音 → 实时音频（无需转录）、语音+文字 → HTTP
- 📡 **实时会话管理** - 每会话 WebSocket、空闲清理、provider_type 跟踪
- 🔀 **模式切换** - 命令：`/live`、`/text`、`/voice`、`/audio`、`/http`
- ⏮️ **实时降级** - 实时错误时自动降级到 HTTP LLM
- 🔒 **并发安全** - 每会话互斥锁处理并发实时请求

---

## ⚡ 快速开始

### 前置要求

- Go 1.22+
- GCC（用于 SQLite/CGO）

```bash
# Linux 依赖
sudo apt-get install -y libgomp1 libblas3 liblapack3 libopenblas0 libgfortran5
```

### 构建

```bash
git clone https://github.com/gliderlab/OCG.git
cd OCG
make
```

OCG 由 4 个独立二进制组成：
- `ocg` - 主入口 CLI（轻量级）
- `ocg-gateway` - 网关服务（消息路由）
- `ocg-agent` - AI 核心（LLM + 工具）
- `ocg-embedding` - 向量服务

默认 All-in-One（全部服务在同一机器）。生产环境可分布式部署 Gateway/Agent/Embedding。

### 运行

```bash
# 配置环境
export OCG_UI_TOKEN="your-token"

# 启动服务
./bin/ocg start
```

### 访问

- **Web UI**: http://localhost:55003
- **API**: http://localhost:55003/v1/chat/completions

---

## 🏗️ 架构

```
┌─────────────────────────────────────────┐
│           Gateway (端口 55003)           │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │  Web UI │ │   WS    │ │ Channels │ │
│  └─────────┘ └─────────┘ └──────────┘ │
└────────────────┬───────────────────────┘
                 │ RPC (Unix Socket)
                 ▼
┌─────────────────────────────────────────┐
│           Agent (LLM 引擎)              │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │Sessions │ │ Memory  │ │  Tools   │ │
│  │         │ │(FAISS)  │ │   (17+)  └─────────┘ └────────  │ │
│─┘ └──────────┘ │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │  Pulse  │ │  Cron   │ │   LLM    │ │
│  │Heartbeat│ │Schedule │ │ Adapter  │ │
│  └─────────┘ └─────────┘ └──────────┘ │
└─────────────────────────────────────────┘
```

---

## 🛠️ 工具

| 工具 | 描述 |
|------|------|
| `exec` | 安全 Shell 执行 |
| `read` | 读取文件 |
| `write` | 写入文件 |
| `edit` | 智能文件编辑 |
| `apply_patch` | 多文件补丁 |
| `process` | 进程管理 |
| `browser` | CDP 浏览器控制 |
| `image` | 视觉模型分析 |
| `memory` | 向量记忆搜索/存储 |
| `message` | 多通道消息 |
| `cron` | 任务调度 |
| `sessions` | 会话管理 |
| `webhooks` | HTTP 触发器 |

---

## 🔌 LLM Provider (13)

| Provider | 环境变量 | 默认模型 |
|----------|---------|---------|
| OpenAI | `OPENAI_API_KEY` | gpt-4o |
| Anthropic | `ANTHROPIC_API_KEY` | claude-3.5-sonnet |
| Google | `GOOGLE_API_KEY` | gemini-2.0-flash |
| MiniMax | `MINIMAX_API_KEY` | MiniMax-M2.1 |
| Ollama | - | llama3 |
| OpenRouter | `OPENROUTER_API_KEY` | claude-3.5-sonnet |
| Moonshot | `MOONSHOT_API_KEY` | moonshot-v1-8k |
| GLM | `ZHIPU_API_KEY` | glm-4 |
| Qianfan | `QIANFAN_ACCESS_KEY` | ernie-speed-8k |
| Bedrock | `AWS_ACCESS_KEY_ID` | claude-3-sonnet |
| Vercel | `VERCEL_API_KEY` | gpt-4o |
| Z.AI | `ZAI_API_KEY` | default |
| Custom | `CUSTOM_API_KEY` | custom |

---

## 💬 通道 (18)

Telegram、Discord、Slack、WhatsApp、Signal、IRC、Google Chat、MS Teams、WebChat、Mattermost、LINE、Matrix、飞书、Zalo、Threema、Session、Tox、iMessage

### Telegram 功能
- **Long Polling**（默认）- 无需公网 URL
- **会话上下文** - 每个聊天独立历史
- **输入指示器** - AI 响应时显示

---

## 🆕 最近更新 (2026-02-19)

### 会话/任务上下文控制

- `/task list [limit]`
- `/task summary <task-id>`
- `/task detail <task-id> [page] [pageSize]`
- Marker 自动解析：`[task_done:task-...]`（支持单条消息多 marker）
- `/debug archive [session]` - 压缩水位线/归档验证

### 压缩可靠性

- `session_meta.last_compacted_message_id` 水位线
- `messages_archive.source_message_id` + 唯一索引去重
- 归档路径跳过 `[summary]` 系统消息

### Google Realtime

- 完整 `RealtimeProvider` 回调面（audio/text/tools/transcription/VAD/usage/session events）
- 函数调用及工具参数模式转换
- 原生音频工作流及最终 WAV 回调

## ⚙️ CLI 命令

```bash
# 进程管理
ocg start/stop/status/restart

# 交互式聊天
ocg agent

# 网关管理
ocg gateway [config.get|config.apply|config.patch|status]

# 自动化
ocg hooks [list|enable|disable|info]
ocg webhook [status|test|send]

# 监控
ocg llmhealth [--action status|start|stop|failover|events|reset|test]
ocg task [list|status]
```

---

## 📁 项目结构

```
OCG/
├── cmd/
│   ├── ocg/           # 主入口 (CLI)
│   ├── gateway/       # HTTP/WebSocket 服务器
│   ├── agent/         # LLM Agent
│   └── embedding-server/ # 向量嵌入
├── pkg/
│   ├── llm/           # LLM 适配器
│   ├── memory/        # 向量存储
│   ├── hooks/         # 事件钩子
│   └── config/        # 配置
├── gateway/
│   ├── static/        # Web UI
│   └── channels/      # 通道适配器
└── docs/             # 文档
```

---

## 📄 License

MIT © Glider Labs
