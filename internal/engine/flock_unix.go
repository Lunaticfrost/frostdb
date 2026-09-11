//go:build unix || darwin || linux

package engine

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrDatabaseLocked indicates that another process is currently holding the directory lock.
var ErrDatabaseLocked = errors.New("database is locked by another process")

// FileLock represents an exclusive POSIX advisory lock on frost.lock.
type FileLock struct {
	file *os.File
}

// AcquireFileLock attempts to acquire a non-blocking exclusive file lock on lockPath.
func AcquireFileLock(lockPath string) (*FileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
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
	_ = syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
	err := fl.file.Close()
	fl.file = nil
	return err
}
