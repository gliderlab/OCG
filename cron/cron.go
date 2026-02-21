// Cron job system for OCG-Go
// Provides scheduled task execution with multiple schedule types and delivery modes

package cron

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Schedule kinds
const (
	ScheduleKindAt    = "at"
	ScheduleKindEvery = "every"
	ScheduleKindCron  = "cron"
)

// Session targets
const (
	SessionTargetMain     = "main"
	SessionTargetIsolated = "isolated"
)

// Wake modes
const (
	WakeModeNow           = "now"
	WakeModeNextHeartbeat = "next-heartbeat"
)

// Delivery modes
const (
	DeliveryModeAnnounce = "announce"
	DeliveryModeNone     = "none"
)

// Payload kinds
const (
	PayloadKindSystemEvent = "systemEvent"
	PayloadKindAgentTurn   = "agentTurn"
)

// Schedule defines when a job should run
type Schedule struct {
	Kind      string `json:"kind"`               // "at", "every", "cron"
	At        string `json:"at,omitempty"`       // ISO 8601 timestamp (RFC3339)
	EveryMs   int64  `json:"everyMs,omitempty"`  // milliseconds
	Expr      string `json:"expr,omitempty"`     // cron expression (5 or 6 fields)
	Tz        string `json:"tz,omitempty"`       // IANA timezone
	StaggerMs int64  `json:"staggerMs,omitempty"` // stagger window in milliseconds
	AnchorMs  int64  `json:"anchorMs,omitempty"` // anchor point for every scheduling
}

// Payload defines what the job should do
type Payload struct {
	Kind           string `json:"kind"`              // "systemEvent", "agentTurn"
	Text           string `json:"text,omitempty"`    // for systemEvent
	Message        string `json:"message,omitempty"` // for agentTurn
	Model          string `json:"model,omitempty"`
	Thinking       string `json:"thinking,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

// Delivery defines how to deliver job output
type Delivery struct {
	Mode       string `json:"mode"`               // "announce", "webhook", "none"
	Channel    string `json:"channel,omitempty"`   // "telegram", "discord", etc.
	To         string `json:"to,omitempty"`       // channel-specific target or webhook URL
	BestEffort bool   `json:"bestEffort"`         // don't fail job if delivery fails
	Webhook    string `json:"webhook,omitempty"`   // explicit webhook URL (alternative to "to")
}

// Job represents a scheduled job
type Job struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	AgentID        string    `json:"agentId,omitempty"` // specific agent or empty for default
	Enabled        bool      `json:"enabled"`
	Schedule       Schedule  `json:"schedule"`
	SessionTarget  string    `json:"sessionTarget"` // "main" or "isolated"
	WakeMode       string    `json:"wakeMode"`      // "now" or "next-heartbeat"
	Payload        Payload   `json:"payload"`
	Delivery       *Delivery `json:"delivery,omitempty"`
	DeleteAfterRun bool      `json:"deleteAfterRun"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	// State
	State struct {
		NextRunAtMs       int64  `json:"nextRunAtMs"`
		LastRunAtMs       int64  `json:"lastRunAtMs"`
		LastStatus        string `json:"lastStatus"` // "ok", "error", "skipped"
		LastDurationMs    int64  `json:"lastDurationMs"`
		ConsecutiveErrors int    `json:"consecutiveErrors"`
	} `json:"state"`
}

// RunHistoryEntry represents a single run of a cron job
type RunHistoryEntry struct {
	JobID        string    `json:"jobId"`
	JobName      string    `json:"jobName"`
	StartedAtMs  int64     `json:"startedAtMs"`
	EndedAtMs    int64     `json:"endedAtMs"`
	Status       string    `json:"status"`       // "ok", "error", "skipped"
	DurationMs   int64     `json:"durationMs"`
	Error        string    `json:"error,omitempty"`
	Result       string    `json:"result,omitempty"` // output from agentTurn
}

// JobStore manages cron jobs
type JobStore struct {
	mu           sync.RWMutex
	jobs         map[string]*Job
	runs         map[string][]RunHistoryEntry // jobId -> run history
	runsFilePath string
	filePath     string
}

// NewJobStore creates a new job store
func NewJobStore(filePath string) *JobStore {
	js := &JobStore{
		jobs:         make(map[string]*Job),
		runs:         make(map[string][]RunHistoryEntry),
		filePath:     filePath,
		runsFilePath: filePath + ".runs",
	}
	js.load()
	js.loadRuns()
	return js
}

// load loads jobs from file
func (js *JobStore) load() {
	data, err := os.ReadFile(js.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Cron] Failed to load jobs: %v", err)
		}
		return
	}

	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		log.Printf("[Cron] Failed to parse jobs: %v", err)
		return
	}

	js.mu.Lock()
	defer js.mu.Unlock()
	for _, job := range jobs {
		js.jobs[job.ID] = job
	}
	log.Printf("[Cron] Loaded %d jobs", len(jobs))
}

// loadRuns loads run history from file
func (js *JobStore) loadRuns() {
	data, err := os.ReadFile(js.runsFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Cron] Failed to load runs: %v", err)
		}
		return
	}

	var runs map[string][]RunHistoryEntry
	if err := json.Unmarshal(data, &runs); err != nil {
		log.Printf("[Cron] Failed to parse runs: %v", err)
		return
	}

	js.mu.Lock()
	defer js.mu.Unlock()
	js.runs = runs
	log.Printf("[Cron] Loaded run history for %d jobs", len(runs))
}

// saveRuns saves run history to file (caller must hold write lock)
func (js *JobStore) saveRuns() {
	data, err := json.MarshalIndent(js.runs, "", "  ")
	if err != nil {
		log.Printf("[Cron] Failed to marshal runs: %v", err)
		return
	}

	if err := writeFileAtomic(js.runsFilePath, data); err != nil {
		log.Printf("[Cron] Failed to write runs: %v", err)
	}
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

// AddRun adds a run history entry
func (js *JobStore) AddRun(jobId string, entry RunHistoryEntry) {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.runs[jobId] = append(js.runs[jobId], entry)
	// Keep only last 100 runs per job
	if len(js.runs[jobId]) > 100 {
		js.runs[jobId] = js.runs[jobId][len(js.runs[jobId])-100:]
	}
	js.saveRuns()
}

// GetRuns returns run history for a job
func (js *JobStore) GetRuns(jobId string, limit int) []RunHistoryEntry {
	js.mu.RLock()
	defer js.mu.RUnlock()

	runs := js.runs[jobId]
	if len(runs) == 0 {
		return []RunHistoryEntry{}
	}
	if limit <= 0 || limit > len(runs) {
		limit = len(runs)
	}
	return runs[len(runs)-limit:]
}

// save saves jobs to file (caller must hold write lock)
func (js *JobStore) save() {
	// Note: caller must hold js.mu.Lock() before calling this method
	// This avoids deadlock where save() tries to acquire RLock while caller holds Lock

	jobs := make([]*Job, 0, len(js.jobs))
	for _, job := range js.jobs {
		jobs = append(jobs, job)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		log.Printf("[Cron] Failed to marshal jobs: %v", err)
		return
	}

	if err := writeFileAtomic(js.filePath, data); err != nil {
		log.Printf("[Cron] Failed to write jobs: %v", err)
	}
}

// Add adds a new job
func (js *JobStore) Add(job *Job) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.jobs[job.ID] = job
	// save() is called with lock held - no deadlock
	js.save()
	return nil
}

// Get returns a job by ID
func (js *JobStore) Get(id string) (*Job, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	job, ok := js.jobs[id]
	return job, ok
}

// GetUnsafe returns a job by ID (caller must hold js.mu.Lock())
func (js *JobStore) GetUnsafe(id string) (*Job, bool) {
	job, ok := js.jobs[id]
	return job, ok
}

// LockJob acquires a write lock for a specific job
func (js *JobStore) LockJob(id string) {
	js.mu.Lock()
}

// UnlockJob releases the write lock for a specific job
func (js *JobStore) UnlockJob(id string) {
	js.mu.Unlock()
}

// Save persists jobs to file (exposed for external callers)
func (js *JobStore) Save() {
	js.mu.Lock()
	js.save()
	js.mu.Unlock()
}

// List returns all jobs
func (js *JobStore) List() []*Job {
	js.mu.RLock()
	defer js.mu.RUnlock()

	jobs := make([]*Job, 0, len(js.jobs))
	for _, job := range js.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// Update updates a job
func (js *JobStore) Update(id string, updates map[string]interface{}) (*Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	// Apply updates
	if v, ok := updates["name"].(string); ok {
		job.Name = v
	}
	if v, ok := updates["description"].(string); ok {
		job.Description = v
	}
	if v, ok := updates["enabled"].(bool); ok {
		job.Enabled = v
	}
	if v, ok := updates["schedule"].(map[string]interface{}); ok {
		if kind, ok := v["kind"].(string); ok {
			job.Schedule.Kind = kind
		}
		if at, ok := v["at"].(string); ok {
			job.Schedule.At = at
		}
		if everyMs, ok := v["everyMs"].(float64); ok {
			job.Schedule.EveryMs = int64(everyMs)
		}
		if expr, ok := v["expr"].(string); ok {
			job.Schedule.Expr = expr
		}
		if tz, ok := v["tz"].(string); ok {
			job.Schedule.Tz = tz
		}
	}
	if v, ok := updates["payload"].(map[string]interface{}); ok {
		if kind, ok := v["kind"].(string); ok {
			job.Payload.Kind = kind
		}
		if text, ok := v["text"].(string); ok {
			job.Payload.Text = text
		}
		if message, ok := v["message"].(string); ok {
			job.Payload.Message = message
		}
		if model, ok := v["model"].(string); ok {
			job.Payload.Model = model
		}
		if thinking, ok := v["thinking"].(string); ok {
			job.Payload.Thinking = thinking
		}
	}

	job.UpdatedAt = time.Now()
	js.jobs[id] = job

	// save() is called with lock held - no deadlock
	js.save()

	return job, nil
}

// Remove removes a job
func (js *JobStore) Remove(id string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	if _, ok := js.jobs[id]; !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	delete(js.jobs, id)
	// save() is called with lock held - no deadlock
	js.save()
	return nil
}

// GetDueJobs returns jobs that are due to run
func (js *JobStore) GetDueJobs() []*Job {
	js.mu.RLock()
	defer js.mu.RUnlock()

	now := time.Now().UnixMilli()
	var due []*Job

	for _, job := range js.jobs {
		if !job.Enabled {
			continue
		}
		if job.State.NextRunAtMs > 0 && job.State.NextRunAtMs <= now {
			due = append(due, job)
		}
	}

	return due
}

// CalculateNextRun calculates the next run time for a job
func (js *JobStore) CalculateNextRun(job *Job) int64 {
	// Determine timezone
	loc := time.Local
	if job.Schedule.Tz != "" {
		if l, err := time.LoadLocation(job.Schedule.Tz); err == nil {
			loc = l
		}
	}
	now := time.Now().In(loc)

	var baseNext int64

	switch job.Schedule.Kind {
	case ScheduleKindAt:
		if job.Schedule.At == "" {
			return 0
		}
		// Support both RFC3339 and simple ISO8601 without timezone
		t, err := time.Parse(time.RFC3339, job.Schedule.At)
		if err != nil {
			// Try parsing as simple datetime (assume UTC)
			t, err = time.Parse("2006-01-02T15:04:05", job.Schedule.At)
			if err != nil {
				return 0
			}
			t = t.UTC() // No timezone = UTC
		}
		baseNext = t.UnixMilli()

	case ScheduleKindEvery:
		if job.Schedule.EveryMs <= 0 {
			return 0
		}
		// Apply anchorMs if specified
		if job.Schedule.AnchorMs > 0 {
			// Align to anchor point
			nowMs := now.UnixMilli()
			anchor := job.Schedule.AnchorMs
			if nowMs < anchor {
				baseNext = anchor
			} else {
				elapsed := nowMs - anchor
				intervals := elapsed / job.Schedule.EveryMs
				baseNext = anchor + (intervals+1)*job.Schedule.EveryMs
			}
		} else {
			baseNext = now.Add(time.Duration(job.Schedule.EveryMs) * time.Millisecond).UnixMilli()
		}

	case ScheduleKindCron:
		// Simple cron parser for common patterns (supports 5 and 6 fields)
		return js.parseCronExpression(job.Schedule.Expr, now)

	default:
		return 0
	}

	// Apply staggerMs if specified (deterministic stagger based on job ID)
	if job.Schedule.StaggerMs > 0 {
		// Use job ID hash for deterministic stagger
		hash := int64(0)
		for _, c := range job.ID {
			hash = hash*31 + int64(c)
		}
		stagger := (hash % job.Schedule.StaggerMs)
		baseNext += stagger
	}

	// If the calculated time is in the past, advance to next occurrence
	for baseNext <= now.UnixMilli() {
		switch job.Schedule.Kind {
		case ScheduleKindAt:
			// For "at", don't advance - return 0 if in past
			return 0
		case ScheduleKindEvery:
			baseNext += job.Schedule.EveryMs
		case ScheduleKindCron:
			// For cron, recalculate
			return js.parseCronExpression(job.Schedule.Expr, now.Add(time.Duration(baseNext-now.UnixMilli())*time.Millisecond))
		}
	}

	return baseNext
}

// parseCronExpression handles common cron expression patterns
// Supports: minute, hour, day, month, weekday
// Examples: "0 * * * *" (hourly), "0 0 * * *" (daily), "0 0 * * 0" (weekly)
// Also supports: */n (every n), n,m,o (list), n-m (range)
func (js *JobStore) parseCronExpression(expr string, now time.Time) int64 {
	if expr == "" {
		return now.Add(1 * time.Hour).UnixMilli()
	}

	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return now.Add(1 * time.Hour).UnixMilli()
	}

	// Support both 5-field and 6-field (with seconds) cron expressions
	// 5-field: minute hour day month weekday
	// 6-field: second minute hour day month weekday
	var second, minute, hour, day, month, weekday string
	if len(parts) >= 6 {
		// 6-field: second minute hour day month weekday
		second = parts[0]
		minute = parts[1]
		hour = parts[2]
		day = parts[3]
		month = parts[4]
		weekday = parts[5]
	} else {
		// 5-field: minute hour day month weekday
		second = "0"
		minute = parts[0]
		hour = parts[1]
		day = parts[2]
		month = parts[3]
		weekday = parts[4]
	}

	dayIsWildcard := strings.TrimSpace(day) == "*"
	weekdayIsWildcard := strings.TrimSpace(weekday) == "*"
	step := time.Minute
	if len(parts) >= 6 {
		step = time.Second
	}

	// Helper to parse field value
	parseField := func(field string, min, max int) []int {
		field = strings.TrimSpace(field)
		if field == "" {
			return []int{}
		}
		parseInt := func(val string) (int, bool) {
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return 0, false
			}
			return n, true
		}
		if field == "*" {
			result := make([]int, max-min+1)
			for i := range result {
				result[i] = min + i
			}
			return result
		}
		if strings.HasPrefix(field, "*/") {
			// Step values (e.g., */5)
			step, ok := parseInt(field[2:])
			if !ok || step <= 0 {
				return []int{}
			}
			result := []int{}
			for i := min; i <= max; i += step {
				result = append(result, i)
			}
			return result
		}
		if strings.Contains(field, ",") {
			// List values (e.g., 1,3,5)
			result := []int{}
			for _, v := range strings.Split(field, ",") {
				if n, ok := parseInt(v); ok && n >= min && n <= max {
					result = append(result, n)
				}
			}
			return result
		}
		if strings.Contains(field, "-") {
			// Range values (e.g., 1-5)
			rangeParts := strings.Split(field, "-")
			if len(rangeParts) == 2 {
				start, okStart := parseInt(rangeParts[0])
				end, okEnd := parseInt(rangeParts[1])
				if !okStart || !okEnd || start > end {
					return []int{}
				}
				result := []int{}
				for i := start; i <= end && i <= max; i++ {
					if i >= min {
						result = append(result, i)
					}
				}
				return result
			}
		}
		// Single value
		if n, ok := parseInt(field); ok {
			if n >= min && n <= max {
				return []int{n}
			}
		}
		return []int{}
	}

	seconds := parseField(second, 0, 59)
	minutes := parseField(minute, 0, 59)
	hours := parseField(hour, 0, 23)
	days := parseField(day, 1, 31)
	months := parseField(month, 1, 12)
	weekdays := parseField(weekday, 0, 6)

	if len(seconds) == 0 || len(minutes) == 0 || len(hours) == 0 || len(days) == 0 || len(months) == 0 || len(weekdays) == 0 {
		return now.Add(1 * time.Hour).UnixMilli()
	}

	// Check if we have very specific constraints
	isSpecific := len(months) == 1 || len(days) == 1 || len(hours) == 1

	// Calculate next run based on pattern
	next := now
	if step == time.Minute {
		next = next.Truncate(time.Minute)
	} else {
		next = next.Truncate(time.Second)
	}

	// Smart iteration limit based on specificity
	maxIterations := 10080 // 7 days * 24 hours * 60 minutes
	if step == time.Second {
		maxIterations = 7 * 24 * 60 * 60
	}
	if isSpecific {
		if step == time.Second {
			maxIterations = 31 * 24 * 60 * 60
		} else {
			maxIterations = 44640 // 31 days * 24 hours * 60 minutes for specific dates
		}
	}

	for i := 0; i < maxIterations; i++ {
		next = next.Add(step)
		m := int(next.Month())
		d := next.Day()
		h := next.Hour()
		min := next.Minute()
		sec := next.Second()
		wd := int(next.Weekday())

		// Check month
		if len(months) > 0 && !contains(months, m) {
			// Skip to first day of next matching month if day is specific
			if len(days) == 1 {
				// Find next month that matches
				for len(months) > 0 && !contains(months, m) {
					firstSec := 0
					if len(seconds) > 0 {
						firstSec = seconds[0]
					}
					next = time.Date(next.Year(), next.Month()+1, days[0], hours[0], minutes[0], firstSec, 0, next.Location())
					m = int(next.Month())
				}
				if contains(months, m) {
					return next.UnixMilli()
				}
			}
			continue
		}
		// Check day/weekday (cron semantics: if both specified, match either)
		dayMatch := len(days) > 0 && contains(days, d)
		weekdayMatch := len(weekdays) > 0 && contains(weekdays, wd)
		if !dayIsWildcard && !weekdayIsWildcard {
			if !dayMatch && !weekdayMatch {
				continue
			}
		} else {
			if !dayIsWildcard && !dayMatch {
				continue
			}
			if !weekdayIsWildcard && !weekdayMatch {
				continue
			}
		}
		// Check hour
		if len(hours) > 0 && !contains(hours, h) {
			continue
		}
		// Check minute
		if len(minutes) > 0 && !contains(minutes, min) {
			continue
		}
		// Check second (for 6-field expressions)
		if len(seconds) > 0 && !contains(seconds, sec) {
			continue
		}
		// Found match
		return next.UnixMilli()
	}

	// Fallback: return 1 hour from now if no match found within limit
	return now.Add(1 * time.Hour).UnixMilli()
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// CronHandler manages the cron system
type CronHandler struct {
	store    *JobStore
	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
	interval time.Duration
	// Callbacks
	onSystemEvent func(string)                                      // (message)
	onAgentTurn   func(string, string, string) (string, error)    // (message, model, thinking)
	onBroadcast  func(string, string, string) error                // (message, channel, target)
	onWebhook    func(string, string) error                        // (url, payload) - for webhook delivery
	onWake       func() error                                       // trigger heartbeat for main session
}

// NewCronHandler creates a new cron handler
func NewCronHandler(storePath string) *CronHandler {
	return &CronHandler{
		store:    NewJobStore(storePath),
		stopCh:   make(chan struct{}),
		interval: 1 * time.Second,
	}
}

// SetSystemEventCallback sets the callback for system events
func (c *CronHandler) SetSystemEventCallback(cb func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSystemEvent = cb
}

// SetAgentTurnCallback sets the callback for agent turns
func (c *CronHandler) SetAgentTurnCallback(cb func(string, string, string) (string, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAgentTurn = cb
}

// SetBroadcastCallback sets the callback for broadcasting
func (c *CronHandler) SetBroadcastCallback(cb func(string, string, string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onBroadcast = cb
}

// SetWebhookCallback sets the callback for webhook delivery
func (c *CronHandler) SetWebhookCallback(cb func(string, string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onWebhook = cb
}

// SetWakeCallback sets the callback for wake (heartbeat trigger)
func (c *CronHandler) SetWakeCallback(cb func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onWake = cb
}

// Start starts the cron scheduler
func (c *CronHandler) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	log.Printf("[Cron] Starting cron scheduler")

	// Calculate initial next run times
	for _, job := range c.store.List() {
		nextRun := c.store.CalculateNextRun(job)
		job.State.NextRunAtMs = nextRun
	}
	c.store.Save()

	go c.runLoop()
}

// Stop stops the cron scheduler
func (c *CronHandler) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	c.mu.Unlock()

	log.Printf("[Cron] Stopped cron scheduler")
}

// IsRunning returns whether the cron is running
func (c *CronHandler) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// runLoop runs the main cron loop
func (c *CronHandler) runLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.tick()
		}
	}
}

// tick performs one cron check
// Fix: Run independent jobs in parallel for better throughput
func (c *CronHandler) tick() {
	dueJobs := c.store.GetDueJobs()
	if len(dueJobs) == 0 {
		return
	}

	// Use a semaphore to limit concurrent job executions
	sem := make(chan struct{}, 4) // Max 4 concurrent jobs
	var wg sync.WaitGroup

	for _, job := range dueJobs {
		// Skip if already running (from Bug #4 fix)
		if job.State.LastStatus == "running" {
			continue
		}

		wg.Add(1)
		go func(j *Job) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore
			c.executeJob(j)
		}(job)
	}

	wg.Wait()
}

// executeJob runs a single job
func (c *CronHandler) executeJob(job *Job) {
	log.Printf("[Cron] Executing job: %s (%s)", job.Name, job.ID)

	startTime := time.Now()

	// Fix Bug #2: Wrap job state modifications in lock to prevent race conditions
	c.store.LockJob(job.ID)
	job.State.LastRunAtMs = startTime.UnixMilli()
	job.State.LastStatus = "running"
	c.store.UnlockJob(job.ID)

	var err error
	var result string

	// Handle wakeMode before execution
	if job.WakeMode == WakeModeNextHeartbeat {
		c.mu.RLock()
		wakeCb := c.onWake
		c.mu.RUnlock()
		if wakeCb != nil {
			_ = wakeCb() // Trigger heartbeat but don't wait
		}
	}

	// Execute based on payload kind
	switch job.Payload.Kind {
	case PayloadKindSystemEvent:
		// Execute in main session
		c.mu.RLock()
		cb := c.onSystemEvent
		c.mu.RUnlock()

		if cb != nil {
			cb(job.Payload.Text)
		} else {
			err = fmt.Errorf("no callback configured")
		}

		// Handle delivery for systemEvent (announce to main session)
		if job.Delivery != nil && job.Delivery.Mode == DeliveryModeAnnounce {
			c.mu.RLock()
			broadcastCb := c.onBroadcast
			c.mu.RUnlock()

			if broadcastCb != nil && job.Payload.Text != "" {
				_ = broadcastCb(job.Payload.Text, job.Delivery.Channel, job.Delivery.To) //nolint:errcheck
			}
		}

	case PayloadKindAgentTurn:
		// Execute as isolated agent turn
		c.mu.RLock()
		cb := c.onAgentTurn
		c.mu.RUnlock()

		if cb != nil {
			result, err = cb(job.Payload.Message, job.Payload.Model, job.Payload.Thinking)
		} else {
			err = fmt.Errorf("no callback configured")
		}

		// Handle delivery for agentTurn
		if job.Delivery != nil && result != "" {
			switch job.Delivery.Mode {
			case DeliveryModeAnnounce:
				c.mu.RLock()
				broadcastCb := c.onBroadcast
				c.mu.RUnlock()
				if broadcastCb != nil {
					deliverErr := broadcastCb(result, job.Delivery.Channel, job.Delivery.To)
					if deliverErr != nil && !job.Delivery.BestEffort {
						err = deliverErr
					}
				}
			case "webhook":
				// Webhook delivery
				c.mu.RLock()
				webhookCb := c.onWebhook
				c.mu.RUnlock()
				if webhookCb != nil {
					webhookURL := job.Delivery.Webhook
					if webhookURL == "" {
						webhookURL = job.Delivery.To
					}
					if webhookURL != "" {
						deliverErr := webhookCb(webhookURL, result)
						if deliverErr != nil && !job.Delivery.BestEffort {
							err = deliverErr
						}
					}
				}
			}
		}

	default:
		err = fmt.Errorf("unknown payload kind: %s", job.Payload.Kind)
	}

	// Update job state under lock
	c.store.LockJob(job.ID)
	job.State.LastDurationMs = time.Since(startTime).Milliseconds()

	if err != nil {
		job.State.LastStatus = "error"
		job.State.ConsecutiveErrors++
		log.Printf("[Cron] Job error: %s - %v", job.Name, err)
	} else {
		job.State.LastStatus = "ok"
		job.State.ConsecutiveErrors = 0
		log.Printf("[Cron] Job completed: %s", job.Name)
	}

	// Calculate next run
	job.State.NextRunAtMs = c.store.CalculateNextRun(job)

	// Handle one-shot jobs
	if job.Schedule.Kind == ScheduleKindAt && job.DeleteAfterRun {
		if job.State.LastStatus == "ok" || job.State.LastStatus == "error" {
			job.Enabled = false
		}
	}

	c.store.UnlockJob(job.ID)
	c.store.Save()

	// Record run history
	endTime := time.Now()
	runEntry := RunHistoryEntry{
		JobID:       job.ID,
		JobName:     job.Name,
		StartedAtMs: startTime.UnixMilli(),
		EndedAtMs:   endTime.UnixMilli(),
		Status:      job.State.LastStatus,
		DurationMs:  endTime.Sub(startTime).Milliseconds(),
	}
	if err != nil {
		runEntry.Error = err.Error()
	}
	if result != "" {
		runEntry.Result = result
	}
	c.store.AddRun(job.ID, runEntry)
}

// AddJob adds a new job
func (c *CronHandler) AddJob(job *Job) error {
	job.ID = generateJobID()
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	job.State.NextRunAtMs = c.store.CalculateNextRun(job)

	return c.store.Add(job)
}

// ListJobs returns all jobs
func (c *CronHandler) ListJobs() []*Job {
	return c.store.List()
}

// GetJob returns a job by ID
func (c *CronHandler) GetJob(id string) (*Job, bool) {
	return c.store.Get(id)
}

// UpdateJob updates a job
func (c *CronHandler) UpdateJob(id string, updates map[string]interface{}) (*Job, error) {
	job, err := c.store.Update(id, updates)
	if err != nil {
		return nil, err
	}
	job.State.NextRunAtMs = c.store.CalculateNextRun(job)
	c.store.Save()
	return job, nil
}

// RemoveJob removes a job
func (c *CronHandler) RemoveJob(id string) error {
	return c.store.Remove(id)
}

// RunJob immediately runs a job
// Fix D: Add concurrency check to prevent same job running multiple times simultaneously
func (c *CronHandler) RunJob(id string) error {
	c.store.LockJob(id)
	job, ok := c.store.GetUnsafe(id)
	if !ok {
		c.store.UnlockJob(id)
		return fmt.Errorf("job not found: %s", id)
	}

	// Check if job is already running
	if job.State.LastStatus == "running" {
		c.store.UnlockJob(id)
		return fmt.Errorf("job is already running: %s", id)
	}

	// Mark job as running (with lock to prevent race condition)
	job.State.LastStatus = "running"
	c.store.UnlockJob(id)
	c.store.Save()

	go c.executeJob(job)
	return nil
}

// GetRuns returns run history for a job
func (c *CronHandler) GetRuns(jobId string, limit int) []RunHistoryEntry {
	return c.store.GetRuns(jobId, limit)
}

// GetStatus returns the cron status
func (c *CronHandler) GetStatus() map[string]interface{} {
	jobs := c.store.List()

	enabled := 0
	disabled := 0
	dueNow := 0

	for _, job := range jobs {
		if job.Enabled {
			enabled++
		} else {
			disabled++
		}
		if job.Enabled && job.State.NextRunAtMs > 0 && job.State.NextRunAtMs <= time.Now().UnixMilli() {
			dueNow++
		}
	}

	return map[string]interface{}{
		"running":    c.IsRunning(),
		"total_jobs": len(jobs),
		"enabled":    enabled,
		"disabled":   disabled,
		"due_now":    dueNow,
		"next_check": time.Now().Add(c.interval).UnixMilli(),
	}
}

// generateJobID generates a unique job ID
func generateJobID() string {
	// Fix: use crypto/rand for collision resistance
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("job-%d-%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
	}
	return fmt.Sprintf("job-%d-%x", time.Now().UnixMilli(), b)
}

// CreateJobFromMap creates a Job from a map (for API calls)
func CreateJobFromMap(data map[string]interface{}) (*Job, error) {
	job := &Job{
		Enabled: true,
	}

	// Basic fields
	if v, ok := data["name"].(string); ok {
		job.Name = v
	}
	if v, ok := data["description"].(string); ok {
		job.Description = v
	}
	if v, ok := data["agentId"].(string); ok {
		job.AgentID = v
	}

	// Schedule
	if sched, ok := data["schedule"].(map[string]interface{}); ok {
		if v, ok := sched["kind"].(string); ok {
			job.Schedule.Kind = v
		}
		if v, ok := sched["at"].(string); ok {
			job.Schedule.At = v
		}
		if v, ok := sched["everyMs"].(float64); ok {
			job.Schedule.EveryMs = int64(v)
		}
		if v, ok := sched["expr"].(string); ok {
			job.Schedule.Expr = v
		}
		if v, ok := sched["tz"].(string); ok {
			job.Schedule.Tz = v
		}
		if v, ok := sched["staggerMs"].(float64); ok {
			job.Schedule.StaggerMs = int64(v)
		}
		if v, ok := sched["anchorMs"].(float64); ok {
			job.Schedule.AnchorMs = int64(v)
		}
	}

	// Session target
	if v, ok := data["sessionTarget"].(string); ok {
		job.SessionTarget = v
	} else {
		job.SessionTarget = SessionTargetMain
	}

	// Wake mode
	if v, ok := data["wakeMode"].(string); ok {
		job.WakeMode = v
	} else {
		job.WakeMode = WakeModeNow
	}

	// Payload
	if payload, ok := data["payload"].(map[string]interface{}); ok {
		if v, ok := payload["kind"].(string); ok {
			job.Payload.Kind = v
		}
		if v, ok := payload["text"].(string); ok {
			job.Payload.Text = v
		}
		if v, ok := payload["message"].(string); ok {
			job.Payload.Message = v
		}
		if v, ok := payload["model"].(string); ok {
			job.Payload.Model = v
		}
		if v, ok := payload["thinking"].(string); ok {
			job.Payload.Thinking = v
		}
		if v, ok := payload["timeoutSeconds"].(float64); ok {
			job.Payload.TimeoutSeconds = int(v)
		}
	}

	// Delivery
	if delivery, ok := data["delivery"].(map[string]interface{}); ok {
		job.Delivery = &Delivery{}
		if v, ok := delivery["mode"].(string); ok {
			job.Delivery.Mode = v
		}
		if v, ok := delivery["channel"].(string); ok {
			job.Delivery.Channel = v
		}
		if v, ok := delivery["to"].(string); ok {
			job.Delivery.To = v
		}
		if v, ok := delivery["bestEffort"].(bool); ok {
			job.Delivery.BestEffort = v
		}
		if v, ok := delivery["webhook"].(string); ok {
			job.Delivery.Webhook = v
		}
	}

	// Delete after run
	if v, ok := data["deleteAfterRun"].(bool); ok {
		job.DeleteAfterRun = v
	} else if job.Schedule.Kind == ScheduleKindAt {
		job.DeleteAfterRun = true
	}

	// Validate
	if job.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if job.Schedule.Kind == "" {
		return nil, fmt.Errorf("schedule.kind is required")
	}
	if job.SessionTarget == SessionTargetMain && job.Payload.Kind != PayloadKindSystemEvent {
		job.Payload.Kind = PayloadKindSystemEvent
	}
	if job.SessionTarget == SessionTargetIsolated && job.Payload.Kind != PayloadKindAgentTurn {
		job.Payload.Kind = PayloadKindAgentTurn
	}

	return job, nil
}
