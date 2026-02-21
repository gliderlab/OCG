# Built-in Hooks

Pre-configured hooks included with OCG.

---

## session-memory

Saves conversation to memory files.

```bash
./bin/ocg hooks enable session-memory
```

**Trigger:** `message.receive`  
**Action:** Saves conversation to `memory/YYYY-MM-DD.md`

---

## command-logger

Logs executed commands.

```bash
./bin/ocg hooks enable command-logger
```

**Trigger:** `exec` tool calls  
**Action:** Logs command to file

---

## boot-md

Runs BOOT.md on startup.

```bash
./bin/ocg hooks enable boot-md
```

**Trigger:** `agent.start`  
**Action:** Executes BOOT.md content

---

## bootstrap-extra-files

Injects extra bootstrap files.

```bash
./bin/ocg hooks enable bootstrap-extra-files
```

**Trigger:** `agent.start`  
**Action:** Loads additional bootstrap files

---

## All Built-in Hooks

| Hook | Trigger | Description |
|------|---------|-------------|
| session-memory | message.receive | Save conversations |
| command-logger | exec | Log commands |
| boot-md | agent.start | Run BOOT.md |
| bootstrap-extra-files | agent.start | Load bootstrap files |

---

## See Also

- [Hooks Overview](hooks.md)
