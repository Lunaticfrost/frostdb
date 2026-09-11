package frostdb_test

import (
	"bytes"
	"testing"

	"github.com/Lunaticfrost/frostdb"
)

func TestPublicAPI(t *testing.T) {
	tempDir := t.TempDir()

	db, err := frostdb.Open(tempDir, frostdb.DefaultOptions)
	if err != nil {
		t.Fatalf("frostdb.Open failed: %v", err)
	}
	defer db.Close()

	// 1. Binary Set and Get
	payload := []byte(`{"user": "Alice", "role": "admin"}`)
	if err := db.Set("user:101", payload); err != nil {
		t.Fatalf("db.Set failed: %v", err)
	}

	val, exists := db.Get("user:101")
	if !exists || !bytes.Equal(val, payload) {
		t.Fatalf("db.Get mismatch: got %s", string(val))
	}

	// 2. Convenience String helpers
	if err := db.SetString("key:str", "value:str"); err != nil {
		t.Fatalf("db.SetString failed: %v", err)
	}
	strVal, exists := db.GetString("key:str")
	if !exists || strVal != "value:str" {
		t.Fatalf("db.GetString mismatch: got %s", strVal)
	}

	// 3. Atomic WriteBatch
	batch := frostdb.NewWriteBatch()
	_ = batch.Set("batch:1", []byte("val1"))
	_ = batch.Set("batch:2", []byte("val2"))
	_ = batch.Delete("user:101")

	if err := db.Write(batch); err != nil {
		t.Fatalf("db.Write batch failed: %v", err)
	}

	if db.Exists("user:101") {
		t.Error("user:101 should be deleted by batch")
	}
	if !db.Exists("batch:1") || !db.Exists("batch:2") {
		t.Error("batch keys should exist")
	}

	// 4. Compaction
	stats, err := db.Compact()
	if err != nil {
		t.Fatalf("db.Compact failed: %v", err)
	}
	if stats.KeysCompacted != 3 {
		t.Errorf("expected 3 keys compacted, got %d", stats.KeysCompacted)
	}
}

func TestInMemoryPublicAPI(t *testing.T) {
	db := frostdb.New()
	_ = db.SetString("temp", "in-memory")
	val, ok := db.GetString("temp")
	if !ok || val != "in-memory" {
		t.Errorf("expected 'in-memory', got %s", val)
	}
}
