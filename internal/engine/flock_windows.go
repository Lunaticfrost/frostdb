//go:build windows

package engine

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// ErrDatabaseLocked indicates that another process is currently holding the directory lock.
var ErrDatabaseLocked = errors.New("database is locked by another process")

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errLockViolation        = 33
	errSharingViolation     = 32
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

// FileLock represents an exclusive file lock on Windows.
type FileLock struct {
	file *os.File
}

// AcquireFileLock attempts to acquire a non-blocking exclusive lock using Windows LockFileEx.
func AcquireFileLock(lockPath string) (*FileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	var overlapped syscall.Overlapped
	flags := uintptr(lockfileExclusiveLock | lockfileFailImmediately)
	r1, _, err := procLockFileEx.Call(
		f.Fd(),
		flags,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 == 0 {
		_ = f.Close()
		if errno, ok := err.(syscall.Errno); ok && (errno == errLockViolation || errno == errSharingViolation) {
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
	var overlapped syscall.Overlapped
	_, _, _ = procUnlockFileEx.Call(
		fl.file.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	err := fl.file.Close()
	fl.file = nil
	return err
}
