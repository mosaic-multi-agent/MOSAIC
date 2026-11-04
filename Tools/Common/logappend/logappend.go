// Package logappend appends whole lines to MOSAIC log files under the cross-language
// lock of the log format, section 4.1.
package logappend

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	WaitLimit  = 2 * time.Second
	StaleAfter = 10 * time.Second

	pollInterval = 20 * time.Millisecond
)

// ErrLockTimeout is returned when the lock could not be acquired within WaitLimit.
// Nothing was written.
var ErrLockTimeout = errors.New("logappend: lock wait limit reached")

// LockPath returns the lock file path of target: <dir>/.<name>.lock.
func LockPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".lock")
}

// Append creates target's directory if needed and appends line (one whole line; the
// caller supplies the trailing '\n') under the lock. Returns ErrLockTimeout on timeout,
// another error on an I/O failure, nil on success.
func Append(target string, line []byte) error {
	return WithLock(target, func() error {
		f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = f.Write(line)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// WithLock runs fn while holding the lock of target. fn is not called on timeout
// (ErrLockTimeout is returned). The lock is released after fn returns; fn's error is returned.
// fn writes to target with a plain append, never through Append.
func WithLock(target string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	lock := LockPath(target)
	if !acquire(lock) {
		return ErrLockTimeout
	}
	defer func() { _ = os.Remove(lock) }()
	return fn()
}

// acquire waits up to WaitLimit for the exclusive-create lock file; true when this writer holds it.
func acquire(lock string) bool {
	deadline := time.Now().Add(WaitLimit)
	for {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return true
		}
		if isStale(lock) {
			_ = os.Remove(lock)
			continue
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

func isStale(lock string) bool {
	info, err := os.Stat(lock)
	return err == nil && time.Since(info.ModTime()) > StaleAfter
}
