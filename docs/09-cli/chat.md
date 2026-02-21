# Interactive Chat

Use OCG via command-line interface.

---

## Start Chat

```bash
./bin/ocg agent
```

Starts interactive chat session.

---

## Commands

| Command | Description |
|---------|-------------|
| `//quit` | Exit chat |
| `//help` | Show help |
| `//new` | New conversation |
| `//reset` | Reset context |

---

## Usage

```
$ ./bin/ocg agent
OCG Chat Agent
Type //quit to exit, //help for commands.

You: Hello, how are you?
OCG: I'm doing well, thanks for asking! How can I help you today?
You: Explain vector memory
OCG: Vector memory uses...
You: //quit
Goodbye!
```

---

## Options

```bash
./bin/ocg agent --model gpt-4o
./bin/ocg agent --system "You are a helpful assistant"
```

---

## Session Behavior

- Maintains conversation context
- Supports all OCG tools
- Works with all configured channels

---

## See Also

- [CLI Overview](../overview.md)
