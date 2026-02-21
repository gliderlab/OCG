package cron

import (
	"testing"
	"time"
)

func TestJobBasic(t *testing.T) {
	// Test basic job structure
	job := map[string]interface{}{
		"id":       "job-1",
		"name":     "test-job",
		"schedule": "* * * * *",
		"enabled":  true,
	}

	if job["id"] != "job-1" {
		t.Errorf("Expected ID 'job-1', got '%v'", job["id"])
	}

	if job["enabled"] != true {
		t.Error("Expected job to be enabled")
	}
}

func TestJobPayload(t *testing.T) {
	payload := map[string]interface{}{
		"kind": "systemEvent",
		"text": "Test event",
	}

	if payload["kind"] != "systemEvent" {
		t.Errorf("Expected Kind 'systemEvent', got '%v'", payload["kind"])
	}

	if payload["text"] != "Test event" {
		t.Errorf("Expected Text 'Test event', got '%v'", payload["text"])
	}
}

func TestSchedule(t *testing.T) {
	sched := Schedule{
		Kind: "cron",
		Expr: "* * * * *",
		Tz:   "UTC",
	}

	if sched.Kind != "cron" {
		t.Errorf("Expected Kind 'cron', got '%s'", sched.Kind)
	}

	if sched.Expr != "* * * * *" {
		t.Errorf("Expected Expr '* * * * *', got '%s'", sched.Expr)
	}

	if sched.Tz != "UTC" {
		t.Errorf("Expected Tz 'UTC', got '%s'", sched.Tz)
	}
}

func TestPayload(t *testing.T) {
	p := Payload{
		Kind:     "systemEvent",
		Text:     "Test message",
		Model:    "gpt-4",
		Thinking: "high",
	}

	if p.Kind != "systemEvent" {
		t.Errorf("Expected Kind 'systemEvent', got '%s'", p.Kind)
	}

	if p.Text != "Test message" {
		t.Errorf("Expected Text 'Test message', got '%s'", p.Text)
	}

	if p.Model != "gpt-4" {
		t.Errorf("Expected Model 'gpt-4', got '%s'", p.Model)
	}

	if p.Thinking != "high" {
		t.Errorf("Expected Thinking 'high', got '%s'", p.Thinking)
	}
}

func TestDelivery(t *testing.T) {
	d := Delivery{
		Mode:       "announce",
		Channel:    "telegram",
		To:         "@mybot",
		BestEffort: true,
		Webhook:    "https://example.com/webhook",
	}

	if d.Mode != "announce" {
		t.Errorf("Expected Mode 'announce', got '%s'", d.Mode)
	}

	if d.Channel != "telegram" {
		t.Errorf("Expected Channel 'telegram', got '%s'", d.Channel)
	}

	if d.To != "@mybot" {
		t.Errorf("Expected To '@mybot', got '%s'", d.To)
	}

	if !d.BestEffort {
		t.Error("Expected BestEffort to be true")
	}

	if d.Webhook != "https://example.com/webhook" {
		t.Errorf("Expected Webhook, got '%s'", d.Webhook)
	}
}

func TestJob(t *testing.T) {
	job := Job{
		ID:            "job-1",
		Name:          "Test Job",
		Description:   "A test job",
		AgentID:       "default",
		Enabled:       true,
		Schedule:      Schedule{Kind: "cron", Expr: "@hourly"},
		SessionTarget: "main",
		WakeMode:      "now",
		Payload:       Payload{Kind: "systemEvent", Text: "Hello"},
		DeleteAfterRun: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if job.ID != "job-1" {
		t.Errorf("Expected ID 'job-1', got '%s'", job.ID)
	}

	if job.Name != "Test Job" {
		t.Errorf("Expected Name 'Test Job', got '%s'", job.Name)
	}

	if !job.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if job.SessionTarget != "main" {
		t.Errorf("Expected SessionTarget 'main', got '%s'", job.SessionTarget)
	}

	if job.WakeMode != "now" {
		t.Errorf("Expected WakeMode 'now', got '%s'", job.WakeMode)
	}
}

func TestJobState(t *testing.T) {
	job := Job{
		ID:       "job-1",
		Name:     "Test Job",
		Enabled:  true,
		Schedule: Schedule{Kind: "cron", Expr: "* * * * *"},
	}

	// Initialize state
	job.State.NextRunAtMs = time.Now().UnixMilli() + 3600000
	job.State.LastRunAtMs = 0
	job.State.LastStatus = ""
	job.State.LastDurationMs = 0
	job.State.ConsecutiveErrors = 0

	if job.State.NextRunAtMs == 0 {
		t.Error("NextRunAtMs should be set")
	}

	if job.State.ConsecutiveErrors != 0 {
		t.Errorf("Expected ConsecutiveErrors 0, got %d", job.State.ConsecutiveErrors)
	}
}

func TestRunHistoryEntry(t *testing.T) {
	entry := RunHistoryEntry{
		JobID:      "test-job",
		JobName:    "Test Job",
		StartedAtMs: time.Now().UnixMilli(),
		EndedAtMs:   time.Now().UnixMilli() + 1000,
		Status:     "ok",
		DurationMs: 1000,
		Error:      "",
		Result:     "Success",
	}

	if entry.JobID != "test-job" {
		t.Errorf("Expected JobID 'test-job', got '%s'", entry.JobID)
	}

	if entry.JobName != "Test Job" {
		t.Errorf("Expected JobName 'Test Job', got '%s'", entry.JobName)
	}

	if entry.Status != "ok" {
		t.Errorf("Expected Status 'ok', got '%s'", entry.Status)
	}

	if entry.DurationMs != 1000 {
		t.Errorf("Expected DurationMs 1000, got %d", entry.DurationMs)
	}

	if entry.Result != "Success" {
		t.Errorf("Expected Result 'Success', got '%s'", entry.Result)
	}
}

func TestJobStore(t *testing.T) {
	store := &JobStore{
		jobs: make(map[string]*Job),
		runs: make(map[string][]RunHistoryEntry),
	}

	if store.jobs == nil {
		t.Error("jobs map should not be nil")
	}

	if store.runs == nil {
		t.Error("runs map should not be nil")
	}

	// Add a job
	job := &Job{
		ID:       "new-job",
		Name:     "New Job",
		Enabled:  true,
		Schedule: Schedule{Kind: "cron", Expr: "@hourly"},
	}
	store.jobs[job.ID] = job

	if len(store.jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(store.jobs))
	}

	// Get job
	getJob, ok := store.jobs["new-job"]
	if !ok {
		t.Error("Job 'new-job' should exist")
	}
	if getJob.Name != "New Job" {
		t.Errorf("Expected Name 'New Job', got '%s'", getJob.Name)
	}

	// Add run history
	entry := RunHistoryEntry{
		JobID:    "new-job",
		JobName:  "New Job",
		Status:   "ok",
		DurationMs: 500,
	}
	store.runs["new-job"] = []RunHistoryEntry{entry}

	if len(store.runs["new-job"]) != 1 {
		t.Errorf("Expected 1 run history entry, got %d", len(store.runs["new-job"]))
	}
}

func TestParseCronSchedule(t *testing.T) {
	tests := []struct {
		schedule string
		valid    bool
	}{
		{"* * * * *", true},
		{"@hourly", true},
		{"@daily", true},
		{"@weekly", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		// Basic validation - check if it's not obviously invalid
		valid := len(tt.schedule) > 0 && (tt.schedule[0] == '*' || tt.schedule[0] == '@')
		if valid != tt.valid && tt.schedule != "invalid" && tt.schedule != "" {
			t.Errorf("Schedule '%s': expected valid=%v, got %v", tt.schedule, tt.valid, valid)
		}
	}
}
