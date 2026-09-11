package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	data    map[string][]byte
	mu      sync.RWMutex
	wal     *WAL
	dataDir string
	flock   *FileLock
}

// NewStore creates a new in-memory Store instance without disk persistence.
func NewStore() *Store {
	return &Store{
		data: make(map[string][]byte),
	}
}

// Open opens or creates a persistent database in dataDir.
// It acquires an exclusive process lock on frost.lock, initializes the WAL,
// and automatically replays existing records to restore state.
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

	// Acquire process lock on the database directory
	lockPath := filepath.Join(dataDir, "frost.lock")
	flock, err := AcquireFileLock(lockPath)
	if err != nil {
		return nil, err
	}

	walPath := filepath.Join(dataDir, "frost.wal")
	wal, err := OpenWAL(walPath, opt.SyncPolicy)
	if err != nil {
		_ = flock.Release()
		return nil, fmt.Errorf("failed to initialize wal: %w", err)
	}

	store := &Store{
		data:    make(map[string][]byte),
		wal:     wal,
		dataDir: dataDir,
		flock:   flock,
	}

	// Replay existing log records to restore in-memory state
	_, err = wal.Replay(func(rec *Record) error {
		switch rec.Op {
		case OpSet:
			store.data[rec.Key] = rec.Value
		case OpDelete:
			delete(store.data, rec.Key)
		case OpClear:
			store.data = make(map[string][]byte)
		}
		return nil
	})

	if err != nil {
		_ = wal.Close()
		_ = flock.Release()
		return nil, fmt.Errorf("failed during wal recovery: %w", err)
	}

	return store, nil
}

// Set stores a key-value pair with binary-safe value. If persistence is enabled,
// the mutation is written to the WAL before updating in-memory state.
func (s *Store) Set(key string, value []byte) error {
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

	valCopy := make([]byte, len(value))
	copy(valCopy, value)
	s.data[key] = valCopy
	return nil
}

// SetString is a convenience helper for storing string values.
func (s *Store) SetString(key, value string) error {
	return s.Set(key, []byte(value))
}

// Get retrieves a binary value by key from the store.
func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
	if !exists {
		return nil, false
	}
	valCopy := make([]byte, len(value))
	copy(valCopy, value)
	return valCopy, true
}

// GetString is a convenience helper for retrieving values as strings.
func (s *Store) GetString(key string) (string, bool) {
	val, exists := s.Get(key)
	if !exists {
		return "", false
	}
	return string(val), true
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
		rec := NewRecord(OpDelete, key, nil)
		if err := s.wal.Append(rec); err != nil {
			return false
		}
	}

	delete(s.data, key)
	return true
}

// Write applies all mutations in a WriteBatch atomically to disk and memory.
func (s *Store) Write(batch *WriteBatch) error {
	if batch == nil || len(batch.ops) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Write all batch records to WAL in a single disk operation
	if s.wal != nil {
		records := make([]*Record, 0, len(batch.ops))
		for _, op := range batch.ops {
			records = append(records, NewRecord(op.op, op.key, op.value))
		}
		if err := s.wal.AppendBatch(records); err != nil {
			return fmt.Errorf("failed to write batch to wal: %w", err)
		}
	}

	// 2. Apply all mutations to in-memory state
	for _, op := range batch.ops {
		switch op.op {
		case OpSet:
			valCopy := make([]byte, len(op.value))
			copy(valCopy, op.value)
			s.data[op.key] = valCopy
		case OpDelete:
			delete(s.data, op.key)
		case OpClear:
			s.data = make(map[string][]byte)
		}
	}

	return nil
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
		rec := NewRecord(OpClear, "", nil)
		_ = s.wal.Append(rec)
	}

	s.data = make(map[string][]byte)
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

// Close gracefully flushes pending writes and releases file handles and locks.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var walErr error
	if s.wal != nil {
		walErr = s.wal.Close()
		s.wal = nil
	}

	var lockErr error
	if s.flock != nil {
		lockErr = s.flock.Release()
		s.flock = nil
	}

	if walErr != nil {
		return walErr
	}
	return lockErr
}

// CompactionStats contains metrics produced by a log compaction run.
type CompactionStats struct {
	BeforeBytes    int64
	AfterBytes     int64
	ReclaimedBytes int64
	ReclaimedPct   float64
	KeysCompacted  int
	Duration       time.Duration
}

// Compact reclaims disk space by writing active keys to a fresh WAL file
// and atomically replacing the existing WAL via a POSIX rename.
func (s *Store) Compact() (CompactionStats, error) {
	start := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal == nil {
		return CompactionStats{}, fmt.Errorf("compaction is not supported for in-memory stores")
	}

	beforeSize, err := s.wal.FileSize()
	if err != nil {
		return CompactionStats{}, fmt.Errorf("failed to determine wal file size: %w", err)
	}

	walPath := s.wal.Path()
	compactPath := filepath.Join(s.dataDir, "frost.wal.compact")

	// Open a clean temporary WAL file with SyncAlways for durability during compaction
	compactWAL, err := OpenWAL(compactPath, SyncAlways)
	if err != nil {
		return CompactionStats{}, fmt.Errorf("failed to open compact wal: %w", err)
	}

	// Write only currently active, live keys
	var keysCount int
	for k, v := range s.data {
		rec := NewRecord(OpSet, k, v)
		if err := compactWAL.Append(rec); err != nil {
			_ = compactWAL.Close()
			_ = os.Remove(compactPath)
			return CompactionStats{}, fmt.Errorf("failed to write record during compaction: %w", err)
		}
		keysCount++
	}

	// Flush and close temporary compact WAL
	if err := compactWAL.Close(); err != nil {
		_ = os.Remove(compactPath)
		return CompactionStats{}, fmt.Errorf("failed to close compact wal: %w", err)
	}

	// Close the current active WAL handle before replacing it
	policy := s.wal.SyncPolicy()
	if err := s.wal.Close(); err != nil {
		_ = os.Remove(compactPath)
		return CompactionStats{}, fmt.Errorf("failed to close active wal before swap: %w", err)
	}

	// Atomically replace the old WAL with the compacted WAL
	if err := os.Rename(compactPath, walPath); err != nil {
		reopenWAL, _ := OpenWAL(walPath, policy)
		s.wal = reopenWAL
		return CompactionStats{}, fmt.Errorf("failed to atomically replace wal file: %w", err)
	}

	// Re-open active WAL at the original path
	newWAL, err := OpenWAL(walPath, policy)
	if err != nil {
		return CompactionStats{}, fmt.Errorf("failed to reopen wal after compaction: %w", err)
	}
	s.wal = newWAL

	afterSize, err := s.wal.FileSize()
	if err != nil {
		afterSize = 0
	}

	reclaimed := beforeSize - afterSize
	if reclaimed < 0 {
		reclaimed = 0
	}
	var pct float64
	if beforeSize > 0 {
		pct = (float64(reclaimed) / float64(beforeSize)) * 100.0
	}

	return CompactionStats{
		BeforeBytes:    beforeSize,
		AfterBytes:     afterSize,
		ReclaimedBytes: reclaimed,
		ReclaimedPct:   pct,
		KeysCompacted:  keysCount,
		Duration:       time.Since(start),
	}, nil
}