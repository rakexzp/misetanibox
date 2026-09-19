//go:build !windows

package instance

import (
	"os"
	"path/filepath"
	"syscall"
)

type Lock struct{ file *os.File }

func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{file: f}, nil
}
func (l *Lock) Close() error { return l.file.Close() }
