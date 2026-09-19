//go:build windows

package instance

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

// Lock owns a non-inheritable OS file lock. Process death releases ownership.
// Keep the lock file: unlinking it would permit two owners of different inodes.
type Lock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	lock := &Lock{file: f}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lock.overlapped); err != nil {
		f.Close()
		return nil, err
	}
	return lock, nil
}
func (l *Lock) Close() error { return l.file.Close() }
