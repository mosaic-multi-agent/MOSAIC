package otelingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockFileName   = "lock"
	lockTimeFormat = "2006-01-02T15:04:05.000Z"
)

// lockContent is the content of .otel-ingest/lock.
type lockContent struct {
	Holder     string `json:"holder"`
	AcquiredAt string `json:"acquired_at"`
}

func lockPath(runDir string) string {
	return filepath.Join(runDir, stateDirName, lockFileName)
}

// acquireLock takes the run's ingest lock. held is true when another ingest holds a fresh lock; a lock older than
// LockStaleAfter was left by a crashed ingest and is broken. The returned release function removes the lock.
func acquireLock(runDir string, now time.Time) (release func(), held bool, err error) {
	if err := os.MkdirAll(filepath.Join(runDir, stateDirName), 0o755); err != nil {
		return nil, false, err
	}
	path := lockPath(runDir)
	for attempt := 0; attempt < 2; attempt++ {
		created, err := createLock(path, now)
		if err != nil {
			return nil, false, err
		}
		if created {
			return func() { os.Remove(path) }, false, nil
		}
		if !lockIsStale(path, now) {
			return nil, true, nil
		}
		os.Remove(path)
	}
	return nil, true, nil
}

// createLock creates the lock file exclusively; created is false when it already exists.
func createLock(path string, now time.Time) (bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, os.ErrExist) || (err != nil && os.IsPermission(err) && fileExists(path)) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	data, _ := json.Marshal(lockContent{
		Holder:     fmt.Sprintf("go/%d", os.Getpid()),
		AcquiredAt: now.UTC().Format(lockTimeFormat),
	})
	_, werr := f.Write(data)
	cerr := f.Close()
	return true, errors.Join(werr, cerr)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// lockIsStale reports whether the lock is at least LockStaleAfter old. Its age comes from acquired_at, else from
// the modification time of the lock file.
func lockIsStale(path string, now time.Time) bool {
	acquired, ok := time.Time{}, false
	if data, err := os.ReadFile(path); err == nil {
		var c lockContent
		if json.Unmarshal(data, &c) == nil {
			if t, err := time.Parse(lockTimeFormat, c.AcquiredAt); err == nil {
				acquired, ok = t, true
			}
		}
	}
	if !ok {
		info, err := os.Stat(path)
		if err != nil {
			return true
		}
		acquired = info.ModTime()
	}
	return now.Sub(acquired) >= LockStaleAfter
}
