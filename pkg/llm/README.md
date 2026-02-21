# LLM Provider

LLM Provider abstraction layer for OCG-Go.

## Supported Providers

| Provider | Environment Variables | Default URL |
|----------|---------------------|-------------|
| OpenAI | `OPENAI_API_KEY` | https://api.openai.com/v1 |
| Anthropic | `ANTHROPIC_API_KEY` | https://api.anthropic.com/v1 |
| Google Gemini | `GOOGLE_API_KEY` | https://generativelanguage.googleapis.com/v1 |
| MiniMax | `MINIMAX_API_KEY` | https://api.minimax.chat/v1 |
| Ollama (local) | - | http://localhost:11434 |
| Custom | `CUSTOM_API_KEY`, `CUSTOM_BASE_URL` | Custom |

## Usage

```go
import (
    "github.com/gliderlab/cogate/pkg/llm"
    "github.com/gliderlab/cogate/pkg/llm/factory"
)

// Initialize all providers
factory.InitProviders()

// Get default provider
p, _ := factory.GetDefaultProvider()

// Send chat request
resp, _ := p.Chat(ctx, &llm.ChatRequest{
    Model:    "gpt-4o",
    Messages: []llm.Message{{Role: "user", Content: "Hello"}},
})

fmt.Println(resp.Choices[0].Message.Content)
```

## Environment Variables

Each provider can be configured via environment variables:

- `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`
- `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_MODEL`
- `GOOGLE_API_KEY`, `GOOGLE_BASE_URL`, `GOOGLE_MODEL`
- `MINIMAX_API_KEY`, `MINIMAX_BASE_URL`, `MINIMAX_MODEL`, `MINIMAX_GROUP_ID`
- `OLLAMA_BASE_URL`, `OLLAMA_MODEL`
- `CUSTOM_API_KEY`, `CUSTOM_BASE_URL`, `CUSTOM_MODEL`
