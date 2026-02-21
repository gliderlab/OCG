// Storage module - SQLite data storage

package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
	_ "github.com/mattn/go-sqlite3"
)

// addColumnSafe adds a column to a table if it doesn't exist
// Returns true if column was added, false if it already exists or error
func addColumnSafe(db *sql.DB, table, column, definition string) bool {
	// Check if column already exists
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table), column).Scan(&count)
	if err == nil && count > 0 {
		return false // column already exists
	}

	// Column doesn't exist, try to add it
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil {
		log.Printf("[WARN] Migration: add column %s.%s failed: %v (may be OK if already exists)", table, column, err)
		return false
	}
	return true
}

type Storage struct {
	db *sql.DB

	// Prepared statements for performance optimization
	stmtAddMessage     *sql.Stmt
	stmtGetMessages    *sql.Stmt
	stmtClearMessages  *sql.Stmt
	stmtArchiveMessage *sql.Stmt
	stmtGetMemory      *sql.Stmt
	stmtSetMemory      *sql.Stmt
	stmtSearchMemory   *sql.Stmt
	stmtGetConfig      *sql.Stmt
	stmtSetConfig      *sql.Stmt
}

type Message struct {
	ID         int64     `json:"id"`
	SessionKey string    `json:"session_key"`
	Role       string    `json:"role"` // user, assistant, system
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type Memory struct {
	ID         int64     `json:"id"`
	Key        string    `json:"key"`
	Text       string    `json:"text"`     // memory content
	Category   string    `json:"category"` // preference, decision, fact, entity, other
	Importance float64   `json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type FileRecord struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}

type Config struct {
	ID        int64     `json:"id"`
	Section   string    `json:"section"` // e.g., "llm", "gateway", "storage"
	Key       string    `json:"key"`     // e.g., "apiKey", "baseUrl", "model"
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionMeta struct {
	SessionKey               string    `json:"session_key"`
	ProviderType             string    `json:"provider_type,omitempty"` // llm_http | live | hybrid
	RealtimeLastActiveAt     time.Time `json:"realtime_last_active_at,omitempty"`
	TotalTokens              int       `json:"total_tokens"`
	CompactionCount          int       `json:"compaction_count"`
	LastSummary              string    `json:"last_summary"`
	LastCompactedMessageID   int64     `json:"last_compacted_message_id"`
	MemoryFlushAt            time.Time `json:"memory_flush_at"`
	MemoryFlushCompactionCnt int       `json:"memory_flush_compaction_count"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// EventPriority levels (lower = higher priority)
// 0 = Critical (broadcast to all channels immediately)
// 1 = Important (channel broadcast)
// 2 = Normal (process when idle)
// 3 = Low (process when available)
type EventPriority int

const (
	PriorityCritical EventPriority = 0 // Broadcast to all channels
	PriorityHigh     EventPriority = 1 // Broadcast to configured channels
	PriorityNormal   EventPriority = 2 // Process when idle
	PriorityLow      EventPriority = 3 // Process when available
)

type Event struct {
	ID          int64         `json:"id"`
	Title       string        `json:"title"`
	Content     string        `json:"content"`
	Response    string        `json:"response,omitempty"`
	Priority    EventPriority `json:"priority"` // 0-3
	Status      string        `json:"status"`   // pending, processing, completed, dismissed
	Channel     string        `json:"channel"`  // telegram, discord, etc (empty = all)
	CreatedAt   time.Time     `json:"created_at"`
	ProcessedAt *time.Time    `json:"processed_at,omitempty"`

	// Hook-specific fields
	EventType string `json:"event_type,omitempty"` // hook:command:new, hook:message:received
	HookName  string `json:"hook_name,omitempty"`  // session-memory, command-logger
	Metadata  string `json:"metadata,omitempty"`    // JSON additional data
}

func New(dbPath string) (*Storage, error) {
	cfg := config.DefaultStorageConfig()
	cfg.DBPath = dbPath
	return NewWithConfig(*cfg)
}

// NewWithConfig creates storage with injected configuration
func NewWithConfig(cfg config.StorageConfig) (*Storage, error) {
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("db path required")
	}
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Bug #3 Fix: Add database connection health check
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database connection failed: %v", err)
	}

	s := &Storage{db: db}

	// Set WAL mode
	if cfg.WalMode {
		if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			return nil, fmt.Errorf("failed to set WAL: %v", err)
		}
	}

	// Sync mode
	syncMode := cfg.SyncMode
	if syncMode == "" {
		syncMode = "NORMAL"
	}
	if _, err := db.Exec("PRAGMA synchronous=" + syncMode + ";"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous: %v", err)
	}

	// Connection pool
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	// Initialize tables
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %v", err)
	}

	// Optimization #1: Prepare statements for frequently used queries
	if err := s.initPreparedStmts(); err != nil {
		log.Printf("[WARN] Failed to prepare statements: %v (continuing without prepared statements)", err)
	}

	// Optional: bind executable with database (build tag binddb)
	if err := BindExecutable(s, cfg.DBPath); err != nil {
		return nil, fmt.Errorf("bind executable failed: %v", err)
	}

	log.Printf("[OK] Storage: database %s", cfg.DBPath)
	return s, nil
}

// initPreparedStmts prepares frequently used SQL statements for performance
func (s *Storage) initPreparedStmts() error {
	var err error

	// Messages
	if s.stmtAddMessage, err = s.db.Prepare("INSERT INTO messages (session_key, role, content) VALUES (?, ?, ?)"); err != nil {
		return fmt.Errorf("AddMessage: %v", err)
	}
	if s.stmtGetMessages, err = s.db.Prepare("SELECT id, session_key, role, content, created_at FROM messages WHERE session_key = ? ORDER BY created_at ASC LIMIT ?"); err != nil {
		return fmt.Errorf("GetMessages: %v", err)
	}
	if s.stmtClearMessages, err = s.db.Prepare("DELETE FROM messages WHERE session_key = ?"); err != nil {
		return fmt.Errorf("ClearMessages: %v", err)
	}
	if s.stmtArchiveMessage, err = s.db.Prepare("INSERT OR IGNORE INTO messages_archive (session_key, source_message_id, role, content, created_at) SELECT session_key, id, role, content, created_at FROM messages WHERE session_key = ? AND id <= ?"); err != nil {
		return fmt.Errorf("ArchiveMessage: %v", err)
	}

	// Memories
	if s.stmtGetMemory, err = s.db.Prepare("SELECT id, key, value, category, importance, created_at, updated_at FROM memories WHERE key = ?"); err != nil {
		return fmt.Errorf("GetMemory: %v", err)
	}
	if s.stmtSetMemory, err = s.db.Prepare("INSERT INTO memories (key, value, category, importance) VALUES (?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, category=excluded.category, importance=excluded.importance, updated_at=CURRENT_TIMESTAMP"); err != nil {
		return fmt.Errorf("SetMemory: %v", err)
	}
	if s.stmtSearchMemory, err = s.db.Prepare("SELECT id, key, value, category, importance, created_at, updated_at FROM memories ORDER BY created_at DESC LIMIT ?"); err != nil {
		return fmt.Errorf("SearchMemory: %v", err)
	}

	// Config
	if s.stmtGetConfig, err = s.db.Prepare("SELECT value FROM config WHERE section = ? AND key = ?"); err != nil {
		return fmt.Errorf("GetConfig: %v", err)
	}
	if s.stmtSetConfig, err = s.db.Prepare("INSERT INTO config (section, key, value) VALUES (?, ?, ?) ON CONFLICT(section, key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP"); err != nil {
		return fmt.Errorf("SetConfig: %v", err)
	}

	return nil
}

func (s *Storage) initSchema() error {
	// Messages table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_key TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Memories table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT UNIQUE,
			value TEXT,
			category TEXT,
			importance REAL DEFAULT 0.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Files table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE,
			content TEXT,
			mime_type TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Config table (persistent config)
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			section TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(section, key)
		)
	`)
	if err != nil {
		return err
	}

	// Session meta
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS session_meta (
			session_key TEXT PRIMARY KEY,
			provider_type TEXT DEFAULT '',
			realtime_last_active_at DATETIME,
			total_tokens INTEGER DEFAULT 0,
			compaction_count INTEGER DEFAULT 0,
			last_summary TEXT,
			last_compacted_message_id INTEGER DEFAULT 0,
			memory_flush_at DATETIME,
			memory_flush_compaction_count INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	// Migration: add provider_type + realtime_last_active_at + last_compacted_message_id
	addColumnSafe(s.db, "session_meta", "provider_type", "TEXT DEFAULT ''")
	addColumnSafe(s.db, "session_meta", "realtime_last_active_at", "DATETIME")
	addColumnSafe(s.db, "session_meta", "last_compacted_message_id", "INTEGER DEFAULT 0")

	// Archive table (optional)
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages_archive (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_key TEXT NOT NULL,
			source_message_id INTEGER,
			role TEXT NOT NULL,
			content TEXT,
			created_at DATETIME,
			archived_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	// Migration: add source_message_id and dedupe unique index
	addColumnSafe(s.db, "messages_archive", "source_message_id", "INTEGER")

	// Create indexes
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_key)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_config_section ON config(section, key)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_session_meta ON session_meta(session_key)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_archive_session_src ON messages_archive(session_key, source_message_id)`); err != nil {
		return err
	}

	// Events table (for pulse/heartbeat system)
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT,
			response TEXT,
			priority INTEGER DEFAULT 2,
			status TEXT DEFAULT 'pending',
			channel TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			processed_at DATETIME,
			event_type TEXT DEFAULT '',
			hook_name TEXT DEFAULT '',
			metadata TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	// Add new columns if they don't exist
	if _, err := s.db.Exec("ALTER TABLE events ADD COLUMN response TEXT"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("[WARN] failed to add events.response column: %v", err)
		}
	}
	if _, err := s.db.Exec("ALTER TABLE events ADD COLUMN event_type TEXT DEFAULT ''"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("[WARN] failed to add events.event_type column: %v", err)
		}
	}
	if _, err := s.db.Exec("ALTER TABLE events ADD COLUMN hook_name TEXT DEFAULT ''"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("[WARN] failed to add events.hook_name column: %v", err)
		}
	}
	if _, err := s.db.Exec("ALTER TABLE events ADD COLUMN metadata TEXT DEFAULT ''"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("[WARN] failed to add events.metadata column: %v", err)
		}
	}

	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_priority ON events(priority)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_status ON events(status)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_hook_name ON events(hook_name)`); err != nil {
		return err
	}

	// Rate limiting table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS rate_limits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint TEXT NOT NULL,
			key TEXT NOT NULL,
			requests INTEGER DEFAULT 0,
			window_start DATETIME DEFAULT CURRENT_TIMESTAMP,
			max_requests INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(endpoint, key)
		)
	`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_rate_limits_endpoint ON rate_limits(endpoint, key)`); err != nil {
		return err
	}

	// Task history table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS task_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			task_name TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			input TEXT,
			output TEXT,
			error TEXT,
			depends_on TEXT,
			retry_count INTEGER DEFAULT 0,
			max_retries INTEGER DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_history_task_id ON task_history(task_id)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_history_status ON task_history(status)`); err != nil {
		return err
	}

	// User task splitting tables
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_tasks (
			id TEXT PRIMARY KEY,
			session TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			completed_at INTEGER,
			total INTEGER NOT NULL,
			completed INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			instructions TEXT NOT NULL,
			result TEXT,
			started_at INTEGER,
			error TEXT
		)
	`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_tasks_session ON user_tasks(session)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_tasks_status ON user_tasks(status)`); err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_subtasks (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			index_num INTEGER NOT NULL,
			description TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			result TEXT,
			process TEXT,
			started_at INTEGER,
			completed_at INTEGER,
			error TEXT,
			FOREIGN KEY (task_id) REFERENCES user_tasks(id)
		)
	`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_subtasks_task_id ON user_subtasks(task_id)`); err != nil {
		return err
	}

	return nil
}

// ============ Messages ============

func (s *Storage) AddMessage(sessionKey, role, content string) error {
	// Use prepared statement if available, fallback to regular exec
	if s.stmtAddMessage != nil {
		_, err := s.stmtAddMessage.Exec(sessionKey, role, content)
		return err
	}
	_, err := s.db.Exec(
		"INSERT INTO messages (session_key, role, content) VALUES (?, ?, ?)",
		sessionKey, role, content,
	)
	return err
}

func (s *Storage) GetMessages(sessionKey string, limit int) ([]Message, error) {
	var rows *sql.Rows
	var err error

	// Use prepared statement if available
	if s.stmtGetMessages != nil {
		rows, err = s.stmtGetMessages.Query(sessionKey, limit)
	} else {
		rows, err = s.db.Query(
			"SELECT id, session_key, role, content, created_at FROM messages WHERE session_key = ? ORDER BY created_at ASC LIMIT ?",
			sessionKey, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs := make([]Message, 0) // Initialize empty slice instead of nil
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionKey, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}

	// Fix: check rows.Err() before returning
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Already in correct order (ASC), no reversal needed
	return msgs, nil
}

func (s *Storage) ClearMessages(sessionKey string) error {
	if s.stmtClearMessages != nil {
		_, err := s.stmtClearMessages.Exec(sessionKey)
		return err
	}
	_, err := s.db.Exec("DELETE FROM messages WHERE session_key = ?", sessionKey)
	return err
}

// ============ Session Meta ============

func (s *Storage) GetSessionMeta(sessionKey string) (SessionMeta, error) {
	var meta SessionMeta
	var memoryFlushAt, updatedAt, realtimeLastActiveAt string
	err := s.db.QueryRow(`
		SELECT session_key, COALESCE(provider_type, ''),
		       COALESCE(realtime_last_active_at, datetime('now')),
		       total_tokens, compaction_count, last_summary,
		       COALESCE(last_compacted_message_id, 0),
		       COALESCE(memory_flush_at, datetime('now')),
		       COALESCE(memory_flush_compaction_count, 0),
		       COALESCE(updated_at, datetime('now'))
		FROM session_meta WHERE session_key = ?
	`, sessionKey).Scan(&meta.SessionKey, &meta.ProviderType, &realtimeLastActiveAt, &meta.TotalTokens, &meta.CompactionCount, &meta.LastSummary, &meta.LastCompactedMessageID, &memoryFlushAt, &meta.MemoryFlushCompactionCnt, &updatedAt)
	if err == sql.ErrNoRows {
		return SessionMeta{SessionKey: sessionKey}, nil
	}
	if err == nil {
		meta.RealtimeLastActiveAt, _ = time.Parse("2006-01-02 15:04:05", realtimeLastActiveAt)
		meta.MemoryFlushAt, _ = time.Parse("2006-01-02 15:04:05", memoryFlushAt)
		meta.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	return meta, err
}

func (s *Storage) UpsertSessionMeta(meta SessionMeta) error {
	_, err := s.db.Exec(`
		INSERT INTO session_meta (session_key, provider_type, realtime_last_active_at, total_tokens, compaction_count, last_summary, last_compacted_message_id, memory_flush_at, memory_flush_compaction_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(session_key) DO UPDATE SET
			provider_type=excluded.provider_type,
			realtime_last_active_at=excluded.realtime_last_active_at,
			total_tokens=excluded.total_tokens,
			compaction_count=excluded.compaction_count,
			last_summary=excluded.last_summary,
			last_compacted_message_id=excluded.last_compacted_message_id,
			memory_flush_at=excluded.memory_flush_at,
			memory_flush_compaction_count=excluded.memory_flush_compaction_count,
			updated_at=CURRENT_TIMESTAMP
	`, meta.SessionKey, meta.ProviderType, meta.RealtimeLastActiveAt, meta.TotalTokens, meta.CompactionCount, meta.LastSummary, meta.LastCompactedMessageID, meta.MemoryFlushAt, meta.MemoryFlushCompactionCnt)
	return err
}

func (s *Storage) SetSessionProviderType(sessionKey, providerType string) error {
	_, err := s.db.Exec(`
		INSERT INTO session_meta (session_key, provider_type, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(session_key) DO UPDATE SET
			provider_type=excluded.provider_type,
			updated_at=CURRENT_TIMESTAMP
	`, sessionKey, providerType)
	return err
}

func (s *Storage) TouchRealtimeSession(sessionKey string) error {
	_, err := s.db.Exec(`
		INSERT INTO session_meta (session_key, provider_type, realtime_last_active_at, updated_at)
		VALUES (?, 'live', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(session_key) DO UPDATE SET
			realtime_last_active_at=CURRENT_TIMESTAMP,
			updated_at=CURRENT_TIMESTAMP
	`, sessionKey)
	return err
}

// ResetSession clears all messages for a session (but keeps the session entry)
func (s *Storage) ResetSession(sessionKey string) error {
	// Delete messages for the session
	_, err := s.db.Exec("DELETE FROM messages WHERE session_key = ?", sessionKey)
	if err != nil {
		return err
	}
	// Reset session metadata (keep the entry but clear stats)
	_, err = s.db.Exec(`
		UPDATE session_meta SET
			total_tokens = 0,
			compaction_count = 0,
			last_summary = '',
			last_compacted_message_id = 0,
			updated_at = CURRENT_TIMESTAMP
		WHERE session_key = ?
	`, sessionKey)
	return err
}

// GetSessionsForReset returns sessions that need reset based on daily or idle policy
func (s *Storage) GetSessionsForReset(mode string, atHour int, idleMinutes int) ([]SessionMeta, error) {
	var rows *sql.Rows
	var err error

	if mode == "daily" {
		// Daily reset: reset sessions not updated since yesterday's reset time
		// Reset time = today at atHour UTC, or yesterday if past that time
		rows, err = s.db.Query(`
			SELECT session_key, total_tokens, compaction_count, last_summary, memory_flush_at, memory_flush_compaction_count, updated_at
			FROM session_meta
			WHERE updated_at < datetime('now', '-' || ? || ' hours')
			AND session_key NOT LIKE 'cron:%'
			AND session_key NOT LIKE 'hook:%'
		`, atHour)
	} else if mode == "idle" && idleMinutes > 0 {
		// Idle reset: reset sessions not updated in idleMinutes
		rows, err = s.db.Query(`
			SELECT session_key, total_tokens, compaction_count, last_summary, memory_flush_at, memory_flush_compaction_count, updated_at
			FROM session_meta
			WHERE updated_at < datetime('now', '-' || ? || ' minutes')
			AND session_key NOT LIKE 'cron:%'
			AND session_key NOT LIKE 'hook:%'
		`, idleMinutes)
	} else {
		return []SessionMeta{}, nil
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionMeta
	for rows.Next() {
		var m SessionMeta
		if err := rows.Scan(&m.SessionKey, &m.TotalTokens, &m.CompactionCount, &m.LastSummary, &m.MemoryFlushAt, &m.MemoryFlushCompactionCnt, &m.UpdatedAt); err != nil {
			continue
		}
		sessions = append(sessions, m)
	}
	return sessions, nil
}

func (s *Storage) GetAllSessions() ([]SessionMeta, error) {
	rows, err := s.db.Query(`
		SELECT session_key, total_tokens, compaction_count, last_summary, memory_flush_at, memory_flush_compaction_count, updated_at
		FROM session_meta
		ORDER BY updated_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionMeta
	for rows.Next() {
		var m SessionMeta
		if err := rows.Scan(&m.SessionKey, &m.TotalTokens, &m.CompactionCount, &m.LastSummary, &m.MemoryFlushAt, &m.MemoryFlushCompactionCnt, &m.UpdatedAt); err != nil {
			continue
		}
		sessions = append(sessions, m)
	}
	return sessions, nil
}

func (s *Storage) ArchiveMessages(sessionKey string, beforeID int64) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO messages_archive (session_key, source_message_id, role, content, created_at)
		SELECT m.session_key, m.id, m.role, m.content, m.created_at
		FROM messages m
		LEFT JOIN session_meta sm ON sm.session_key = m.session_key
		WHERE m.session_key = ?
		  AND m.id > COALESCE(sm.last_compacted_message_id, 0)
		  AND m.id <= ?
		  AND NOT (m.role = 'system' AND m.content LIKE '[summary]%')
	`, sessionKey, beforeID)
	return err
}

func (s *Storage) GetArchiveStats(sessionKey string) (ArchiveStats, error) {
	stats := ArchiveStats{SessionKey: sessionKey}
	err := s.db.QueryRow(`
		SELECT COALESCE(COUNT(*),0), COALESCE(MAX(source_message_id),0)
		FROM messages_archive
		WHERE session_key = ?
	`, sessionKey).Scan(&stats.ArchivedCount, &stats.LastSourceMessage)
	return stats, err
}

// ============ Memories ============

func (s *Storage) SetMemory(key, text, category string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO memories (key, value, category, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, key, text, category)
	return err
}

func (s *Storage) AddMemory(text, category string, importance float64) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO memories (key, value, category, importance, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, generateMemoryKey(), text, category, importance)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func generateMemoryKey() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixNano())
}

func (s *Storage) GetMemory(idOrKey string) (Memory, error) {
	// Try lookup by ID first
	var m Memory
	err := s.db.QueryRow(`
		SELECT id, key, value AS text, category, COALESCE(importance, 0.0), 
		       COALESCE(created_at, datetime('now')), COALESCE(updated_at, datetime('now'))
		FROM memories WHERE id = ? OR key = ?
	`, idOrKey, idOrKey).Scan(&m.ID, &m.Key, &m.Text, &m.Category, &m.Importance, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return Memory{}, fmt.Errorf("memory not found: %s", idOrKey)
	}
	return m, err
}

func (s *Storage) GetMemoriesByCategory(category string) ([]Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, key, value AS text, category, importance, created_at, updated_at
		FROM memories WHERE category = ?
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMemories(rows)
}

func (s *Storage) DeleteMemory(key string) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE key = ?", key)
	return err
}

func (s *Storage) DeleteMemoryByID(id int64) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

func (s *Storage) SearchMemories(keyword string) ([]Memory, error) {
	// First, match key exactly
	var m Memory
	err := s.db.QueryRow(`
		SELECT id, key, value AS text, category, COALESCE(importance, 0.0), 
		       COALESCE(created_at, datetime('now')), COALESCE(updated_at, datetime('now'))
		FROM memories WHERE key = ?
	`, keyword).Scan(&m.ID, &m.Key, &m.Text, &m.Category, &m.Importance, &m.CreatedAt, &m.UpdatedAt)
	if err == nil {
		return []Memory{m}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Then fuzzy search by value
	rows, err := s.db.Query(`
		SELECT id, key, value AS text, category, COALESCE(importance, 0.0), 
		       COALESCE(created_at, datetime('now')), COALESCE(updated_at, datetime('now'))
		FROM memories WHERE value LIKE ? OR category LIKE ?
		ORDER BY importance DESC, created_at DESC LIMIT ?
	`, "%"+keyword+"%", "%"+keyword+"%", 10)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMemories(rows)
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Key, &m.Text, &m.Category, &m.Importance, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

func (s *Storage) GetAllMemories(limit int) ([]Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, key, value AS text, category, importance, created_at, updated_at
		FROM memories ORDER BY importance DESC, created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ============ Files ============

func (s *Storage) AddFile(path, content, mimeType string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO files (path, content, mime_type) VALUES (?, ?, ?)",
		path, content, mimeType,
	)
	return err
}

func (s *Storage) GetFile(path string) (*FileRecord, error) {
	var f FileRecord
	err := s.db.QueryRow(
		"SELECT id, path, content, mime_type, created_at FROM files WHERE path = ?",
		path,
	).Scan(&f.ID, &f.Path, &f.Content, &f.MimeType, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &f, err
}

func (s *Storage) ListFiles() ([]FileRecord, error) {
	rows, err := s.db.Query(
		"SELECT id, path, content, mime_type, created_at FROM files ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.Path, &f.Content, &f.MimeType, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ============ Tools ============

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) Stats() (map[string]int, error) {
	stats := make(map[string]int)

	// Single query for all counts
	row := s.db.QueryRow(`
		SELECT 
			(SELECT COUNT(*) FROM messages),
			(SELECT COUNT(*) FROM vector_memories),
			(SELECT COUNT(*) FROM files),
			(SELECT COUNT(*) FROM session_meta)
	`)
	var msgs, mems, files, sessions int
	if err := row.Scan(&msgs, &mems, &files, &sessions); err != nil {
		return nil, err
	}
	stats["messages"] = msgs
	stats["memories"] = mems
	stats["files"] = files
	stats["sessions"] = sessions

	return stats, nil
}

// Import from MD-style data (simplified)
func (s *Storage) ImportMemory(key, value, category string) error {
	return s.SetMemory(key, value, category)
}

// Export memories to JSON
func (s *Storage) ExportMemories() ([]byte, error) {
	rows, err := s.db.Query("SELECT id, key, value, category, updated_at FROM memories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type ExportMem struct {
		ID        int64     `json:"id"`
		Key       string    `json:"key"`
		Value     string    `json:"value"`
		Category  string    `json:"category"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var memories []ExportMem
	for rows.Next() {
		var m ExportMem
		if err := rows.Scan(&m.ID, &m.Key, &m.Value, &m.Category, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan export memory failed: %v", err)
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("export memories iteration error: %v", err)
	}

	return json.MarshalIndent(memories, "", "  ")
}

// ============ Config (persistence) ============

// SetConfig writes a config entry to the database
func (s *Storage) SetConfig(section, key, value string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO config (section, key, value, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
		section, key, value,
	)
	return err
}

// GetConfig reads a config value
func (s *Storage) GetConfig(section, key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM config WHERE section = ? AND key = ?", section, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// GetConfigSection reads all config values in a section
func (s *Storage) GetConfigSection(section string) (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM config WHERE section = ?", section)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		config[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return config, nil
}

// ConfigExists checks whether a section exists
func (s *Storage) ConfigExists(section string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM config WHERE section = ?", section).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteConfig deletes a config entry
func (s *Storage) DeleteConfig(section, key string) error {
	_, err := s.db.Exec("DELETE FROM config WHERE section = ? AND key = ?", section, key)
	return err
}

// ClearConfigSection clears a section
func (s *Storage) ClearConfigSection(section string) error {
	_, err := s.db.Exec("DELETE FROM config WHERE section = ?", section)
	return err
}

// ExportConfig exports all configs as JSON
func (s *Storage) ExportConfig() ([]byte, error) {
	rows, err := s.db.Query("SELECT section, key, value FROM config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type ExportConfig struct {
		Section string `json:"section"`
		Key     string `json:"key"`
		Value   string `json:"value"`
	}

	var configs []ExportConfig
	for rows.Next() {
		var c ExportConfig
		if err := rows.Scan(&c.Section, &c.Key, &c.Value); err != nil {
			return nil, fmt.Errorf("scan export config failed: %v", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("export configs iteration error: %v", err)
	}

	return json.MarshalIndent(configs, "", "  ")
}

// ============ Events (Pulse/Heartbeat System) ============

// AddEvent adds a new event to the database
func (s *Storage) AddEvent(title, content string, priority EventPriority, channel string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO events (title, content, priority, status, channel) VALUES (?, ?, ?, 'pending', ?)",
		title, content, priority, channel,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// AddHookEvent adds a new hook event to the database
// Hook events have PriorityHigh (1) by default for immediate processing
func (s *Storage) AddHookEvent(eventType, hookName, content, metadata string) (int64, error) {
	title := fmt.Sprintf("hook:%s:%s", eventType, hookName)
	result, err := s.db.Exec(
		"INSERT INTO events (title, content, priority, status, channel, event_type, hook_name, metadata) VALUES (?, ?, ?, 'pending', '', ?, ?, ?)",
		title, content, PriorityHigh, eventType, hookName, metadata,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetHookEvents returns hook events by event type and/or hook name
func (s *Storage) GetHookEvents(eventType, hookName string, limit int) ([]Event, error) {
	query := "SELECT id, title, content, response, priority, status, channel, created_at, processed_at, event_type, hook_name, metadata FROM events WHERE event_type != ''"
	args := []interface{}{}

	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}
	if hookName != "" {
		query += " AND hook_name = ?"
		args = append(args, hookName)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var processedAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &e.Response, &e.Priority, &e.Status, &e.Channel, &e.CreatedAt, &processedAt, &e.EventType, &e.HookName, &e.Metadata); err != nil {
			return nil, err
		}
		if processedAt.Valid {
			e.ProcessedAt = &processedAt.Time
		}
		events = append(events, e)
	}

	return events, nil
}

// GetPendingEvents returns pending events ordered by priority (0 first)
func (s *Storage) GetPendingEvents(limit int) ([]Event, error) {
	rows, err := s.db.Query(`
		SELECT id, title, content, response, priority, status, channel, created_at, processed_at
		FROM events
		WHERE status = 'pending'
		ORDER BY priority ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var processedAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &e.Response, &e.Priority, &e.Status, &e.Channel, &e.CreatedAt, &processedAt); err != nil {
			return nil, err
		}
		if processedAt.Valid {
			e.ProcessedAt = &processedAt.Time
		}
		events = append(events, e)
	}
	return events, nil
}

// GetNextEvent returns the highest priority pending event
func (s *Storage) GetNextEvent() (*Event, error) {
	var e Event
	var processedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, title, content, response, priority, status, channel, created_at, processed_at
		FROM events
		WHERE status = 'pending'
		ORDER BY priority ASC, created_at ASC
		LIMIT 1
	`).Scan(&e.ID, &e.Title, &e.Content, &e.Response, &e.Priority, &e.Status, &e.Channel, &e.CreatedAt, &processedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if processedAt.Valid {
		e.ProcessedAt = &processedAt.Time
	}
	return &e, nil
}

// ClaimNextEvent atomically claims the next pending event for processing
// NOTE: This implementation has a race condition - use ClaimNextEventV2 for production
func (s *Storage) ClaimNextEvent() (*Event, error) {
	return s.ClaimNextEventV2()
}

// ClaimNextEventV2 atomically claims the next pending event using UPDATE-first approach
// This prevents duplicate claims from concurrent workers
func (s *Storage) ClaimNextEventV2() (*Event, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Atomic claim: UPDATE first, then SELECT the claimed row
	// This prevents race conditions where multiple workers claim the same event
	result, execErr := tx.Exec(`
		UPDATE events
		SET status = 'processing'
		WHERE id = (
			SELECT id FROM events
			WHERE status = 'pending'
			ORDER BY priority ASC, created_at ASC
			LIMIT 1
		)
	`)
	if execErr != nil {
		err = execErr
		return nil, err
	}

	// Check if any row was actually updated
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// No pending event to claim
		_ = tx.Rollback()
		return nil, nil
	}

	// Now select the event we just claimed
	var e Event
	var processedAt sql.NullTime
	row := tx.QueryRow(`
		SELECT id, title, content, response, priority, status, channel, created_at, processed_at
		FROM events
		WHERE status = 'processing'
		ORDER BY priority ASC, created_at ASC
		LIMIT 1
	`)
	if scanErr := row.Scan(&e.ID, &e.Title, &e.Content, &e.Response, &e.Priority, &e.Status, &e.Channel, &e.CreatedAt, &processedAt); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			_ = tx.Rollback()
			return nil, nil
		}
		err = scanErr
		return nil, err
	}
	if processedAt.Valid {
		e.ProcessedAt = &processedAt.Time
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = commitErr
		return nil, err
	}

	return &e, nil
}

// PeekNextEvent returns the next pending event without claiming it
func (s *Storage) PeekNextEvent() (*Event, error) {
	var e Event
	var processedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, title, content, response, priority, status, channel, created_at, processed_at
		FROM events
		WHERE status = 'pending'
		ORDER BY priority ASC, created_at ASC
		LIMIT 1
	`).Scan(&e.ID, &e.Title, &e.Content, &e.Response, &e.Priority, &e.Status, &e.Channel, &e.CreatedAt, &processedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if processedAt.Valid {
		e.ProcessedAt = &processedAt.Time
	}
	return &e, nil
}

// UpdateEventStatus updates an event's status
func (s *Storage) UpdateEventStatus(id int64, status string) error {
	if status == "processing" || status == "processing_llm" {
		_, err := s.db.Exec(
			"UPDATE events SET status = ? WHERE id = ?",
			status, id,
		)
		return err
	}
	_, err := s.db.Exec(
		"UPDATE events SET status = ?, processed_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, id,
	)
	return err
}

// UpdateEventStatusWithResponse updates status and response payload
func (s *Storage) UpdateEventStatusWithResponse(id int64, status, response string) error {
	if status == "processing" || status == "processing_llm" {
		_, err := s.db.Exec(
			"UPDATE events SET status = ?, response = ? WHERE id = ?",
			status, response, id,
		)
		return err
	}
	_, err := s.db.Exec(
		"UPDATE events SET status = ?, response = ?, processed_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, response, id,
	)
	return err
}

// GetEventCount returns counts by status
func (s *Storage) GetEventCount() (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT status, COUNT(*) as count
		FROM events
		GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, nil
}

// ClearOldEvents removes completed/dismissed events older than specified hours
func (s *Storage) ClearOldEvents(olderThanHours int) error {
	// Fix: use Go's time calculation instead of SQL time offset with parameter
	cutoffTime := time.Now().Add(-time.Duration(olderThanHours) * time.Hour).Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(`
		DELETE FROM events
		WHERE status IN ('completed', 'dismissed')
		AND processed_at < ?
	`, cutoffTime)
	return err
}

// Exec executes a raw SQL query
func (s *Storage) Exec(query string, args ...interface{}) (interface{}, error) {
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Query executes a raw SQL query and returns rows
func (s *Storage) Query(query string, args ...interface{}) (interface{}, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ============ Rate Limiting ============

type RateLimit struct {
	ID          int64     `json:"id"`
	Endpoint    string    `json:"endpoint"`
	Key         string    `json:"key"`
	Requests    int       `json:"requests"`
	WindowStart time.Time `json:"window_start"`
	MaxRequests int       `json:"max_requests"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SetRateLimit sets the rate limit for an endpoint/key
func (s *Storage) SetRateLimit(endpoint, key string, maxRequests int) error {
	_, err := s.db.Exec(`
		INSERT INTO rate_limits (endpoint, key, max_requests, requests, window_start)
		VALUES (?, ?, ?, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(endpoint, key) DO UPDATE SET
			max_requests = excluded.max_requests,
			updated_at = CURRENT_TIMESTAMP
	`, endpoint, key, maxRequests)
	return err
}

// GetRateLimit gets the rate limit status for an endpoint/key
func (s *Storage) GetRateLimit(endpoint, key string) (*RateLimit, error) {
	var r RateLimit
	err := s.db.QueryRow(`
		SELECT id, endpoint, key, requests, window_start, max_requests, updated_at
		FROM rate_limits
		WHERE endpoint = ? AND key = ?
	`, endpoint, key).Scan(&r.ID, &r.Endpoint, &r.Key, &r.Requests, &r.WindowStart, &r.MaxRequests, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No limit set = unlimited
		}
		return nil, err
	}
	return &r, nil
}

// CheckRateLimit checks if request is allowed, returns true if allowed
// Fix F: Use atomic check-and-increment to prevent race conditions
func (s *Storage) CheckRateLimit(endpoint, key string) (bool, error) {
	// Get rate limit config
	limit, err := s.GetRateLimit(endpoint, key)
	if err != nil {
		return false, err
	}
	// No limit configured = unlimited
	if limit == nil || limit.MaxRequests == 0 {
		return true, nil
	}

	// Check if window has expired (reset if > 1 hour)
	if time.Since(limit.WindowStart) > time.Hour {
		_, err := s.db.Exec(`
			UPDATE rate_limits
			SET requests = 0, window_start = CURRENT_TIMESTAMP
			WHERE endpoint = ? AND key = ?
		`, endpoint, key)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	// Atomic check-and-increment: only increment if requests < max_requests
	// This prevents race conditions where multiple requests could exceed the limit
	result, err := s.db.Exec(`
		UPDATE rate_limits
		SET requests = requests + 1
		WHERE endpoint = ? AND key = ? AND requests < max_requests
	`, endpoint, key)
	if err != nil {
		return false, err
	}

	// Check if the update affected any rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		// Either the record doesn't exist (shouldn't happen here) or requests >= max_requests
		return false, nil
	}

	return true, nil
}

// GetAllRateLimits returns all rate limit configs
func (s *Storage) GetAllRateLimits() ([]RateLimit, error) {
	rows, err := s.db.Query(`
		SELECT id, endpoint, key, requests, window_start, max_requests, updated_at
		FROM rate_limits
		ORDER BY endpoint, key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var limits []RateLimit
	for rows.Next() {
		var r RateLimit
		if err := rows.Scan(&r.ID, &r.Endpoint, &r.Key, &r.Requests, &r.WindowStart, &r.MaxRequests, &r.UpdatedAt); err != nil {
			return nil, err
		}
		limits = append(limits, r)
	}
	return limits, nil
}

// DeleteRateLimit removes a rate limit config
func (s *Storage) DeleteRateLimit(endpoint, key string) error {
	_, err := s.db.Exec(`DELETE FROM rate_limits WHERE endpoint = ? AND key = ?`, endpoint, key)
	return err
}

// ============ Task History ============

type TaskHistory struct {
	ID          int64      `json:"id"`
	TaskID      string     `json:"task_id"`
	TaskName    string     `json:"task_name"`
	Status      string     `json:"status"` // pending, running, completed, failed, cancelled
	Input       string     `json:"input"`
	Output      string     `json:"output"`
	Error       string     `json:"error"`
	DependsOn   string     `json:"depends_on"`
	RetryCount  int        `json:"retry_count"`
	MaxRetries  int        `json:"max_retries"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateTask creates a new task record
func (s *Storage) CreateTask(taskID, taskName, input, dependsOn string, maxRetries int) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO task_history (task_id, task_name, status, input, depends_on, max_retries, started_at)
		VALUES (?, ?, 'pending', ?, ?, ?, CURRENT_TIMESTAMP)
	`, taskID, taskName, input, dependsOn, maxRetries)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateTaskStatus updates task status
func (s *Storage) UpdateTaskStatus(taskID, status, output, errMsg string) error {
	completedAt := "CURRENT_TIMESTAMP"
	if status == "running" {
		completedAt = "NULL"
	}
	_, err := s.db.Exec(fmt.Sprintf(`
		UPDATE task_history
		SET status = ?, output = ?, error = ?,
		    completed_at = %s
		WHERE task_id = ?
	`, completedAt), status, output, errMsg, taskID)
	return err
}

// StartTask marks a task as started
func (s *Storage) StartTask(taskID string) error {
	_, err := s.db.Exec(`
		UPDATE task_history
		SET status = 'running', started_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, taskID)
	return err
}

// IncrementRetryCount increments the retry counter
func (s *Storage) IncrementRetryCount(taskID string) error {
	_, err := s.db.Exec(`
		UPDATE task_history
		SET retry_count = retry_count + 1
		WHERE task_id = ?
	`, taskID)
	return err
}

// GetTask gets a task by ID
func (s *Storage) GetTask(taskID string) (*TaskHistory, error) {
	var t TaskHistory
	var startedAt, completedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, task_id, task_name, status, input, output, error, depends_on,
		       retry_count, max_retries, started_at, completed_at, created_at
		FROM task_history
		WHERE task_id = ?
	`, taskID).Scan(&t.ID, &t.TaskID, &t.TaskName, &t.Status, &t.Input, &t.Output, &t.Error,
		&t.DependsOn, &t.RetryCount, &t.MaxRetries, &startedAt, &completedAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

// GetPendingTasks returns tasks that can be executed (dependencies completed)
func (s *Storage) GetPendingTasks(limit int) ([]TaskHistory, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.task_id, t.task_name, t.status, t.input, t.output, t.error,
		       t.depends_on, t.retry_count, t.max_retries, t.started_at, t.completed_at, t.created_at
		FROM task_history t
		WHERE t.status = 'pending'
		  AND (t.depends_on IS NULL OR t.depends_on = ''
		      OR EXISTS (SELECT 1 FROM task_history d WHERE d.task_id = t.depends_on AND d.status = 'completed'))
		ORDER BY t.created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []TaskHistory
	for rows.Next() {
		var t TaskHistory
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.TaskID, &t.TaskName, &t.Status, &t.Input, &t.Output, &t.Error,
			&t.DependsOn, &t.RetryCount, &t.MaxRetries, &startedAt, &completedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			t.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// GetTaskHistory returns task history with filters
func (s *Storage) GetTaskHistory(taskName string, status string, limit int) ([]TaskHistory, error) {
	query := `
		SELECT id, task_id, task_name, status, input, output, error, depends_on,
		       retry_count, max_retries, started_at, completed_at, created_at
		FROM task_history
		WHERE 1=1
	`
	args := []interface{}{}

	if taskName != "" {
		query += " AND task_name = ?"
		args = append(args, taskName)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []TaskHistory
	for rows.Next() {
		var t TaskHistory
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.TaskID, &t.TaskName, &t.Status, &t.Input, &t.Output, &t.Error,
			&t.DependsOn, &t.RetryCount, &t.MaxRetries, &startedAt, &completedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			t.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// RetryFailedTask retries a failed task if within retry limit
func (s *Storage) RetryFailedTask(taskID string) (bool, error) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return false, err
	}
	if task.Status != "failed" {
		return false, nil
	}
	if task.RetryCount >= task.MaxRetries {
		return false, nil
	}

	err = s.UpdateTaskStatus(taskID, "pending", "", "")
	if err != nil {
		return false, err
	}
	err = s.IncrementRetryCount(taskID)
	return true, err
}

// ============ User Task Splitting ============

type UserTask struct {
	ID           string  `json:"id"`
	Session      string  `json:"session"`
	CreatedAt    int64   `json:"created_at"`
	CompletedAt  *int64  `json:"completed_at,omitempty"`
	StartedAt    *int64  `json:"started_at,omitempty"`
	Total        int     `json:"total"`
	Completed    int     `json:"completed"`
	Status       string  `json:"status"`
	Instructions string  `json:"instructions"`
	Result       string  `json:"result,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type UserSubtask struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id"`
	IndexNum    int     `json:"index_num"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Result      string  `json:"result,omitempty"`
	Process     string  `json:"process,omitempty"`
	StartedAt   *int64  `json:"started_at,omitempty"`
	CompletedAt *int64  `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type ArchiveStats struct {
	SessionKey        string `json:"session_key"`
	ArchivedCount     int    `json:"archived_count"`
	LastSourceMessage int64  `json:"last_source_message"`
}

// CreateUserTask creates a new user task with subtasks
func (s *Storage) CreateUserTask(id, session, instructions string, subtasks []string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Insert parent task
	_, err = tx.Exec(`
		INSERT INTO user_tasks (id, session, created_at, total, status, instructions)
		VALUES (?, ?, ?, ?, 'pending', ?)
	`, id, session, now, len(subtasks), instructions)
	if err != nil {
		return "", err
	}

	// Insert subtasks
	for i, desc := range subtasks {
		subtaskID := fmt.Sprintf("%s-sub%d", id, i)
		_, err = tx.Exec(`
			INSERT INTO user_subtasks (id, task_id, index_num, description, status)
			VALUES (?, ?, ?, ?, 'pending')
		`, subtaskID, id, i, desc)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return id, nil
}

// GetUserTask gets a user task by ID
func (s *Storage) GetUserTask(id string) (*UserTask, error) {
	var t UserTask
	var completedAt sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, session, created_at, completed_at, total, completed, status, instructions, result
		FROM user_tasks WHERE id = ?
	`, id).Scan(&t.ID, &t.Session, &t.CreatedAt, &completedAt, &t.Total, &t.Completed, &t.Status, &t.Instructions, &t.Result)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Int64
	}
	return &t, nil
}

// GetUserSubtasks gets all subtasks for a user task
func (s *Storage) GetUserSubtasks(taskID string) ([]UserSubtask, error) {
	rows, err := s.db.Query(`
		SELECT id, task_id, index_num, description, status, result, process, started_at, completed_at, error
		FROM user_subtasks WHERE task_id = ? ORDER BY index_num
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subtasks []UserSubtask
	for rows.Next() {
		var st UserSubtask
		var completedAt, startedAt sql.NullInt64
		if err := rows.Scan(&st.ID, &st.TaskID, &st.IndexNum, &st.Description, &st.Status, &st.Result, &st.Process, &startedAt, &completedAt, &st.Error); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			st.StartedAt = &startedAt.Int64
		}
		if completedAt.Valid {
			st.CompletedAt = &completedAt.Int64
		}
		subtasks = append(subtasks, st)
	}
	return subtasks, nil
}

// GetPendingSubtask gets the next pending subtask
func (s *Storage) GetPendingSubtask(taskID string) (*UserSubtask, error) {
	var st UserSubtask
	var completedAt, startedAt sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, task_id, index_num, description, status, result, process, started_at, completed_at, error
		FROM user_subtasks WHERE task_id = ? AND status = 'pending' ORDER BY index_num LIMIT 1
	`, taskID).Scan(&st.ID, &st.TaskID, &st.IndexNum, &st.Description, &st.Status, &st.Result, &st.Process, &startedAt, &completedAt, &st.Error)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		st.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		st.CompletedAt = &completedAt.Int64
	}
	return &st, nil
}

// StartSubtask marks a subtask as started
func (s *Storage) StartSubtask(subtaskID string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`
		UPDATE user_subtasks SET status = 'running', started_at = ? WHERE id = ?
	`, now, subtaskID)
	return err
}

// UpdateSubtaskProcess updates the execution process of a subtask
func (s *Storage) UpdateSubtaskProcess(subtaskID, process string) error {
	_, err := s.db.Exec(`
		UPDATE user_subtasks SET process = ? WHERE id = ?
	`, process, subtaskID)
	return err
}

// UpdateSubtaskStatus updates subtask status and result
func (s *Storage) UpdateSubtaskStatus(subtaskID, status, result string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Update subtask
	_, err = tx.Exec(`
		UPDATE user_subtasks SET status = ?, result = ?, completed_at = ? WHERE id = ?
	`, status, result, now, subtaskID)
	if err != nil {
		return err
	}

	// Get task_id from subtask
	var taskID string
	err = tx.QueryRow(`SELECT task_id FROM user_subtasks WHERE id = ?`, subtaskID).Scan(&taskID)
	if err != nil {
		return err
	}

	// Update parent task completed count
	switch status {
	case "completed":
		_, err = tx.Exec(`
			UPDATE user_tasks SET completed = completed + 1 WHERE id = ?
		`, taskID)
		if err != nil {
			return err
		}
	case "running":
		// Mark task as running
		_, err = tx.Exec(`
			UPDATE user_tasks SET status = 'running', started_at = ? WHERE id = ? AND status = 'pending'
		`, now, taskID)
		if err != nil {
			return err
		}
	}

	// Check if all subtasks completed
	var total, completed int
	err = tx.QueryRow(`SELECT total, completed FROM user_tasks WHERE id = ?`, taskID).Scan(&total, &completed)
	if err != nil {
		return err
	}

	if completed >= total {
		_, err = tx.Exec(`UPDATE user_tasks SET status = 'completed', completed_at = ? WHERE id = ?`, now, taskID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateUserTaskError updates task error message
func (s *Storage) UpdateUserTaskError(taskID, errMsg string) error {
	_, err := s.db.Exec(`UPDATE user_tasks SET error = ?, status = 'failed' WHERE id = ?`, errMsg, taskID)
	return err
}

// UpdateUserTaskResult updates the final result of a user task
func (s *Storage) UpdateUserTaskResult(taskID, result string) error {
	_, err := s.db.Exec(`UPDATE user_tasks SET result = ? WHERE id = ?`, result, taskID)
	return err
}

// GetUserTasksBySession gets all tasks for a session
func (s *Storage) GetUserTasksBySession(session string, limit int) ([]UserTask, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT id, session, created_at, completed_at, total, completed, status, instructions, result
		FROM user_tasks WHERE session = ? ORDER BY created_at DESC LIMIT ?
	`, session, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []UserTask
	for rows.Next() {
		var t UserTask
		var completedAt sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Session, &t.CreatedAt, &completedAt, &t.Total, &t.Completed, &t.Status, &t.Instructions, &t.Result); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Int64
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

