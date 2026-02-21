# Web Tools

Search the web and fetch URL content.

---

## web_search

AI-optimized web search via Tavily API.

### Usage

```bash
web_search(query="OCG documentation", max_results=5)
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query |
| `max_results` | int | Maximum results (default: 5) |
| `include_raw_content` | bool | Include raw content (optional) |

### Example

```bash
web_search(query="OCG AI agent Go")
web_search(query="Tavily API documentation", max_results=10)
```

### Returns

```json
{
  "results": [
    {
      "title": "Result Title",
      "url": "https://example.com",
      "content": "Brief content snippet",
      "score": 0.95
    }
  ]
}
```

### Configuration

```bash
# Tavily API key (optional - may have free tier)
export TAVILY_API_KEY="your-api-key"
```

---

## web_fetch

Fetch and extract readable content from URLs.

### Usage

```bash
web_fetch(url="https://github.com/gliderlab/OCG")
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | HTTP/HTTPS URL |
| `extract_mode` | string | "markdown" or "text" (default: markdown) |
| `max_chars` | int | Maximum characters (optional) |

### Example

```bash
web_fetch(url="https://docs.openclaw.ai")
web_fetch(url="https://github.com/gliderlab/OCG", extract_mode="text")
web_fetch(url="https://example.com", max_chars=10000)
```

### Returns

```
Extracted content in markdown or plain text format.
```

### Notes

- Converts HTML to readable markdown/text
- Automatically handles redirects
- Follows robots.txt

---

## Limitations

### web_search

- Requires Tavily API key for full access
- May have rate limits on free tier
- Results depend on search engine coverage

### web_fetch

- Timeout: 15-30 seconds
- Body limit: 2-5MB
- Some sites may block bots
- JavaScript-rendered content not supported

---

## Use Cases

### Research

```bash
web_search(query="Go AI agent frameworks 2024")
web_fetch(url="https://github.com/topics/go-agent")
```

### Documentation

```bash
web_fetch(url="https://pkg.go.dev/github.com/gliderlab/OCG")
```

### Current Events

```bash
web_search(query="OCG latest release")
```

---

## See Also

- [Tools Overview](../overview.md)
- [Browser Tool](browser.md)
