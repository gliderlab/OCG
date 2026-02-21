# Compaction

Compaction keeps sessions within model context limits while preserving recoverable history.

---

## Overview

OCG compaction has two goals:

1. Keep active context small (summary + recent messages)
2. Persist old conversation safely for later inspection

Compaction can run:

- Automatically when context crosses threshold
- Manually via `/compact`

---

## Current Behavior (Strict Incremental Archive)

During compaction, OCG now archives **only newly un-compacted messages**.

### Guarantees

- Uses watermark: `session_meta.last_compacted_message_id`
- Archives only messages in `(last_compacted_message_id, current_cutoff]`
- Skips generated summary entries (`[summary]...`) from archive payload
- Uses dedupe index on archive source ids to avoid duplicate archive rows on retries

### Archive Deduplication

`messages_archive` includes:

- `source_message_id`
- unique index on `(session_key, source_message_id)`

So repeated compaction/retry paths won't duplicate archived rows.

---

## Process

1. Estimate tokens and check compaction threshold
2. Split into `old` and `keep`
3. Archive `old` incrementally (watermark + dedupe)
4. Clear active messages
5. Re-insert `keep`
6. Add `[summary]` system message
7. Update `session_meta` counters + watermark

---

## Commands

```bash
/compact
/compact Focus on decisions and unresolved items
```

Debug archive state:

```bash
/debug archive
/debug archive default
```

Returns watermark and archive stats for validation.

---

## Notes

- `messages_archive` is long-term history storage, not active prompt context.
- Active context remains compacted transcript + recent turns.
- This design supports repeated compactions (2nd, 3rd, ...), each archiving only new un-compacted ranges.
