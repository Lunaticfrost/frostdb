package engine

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWriteBatchAtomic(t *testing.T) {
	tempDir := t.TempDir()

	db, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	batch := NewWriteBatch()
	_ = batch.Set("user:101", []byte("Alice"))
	_ = batch.Set("user:102", []byte("Bob"))
	_ = batch.Set("user:103", []byte("Charlie"))

	if batch.Len() != 3 {
		t.Errorf("expected batch len 3, got %d", batch.Len())
	}

	if err := db.Write(batch); err != nil {
		t.Fatalf("Write batch failed: %v", err)
	}

	if db.Size() != 3 {
		t.Errorf("expected db size 3, got %d", db.Size())
	}

	v1, ok := db.Get("user:101")
	if !ok || !bytes.Equal(v1, []byte("Alice")) {
		t.Errorf("expected 'Alice', got %q", string(v1))
	}

	// Staged updates and deletes in a second batch
	batch.Reset()
	_ = batch.Set("user:101", []byte("Alice Updated"))
	_ = batch.Delete("user:102")
	_ = batch.Set("user:104", []byte("Diana"))

	if err := db.Write(batch); err != nil {
		t.Fatalf("Write second batch failed: %v", err)
	}

	if db.Size() != 3 {
		t.Errorf("expected size 3, got %d", db.Size())
	}

	v1Updated, _ := db.Get("user:101")
	if string(v1Updated) != "Alice Updated" {
		t.Errorf("expected 'Alice Updated', got %q", string(v1Updated))
	}

	if db.Exists("user:102") {
		t.Error("user:102 should be deleted")
	}
}

func TestWriteBatchPersistenceAndRecovery(t *testing.T) {
	tempDir := t.TempDir()

	// Session 1: Batch write and close
	db1, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open db1 failed: %v", err)
	}

	batch := NewWriteBatch()
	for i := 0; i < 50; i++ {
		_ = batch.Set(fmt.Sprintf("k:%d", i), []byte(fmt.Sprintf("v:%d", i)))
	}
	_ = db1.Write(batch)
	_ = db1.Close()

	// Session 2: Reopen and verify all 50 recovered
	db2, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open db2 failed: %v", err)
	}
	defer db2.Close()

	if db2.Size() != 50 {
		t.Errorf("expected size 50, got %d", db2.Size())
	}

	for i := 0; i < 50; i++ {
		val, ok := db2.Get(fmt.Sprintf("k:%d", i))
		if !ok || string(val) != fmt.Sprintf("v:%d", i) {
			t.Errorf("key k:%d mismatch: got %q", i, string(val))
		}
	}
}

func TestWriteBatchEmptyOrNil(t *testing.T) {
	store := NewStore()

	if err := store.Write(nil); err != nil {
		t.Errorf("nil batch should return nil, got %v", err)
	}

	emptyBatch := NewWriteBatch()
	if err := store.Write(emptyBatch); err != nil {
		t.Errorf("empty batch should return nil, got %v", err)
	}
}
