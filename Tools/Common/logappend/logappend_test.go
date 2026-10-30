package logappend_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mosaic-common/logappend"
)

func TestLockPathIsDotPrefixedSidecarInTargetFolder(t *testing.T) {
	target := filepath.Join("some", "dir", "00_orchestrator_events.jsonl")

	got := logappend.LockPath(target)

	want := filepath.Join("some", "dir", ".00_orchestrator_events.jsonl.lock")
	if got != want {
		t.Errorf("LockPath = %q, want %q", got, want)
	}
}

func TestConstantsMatchTheLogFormat(t *testing.T) {
	if logappend.WaitLimit != 2*time.Second {
		t.Errorf("WaitLimit = %v, want 2s", logappend.WaitLimit)
	}
	if logappend.StaleAfter != 10*time.Second {
		t.Errorf("StaleAfter = %v, want 10s", logappend.StaleAfter)
	}
}

func TestAppendCreatesFolderWritesLineAndRemovesLock(t *testing.T) {
	target := newTarget(t)
	line := makeLine("g0-0", 100)

	err := logappend.Append(target, line)

	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	assertWholeLines(t, readFile(t, target), 100, []string{"g0-0"})
	if fileExists(lockFileOf(target)) {
		t.Error("lock file left behind after a successful append")
	}
}

func TestAppendAddsToExistingContent(t *testing.T) {
	target := newTarget(t)
	_ = logappend.Append(target, makeLine("g0-0", 10))

	err := logappend.Append(target, makeLine("g0-1", 10))

	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	assertWholeLines(t, readFile(t, target), 10, []string{"g0-0", "g0-1"})
}

func TestAppendWaitsForHeldLockThenWrites(t *testing.T) {
	target := newTarget(t)
	takeLockFile(t, target)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = os.Remove(lockFileOf(target))
	}()
	var err error

	elapsed := measure(func() { err = logappend.Append(target, makeLine("g0-0", 50)) })

	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if elapsed < 400*time.Millisecond {
		t.Errorf("Append returned after %v, it did not wait for the held lock", elapsed)
	}
	assertWholeLines(t, readFile(t, target), 50, []string{"g0-0"})
}

func TestAppendGivesUpWithTimeoutWhenLockStaysHeld(t *testing.T) {
	target := newTarget(t)
	takeLockFile(t, target)
	var err error

	elapsed := measure(func() { err = logappend.Append(target, makeLine("g0-0", 50)) })

	if !errors.Is(err, logappend.ErrLockTimeout) {
		t.Fatalf("err = %v, want ErrLockTimeout", err)
	}
	if elapsed < 1500*time.Millisecond || elapsed > 6*time.Second {
		t.Errorf("gave up after %v, want about the 2s wait limit", elapsed)
	}
	if len(readFile(t, target)) != 0 {
		t.Error("a timed-out append wrote to the target")
	}
	if !fileExists(lockFileOf(target)) {
		t.Error("a timed-out append removed a lock it does not own")
	}
}

func TestAppendBreaksLockOlderThanStaleAge(t *testing.T) {
	target := newTarget(t)
	takeLockFile(t, target)
	old := time.Now().Add(-logappend.StaleAfter - 5*time.Second)
	if err := os.Chtimes(lockFileOf(target), old, old); err != nil {
		t.Fatal(err)
	}
	var err error

	elapsed := measure(func() { err = logappend.Append(target, makeLine("g0-0", 50)) })

	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("breaking a stale lock took %v, want it at once", elapsed)
	}
	assertWholeLines(t, readFile(t, target), 50, []string{"g0-0"})
	if fileExists(lockFileOf(target)) {
		t.Error("stale lock not cleaned up")
	}
}

func TestAppendDoesNotBreakFreshLock(t *testing.T) {
	target := newTarget(t)
	takeLockFile(t, target)
	recent := time.Now().Add(-5 * time.Second)
	if err := os.Chtimes(lockFileOf(target), recent, recent); err != nil {
		t.Fatal(err)
	}

	err := logappend.Append(target, makeLine("g0-0", 50))

	if !errors.Is(err, logappend.ErrLockTimeout) {
		t.Fatalf("err = %v, want ErrLockTimeout for a lock younger than the stale age", err)
	}
}

func TestAppendReportsIOErrorNotTimeoutWhenFolderCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(blocker, "sub", "events.jsonl")
	var err error

	elapsed := measure(func() { err = logappend.Append(target, makeLine("g0-0", 5)) })

	if err == nil || errors.Is(err, logappend.ErrLockTimeout) {
		t.Fatalf("err = %v, want an I/O error that is not ErrLockTimeout", err)
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("err = %v, want it to wrap the operating system's *fs.PathError", err)
	}
	if elapsed > time.Second {
		t.Errorf("an I/O error took %v, want it reported without waiting", elapsed)
	}
}

func TestParallelGoroutinesLoseTearOrBlankNoLine(t *testing.T) {
	target := newTarget(t)
	const writers, perWriter = 8, 5
	var wg sync.WaitGroup
	errs := make(chan error, writers*perWriter)

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				tag := tags("g", writers, perWriter)[w*perWriter+i]
				if err := logappend.Append(target, makeLine(tag, longLineSize)); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("Append: %v", err)
	}
	assertWholeLines(t, readFile(t, target), longLineSize, tags("g", writers, perWriter))
	if fileExists(lockFileOf(target)) {
		t.Error("lock file left behind")
	}
}

func TestWithLockHoldsLockDuringFnAndReleasesAfter(t *testing.T) {
	target := newTarget(t)
	var heldInside bool

	err := logappend.WithLock(target, func() error {
		heldInside = fileExists(lockFileOf(target))
		return os.WriteFile(target, makeLine("g0-0", 20), 0o644)
	})

	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}
	if !heldInside {
		t.Error("lock file did not exist while fn ran")
	}
	if fileExists(lockFileOf(target)) {
		t.Error("lock file left behind after fn returned")
	}
	assertWholeLines(t, readFile(t, target), 20, []string{"g0-0"})
}

func TestWithLockReturnsFnErrorAndStillReleases(t *testing.T) {
	target := newTarget(t)
	boom := errors.New("boom")

	err := logappend.WithLock(target, func() error { return boom })

	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want fn's error", err)
	}
	if fileExists(lockFileOf(target)) {
		t.Error("lock file left behind after fn failed")
	}
}

func TestWithLockDoesNotCallFnOnTimeout(t *testing.T) {
	target := newTarget(t)
	takeLockFile(t, target)
	called := false

	err := logappend.WithLock(target, func() error { called = true; return nil })

	if !errors.Is(err, logappend.ErrLockTimeout) {
		t.Fatalf("err = %v, want ErrLockTimeout", err)
	}
	if called {
		t.Error("fn ran without the lock")
	}
}

func TestAppendWaitsWhileWithLockHoldsTheLock(t *testing.T) {
	target := newTarget(t)
	inside := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- logappend.WithLock(target, func() error {
			close(inside)
			<-release
			return os.WriteFile(target, makeLine("h0-0", 20), 0o644)
		})
	}()
	waitInside(t, inside, done)
	go func() {
		time.Sleep(400 * time.Millisecond)
		close(release)
	}()
	var err error

	elapsed := measure(func() { err = logappend.Append(target, makeLine("g0-0", 20)) })

	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if elapsed < 300*time.Millisecond {
		t.Errorf("Append returned after %v while WithLock held the lock", elapsed)
	}
	if werr := <-done; werr != nil {
		t.Fatalf("WithLock: %v", werr)
	}
	assertWholeLines(t, readFile(t, target), 20, []string{"h0-0", "g0-0"})
}
