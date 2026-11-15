package store

import (
	"os"
	"path/filepath"
	"syscall"
)

// WithOpLock runs fn holding cswitch's own operation lock, so a background
// auto-switch and a command typed in a terminal never change logins at the
// same time. Take it once, at a command's entry point: the lock is an flock
// on a file and is not reentrant within one process.
func WithOpLock(fn func() error) error {
	if err := os.MkdirAll(Home(), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(Home(), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
