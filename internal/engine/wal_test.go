package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRecordEncodeDecode(t *testing.T) {
	rec := NewRecord(OpSet, "user:1", []byte("Alice"))
	data, err := EncodeRecord(rec)
	if err != nil {
		t.Fatalf("EncodeRecord failed: %v", err)
	}

	decoded, err := DecodeRecord(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRecord failed: %v", err)
	}

	if decoded.Op != rec.Op {
		t.Errorf("expected Op %v, got %v", rec.Op, decoded.Op)
	}
	if decoded.Key != rec.Key {
		t.Errorf("expected Key %q, got %q", rec.Key, decoded.Key)
	}
	if !bytes.Equal(decoded.Value, rec.Value) {
		t.Errorf("expected Value %q, got %q", rec.Value, decoded.Value)
	}
	if decoded.CRC != rec.CRC {
		t.Errorf("expected CRC %x, got %x", rec.CRC, decoded.CRC)
	}
}

func TestRecordCorruption(t *testing.T) {
	rec := NewRecord(OpSet, "key", []byte("value"))
	data, err := EncodeRecord(rec)
	if err != nil {
		t.Fatalf("EncodeRecord failed: %v", err)
	}

	// Corrupt a byte in the payload
	data[len(data)-1] ^= 0xFF

	_, err = DecodeRecord(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error on corrupted data, got nil")
	}
}

func TestWALAppendAndReplay(t *testing.T) {
	tempDir := t.TempDir()
	walPath := filepath.Join(tempDir, "test.wal")

	wal, err := OpenWAL(walPath, SyncAlways)
	if err != nil {
		t.Fatalf("OpenWAL failed: %v", err)
	}

	rec1 := NewRecord(OpSet, "k1", []byte("v1"))
	rec2 := NewRecord(OpSet, "k2", []byte("v2"))
	rec3 := NewRecord(OpDelete, "k1", nil)

	if err := wal.Append(rec1); err != nil {
		t.Fatalf("Append rec1 failed: %v", err)
	}
	if err := wal.Append(rec2); err != nil {
		t.Fatalf("Append rec2 failed: %v", err)
	}
	if err := wal.Append(rec3); err != nil {
		t.Fatalf("Append rec3 failed: %v", err)
	}

	var replayed []*Record
	count, err := wal.Replay(func(r *Record) error {
		replayed = append(replayed, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 records, got %d", count)
	}

	if replayed[0].Key != "k1" || string(replayed[0].Value) != "v1" || replayed[0].Op != OpSet {
		t.Errorf("unexpected record 0: %+v", replayed[0])
	}
	if replayed[1].Key != "k2" || string(replayed[1].Value) != "v2" || replayed[1].Op != OpSet {
		t.Errorf("unexpected record 1: %+v", replayed[1])
	}
	if replayed[2].Key != "k1" || replayed[2].Op != OpDelete {
		t.Errorf("unexpected record 2: %+v", replayed[2])
	}

	_ = wal.Close()
}

func TestWALTornWriteRecovery(t *testing.T) {
	tempDir := t.TempDir()
	walPath := filepath.Join(tempDir, "test_torn.wal")

	wal, err := OpenWAL(walPath, SyncAlways)
	if err != nil {
		t.Fatalf("OpenWAL failed: %v", err)
	}

	// Write 2 valid records
	_ = wal.Append(NewRecord(OpSet, "k1", []byte("v1")))
	_ = wal.Append(NewRecord(OpSet, "k2", []byte("v2")))
	_ = wal.Close()

	// Append corrupt / partial bytes to simulate crash mid-write
	f, err := os.OpenFile(walPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("failed to open file for tampering: %v", err)
	}
	// Write 10 garbage bytes (less than HeaderSize)
	_, _ = f.Write([]byte("garbage123"))
	_ = f.Close()

	// Re-open WAL and replay
	wal2, err := OpenWAL(walPath, SyncAlways)
	if err != nil {
		t.Fatalf("OpenWAL second time failed: %v", err)
	}
	defer wal2.Close()

	var replayed []*Record
	count, err := wal2.Replay(func(r *Record) error {
		replayed = append(replayed, r)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay failed after torn write: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 valid records recovered, got %d", count)
	}

	// Verify we can continue appending after truncation
	err = wal2.Append(NewRecord(OpSet, "k3", []byte("v3")))
	if err != nil {
		t.Fatalf("Append after torn recovery failed: %v", err)
	}

	var totalCount int
	_, err = wal2.Replay(func(r *Record) error {
		totalCount++
		return nil
	})
	if err != nil {
		t.Fatalf("Second replay failed: %v", err)
	}
	if totalCount != 3 {
		t.Fatalf("expected 3 records, got %d", totalCount)
	}
}

func TestStorePersistenceAcrossRestarts(t *testing.T) {
	tempDir := t.TempDir()

	// 1. First session: open store, write keys, delete a key, close
	db1, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("failed to open db1: %v", err)
	}

	if !db1.IsPersistent() {
		t.Error("expected db1 to be persistent")
	}

	_ = db1.Set("city", []byte("Reykjavik"))
	_ = db1.Set("temp", []byte("-5"))
	_ = db1.Set("to_delete", []byte("temporary"))
	db1.Delete("to_delete")

	if err := db1.Close(); err != nil {
		t.Fatalf("failed to close db1: %v", err)
	}

	// 2. Second session: open store from same dir, verify recovered state
	db2, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("failed to open db2: %v", err)
	}
	defer db2.Close()

	if db2.Size() != 2 {
		t.Errorf("expected size 2, got %d", db2.Size())
	}

	city, exists := db2.Get("city")
	if !exists || string(city) != "Reykjavik" {
		t.Errorf("expected 'Reykjavik', got %q (exists=%v)", string(city), exists)
	}

	temp, exists := db2.Get("temp")
	if !exists || string(temp) != "-5" {
		t.Errorf("expected '-5', got %q (exists=%v)", string(temp), exists)
	}

	if db2.Exists("to_delete") {
		t.Error("deleted key 'to_delete' should not exist after recovery")
	}
}

func TestStoreConcurrentWritesWithWAL(t *testing.T) {
	tempDir := t.TempDir()

	db, err := Open(tempDir, Options{SyncPolicy: SyncPeriodic})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var wg sync.WaitGroup
	numWorkers := 20
	opsPerWorker := 50

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				key := fmt.Sprintf("k-%d-%d", workerID, j)
				val := []byte(fmt.Sprintf("v-%d-%d", workerID, j))
				if err := db.Set(key, val); err != nil {
					t.Errorf("Set failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numWorkers * opsPerWorker
	if db.Size() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, db.Size())
	}
}
