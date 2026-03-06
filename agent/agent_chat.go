// agent_chat.go - core chat routing: ChatWithSession, Chat, chatInternal, and realtime dispatch
package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
	googleprovider "github.com/gliderlab/cogate/pkg/llm/providers/google"
)

func realtimeDirective(last string) string {
	l := strings.ToLower(strings.TrimSpace(last))
	switch {
	case strings.HasPrefix(l, "/text "), strings.HasPrefix(l, "/http "):
		return "force_http"
	case strings.HasPrefix(l, "/live-audio-file "), strings.HasPrefix(l, "/live "), strings.HasPrefix(l, "/voice "), strings.HasPrefix(l, "/audio "):
		return "force_live"
	default:
		return "auto"
	}
}

func looksLikeAudioInput(last string) bool {
	l := strings.ToLower(strings.TrimSpace(last))
	return strings.HasPrefix(l, "data:audio/") ||
		strings.HasPrefix(l, "[audio]") ||
		strings.Contains(l, "mime:audio/") ||
		strings.Contains(l, `\"type\":\"audio\"`) ||
		strings.Contains(l, "voice message")
}

func (a *Agent) shouldUseRealtime(sessionKey string, messages []Message) bool {
	if sessionKey == "" || len(messages) == 0 {
		return false
	}

	last := strings.TrimSpace(messages[len(messages)-1].Content)
	switch realtimeDirective(last) {
	case "force_http":
		return false
	case "force_live":
		return true
	}

	if looksLikeAudioInput(last) {
		return true
	}

	if strings.HasPrefix(sessionKey, "live:") || strings.HasPrefix(sessionKey, "realtime:") {
		return true
	}
	if a.store != nil {
		if meta, err := a.store.GetSessionMeta(sessionKey); err == nil && meta.ProviderType == "live" {
			return true
		}
	}
	return false
}

func (a *Agent) realtimeModel() string {
	m := strings.TrimSpace(a.cfg.Model)
	if strings.Contains(strings.ToLower(m), "gemini") {
		return m
	}
	return "models/gemini-2.5-flash-native-audio-preview-12-2025"
}

func (a *Agent) getRealtimeProvider(sessionKey string) (llm.RealtimeProvider, error) {
	if p, ok := a.getCachedRealtime(sessionKey); ok {
		return p, nil
	}

	// Priority: GEMINI_API_KEY -> GOOGLE_API_KEY -> config API key
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(a.cfg.APIKey)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("google live API key not configured (set GEMINI_API_KEY or GOOGLE_API_KEY env var)")
	}
	shortKey := "****"
	if len(apiKey) > 4 {
		shortKey = apiKey[:4] + "****"
	}
	log.Printf("[realtime] Using API key: %s", shortKey)

	cfg := llm.RealtimeConfig{
		Model:                    a.realtimeModel(),
		APIKey:                   apiKey,
		Voice:                    "Kore",
		InputAudioTranscription:  true,
		OutputAudioTranscription: true,
	}
	provider := googleprovider.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: cfg.Model})
	rt, err := provider.Realtime(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if err := rt.Connect(context.Background(), cfg); err != nil {
		return nil, err
	}

	a.cacheRealtime(sessionKey, rt)
	return rt, nil
}

func (a *Agent) chatWithRealtimeSession(sessionKey string, messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	unlock := a.getRealtimeSessionLock(sessionKey)
	defer unlock()

	lastMsg := strings.TrimSpace(messages[len(messages)-1].Content)
	audioFile := ""
	lowerLast := strings.ToLower(lastMsg)
	if strings.HasPrefix(lowerLast, "/live-audio-file ") {
		audioFile = strings.TrimSpace(lastMsg[len("/live-audio-file "):])
		lastMsg = ""
	} else {
		for _, pfx := range []string{"/live ", "/voice ", "/audio ", "[audio]"} {
			if strings.HasPrefix(strings.ToLower(lastMsg), pfx) {
				lastMsg = strings.TrimSpace(lastMsg[len(pfx):])
				break
			}
		}
	}
	if lastMsg == "" && audioFile == "" {
		return "(live) empty input"
	}

	rt, err := a.getRealtimeProvider(sessionKey)
	if err != nil {
		log.Printf("[realtime] init failed: %v, falling back to http", err)
		return a.fallbackToHTTP(sessionKey, messages, err.Error())
	}

	if a.store != nil {
		storedUser := lastMsg
		if audioFile != "" && storedUser == "" {
			storedUser = "[voice_input]"
		}
		a.storeMessage(sessionKey, "user", storedUser)
		_ = a.store.SetSessionProviderType(sessionKey, "live")
		_ = a.store.TouchRealtimeSession(sessionKey)
	}

	var (
		mu         sync.Mutex
		textBuf    strings.Builder
		lastUpdate time.Time
		errReason  string
	)
	rt.OnText(func(text string) {
		if text == "" {
			return
		}
		mu.Lock()
		textBuf.WriteString(text)
		lastUpdate = time.Now()
		mu.Unlock()
	})
	rt.OnError(func(err error) {
		mu.Lock()
		if errReason == "" {
			errReason = err.Error()
		}
		mu.Unlock()
	})

	if audioFile != "" {
		pcmData, err := os.ReadFile(audioFile)
		_ = os.Remove(audioFile)
		if err != nil {
			log.Printf("[realtime] read audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
		if err := rt.SendAudio(context.Background(), pcmData); err != nil {
			log.Printf("[realtime] send audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
		if err := rt.EndAudio(context.Background()); err != nil {
			log.Printf("[realtime] end audio failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
	}
	if lastMsg != "" {
		if err := rt.SendText(context.Background(), lastMsg); err != nil {
			log.Printf("[realtime] send text failed: %v, falling back to http", err)
			return a.fallbackToHTTP(sessionKey, messages, err.Error())
		}
	}
	a.touchRealtimeInMemory(sessionKey)

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(120 * time.Millisecond)
		mu.Lock()
		content := strings.TrimSpace(textBuf.String())
		lu := lastUpdate
		errMsg := errReason
		mu.Unlock()
		if errMsg != "" {
			log.Printf("[realtime] error during session: %s, falling back to http", errMsg)
			return a.fallbackToHTTP(sessionKey, messages, errMsg)
		}
		if content != "" && !lu.IsZero() && time.Since(lu) > 800*time.Millisecond {
			if a.store != nil {
				a.storeMessage(sessionKey, "assistant", content)
				_ = a.store.TouchRealtimeSession(sessionKey)
			}
			a.touchRealtimeInMemory(sessionKey)
			return content
		}
	}

	mu.Lock()
	content := strings.TrimSpace(textBuf.String())
	errMsg := errReason
	mu.Unlock()
	if content == "" {
		if errMsg != "" {
			log.Printf("[realtime] session error: %s, falling back to http", errMsg)
			return a.fallbackToHTTP(sessionKey, messages, errMsg)
		}
		content = "(live) no response received"
	}
	if a.store != nil {
		a.storeMessage(sessionKey, "assistant", content)
		_ = a.store.TouchRealtimeSession(sessionKey)
	}
	a.touchRealtimeInMemory(sessionKey)
	return content
}

// SendAudioChunk sends audio data to an active live session (for streaming voice input)
func (a *Agent) SendAudioChunk(sessionKey string, pcmData []byte) error {
	a.realtimeMu.Lock()
	rt, ok := a.realtimeSessions[sessionKey]
	a.realtimeMu.Unlock()
	if !ok || rt == nil || !rt.IsConnected() {
		return fmt.Errorf("no active live session for %s", sessionKey)
	}
	if len(pcmData) == 0 {
		return nil
	}
	return rt.SendAudio(context.Background(), pcmData)
}

// EndAudioStream signals end of audio stream for a live session
func (a *Agent) EndAudioStream(sessionKey string) error {
	a.realtimeMu.Lock()
	rt, ok := a.realtimeSessions[sessionKey]
	a.realtimeMu.Unlock()
	if !ok || rt == nil || !rt.IsConnected() {
		return fmt.Errorf("no active live session for %s", sessionKey)
	}
	return rt.EndAudio(context.Background())
}

func (a *Agent) fallbackToHTTP(sessionKey string, messages []Message, reason string) string {
	log.Printf("[realtime->http] falling back: %s", reason)
	cleaned := make([]Message, len(messages))
	copy(cleaned, messages)
	for i := range cleaned {
		cleaned[i].Content = strings.TrimSpace(cleaned[i].Content)
		for _, pfx := range []string{"/live ", "/voice ", "/audio ", "/live-audio-file ", "[audio]"} {
			if strings.HasPrefix(strings.ToLower(cleaned[i].Content), pfx) {
				cleaned[i].Content = strings.TrimSpace(cleaned[i].Content[len(pfx):])
				break
			}
		}
	}
	if len(cleaned) > 0 {
		cleaned[len(cleaned)-1].Content = "[realtime-fallback] " + cleaned[len(cleaned)-1].Content
	}
	return a.ChatWithSession(sessionKey, cleaned)
}

func (a *Agent) ChatWithSession(sessionKey string, messages []Message) string {
	if len(messages) > 0 {
		idx := len(messages) - 1
		last := strings.TrimSpace(messages[idx].Content)
		if strings.HasPrefix(strings.ToLower(last), "/text ") {
			messages[idx].Content = strings.TrimSpace(last[len("/text "):])
		} else if strings.HasPrefix(strings.ToLower(last), "/http ") {
			messages[idx].Content = strings.TrimSpace(last[len("/http "):])
		}
	}

	if a.shouldUseRealtime(sessionKey, messages) {
		return a.chatWithRealtimeSession(sessionKey, messages)
	}

	isNewSession := false

	// Load history for this session if store is available
	if a.store != nil && sessionKey != "" {
		history, err := a.store.GetMessages(sessionKey, 100)
		if err == nil && len(history) > 0 {
			histMsgs := make([]Message, len(history))
			for i, m := range history {
				histMsgs[i] = Message{
					Role:    m.Role,
					Content: m.Content,
				}
			}
			messages = append(histMsgs, messages...)
			log.Printf("[HISTORY] Loaded %d historical messages for session %s", len(history), sessionKey)
		} else {
			isNewSession = true
		}
	}

	// Inject BOOT.md for new sessions
	if isNewSession {
		if bootMsg := a.loadBootMD(); bootMsg != nil {
			messages = append([]Message{*bootMsg}, messages...)
			log.Printf("[BOOT] BOOT.md injected as system message")
		}
	}

	return a.chatInternal(sessionKey, messages)
}

func (a *Agent) Chat(messages []Message) string {
	return a.chatInternal("default", messages)
}

// chatInternal is the core chat logic with explicit sessionKey to avoid double-storing.
func (a *Agent) chatInternal(sessionKey string, messages []Message) string {
	lastMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastMsg = messages[i].Content
			break
		}
	}
	finalize := func(reply string) string {
		// Strip thinking tags if thinking mode is not enabled (default: off)
		if a.thinkingConfig.Mode == "" || a.thinkingConfig.Mode == ThinkingModeOff {
			reply = StripThinkingTags(reply)
		}
		if a.store != nil && lastMsg != "" && !strings.HasPrefix(lastMsg, "[realtime-fallback]") {
			a.storeMessage(sessionKey, "user", lastMsg)
			a.storeMessage(sessionKey, "assistant", reply)
		}
		return reply
	}

	// NOTE: realtime/live mode check was removed here to prevent double-checking
	// and fallback loops. It is correctly handled in ChatWithSession.

	if out, ok := a.runCommandIfRequested(lastMsg); ok {
		return finalize(out)
	}

	// Shared preprocessing: auto memory capture + flush + compaction
	a.preprocessChat(sessionKey, lastMsg, messages)

	// Handle tool calls
	if len(messages) > 0 && len(messages[len(messages)-1].ToolCalls) > 0 {
		return finalize(a.handleToolCalls(messages, messages[len(messages)-1].ToolCalls, nil, 0, nil))
	}

	// Detect edit intent
	if len(messages) > 0 {
		lastUserMsg := messages[len(messages)-1].Content
		if editArgs := detectEditIntent(lastUserMsg); editArgs != nil {
			return finalize(a.handleEdit(editArgs))
		}
	}

	// Explicit recall trigger
	if len(messages) > 0 && a.memoryStore != nil {
		lastUserMsg := messages[len(messages)-1].Content
		if isRecallRequest(lastUserMsg) {
			if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
				log.Printf("recall command injected %d memories", strings.Count(memories, "- ["))
				injected := Message{Role: "system", Content: memories}
				messages = append([]Message{injected}, messages...)
			}
		}
	}

	// Auto recall: inject relevant memories as a system message before sending to model
	if a.cfg.AutoRecall && a.memoryStore != nil && len(messages) > 0 {
		lastUserMsg := messages[len(messages)-1].Content
		if memories := a.recallRelevantMemories(lastUserMsg); memories != "" {
			log.Printf("auto-recall injected %d memories", strings.Count(memories, "- ["))
			injected := Message{Role: "system", Content: memories}
			messages = append([]Message{injected}, messages...)
		}
	}

	// Inject core System Prompt
	sysPrompt := Message{Role: "system", Content: a.GetSystemPrompt()}
	messages = append([]Message{sysPrompt}, messages...)

	// overflow handling
	if a.store != nil {
		messages = a.handleContextOverflow(sessionKey, messages)
	}

	if a.cfg.APIKey == "" {
		return finalize(a.simpleResponse(messages))
	}

	return finalize(a.callAPI(messages))
}
