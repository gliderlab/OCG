# Changelog

All notable changes to OCG-Go (OCG) will be documented in this file.

## [v0.0.1beta10] - 2026-02-19

### Features - Google Gemini Live Realtime (2026-02-19 Evening)
- Full `RealtimeProvider` implementation with official Google SDK
- Bidirectional audio: PCM uplink via `SendAudio`, WAV downlink via finalized callback
- Function Calling support with tool parameter schema conversion
- Complete callback surface: transcription, VAD, usage, session events
- Recommended model: `models/gemini-2.5-flash-native-audio-preview-12-2025`

### Features - Task Marker Context Strategy (2026-02-19 Evening)
- `/split` now returns compact `[task_done:task-...]` marker to main session
- Full task details persisted in SQLite (`user_tasks`/`user_subtasks`)
- New commands:
  - `/task list [limit]` - list recent tasks
  - `/task summary <task-id>` - compact status
  - `/task detail <task-id> [page] [pageSize]` - full details with pagination
- Marker auto-resolve: messages containing `[task_done:task-...]` trigger automatic DB lookup
- Human-readable timestamps in task metadata

### Features - Strict Incremental Compaction Archive (2026-02-19 Evening)
- Added `session_meta.last_compacted_message_id` watermark field
- Added `messages_archive.source_message_id` with unique dedupe index `(session_key, source_message_id)`
- Archive SQL now skips `[summary]` system messages
- New command: `/debug archive [session]` - validate watermark and archive stats
- Guarantees no duplicate archives across repeated compactions

### Features - Tool Enhancements
- Added `tool_result_truncation.go` - automatic result truncation for large outputs
- Added `tool_loop_detection.go` - prevents infinite tool call loops
- Added `thinking.go` - thinking/reasoning mode support (off/on/stream)

### Features - CLI Improvements
- Added `ocg gateway` subcommand: config.get, config.apply, config.patch, status
- Added `ocg llmhealth --action reset` - reset health check state
- Added `ocg llmhealth --action test` - test provider connectivity
- Improved CLI output formatting (human-readable tool results)

### Features - Security
- Changed default bind address from 0.0.0.0 to 127.0.0.1
- Unified environment variables: OCG_PORT, OCG_HOST

### Bug Fixes - Blocking Issues
- Fixed Windows GatewayTool signature mismatch
- Fixed syscall.Kill not available on Windows
- Fixed unused bytes import in thinking.go
- Fixed default port mismatch (18789 → 55003)
- Fixed WebSocket connection count leak on Accept failure

### Bug Fixes - High Risk
- Fixed Webhook sessionKey not passed to RPC
- Fixed channel routing (added 13 more channels)
- Fixed web_search API key loading from environment
- Fixed embedding dimension for text-embedding-ada-002 (1024 → 1536)
- Fixed HybridEnabled cannot be disabled

### Documentation
- Added bilingual documentation (EN/ZH) - 86 files
- Fixed Markdown syntax errors in README
- Fixed broken internal links
- Added quality gate: .github/workflows/docs.yml
- Added local link check script: scripts/check-docs-links.sh

### CI/CD
- Unified Go version: 1.24 (go.mod, workflows)
- Added version consistency check in CI
- Added multi-platform testing (Linux/Windows/macOS)

## [v0.0.1beta9] - 2026-02-18

### Features - Dynamic Context Window Auto-Detection
- Added `GetContextWindow()` function to auto-detect model context window
- Priority: API Query → Built-in Map → Config File → Default
- Supported providers: OpenAI, Anthropic, Google, MiniMax, Ollama
- Config via `OCG_MODELS` environment variable
- Built-in context window maps for MiniMax, Anthropic, Google models

### Features - KV Engine (BadgerDB)

### Features - KV Engine (BadgerDB)
- New package: `pkg/kv/kv.go` - embedded key-value store
- Pure Go implementation (compiles into binary)
- High performance with LSM tree
- Supports both in-memory and persistent modes
- TTL support for automatic expiration
- Key design:
  - `task:{id}:status` - task status
  - `task:progress:{id}` - progress tracking
  - `token:{session}` - token cache with 10min TTL
- Config: `OCG_KV_DIR` for persistence (default: in-memory)

### Features - Task Splitting
- New file: `agent/task_split.go` - automatic task decomposition
- Now explicit via /split command (not auto-triggered)
- LLM-based task decomposition
- SQLite storage: `user_tasks` + `user_subtasks` tables
- Records execution process, timing, and results
- Integrated with KV for fast status updates

### Bug Fixes - Comprehensive Audit
- Agent: Fixed `compactLocked` race condition/panic
- Process: Thread-safe output buffer (`lockedWriter`)
- Process: Workdir jail with symlink resolution
- Gateway: Added `Vary: Accept-Encoding` header
- Gateway: WebSocket `SetReadLimit(4MB)`
- Gateway: `handleProcessKill` POST-only
- Gateway: Changed `context.Background()` to request context
- Memory: HNSW search mutex protection
- Memory: Fixed `loadExistingVectors` IDs sync
- Memory: Fixed deletion count lock
- Memory: Fixed `rebuildHNSW` close order
- Memory: Added timeout for OpenAI Embedding
- Browser: URL QueryEscape for CDP open
- Browser: CDP HTTP status check
- Browser: Fixed zombie process on stop
- Cron: Atomic file write for persistence
- Cron: Fixed 6-field cron second stepping
- Cron: Fixed DOM/DOW OR semantics
- Cron: Fixed RunJob race condition
- Skills: GenerateTools concurrent lock
- Skills: shellSkillTool workdir jail
- Skills: cliSkillTool shlex parsing
- Skills: YAML frontmatter parsing
- Web: Fixed CheckRedirect to allow redirects

### Code Quality
- Added `golangci-lint` configuration
- Fixed multiple lint issues across codebase
- go mod tidy

## [v0.0.1beta8] - 2026-02-18

### Features - LLM-Powered Compaction Summary
- Implemented LLM-based summary generation for compaction
- New method: `buildSummaryWithInstructionsLLM()` - calls LLM to generate conversation summaries
- New method: `callLLMForSummary()` - non-streaming HTTP POST to LLM API
- Configurable summary instructions (default: "Concisely summarize the key points")
- Lower temperature (0.3) for deterministic summaries
- Automatic fallback to simple concatenation on LLM failure

### Features - Anthropic & Google Embeddings Support
- Implemented `Embeddings()` for Anthropic provider
- Implemented `Embeddings()` for Google provider
- Both use external OpenAI-compatible embedding services
- Configurable via environment variables:
  - `ANTHROPIC_EMBED_API_KEY`, `ANTHROPIC_EMBED_BASE_URL`, `ANTHROPIC_EMBED_MODEL`
  - `GOOGLE_EMBED_API_KEY`, `GOOGLE_EMBED_BASE_URL`, `GOOGLE_EMBED_MODEL`
- Falls back to `OPENAI_API_KEY` if no provider-specific key configured

### [v0.0.1beta7] - 2026-02-18

### Features - LLM Provider Refactor (Adapter Pattern)
- Each provider now in separate directory: `pkg/llm/providers/<name>/`
- Providers: openai, anthropic, google, minimax, ollama, custom
- Each provider self-contained with own HTTP client and retry logic
- Added `GetConfig()` method to Provider interface

### Features - Skills Install Support
- Extended `SkillMetadata` with `Install []InstallItem`
- Parse `install` array from SKILL.md metadata
- Auto-generate install commands (brew/apt/npm/pip/go)
- Added `GetInstallInstructions()` and `FormatInstallInstructions()`

### Features - LLM Health Check & Auto-Failover
- New package: `pkg/llmhealth/health.go`
- Periodic health probe (configurable interval)
- Failure threshold detection
- Auto-failover to backup provider
- Manual failover command: `ocg llmhealth --action failover --provider <type>`
- Environment variables: `LLM_HEALTH_CHECK`, `LLM_HEALTH_INTERVAL`, etc.

### Features - Proactive Overflow Handling
- `handleContextOverflow()` estimates tokens before sending to LLM
- Two-step: pruning first, then compaction
- Async compaction with `TryLock` (non-blocking)
- Automatic retry after compaction

### Documentation
- Added `docs/LLMPROVIDERS.md` - Provider configuration guide
- Added `docs/LLMHEALTH.md` - Health check and failover guide
- Updated `docs/TOOLS.md` - Added Skills system documentation
- Updated `docs/OCG.md` - Added llmhealth command
- Updated `docs/COMPACTION.md` - Added proactive overflow details
- Updated `docs/MEMORY.md` - Added context management integration

### Bug Fixes
- **storage/storage.go**: Pulse claim logic - PeekNextEvent() + ClaimNextEventV2() with atomic UPDATE-first
- **agent/pulse.go**: Add WaitGroup + context for graceful shutdown
- **gateway/websocket.go**: gRPC uses connection ctx, add 5s write timeout
- **pkg/config/server.go**: Add MaxBodyCron config (256KB default)
- **gateway/gateway.go**: Apply MaxBodyCron to all cron handlers
- **tools/web.go**: Add timeouts (15-30s) and body limits (2-5MB)

### Lint Fixes
- Fix SA6003: range over string → []rune
- Fix SA1019: grpc.DialContext → grpc.NewClient
- Fix ST1005: lowercase error strings in channel files
- Fix ineffassign: cron/cron.go, memory/vector_store.go, tools/tools_test.go
- Fix unused fields/variables with nolint directives

### Tooling
- Add .golangci.yml with staticcheck, ineffassign, unused, govet enabled

## [v0.0.1beta5] - 2026-02-17

### Bug Fixes (Critical)
- **gateway/websocket.go**: Fix WebSocket concurrent write race - add writeMu mutex
- **gateway/websocket.go**: Fix blocking - handleWSChat now runs in goroutine
- **agent/pulse.go**: Fix Pulse heartbeat blocking - LLM calls now async
- **cmd/ocg/main.go**: Fix Windows PID management - use "pid timestamp" format

### Features - LLM Provider Adapter
- **pkg/llm/provider.go**: Core interface and types
- **pkg/llm/factory/init.go**: Provider initialization
- **pkg/llm/providers/**: All provider implementations

Supported providers: OpenAI, Anthropic, Google Gemini, MiniMax, Ollama, Custom

### Features - OCG Skills Compatibility
- **pkg/skills/loader.go**: Parse SKILL.md YAML frontmatter
- **pkg/skills/registry.go**: Skills registry with dependency checking
- **pkg/skills/adapter.go**: Convert skills to OCG tools

Tool types: shellSkillTool, cliSkillTool, genericSkillTool

### Features - Complete Browser Tool
- **tools/browser_tool.go**: Full CDP-based browser automation
- Lifecycle: status/start/stop
- Tabs: tabs/tab/new/open/focus/close
- Navigation: navigate/open
- Snapshot: ai/aria formats, interactive, compact
- Screenshot: fullPage support
- Actions: click/type/press/hover/drag/select/fill
- Wait: selector, waitUrl, fn, timeoutMs
- Execute: JavaScript evaluation
- Cookies/Storage: get/set/clear
- Viewport: resize
- Debug: highlight/console/errors/requests
- PDF: document generation

### Features - Channel Adapter (10 Channels)
- **gateway/channels/types/**: Shared types to eliminate circular imports
- **gateway/channels/telegram/**: Full Telegram implementation
- **gateway/channels/discord/**: Discord implementation
- **gateway/channels/slack/**: Slack implementation
- **gateway/channels/whatsapp/**: WhatsApp implementation
- **gateway/channels/signal/**: Signal implementation
- **gateway/channels/irc/**: IRC implementation
- **gateway/channels/googlechat/**: Google Chat implementation
- **gateway/channels/msteams/**: Microsoft Teams implementation
- **gateway/channels/webchat/**: WebChat implementation

### New Tools
- **tools/apply_patch.go**: Multi-file structured patches (Add/Update/Delete/Move)
- **tools/image.go**: Vision model image analysis (OpenAI/Anthropic/Gemini)
- **tools/gateway_tool.go**: Gateway process management
- **tools/message.go**: Multi-channel messaging

### Documentation
- **docs/CHANNELS.md**: Update channel status
- **docs/TOOLS.md**: Update tools documentation
- **README.md**: Comprehensive update with all features

---

## [v0.0.1beta4] - 2026-02-16

### Bug Fixes
- **rpcproto/grpc_transport.go**: Fix gRPC connection timeout - use grpc.Dial + WithBlock
- **cmd/ocg/main.go**: Fix config path resolution relative to binary location
- **cmd/gateway/main.go**: Fix config/db/work dirs relative to binary location
- **cmd/agent/main.go**: Fix config/db/work dirs relative to binary location
- **gateway/gateway.go**: Fix working directory - os.Chdir(workDir) at startup
- **tools/exec.go**: Set default workdir to current directory

### Features (Dependency Injection)
- **pkg/config/server.go**: Add GatewayConfig, AgentConfig, StorageConfig, MemoryConfig, EmbeddingConfig
- **agent/agent.go**: Add DI support - TimeProvider, IDGenerator, Logger interfaces
- **agent/agent_di.go**: New DI builder for Agent
- **gateway/gateway.go**: Add DI support - HTTPClient, IDGenerator, TimeProvider
- **storage/storage.go**: Add NewWithConfig() constructor
- **memory/vector_store_di.go**: Add NewVectorMemoryStoreWithConfig()

### Documentation
- **docs/RPC.md**: Update gRPC connection code
- **docs/OCG.md**: Update config lookup paths

---

## [v0.0.1beta3] - 2026-02-16

### Bug Fixes
- **tools/edit.go**: Add jail check (IsPathAllowed) for path security
- **tools/edit.go**: Remove unused filepath import
- **agent/agent.go**: Fix goroutine timeout handling with buffered channel
- **gateway/websocket.go**: Add NOTE for unused WebSocketHub (dead code)

### Improvements
- Multiple files: Code comments internationalized to English
- **pkg/config/env.go**: Extract readEnvConfig to shared module
- **storage/storage.go**: Improve error handling in various methods

### Features
- **gateway/data/cron**: Add cron jobs persistence directory
- **pkg/config**: Add environment configuration module

---

## [v0.0.1beta2] - 2026-02-15

### Bug Fixes
- SQL injection risk in ClearOldEvents
- Dead code in Search method (duplicate nil check)
- getByID ignoring Scan errors
- GetOrCreateSession deadlock - use RLock + double-check pattern
- Authorization header case inconsistency - use strings.CutPrefix
- Agent fields without concurrent protection - add sync.RWMutex
- Double signal handling causing defer non-execution - remove os.Exit(0)
- WebSocketHub missing exit condition - add stop channel
- upsertFTS only INSERT - change to INSERT OR REPLACE
- JobStore.save() deadlock - save() no longer acquires lock

### Performance
- Bubble sort O(n²) → sort.Slice O(n log n)
- HTTP timeout 30s → 120s
- randomID uses crypto/rand for collision resistance
- Token estimation distinguishes Chinese/English

### Security
- Memory leak - processes map auto-cleanup after 5 minutes
- maybeCompact blocks main thread - make async
- File operations without jail limit - add IsPathAllowed()
- saveConfigToDB errors ignored - add error logging

### Cross-platform
- PID directory uses os.TempDir() - Windows compatible
- parseCustomToolCalls JSON first - more robust parsing
- Panic Recovery - Gateway/Agent all entries

---

## [v0.0.1-beta1] - 2026-02-09

### Features
- Initial beta release
- Vector memory with FAISS HNSW + SQLite
- Hybrid search (vector + keyword)
- Embedding support (OpenAI / Local llama.cpp)
- Telegram channel adapter
- Cron job system
- WebSocket API
- Tool registry and execution

---

[Unreleased]: https://github.com/gliderlab/OCG/compare/v0.0.1beta4...main
[v0.0.1beta4]: https://github.com/gliderlab/OCG/compare/v0.0.1beta3...v0.0.1beta4
[v0.0.1beta3]: https://github.com/gliderlab/OCG/compare/v0.0.1beta2...v0.0.1beta3
[v0.0.1beta2]: https://github.com/gliderlab/OCG/compare/v0.0.1-beta1...v0.0.1beta2
[v0.0.1-beta1]: https://github.com/gliderlab/OCG/releases/tag/v0.0.1-beta1
