package engine

import (
	"fmt"
	"testing"
)

func TestStoreCompactionSpaceReclaimed(t *testing.T) {
	tempDir := t.TempDir()

	db, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Write 100 keys and overwrite each 5 times (500 log records)
	for round := 0; round < 5; round++ {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("user:%d", i)
			val := []byte(fmt.Sprintf("data-round-%d-value-%d", round, i))
			if err := db.Set(key, val); err != nil {
				t.Fatalf("Set failed: %v", err)
			}
		}
	}

	// Delete 50 keys (50 tombstones added to WAL)
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("user:%d", i)
		if !db.Delete(key) {
			t.Fatalf("Delete %s failed", key)
		}
	}

	beforeSize, err := db.wal.FileSize()
	if err != nil {
		t.Fatalf("FileSize failed: %v", err)
	}

	// Execute compaction
	stats, err := db.Compact()
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if stats.KeysCompacted != 50 {
		t.Errorf("expected 50 keys compacted, got %d", stats.KeysCompacted)
	}
	if stats.ReclaimedBytes <= 0 {
		t.Errorf("expected positive reclaimed bytes, got %d", stats.ReclaimedBytes)
	}
	if stats.ReclaimedPct <= 50.0 {
		t.Errorf("expected at least 50%% reclaimed, got %.2f%%", stats.ReclaimedPct)
	}

	afterSize, err := db.wal.FileSize()
	if err != nil {
		t.Fatalf("FileSize after failed: %v", err)
	}

	if afterSize >= beforeSize {
		t.Errorf("expected afterSize (%d) < beforeSize (%d)", afterSize, beforeSize)
	}

	// Verify data integrity: deleted keys must not exist
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("user:%d", i)
		if db.Exists(key) {
			t.Errorf("key %s should not exist", key)
		}
	}

	// Active keys must have the latest round values
	for i := 50; i < 100; i++ {
		key := fmt.Sprintf("user:%d", i)
		val, exists := db.Get(key)
		if !exists {
			t.Errorf("key %s should exist", key)
		}
		expected := fmt.Sprintf("data-round-4-value-%d", i)
		if string(val) != expected {
			t.Errorf("key %s expected %q, got %q", key, expected, string(val))
		}
	}
}

func TestStoreCompactionRecovery(t *testing.T) {
	tempDir := t.TempDir()

	// Session 1: write, compact, close
	db1, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open db1 failed: %v", err)
	}

	_ = db1.Set("city", []byte("Oslo"))
	_ = db1.Set("country", []byte("Norway"))
	_ = db1.Set("temp", []byte("-10"))
	db1.Delete("temp")

	_, err = db1.Compact()
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if err := db1.Close(); err != nil {
		t.Fatalf("Close db1 failed: %v", err)
	}

	// Session 2: reopen from compacted log and verify
	db2, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open db2 failed: %v", err)
	}
	defer db2.Close()

	if db2.Size() != 2 {
		t.Errorf("expected size 2, got %d", db2.Size())
	}

	if val, ok := db2.Get("city"); !ok || string(val) != "Oslo" {
		t.Errorf("expected 'Oslo', got %q", string(val))
	}
	if val, ok := db2.Get("country"); !ok || string(val) != "Norway" {
		t.Errorf("expected 'Norway', got %q", string(val))
	}
	if db2.Exists("temp") {
		t.Error("deleted key 'temp' should not exist")
	}
}

func TestStoreCompactionInMemory(t *testing.T) {
	store := NewStore()
	_, err := store.Compact()
	if err == nil {
		t.Error("expected error when compacting in-memory store, got nil")
	}
}

func TestStoreCompactionEmptyStore(t *testing.T) {
	tempDir := t.TempDir()

	db, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	stats, err := db.Compact()
	if err != nil {
		t.Fatalf("Compact empty store failed: %v", err)
	}

	if stats.KeysCompacted != 0 {
		t.Errorf("expected 0 keys compacted, got %d", stats.KeysCompacted)
	}
	if db.Size() != 0 {
		t.Errorf("expected size 0, got %d", db.Size())
	}
}
