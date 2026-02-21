// Package kv provides a fast in-memory key-value store with persistence using BadgerDB
package kv

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
)

type KV struct {
	db       *badger.DB
	opts     badger.Options
	closed   bool
	closedMu sync.RWMutex
}

// Options for KV store
type Options struct {
	Dir           string // Data directory
	ValueDir      string // Value log directory (optional)
	SyncWrites   bool   // Sync writes to disk
	Compression   bool   // Enable compression
	MemoryMode    bool   // In-memory only (no persistence)
	MaxCacheSize  int64  // Cache size in MB
	ValueLogMaxMB int64  // Max value log size in MB
}

// DefaultOptions returns default options
func DefaultOptions(dir string) Options {
	return Options{
		Dir:           dir,
		SyncWrites:    false, // Async for performance
		Compression:    true,
		MemoryMode:    false,
		MaxCacheSize:  256,
		ValueLogMaxMB: 256, // 256MB - within valid range [1MB, 2GB)
	}
}

// Open opens a KV store
func Open(opt Options) (*KV, error) {
	// For in-memory mode, don't set Dir or ValueLogFileSize
	if !opt.MemoryMode {
		if opt.Dir == "" {
			opt.Dir = filepath.Join(os.TempDir(), "ocg-kv")
		}
	}

	opts := badger.DefaultOptions(opt.Dir)
	opts.SyncWrites = opt.SyncWrites

	if opt.Compression && !opt.MemoryMode {
		opts.Compression = options.ZSTD
	}

	if !opt.MemoryMode && opt.ValueLogMaxMB > 0 {
		opts.ValueLogFileSize = opt.ValueLogMaxMB * 1024 * 1024
	}

	if opt.MemoryMode {
		opts.InMemory = true
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger failed: %w", err)
	}

	kv := &KV{
		db:   db,
		opts: opts,
	}

	log.Printf("[KV] Opened: %s (memory: %v)", opt.Dir, opt.MemoryMode)
	return kv, nil
}

// OpenDefault opens KV with default options in temp dir
func OpenDefault() (*KV, error) {
	return Open(DefaultOptions(""))
}

// Close closes the KV store
func (k *KV) Close() error {
	k.closedMu.Lock()
	defer k.closedMu.Unlock()

	if k.closed {
		return nil
	}

	k.closed = true
	return k.db.Close()
}

// IsClosed returns if the KV is closed
func (k *KV) IsClosed() bool {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()
	return k.closed
}

// Set sets a key-value pair
func (k *KV) Set(key, value string) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	return k.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

// SetWithTTL sets a key-value pair with TTL
func (k *KV) SetWithTTL(key, value string, ttl time.Duration) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	return k.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), []byte(value)).WithTTL(ttl)
		return txn.SetEntry(e)
	})
}

// Get gets a value by key
func (k *KV) Get(key string) (string, error) {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return "", fmt.Errorf("KV is closed")
	}

	var result string
	err := k.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		result = string(val)
		return nil
	})
	return result, err
}

// GetBytes gets raw bytes by key
func (k *KV) GetBytes(key string) ([]byte, error) {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return nil, fmt.Errorf("KV is closed")
	}

	var result []byte
	err := k.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		result = val
		return nil
	})
	return result, err
}

// Delete deletes a key
func (k *KV) Delete(key string) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	return k.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Exists checks if a key exists
func (k *KV) Exists(key string) (bool, error) {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return false, fmt.Errorf("KV is closed")
	}

	exists := false
	err := k.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		exists = err == nil
		return err
	})
	return exists, err
}

// Iterate iterates over keys with given prefix
func (k *KV) Iterate(prefix string, fn func(key, value string) bool) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	return k.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			if !fn(string(item.Key()), string(val)) {
				break
			}
		}
		return nil
	})
}

// Keys returns all keys matching prefix
func (k *KV) Keys(prefix string) ([]string, error) {
	var keys []string
	err := k.Iterate(prefix, func(key, _ string) bool {
		keys = append(keys, key)
		return true
	})
	return keys, err
}

// Count returns count of keys matching prefix
func (k *KV) Count(prefix string) (int, error) {
	count := 0
	err := k.Iterate(prefix, func(_, _ string) bool {
		count++
		return true
	})
	return count, err
}

// DeletePrefix deletes all keys with given prefix
func (k *KV) DeletePrefix(prefix string) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	keys, err := k.Keys(prefix)
	if err != nil {
		return err
	}

	return k.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			if err := txn.Delete([]byte(key)); err != nil {
				log.Printf("[KV] Delete %s failed: %v", key, err)
			}
		}
		return nil
	})
}

// SetMap sets multiple key-value pairs in a single transaction
func (k *KV) SetMap(m map[string]string) error {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return fmt.Errorf("KV is closed")
	}

	return k.db.Update(func(txn *badger.Txn) error {
		for k, v := range m {
			if err := txn.Set([]byte(k), []byte(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetMap gets multiple keys at once
func (k *KV) GetMap(keys []string) (map[string]string, error) {
	k.closedMu.RLock()
	defer k.closedMu.RUnlock()

	if k.closed {
		return nil, fmt.Errorf("KV is closed")
	}

	result := make(map[string]string)
	err := k.db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte(key))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue
				}
				return err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			result[key] = string(val)
		}
		return nil
	})
	return result, err
}

// ===== Task-specific helpers =====

// Task prefixes
const (
	PrefixTask       = "task:"
	PrefixSubtask    = "task:sub:"
	PrefixProgress   = "task:progress:"
	PrefixToken      = "token:"
	PrefixCache      = "cache:"
)

// SetTaskStatus sets task status
func (k *KV) SetTaskStatus(taskID, status string) error {
	return k.Set(PrefixTask+taskID+":status", status)
}

// GetTaskStatus gets task status
func (k *KV) GetTaskStatus(taskID string) (string, error) {
	return k.Get(PrefixTask + taskID + ":status")
}

// SetTaskProgress sets task progress (completed/total)
func (k *KV) SetTaskProgress(taskID string, completed, total int) error {
	return k.Set(PrefixProgress+taskID, fmt.Sprintf("%d/%d", completed, total))
}

// GetTaskProgress gets task progress
func (k *KV) GetTaskProgress(taskID string) (string, error) {
	return k.Get(PrefixProgress + taskID)
}

// SetSubtaskStatus sets subtask status
func (k *KV) SetSubtaskStatus(taskID string, index int, status string) error {
	return k.Set(fmt.Sprintf("%s%d:status", PrefixSubtask+taskID+":", index), status)
}

// GetSubtaskStatus gets subtask status
func (k *KV) GetSubtaskStatus(taskID string, index int) (string, error) {
	return k.Get(fmt.Sprintf("%s%d:status", PrefixSubtask+taskID+":", index))
}

// SetTokenCache caches token count for session
func (k *KV) SetTokenCache(session string, tokens int) error {
	return k.SetWithTTL(PrefixToken+session, fmt.Sprintf("%d", tokens), 10*time.Minute)
}

// GetTokenCache gets cached token count
func (k *KV) GetTokenCache(session string) (int, error) {
	val, err := k.Get(PrefixToken + session)
	if err != nil {
		return 0, err
	}
	var tokens int
	fmt.Sscanf(val, "%d", &tokens)
	return tokens, nil
}

// DeleteTask deletes all keys related to a task
func (k *KV) DeleteTask(taskID string) error {
	return k.DeletePrefix(PrefixTask + taskID)
}

// ===== Stats =====

// Stats returns KV store statistics
func (k *KV) Stats() (map[string]interface{}, error) {
	if k.db == nil {
		return nil, fmt.Errorf("KV not initialized")
	}

	var sz int64
	var keyCount int
	err := k.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(nil); it.Valid(); it.Next() {
			sz += int64(len(it.Item().Key())) + it.Item().EstimatedSize()
			keyCount++
		}
		return nil
	})

	return map[string]interface{}{
		"keys":     keyCount,
		"size_mb":  sz / 1024 / 1024,
		"dir":      k.opts.Dir,
		"inmemory": k.opts.InMemory,
	}, err
}

// Compact compacts the database
func (k *KV) Compact() error {
	return k.db.RunValueLogGC(0.5)
}

// Flush forces flush to disk
func (k *KV) Flush() error {
	return k.db.Sync()
}
