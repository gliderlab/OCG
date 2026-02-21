# Web 工具

搜索网络和获取 URL 内容。

---

## web_search

通过 Tavily API 进行 AI 优化的网络搜索。

### 使用

```bash
web_search(query="OCG 文档", max_results=5)
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `query` | string | 搜索查询 |
| `max_results` | int | 最大结果数 (默认: 5) |
| `include_raw_content` | bool | 包含原始内容 (可选) |

### 示例

```bash
web_search(query="OCG AI agent Go")
web_search(query="Tavily API 文档", max_results=10)
```

### 返回

```json
{
  "results": [
    {
      "title": "结果标题",
      "url": "https://example.com",
      "content": "简要内容片段",
      "score": 0.95
    }
  ]
}
```

### 配置

```bash
# Tavily API key (可选 - 可能有免费层级)
export TAVILY_API_KEY="your-api-key"
```

---

## web_fetch

从 URL 获取和提取可读内容。

### 使用

```bash
web_fetch(url="https://github.com/gliderlab/OCG")
```

### 参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `url` | string | HTTP/HTTPS URL |
| `extract_mode` | string | "markdown" 或 "text" (默认: markdown) |
| `max_chars` | int | 最大字符数 (可选) |

### 示例

```bash
web_fetch(url="https://docs.openclaw.ai")
web_fetch(url="https://github.com/gliderlab/OCG", extract_mode="text")
web_fetch(url="https://example.com", max_chars=10000)
```

### 返回

```
提取的内容，以 markdown 或纯文本格式呈现。
```

### 注意

- 将 HTML 转换为可读的 markdown/文本
- 自动处理重定向
- 遵循 robots.txt

---

## 限制

### web_search

- 需要 Tavily API key 才能完全访问
- 免费层级可能有速率限制
- 结果取决于搜索引擎覆盖范围

### web_fetch

- 超时: 15-30 秒
- 主体限制: 2-5MB
- 某些网站可能阻止机器人
- 不支持 JavaScript 渲染的内容

---

## 使用场景

### 研究

```bash
web_search(query="Go AI agent 框架 2024")
web_fetch(url="https://github.com/topics/go-agent")
```

### 文档

```bash
web_fetch(url="https://pkg.go.dev/github.com/gliderlab/OCG")
```

### 时事

```bash
web_search(query="OCG 最新版本")
```

---

## 相关文档

- [工具概览](overview-zh.md)
- [浏览器工具](browser-zh.md)
