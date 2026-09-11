package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Options holds configuration settings for opening a persistent Store.
type Options struct {
	SyncPolicy SyncPolicy
}

// DefaultOptions provides balanced durability and performance (periodic 1-second sync).
var DefaultOptions = Options{
	SyncPolicy: SyncPeriodic,
}

// Store represents a thread-safe key-value store with optional WAL persistence.
type Store struct {
	data    map[string]string
	mu      sync.RWMutex
	wal     *WAL
	dataDir string
}

// NewStore creates a new in-memory Store instance without disk persistence.
func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Open opens or creates a persistent database in dataDir.
// It initializes the WAL and automatically replays existing records to restore state.
func Open(dataDir string, opts ...Options) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data directory path cannot be empty")
	}

	opt := DefaultOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	walPath := filepath.Join(dataDir, "frost.wal")
	wal, err := OpenWAL(walPath, opt.SyncPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize wal: %w", err)
	}

	store := &Store{
		data:    make(map[string]string),
		wal:     wal,
		dataDir: dataDir,
	}

	// Replay existing log records to restore in-memory state
	_, err = wal.Replay(func(rec *Record) error {
		switch rec.Op {
		case OpSet:
			store.data[rec.Key] = rec.Value
		case OpDelete:
			delete(store.data, rec.Key)
		case OpClear:
			store.data = make(map[string]string)
		}
		return nil
	})

	if err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf("failed during wal recovery: %w", err)
	}

	return store, nil
}

// Set stores a key-value pair. If persistence is enabled, the mutation is written
// to the WAL before updating in-memory state.
func (s *Store) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		rec := NewRecord(OpSet, key, value)
		if err := s.wal.Append(rec); err != nil {
			return fmt.Errorf("wal append failed: %w", err)
		}
	}

	s.data[key] = value
	return nil
}

// Get retrieves a value by key from the store.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
	return value, exists
}

// Delete removes a key-value pair. If persistent, records a tombstone in the WAL.
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.data[key]
	if !exists {
		return false
	}

	if s.wal != nil {
		rec := NewRecord(OpDelete, key, "")
		if err := s.wal.Append(rec); err != nil {
			return false
		}
	}

	delete(s.data, key)
	return true
}

// Exists checks if a key exists in the store.
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.data[key]
	return exists
}

// Keys returns all keys in the store.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all key-value pairs from the store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		rec := NewRecord(OpClear, "", "")
		_ = s.wal.Append(rec)
	}

	s.data = make(map[string]string)
}

// Size returns the number of key-value pairs in the store.
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

// IsPersistent returns true if the store is backed by a WAL on disk.
func (s *Store) IsPersistent() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wal != nil
}

// DataDir returns the path to the database directory (empty for in-memory stores).
func (s *Store) DataDir() string {
	return s.dataDir
}

// Sync forces pending writes to be synced to physical disk.
func (s *Store) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		return s.wal.Sync()
	}
	return nil
}

// Close gracefully flushes pending writes and releases file handles.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		return s.wal.Close()
	}
	return nil
}