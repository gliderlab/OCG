//go:build binddb

package binddb

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

/*
binddb - SQLite Binary Binding

This module binds the executable to a specific SQLite database file.
The binding uses an overlay technique to embed an instance key in the
executable binary, and verifies this key against the database on startup.

Usage:
  - Build with -tags=binddb to enable
  - Use --rebind to rebind to a different executable
  - Database path: <binary-dir>/<binary-name>.db
*/

const (
	overlayMagic  = "CODEXIK1"
	overlayKeyLen = 32
	overlayLen    = len(overlayMagic) + overlayKeyLen
)

// BindDB checks if the executable is bound to a database and binds if needed.
// Returns the database path if binding is successful, or empty string if not applicable.
func BindDB() (string, error) {
	rebind := os.Getenv("OCG_BIND_REBIND") == "1"

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %v", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to normalize executable path: %v", err)
	}

	// Cleanup stale old executables
	cleanupStaleOldExecutables(exePath)

	dbPath := dbPathForExe(exePath)

	// Check if database exists - if not, binding is not applicable
	if !dbExists(dbPath) {
		return "", nil
	}

	instanceKey, hasOverlay, err := readOverlayKey(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to read instance key: %v", err)
	}

	// First run: no overlay yet
	if !hasOverlay {
		key, err := firstRunSetup(dbPath, exePath, rebind)
		if err != nil {
			return "", fmt.Errorf("first run setup failed: %v", err)
		}
		if err := ensureOverlayWithKey(exePath, key); err != nil {
			return "", fmt.Errorf("failed to write overlay: %v", err)
		}
		// Restart to apply binding
		if err := restartSelf(exePath, os.Args[1:]); err != nil {
			return "", fmt.Errorf("restart failed: %v", err)
		}
		os.Exit(0)
		return "", nil
	}

	// Open database and verify binding
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := bindExecutable(db, exePath, instanceKey, rebind); err != nil {
		return "", fmt.Errorf("bind failed: %v", err)
	}

	if err := recordStart(db); err != nil {
		return "", fmt.Errorf("failed to write start record: %v", err)
	}

	log.Printf("OK: %s bound to %s", exePath, dbPath)
	return dbPath, nil
}

func dbPathForExe(exePath string) string {
	base := filepath.Base(exePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(exePath), name+".db")
}

func bindExecutable(db *sql.DB, exePath, instanceKey string, rebind bool) error {
	if err := ensureMeta(db); err != nil {
		return err
	}

	storedKey, ok, err := getMeta(db, "instance_key")
	if err != nil {
		return err
	}
	if !ok {
		if err := setMeta(db, "instance_key", instanceKey); err != nil {
			return err
		}
		if err := setMeta(db, "exe_path", exePath); err != nil {
			return err
		}
		return setMeta(db, "bound_at", time.Now().UTC().Format(time.RFC3339))
	}

	if storedKey == instanceKey {
		return nil
	}

	if !rebind {
		storedPath, _, _ := getMeta(db, "exe_path")
		return fmt.Errorf("database is bound to another executable: %s (key=%s). current=%s (key=%s). Use OCG_BIND_REBIND=1 to migrate",
			storedPath,
			storedKey,
			exePath,
			instanceKey,
		)
	}

	if err := setMeta(db, "instance_key", instanceKey); err != nil {
		return err
	}
	if err := setMeta(db, "exe_path", exePath); err != nil {
		return err
	}
	return setMeta(db, "rebound_at", time.Now().UTC().Format(time.RFC3339))
}

func ensureMeta(db *sql.DB) error {
	cols, err := tableColumns(db, "meta")
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (
        	key TEXT PRIMARY KEY,
        	value TEXT NOT NULL
    	);`)
		return err
	}
	if hasColumns(cols, "key", "value") {
		return nil
	}
	if hasColumns(cols, "k", "v") {
		return migrateMetaKV(db)
	}
	return fmt.Errorf("unsupported meta schema: %v", cols)
}

func getMeta(db *sql.DB, key string) (string, bool, error) {
	var v string
	err := db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func setMeta(db *sql.DB, key, val string) error {
	_, err := db.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, val)
	return err
}

func recordStart(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS starts (
        	id INTEGER PRIMARY KEY AUTOINCREMENT,
        	started_at TEXT NOT NULL
    	);`); err != nil {
		return err
	}
	_, err := db.Exec("INSERT INTO starts (started_at) VALUES (?)", time.Now().UTC().Format(time.RFC3339))
	return err
}

func firstRunSetup(dbPath, exePath string, rebind bool) (string, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	if err := ensureMeta(db); err != nil {
		return "", err
	}

	storedKey, ok, err := getMeta(db, "instance_key")
	if err != nil {
		return "", err
	}
	storedPath, _, err := getMeta(db, "exe_path")
	if err != nil {
		return "", err
	}

	if ok {
		if storedPath != "" && storedPath != exePath && !rebind {
			return "", fmt.Errorf(
				"database is bound to another executable: %s (key=%s). current=%s. Use OCG_BIND_REBIND=1 to migrate",
				storedPath,
				storedKey,
				exePath,
			)
		}
		if storedPath == "" || storedPath != exePath {
			if err := setMeta(db, "exe_path", exePath); err != nil {
				return "", err
			}
			if storedPath != exePath {
				if err := setMeta(db, "rebound_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
					return "", err
				}
			}
		}
		if err := recordStart(db); err != nil {
			return "", err
		}
		return storedKey, nil
	}

	key, err := newInstanceKey()
	if err != nil {
		return "", err
	}
	if err := bindExecutable(db, exePath, key, rebind); err != nil {
		return "", err
	}
	if err := recordStart(db); err != nil {
		return "", err
	}
	return key, nil
}

func dbExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func tableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cols, nil
}

func hasColumns(cols []string, a, b string) bool {
	var hasA, hasB bool
	for _, c := range cols {
		if c == a {
			hasA = true
		}
		if c == b {
			hasB = true
		}
	}
	return hasA && hasB
}

func migrateMetaKV(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE meta_new (
        	key TEXT PRIMARY KEY,
        	value TEXT NOT NULL
    	);`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO meta_new (key, value) SELECT k, v FROM meta;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE meta;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE meta_new RENAME TO meta;`); err != nil {
		return err
	}
	return tx.Commit()
}

// restartSelf starts a new process and returns immediately.
// The new process runs independently.
func restartSelf(exePath string, args []string) error {
	cmd := exec.Command(exePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Start()
}

func cleanupStaleOldExecutables(exePath string) {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	prefix := "." + base + ".old."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

func readOverlayKey(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	return readOverlayKeyFromFile(f)
}

func readOverlayKeyFromFile(f *os.File) (string, bool, error) {
	st, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	if st.Size() < int64(overlayLen) {
		return "", false, nil
	}

	buf := make([]byte, overlayLen)
	if _, err := f.ReadAt(buf, st.Size()-int64(overlayLen)); err != nil {
		return "", false, err
	}
	if !bytes.HasPrefix(buf, []byte(overlayMagic)) {
		return "", false, nil
	}

	key := string(buf[len(overlayMagic):])
	if len(key) != overlayKeyLen || !isHex(key) {
		return "", false, fmt.Errorf("instance key data is corrupted")
	}
	return key, true, nil
}

func ensureOverlayWithKey(exePath, key string) error {
	if existing, ok, err := readOverlayKey(exePath); err != nil {
		return err
	} else if ok {
		if existing != key {
			return fmt.Errorf("overlay key mismatch")
		}
		return nil
	}

	if err := appendOverlay(exePath, key); err == nil {
		return nil
	}

	if existing, ok, err := readOverlayKey(exePath); err != nil {
		return err
	} else if ok {
		if existing != key {
			return fmt.Errorf("overlay key mismatch")
		}
		return nil
	}

	return rewriteWithOverlay(exePath, key)
}

func appendOverlay(path, key string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, ok, err := readOverlayKeyFromFile(f); err != nil {
		return err
	} else if ok {
		return nil
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	if _, err := f.Write([]byte(overlayMagic + key)); err != nil {
		return err
	}
	return f.Sync()
}

func rewriteWithOverlay(exePath, key string) error {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.old.%d.%d", base, os.Getpid(), time.Now().UnixNano()))

	if err := os.Rename(exePath, tmp); err != nil {
		return err
	}

	rollback := func() {
		_ = os.Remove(exePath)
		_ = os.Rename(tmp, exePath)
	}

	src, err := os.Open(tmp)
	if err != nil {
		_ = os.Rename(tmp, exePath)
		return err
	}
	defer src.Close()

	st, err := src.Stat()
	if err != nil {
		_ = os.Rename(tmp, exePath)
		return err
	}

	dst, err := os.OpenFile(exePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode())
	if err != nil {
		_ = os.Rename(tmp, exePath)
		return err
	}

	success := false
	defer func() {
		_ = dst.Close()
		if !success {
			rollback()
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if _, err := dst.Write([]byte(overlayMagic + key)); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}

	success = true
	_ = os.Remove(tmp)
	return nil
}

func newInstanceKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}
