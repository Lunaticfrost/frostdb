package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// SyncPolicy defines how aggressively FrostDB flushes writes to disk.
type SyncPolicy int

const (
	// SyncNone delegates disk flushing entirely to the operating system page cache.
	// Highest throughput, but vulnerable to data loss on sudden power loss.
	SyncNone SyncPolicy = iota

	// SyncPeriodic flushes writes to disk at regular intervals (default: 1 second).
	// Balances performance and safety (similar to Redis appendfsync everysec).
	SyncPeriodic

	// SyncAlways calls fsync on every single mutation.
	// Maximum durability guarantee, but lower write throughput.
	SyncAlways
)

// WAL represents an append-only Write-Ahead Log.
type WAL struct {
	file       *os.File
	path       string
	mu         sync.Mutex
	syncPolicy SyncPolicy
	closed     bool
	closeCh    chan struct{}
	wg         sync.WaitGroup
}

// OpenWAL opens or creates a WAL file at the given path with the specified sync policy.
func OpenWAL(path string, syncPolicy SyncPolicy) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open wal file: %w", err)
	}

	w := &WAL{
		file:       file,
		path:       path,
		syncPolicy: syncPolicy,
		closeCh:    make(chan struct{}),
	}

	if syncPolicy == SyncPeriodic {
		w.startPeriodicSync(1 * time.Second)
	}

	return w, nil
}

func (w *WAL) startPeriodicSync(interval time.Duration) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.mu.Lock()
				if !w.closed {
					_ = w.file.Sync()
				}
				w.mu.Unlock()
			case <-w.closeCh:
				return
			}
		}
	}()
}

// Append serializes and writes a record to the end of the WAL file.
func (w *WAL) Append(rec *Record) error {
	data, err := EncodeRecord(rec)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("wal is closed")
	}

	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("failed to write to wal: %w", err)
	}

	if w.syncPolicy == SyncAlways {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync wal to disk: %w", err)
		}
	}

	return nil
}

// Replay reads the WAL from byte 0, invoking the handler for each valid record.
// If a torn write is encountered at EOF (e.g. from an abrupt system crash),
// it truncates the file back to the last valid byte offset and safely halts recovery.
func (w *WAL) Replay(fn func(rec *Record) error) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, errors.New("wal is closed")
	}

	// Seek to the start of the file for reading
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to start of wal: %w", err)
	}

	var count int
	var validOffset int64

	for {
		rec, err := DecodeRecord(w.file)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			// If a corruption or partial write occurred at EOF, truncate to last valid offset
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrCorruptRecord) {
				if truncErr := w.file.Truncate(validOffset); truncErr != nil {
					return count, fmt.Errorf("failed to truncate corrupt tail in wal: %w", truncErr)
				}
				break
			}

			return count, fmt.Errorf("error reading wal at offset %d: %w", validOffset, err)
		}

		if err := fn(rec); err != nil {
			return count, fmt.Errorf("replay handler error: %w", err)
		}

		count++
		offset, err := w.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return count, fmt.Errorf("failed to determine current offset: %w", err)
		}
		validOffset = offset
	}

	// Ensure the file pointer is set to the end of the file for subsequent appends
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return count, fmt.Errorf("failed to seek to end of wal: %w", err)
	}

	return count, nil
}

// Sync forces an fsync on the underlying file.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return errors.New("wal is closed")
	}

	return w.file.Sync()
}

// Close gracefully flushes any pending writes and closes the WAL file handle.
func (w *WAL) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	// Stop background sync if running
	if w.syncPolicy == SyncPeriodic {
		close(w.closeCh)
		w.wg.Wait()
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	syncErr := w.file.Sync()
	closeErr := w.file.Close()

	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
