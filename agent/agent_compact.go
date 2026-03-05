// agent_compact.go - context overflow handling, compaction, token estimation
package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
	"github.com/gliderlab/cogate/storage"
	"github.com/pkoukk/tiktoken-go"
)

// pruneMessages removes old tool results from messages based on config
func (a *Agent) pruneMessages(_ string, messages []Message) []Message {
	cfg := a.cfg.ContextPruning
	if cfg.Mode == "off" {
		return messages
	}
	if cfg.Mode != "cache-ttl" {
		return messages
	}

	result := make([]Message, 0, len(messages))
	var lastAssistantIdx int

	for i, msg := range messages {
		if msg.Role == "assistant" {
			lastAssistantIdx = i
		}
	}

	protectedFrom := 0
	if lastAssistantIdx > 0 && cfg.KeepLastAssistants > 0 {
		protectedFrom = len(messages) - (cfg.KeepLastAssistants * 2)
		if protectedFrom < 0 {
			protectedFrom = 0
		}
	}

	for i, msg := range messages {
		if len(msg.ToolExecutionResults) == 0 {
			result = append(result, msg)
			continue
		}

		if i >= protectedFrom {
			result = append(result, msg)
			continue
		}

		hasImage := false
		for _, tr := range msg.ToolExecutionResults {
			if tr.Result != nil {
				_, ok := tr.Result.(string)
				if !ok {
					hasImage = true
					break
				}
			}
		}
		if hasImage {
			result = append(result, msg)
			continue
		}

		for j := range msg.ToolExecutionResults {
			if msg.ToolExecutionResults[j].Result != nil {
				content, ok := msg.ToolExecutionResults[j].Result.(string)
				if ok && len(content) > cfg.MinPrunableToolChars {
					headChars := cfg.SoftTrim.HeadChars
					if headChars == 0 {
						headChars = 1500
					}
					tailChars := cfg.SoftTrim.TailChars
					if tailChars == 0 {
						tailChars = 1500
					}

					if len(content) > headChars+tailChars {
						newContent := content[:headChars] + "\n...[truncated, original size: " + fmt.Sprintf("%d", len(content)) + " bytes]...\n" + content[len(content)-tailChars:]
						msg.ToolExecutionResults[j].Result = newContent
					} else if cfg.HardClear.Enabled && len(content) > 0 {
						placeholder := cfg.HardClear.Placeholder
						if placeholder == "" {
							placeholder = "[Old tool result content cleared]"
						}
						msg.ToolExecutionResults[j].Result = placeholder
					}
				}
			}
		}
		result = append(result, msg)
	}

	return result
}

// maybeCompact checks if compaction is needed and executes it if necessary
func (a *Agent) maybeCompact(sessionKey string, _ []Message, compactChan chan<- bool) {
	if !a.compactMu.TryLock() {
		log.Printf("[WARN] maybeCompact skipped: another compaction in progress")
		if compactChan != nil {
			compactChan <- false
		}
		return
	}
	defer a.compactMu.Unlock()

	if a.store == nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}
	meta, err := a.store.GetSessionMeta(sessionKey)
	if err != nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	stored, err := a.store.GetMessages(sessionKey, 500)
	if err != nil {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	tokens := estimateTokensFromStore(stored)
	meta.TotalTokens = tokens
	_ = a.store.UpsertSessionMeta(meta)

	providerType := a.detectProviderType()
	contextWindow := llm.GetContextWindow(providerType, a.cfg.Model, a.cfg.BaseURL, a.cfg.APIKey, a.cfg.Models)
	if contextWindow <= 0 {
		contextWindow = a.cfg.ContextTokens
	}

	threshold := int(float64(contextWindow) * a.cfg.CompactionThreshold)
	log.Printf("[maybeCompact] Model=%s context_window=%d threshold=%d tokens=%d",
		a.cfg.Model, contextWindow, threshold, tokens)

	if tokens < threshold || len(stored) <= a.cfg.KeepMessages {
		if compactChan != nil {
			compactChan <- false
		}
		return
	}

	cut := len(stored) - a.cfg.KeepMessages
	old := stored[:cut]
	keep := stored[cut:]

	summary := buildSummary(old)

	if len(old) > 0 {
		_ = a.store.ArchiveMessages(sessionKey, old[len(old)-1].ID)
		meta.LastCompactedMessageID = old[len(old)-1].ID
	}

	meta.CompactionCount += 1
	meta.LastSummary = summary
	meta.MemoryFlushCompactionCnt = meta.CompactionCount
	meta.MemoryFlushAt = time.Now()
	_ = a.store.UpsertSessionMeta(meta)

	_ = a.store.ClearMessages(sessionKey)
	for _, m := range keep {
		_ = a.store.AddMessage(sessionKey, m.Role, m.Content)
	}
	if summary != "" {
		_ = a.store.AddMessage(sessionKey, "system", "[summary]\n"+summary)
	}
	log.Printf("[CLEAN] Compaction done: session=%s, kept=%d, totalTokens=%d", sessionKey, len(keep), tokens)

	if compactChan != nil {
		compactChan <- true
	}
}

// tokenCounter is a package-level tiktoken instance for accurate counting
var (
	tokenCounter     *tiktoken.Tiktoken
	tokenCounterOnce sync.Once
)

// initTokenCounter initializes tiktoken for accurate token counting
func initTokenCounter() {
	tokenCounterOnce.Do(func() {
		tk, err := tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			log.Printf("[WARN] Token estimation will use fallback method: %v", err)
			return
		}
		tokenCounter = tk
	})
}

func estimateTokensFromStore(messages []storage.Message) int {
	initTokenCounter()

	if tokenCounter != nil {
		total := 0
		for _, m := range messages {
			tokens := tokenCounter.Encode(m.Content, nil, nil)
			total += len(tokens)
		}
		return total
	}

	// Fallback: rough estimate if tokenizer unavailable
	total := 0
	for _, m := range messages {
		ascii := 0
		nonASCII := 0
		for _, r := range m.Content {
			if r <= 127 {
				ascii++
			} else {
				nonASCII++
			}
		}
		total += ascii/4 + nonASCII*2 + 4
	}
	return total
}

func buildSummary(msgs []storage.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s: %s", m.Role, content))
	}
	return strings.Join(lines, "\n")
}

var providerKeywords = []struct {
	keyword  string
	provider llm.ProviderType
}{
	{"anthropic", llm.ProviderAnthropic},
	{"google", llm.ProviderGoogle},
	{"generativelanguage", llm.ProviderGoogle},
	{"minimax", llm.ProviderMiniMax},
	{"ollama", llm.ProviderOllama},
	{"openrouter", llm.ProviderOpenRouter},
	{"moonshot", llm.ProviderMoonshot},
	{"zhipu", llm.ProviderGLM},
	{"glm", llm.ProviderGLM},
	{"qianfan", llm.ProviderQianfan},
	{"bedrock", llm.ProviderBedrock},
	{"vercel", llm.ProviderVercel},
	{"z.ai", llm.ProviderZAi},
	{"z ai", llm.ProviderZAi},
}

// detectProviderType detects LLM provider type from BaseURL
func (a *Agent) detectProviderType() llm.ProviderType {
	baseURL := strings.ToLower(a.cfg.BaseURL)
	for _, pk := range providerKeywords {
		if strings.Contains(baseURL, pk.keyword) {
			return pk.provider
		}
	}
	return llm.ProviderOpenAI
}

// updateAnthropicRateLimit updates the last Anthropic API call timestamp
func (a *Agent) updateAnthropicRateLimit() {
	if a.detectProviderType() == llm.ProviderAnthropic {
		a.rateLimitMu.Lock()
		a.lastAnthropicCall = time.Now()
		a.rateLimitMu.Unlock()
	}
}

// handleContextOverflow estimates context tokens and applies pruning/compaction if needed
func (a *Agent) handleContextOverflow(sessionKey string, messages []Message) []Message {
	if a.store == nil {
		return messages
	}

	providerType := a.detectProviderType()
	contextWindow := llm.GetContextWindow(providerType, a.cfg.Model, a.cfg.BaseURL, a.cfg.APIKey, a.cfg.Models)
	if contextWindow <= 0 {
		contextWindow = a.cfg.ContextTokens
	}
	reserveTokens := a.cfg.ReserveTokens
	if reserveTokens <= 0 {
		reserveTokens = 5000
	}
	threshold := contextWindow - reserveTokens

	log.Printf("[Context] Model=%s context_window=%d reserve=%d threshold=%d",
		a.cfg.Model, contextWindow, reserveTokens, threshold)

	stored, err := a.store.GetMessages(sessionKey, 500)
	if err != nil || len(stored) == 0 {
		return messages
	}

	currentTokens := estimateTokensFromStore(stored)
	newMessageTokens := estimateTokens(messages)
	totalTokens := currentTokens + newMessageTokens

	if totalTokens <= threshold {
		return messages
	}

	log.Printf("[STATS] Context overflow detected: current=%d + new=%d = %d > threshold=%d",
		currentTokens, newMessageTokens, totalTokens, threshold)

	// Step 1: Try pruning first
	if a.cfg.ContextPruning.Mode == "cache-ttl" {
		originalLen := len(messages)
		messages = a.pruneMessages(sessionKey, messages)
		if len(messages) != originalLen {
			log.Printf("[PRUNE] Pruning reduced messages: %d -> %d", originalLen, len(messages))
		}

		prunedTokens := estimateTokens(messages)
		if currentTokens+prunedTokens <= threshold {
			log.Printf("[OK] Pruning sufficient: %d + %d <= %d", currentTokens, prunedTokens, threshold)
			return messages
		}
	}

	// Step 2: If pruning not enough, trigger compaction
	log.Printf("[CLEAN] Pruning not enough, triggering compaction...")
	compactChan := make(chan bool, 1)
	go func() {
		storedMsgs := convertStoredMessages(stored)
		a.maybeCompact(sessionKey, storedMsgs, compactChan)
	}()

	select {
	case compacted := <-compactChan:
		if compacted {
			log.Printf("[RELOAD] Compaction happened, reloading messages...")
			reloaded, err := a.store.GetMessages(sessionKey, 500)
			if err == nil && len(reloaded) > 0 {
				messages = convertStoredMessages(reloaded)
				if a.cfg.AutoRecall && a.memoryStore != nil && len(messages) > 0 {
					lastUserMsg := messages[len(messages)-1].Content
					if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
						injected := Message{Role: "system", Content: memories}
						messages = append([]Message{injected}, messages...)
					}
				}
			}
		}
	case <-time.After(2 * time.Second):
		log.Printf("[WARN] Compaction timed out, continuing...")
	}

	return messages
}

func estimateTokensForString(content string) int {
	ascii := 0
	nonASCII := 0
	for _, r := range content {
		if r <= 127 {
			ascii++
		} else {
			nonASCII++
		}
	}
	return ascii/4 + int(float64(nonASCII)*1.5)
}

// estimateTokens estimates token count for agent Messages (not storage.Message)
func estimateTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateTokensForString(m.Content)
		for _, tc := range m.ToolCalls {
			total += estimateTokensForString(tc.Function.Arguments)
		}
		for _, tr := range m.ToolExecutionResults {
			if tr.Result != nil {
				if s, ok := tr.Result.(string); ok {
					total += estimateTokensForString(s)
				}
			}
		}
	}
	return total
}

// convertStoredMessages converts storage.Message to agent Message
func convertStoredMessages(stored []storage.Message) []Message {
	result := make([]Message, 0, len(stored))
	for _, m := range stored {
		result = append(result, Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return result
}
