//go:build windows

package engine

import (
	"errors"
	"fmt"
	"os"
)

// ErrDatabaseLocked indicates that another process is currently holding the directory lock.
var ErrDatabaseLocked = errors.New("database is locked by another process")

// FileLock represents an exclusive file lock on Windows.
type FileLock struct {
	file *os.File
}

// AcquireFileLock attempts to acquire an exclusive lock on Windows.
func AcquireFileLock(lockPath string) (*FileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrDatabaseLocked
		}
		return nil, fmt.Errorf("failed to acquire file lock: %w", err)
	}

	return &FileLock{file: f}, nil
}

// Release unlocks and closes the lock file.
func (fl *FileLock) Release() error {
	if fl == nil || fl.file == nil {
		return nil
	}
	err := fl.file.Close()
	fl.file = nil
	return err
}
