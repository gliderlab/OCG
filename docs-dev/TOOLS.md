# OCG Tools API Documentation

> OCG Tool System Reference for Secondary Development

## Overview

OCG provides 17+ tools that can be used by the AI agent. Tools follow the OpenAI function calling schema and are registered in a centralized registry.

## Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Execute(args map[string]interface{}) (interface{}, error)
}
```

## Tool Registry

Tools are managed through a centralized registry:

```go
// Create registry
registry := tools.NewRegistry()

// Register tool
registry.Register(&MyTool{})

// Get tool
tool, ok := registry.Get("tool-name")

// List all tools
tools := registry.List()
```

## Tool Policy

Tools can be restricted using policy:

```go
policy := &tools.ToolsPolicy{
    Profile:       "full",    // minimal, coding, messaging, full
    Allow:         []string{}, // nil = all allowed
    Deny:          []string{},
    WorkspaceOnly: false,
}

registry.SetPolicy(policy)
```

---

## Available Tools

### File Operations

#### read
Read file contents.

```json
{
  "path": "/path/to/file",
  "limit": 100,
  "offset": 0
}
```

**Parameters:**
- `path` (string, required) - File path
- `limit` (integer, optional) - Max lines
- `offset` (integer, optional) - Start line

---

#### write
Write content to file.

```json
{
  "content": "file content",
  "path": "/path/to/file"
}
```

**Parameters:**
- `content` (string, required) - Content to write
- `path` (string, required) - Target file path

---

#### edit
Smart file editing.

```json
{
  "path": "/path/to/file",
  "old_string": "old text",
  "new_string": "new text"
}
```

**Parameters:**
- `path` (string, required) - File path
- `old_string` (string, required) - Text to replace
- `new_string` (string, required) - Replacement text

---

#### apply_patch
Apply multi-file patches.

```json
{
  "patches": [
    {"path": "file1.go", "patch": "..."}
  ]
}
```

---

### Runtime

#### exec
Execute shell commands safely.

```json
{
  "command": "ls -la",
  "workdir": "/home/user",
  "timeout": 30,
  "env": {"KEY": "value"}
}
```

**Parameters:**
- `command` (string, required) - Shell command
- `workdir` (string, optional) - Working directory
- `timeout` (integer, optional) - Timeout in seconds
- `env` (object, optional) - Environment variables

---

#### process
Process management.

```json
{
  "action": "list",
  "sessionId": "abc123"
}
```

**Actions:**
- `list` - List running processes
- `start` - Start new process
- `kill` - Kill process
- `write` - Write to stdin
- `log` - Get process logs

---

### Web

#### web_search
Search the web using Tavily API.

```json
{
  "query": "search terms",
  "count": 5
}
```

**Parameters:**
- `query` (string, required) - Search query
- `count` (integer, optional) - Results count (1-10)

---

#### web_fetch
Fetch web page content.

```json
{
  "url": "https://example.com",
  "extractMode": "markdown",
  "maxChars": 10000
}
```

**Parameters:**
- `url` (string, required) - URL to fetch
- `extractMode` (string, optional) - markdown or text
- `maxChars` (integer, optional) - Max characters

---

### Browser

#### browser
CDP browser control.

```json
{
  "action": "snapshot",
  "target": "host"
}
```

**Actions:**
- `status` - Browser status
- `start` - Start browser
- `stop` - Stop browser
- `snapshot` - Get page snapshot
- `navigate` - Navigate to URL
- `act` - Perform action (click, type, etc.)

---

### Memory

#### memory_search
Semantic search in vector memory.

```json
{
  "query": "search terms",
  "maxResults": 5,
  "minScore": 0.5
}
```

**Parameters:**
- `query` (string, required) - Search query
- `maxResults` (integer, optional) - Max results
- `minScore` (float, optional) - Minimum score

---

#### memory_get
Get stored memory content.

```json
{
  "path": "memory/2026-02-21.md",
  "from": 1,
  "lines": 50
}
```

---

#### memory_store
Store content in memory.

```json
{
  "text": "content to store",
  "category": "general",
  "importance": 0.8
}
```

---

### Sessions

#### sessions_list
List all sessions.

```json
{
  "activeMinutes": 60,
  "kinds": ["telegram", "discord"],
  "limit": 50
}
```

---

#### sessions_send
Send message to another session.

```json
{
  "sessionKey": "telegram:123456",
  "message": "Hello"
}
```

---

#### sessions_spawn
Spawn sub-agent.

```json
{
  "agentId": "default",
  "task": "task description",
  "timeoutSeconds": 300
}
```

---

#### sessions_history
Get session history.

```json
{
  "sessionKey": "telegram:123456",
  "limit": 50
}
```

---

#### session_status
Get session status.

```json
{
  "sessionKey": "telegram:123456"
}
```

---

#### agents_list
List available agents.

```json
{}
```

---

### Messaging

#### message
Send messages via channels.

```json
{
  "action": "send",
  "channel": "telegram",
  "target": "123456",
  "message": "Hello"
}
```

**Channels:** telegram, discord, slack, whatsapp, signal, etc.

---

### Automation

#### cron
Task scheduling.

```json
{
  "action": "list"
}
```

**Actions:**
- `list` - List jobs
- `add` - Add job
- `remove` - Remove job
- `run` - Run job now

---

#### gateway
Gateway management.

```json
{
  "action": "config.get"
}
```

---

### Pulse

#### pulse
Heartbeat/pulse system.

```json
{
  "action": "status"
}
```

---

### Task Split

#### task_split
Split complex tasks.

```json
{
  "task": "complex task description"
}
```

---

### Image

#### image
Vision model image analysis.

```json
{
  "url": "https://example.com/image.jpg",
  "prompt": "Describe this image"
}
```

---

## Tool Groups

Tools can be grouped:

```json
{
  "group:runtime": ["exec", "process"],
  "group:fs": ["read", "write", "edit", "apply_patch"],
  "group:sessions": ["sessions_list", "sessions_history", "sessions_send", "sessions_spawn", "session_status"],
  "group:memory": ["memory_search", "memory_get"],
  "group:web": ["web_search", "web_fetch"],
  "group:ui": ["browser", "canvas"],
  "group:automation": ["cron", "gateway"],
  "group:messaging": ["message"],
  "group:nodes": ["nodes"]
}
```

---

## Custom Tool Development

Example of creating a custom tool:

```go
package mytool

import (
    "fmt"
)

type MyTool struct{}

func NewMyTool() *MyTool {
    return &MyTool{}
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "My custom tool description"
}

func (t *MyTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{
                "type":        "string",
                "description": "First parameter",
            },
        },
        "required": []string{"param1"},
    }
}

func (t *MyTool) Execute(args map[string]interface{}) (interface{}, error) {
    param1 := args["param1"].(string)
    // Tool logic here
    return map[string]interface{}{
        "result": fmt.Sprintf("Processed: %s", param1),
    }, nil
}
```

Register in main:

```go
registry.Register(mytool.NewMyTool())
```
