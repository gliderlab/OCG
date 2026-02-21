package agent

import (
	"log"
	"sync"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
)

const (
	realtimeIdleTimeout   = 5 * time.Minute
	realtimeJanitorTicker = 60 * time.Second
)

func (a *Agent) startRealtimeJanitor() {
	go func() {
		ticker := time.NewTicker(realtimeJanitorTicker)
		defer ticker.Stop()
		for range ticker.C {
			closed := a.closeIdleRealtimeSessions(realtimeIdleTimeout)
			if closed > 0 {
				log.Printf("[realtime] closed %d idle session(s)", closed)
			}
		}
	}()
}

func (a *Agent) getCachedRealtime(sessionKey string) (llm.RealtimeProvider, bool) {
	a.realtimeMu.Lock()
	defer a.realtimeMu.Unlock()
	p, ok := a.realtimeSessions[sessionKey]
	if !ok || p == nil || !p.IsConnected() {
		return nil, false
	}
	a.realtimeLastUsed[sessionKey] = time.Now()
	return p, true
}

func (a *Agent) cacheRealtime(sessionKey string, provider llm.RealtimeProvider) {
	a.realtimeMu.Lock()
	defer a.realtimeMu.Unlock()
	a.realtimeSessions[sessionKey] = provider
	a.realtimeLastUsed[sessionKey] = time.Now()
}

func (a *Agent) touchRealtimeInMemory(sessionKey string) {
	a.realtimeMu.Lock()
	defer a.realtimeMu.Unlock()
	if _, ok := a.realtimeSessions[sessionKey]; ok {
		a.realtimeLastUsed[sessionKey] = time.Now()
	}
}

func (a *Agent) getRealtimeSessionLock(sessionKey string) func() {
	a.realtimeMu.Lock()
	defer a.realtimeMu.Unlock()
	if _, ok := a.realtimeSessionMu[sessionKey]; !ok {
		a.realtimeSessionMu[sessionKey] = &sync.Mutex{}
	}
	mu := a.realtimeSessionMu[sessionKey]
	mu.Lock()
	return func() { mu.Unlock() }
}

func (a *Agent) closeIdleRealtimeSessions(idleTimeout time.Duration) int {
	if idleTimeout <= 0 {
		return 0
	}
	now := time.Now()
	closed := 0

	a.realtimeMu.Lock()
	for key, p := range a.realtimeSessions {
		last, ok := a.realtimeLastUsed[key]
		if !ok {
			last = now
			a.realtimeLastUsed[key] = last
		}
		if now.Sub(last) < idleTimeout {
			continue
		}
		if p != nil {
			_ = p.Disconnect()
		}
		delete(a.realtimeSessions, key)
		delete(a.realtimeLastUsed, key)
		closed++
	}
	a.realtimeMu.Unlock()

	return closed
}
