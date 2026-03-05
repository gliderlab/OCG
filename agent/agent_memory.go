// agent_memory.go - memory recall and flush logic
package agent

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlab/cogate/tools"
)

// recallRelevantMemories automatically retrieves memories related to the prompt
func (a *Agent) recallRelevantMemories(prompt string) string {
	if a.memoryStore == nil {
		return ""
	}
	limit := a.cfg.RecallLimit
	if limit <= 0 {
		limit = 3
	}
	minScore := float32(a.cfg.RecallMinScore)
	if minScore <= 0 {
		minScore = 0.3
	}

	results, err := a.memoryStore.Search(prompt, limit*2, minScore)
	if err != nil || len(results) == 0 {
		return ""
	}

	// re-rank by category/importance weighting
	catBoost := map[string]float32{
		"decision":   0.2,
		"preference": 0.15,
		"fact":       0.1,
		"entity":     0.05,
	}
	sort.Slice(results, func(i, j int) bool {
		ri := results[i]
		rj := results[j]
		wi := ri.Score * (1 + float32(ri.Entry.Importance)) * (1 + catBoost[strings.ToLower(ri.Entry.Category)])
		wj := rj.Score * (1 + float32(rj.Entry.Importance)) * (1 + catBoost[strings.ToLower(rj.Entry.Category)])
		return wi > wj
	})
	if len(results) > limit {
		results = results[:limit]
	}

	return tools.FormatMemoriesForContext(results)
}

func isRecallRequest(msg string) bool {
	low := strings.ToLower(strings.TrimSpace(msg))
	return strings.HasPrefix(low, "/recall") ||
		strings.HasPrefix(low, "recall") ||
		strings.HasPrefix(low, "remember")
}

// maybeFlushMemory soft-triggers long memory flush (SQLite storage)
// Rules: trigger every 200 messages with a minimum interval of 10 minutes
func (a *Agent) maybeFlushMemory(lastMsg string) {
	if a.store == nil || a.memoryStore == nil {
		return
	}

	stats, err := a.store.Stats()
	if err != nil {
		return
	}
	msgCount := stats["messages"]
	// Check less frequently (every 200 messages) to reduce DB load
	if msgCount == 0 || msgCount%200 != 0 {
		return
	}

	lastFlushAtStr, _ := a.store.GetConfig("memory", "lastFlushAt")
	lastFlushCountStr, _ := a.store.GetConfig("memory", "lastFlushCount")
	lastFlushAt, _ := strconv.ParseInt(lastFlushAtStr, 10, 64)
	lastFlushCount, _ := strconv.Atoi(lastFlushCountStr)

	if lastFlushCount == msgCount {
		return
	}
	if time.Now().Unix()-lastFlushAt < 600 {
		return
	}

	if lastMsg != "" && tools.ShouldCapture(lastMsg) {
		category := tools.DetectCategory(lastMsg)
		_, _ = a.memoryStore.StoreWithSource(lastMsg, category, 0.5, "flush")
	}

	_ = a.store.SetConfig("memory", "lastFlushAt", fmt.Sprintf("%d", time.Now().Unix()))
	_ = a.store.SetConfig("memory", "lastFlushCount", fmt.Sprintf("%d", msgCount))
	log.Printf("[Memory] Flush triggered at msgCount=%d", msgCount)
}

// preprocessChat performs common pre-chat operations shared by chatInternal and chatStreamInternal:
// 1. Auto memory capture for important messages
// 2. Soft-trigger memory flush
// 3. Async compaction check
func (a *Agent) preprocessChat(sessionKey, lastMsg string, messages []Message) {
	if a.store == nil || lastMsg == "" {
		return
	}

	// Auto memory capture
	if a.memoryStore != nil && tools.ShouldCapture(lastMsg) {
		category := tools.DetectCategory(lastMsg)
		results, _ := a.memoryStore.Search(lastMsg, 1, 0.95)
		if len(results) == 0 {
			_, err := a.memoryStore.StoreWithSource(lastMsg, category, 0.6, "auto")
			if err != nil {
				log.Printf("[WARN] auto memory write failed")
			}
		}
	}

	// Soft-trigger memory flush
	a.maybeFlushMemory(lastMsg)

	// Async compaction check
	go func() {
		if !a.compactMu.TryLock() {
			log.Printf("[WARN] maybeCompact skipped: another compaction in progress")
			return
		}
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WARN] maybeCompact recovered from panic: %v", r)
			}
			a.compactMu.Unlock()
		}()
		a.maybeCompact(sessionKey, messages, nil)
	}()
}
