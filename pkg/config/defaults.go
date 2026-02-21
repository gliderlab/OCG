// Package config provides configuration types and defaults for OCG services
// Centralized management of all constants and default values

package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// ===== Ports =====

const (
	// DefaultGatewayPort is the standard port for OCG Gateway
	DefaultGatewayPort = 55003
	
	// Embedding port range
	DefaultEmbeddingPortMin = 50000
	DefaultEmbeddingPortMax = 60000
	
	// Llama.cpp server port range
	DefaultLlamaPortMin = 18000
	DefaultLlamaPortMax = 19000
	
	// Default CDP port for browser automation
	DefaultCDPPort = 18800
	
	// Default IRC port
	DefaultIRCPort = 6667
)

// ===== Paths =====

// DefaultDataDir returns the default data directory (~/.ocg)
func DefaultDataDir() string {
	if d := os.Getenv("OCG_DATA_DIR"); d != "" {
		return d
	}
	// Default to <binary-dir>/data
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "data")
}

// DefaultSocketPath returns the default Unix socket path (in temp dir)
func DefaultSocketPath() string {
	if s := os.Getenv("OCG_AGENT_SOCK"); s != "" {
		return s
	}
	// Use Unix socket consistently across all platforms for uniformity
	return filepath.Join(os.TempDir(), "ocg-agent.sock")
}

// DefaultPidDir returns the default PID directory (in temp dir)
func DefaultPidDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "ocg")
	}
	return filepath.Join(os.TempDir(), "ocg")
}

// DefaultDBPath returns the default database path (~/.ocg/ocg.db)
func DefaultDBPath() string {
	// Default to <binary-dir>/db/ocg.db
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "db", "ocg.db")
}

// ===== Installation =====

// DefaultInstallDir returns the installation directory (dynamically detected)
func DefaultInstallDir() string {
	// Check environment variable first
	if d := os.Getenv("OCG_INSTALL_DIR"); d != "" {
		return d
	}
	// Detect from current executable path
	exe, err := os.Executable()
	if err == nil {
		// Binary is at <install-dir>/ocg
		// So install dir is parent of binary
		return filepath.Dir(exe)
	}
	// Fallback
	return "/opt/ocg"
}

// DefaultWorkspaceDir returns the workspace directory (default: <binary-dir>/workspace)
func DefaultWorkspaceDir() string {
	// Check environment variable first
	if d := os.Getenv("OCG_WORKSPACE"); d != "" {
		return d
	}
	// Default to <binary-dir>/workspace
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "workspace")
}

// DefaultGatewayURL returns the default gateway URL
func DefaultGatewayURL() string {
	if u := os.Getenv("OCG_GATEWAY_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:55003"
}

// DefaultEmbeddingURL returns the default embedding service URL
func DefaultEmbeddingURL() string {
	if u := os.Getenv("OCG_EMBEDDING_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:50000"
}

// ===== Buffers/Limits =====

const (
	// Socket read buffer size
	SocketReadBufSize = 64 * 1024 // 64KB
	
	// Message limits
	TelegramMaxMsgLen   = 4096
	MaxWebPageChars     = 10000
	MaxProcessOutputChars = 8000
	MaxBrowserPageChars  = 15000
	
	// Browser defaults
	DefaultBrowserWidth  = 1280
	DefaultBrowserHeight = 720
)

// ===== Token/Context =====

const (
	// Context window defaults
	DefaultContextTokens  = 8192
	DefaultReserveTokens  = 1024
	DefaultSoftTokens     = 800
	DefaultMaxTokens      = 1000
)

// ===== Cron =====

const (
	// Cron iteration limits (in minutes)
	CronMaxIterWeekly  = 7 * 24 * 60  // 10080
	CronMaxIterMonthly = 31 * 24 * 60  // 44640
)
