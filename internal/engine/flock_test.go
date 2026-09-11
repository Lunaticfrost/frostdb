package engine

import (
	"errors"
	"testing"
)

func TestFileLockExclusion(t *testing.T) {
	tempDir := t.TempDir()

	// 1. First Open acquires lock
	db1, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}

	// 2. Second Open on same directory should fail with ErrDatabaseLocked
	db2, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err == nil {
		_ = db2.Close()
		t.Fatal("expected second Open to fail with ErrDatabaseLocked, but succeeded")
	}

	if !errors.Is(err, ErrDatabaseLocked) {
		t.Errorf("expected ErrDatabaseLocked, got %v", err)
	}

	// 3. Close db1 releases lock
	if err := db1.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	// 4. Third Open should now succeed
	db3, err := Open(tempDir, Options{SyncPolicy: SyncAlways})
	if err != nil {
		t.Fatalf("Open after release failed: %v", err)
	}
	_ = db3.Close()
}
