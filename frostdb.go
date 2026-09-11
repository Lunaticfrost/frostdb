// Package frostdb provides an embeddable, thread-safe, pure-Go key-value database
// with write-ahead persistence, binary-safe storage, and zero external dependencies.
package frostdb

import (
	"github.com/Lunaticfrost/frostdb/internal/engine"
)

// Re-export core types
type (
	// DB represents an embeddable FrostDB key-value database instance.
	DB = engine.Store

	// Options defines configuration options for opening a database.
	Options = engine.Options

	// SyncPolicy controls how aggressively mutations are flushed to disk.
	SyncPolicy = engine.SyncPolicy

	// WriteBatch stages multiple mutations to be applied atomically.
	WriteBatch = engine.WriteBatch

	// CompactionStats provides metrics produced by a database compaction run.
	CompactionStats = engine.CompactionStats
)

// Re-export SyncPolicy constants
const (
	// SyncNone delegates disk flushing entirely to the operating system page cache.
	SyncNone = engine.SyncNone

	// SyncPeriodic flushes writes to disk at regular 1-second intervals.
	SyncPeriodic = engine.SyncPeriodic

	// SyncAlways issues an fsync on every single write for maximum durability.
	SyncAlways = engine.SyncAlways
)

// Re-export configuration defaults and sentinel errors
var (
	// DefaultOptions provides balanced durability and performance (periodic 1-second sync).
	DefaultOptions = engine.DefaultOptions

	// ErrDatabaseLocked is returned when attempting to open a directory held by another process.
	ErrDatabaseLocked = engine.ErrDatabaseLocked

	// ErrCorruptRecord indicates a checksum mismatch or corrupted data framing.
	ErrCorruptRecord = engine.ErrCorruptRecord

	// ErrKeyTooLarge indicates the key exceeds the maximum permitted size (64 KB).
	ErrKeyTooLarge = engine.ErrKeyTooLarge

	// ErrValueTooLarge indicates the value exceeds the maximum permitted size (16 MB).
	ErrValueTooLarge = engine.ErrValueTooLarge
)

// Open opens or creates a persistent database at dataDir with the provided options.
func Open(dataDir string, opts ...Options) (*DB, error) {
	return engine.Open(dataDir, opts...)
}

// New creates a new in-memory database instance without disk persistence.
func New() *DB {
	return engine.NewStore()
}

// NewWriteBatch creates a new empty batch for staging atomic operations.
func NewWriteBatch() *WriteBatch {
	return engine.NewWriteBatch()
}
