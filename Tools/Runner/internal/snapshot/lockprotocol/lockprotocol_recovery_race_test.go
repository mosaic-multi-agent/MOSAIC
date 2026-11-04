package lockprotocol

// Deterministic regression test for the window between a winning recoverer
// releasing .restoring and removing the backup directory. A second recoverer
// that acquires .restoring inside that window must treat the backup as already
// recovered: return nil and restore nothing.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

// countEvent returns how many times event was logged by the spy.
func (s *spyLogger) countEvent(event string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.events {
		if e == event {
			n++
		}
	}
	return n
}

// TestRecoveryCheck_SecondRecovererInTeardownWindow_NoOpsWithoutRestoring runs
// the second RecoveryCheck synchronously inside the first recoverer's teardown
// window (restored, .restoring released, directory not yet removed). Both must
// return nil, only the first may restore, the backup directory must be gone and
// the agent file must hold its original content.
func TestRecoveryCheck_SecondRecovererInTeardownWindow_NoOpsWithoutRestoring(t *testing.T) {
	// Arrange
	base := t.TempDir()
	agentsDir, _ := newTestFullBackupState(t, base)
	workerPath := filepath.Join(agentsDir, "worker.md")
	const sentinel = "---\nmode: subagent\n---\n\nSENTINEL-WRITTEN-BY-FIRST-RECOVERER\n"

	spy1 := &spyLogger{}
	spy2 := &spyLogger{}
	var secondErr error
	fired := false

	afterRecoveryRestoringReleasedForTest = func() {
		// Set before the nested call so a re-entrant hook returns at once
		// (a failing assertion, not a deadlock, if the teardown fix regresses).
		if fired {
			return
		}
		fired = true
		// The first recoverer has restored; mark the file so a second
		// restore from the still-intact backup would be detectable.
		if err := os.WriteFile(workerPath, []byte(sentinel), 0o644); err != nil {
			t.Errorf("hook: write sentinel: %v", err)
		}
		secondErr = RecoveryCheck(agentsDir, spy2)
	}
	t.Cleanup(func() { afterRecoveryRestoringReleasedForTest = nil })

	// Act
	firstErr := RecoveryCheck(agentsDir, spy1)

	// Assert: both return nil.
	if firstErr != nil {
		t.Errorf("first recoverer: unexpected error: %v", firstErr)
	}
	if secondErr != nil {
		t.Errorf("second recoverer: unexpected error: %v", secondErr)
	}

	// Assert: exactly one recovery event across both calls.
	total := spy1.countEvent(domain.EventSnapshotRecovery) + spy2.countEvent(domain.EventSnapshotRecovery)
	if total != 1 {
		t.Errorf("EventSnapshotRecovery logged %d times across both recoverers; want exactly 1", total)
	}
	if spy1.countEvent(domain.EventSnapshotRecovery) != 1 {
		t.Errorf("the first recoverer must be the one that logs the recovery")
	}

	// Assert: second recoverer wrote nothing to agentsDir (sentinel intact).
	content, err := os.ReadFile(workerPath)
	if err != nil {
		t.Fatalf("read worker.md: %v", err)
	}
	if string(content) != sentinel {
		t.Errorf("second recoverer must not restore; worker.md content:\n%s", content)
	}

	// Assert: backup directory is gone.
	if _, statErr := os.Stat(backupDirPath(agentsDir)); !os.IsNotExist(statErr) {
		t.Errorf("backup directory should be removed after both recoverers return; got: %v", statErr)
	}
}

// TestRecoveryCheck_TeardownWindowSeam_RestoredAndDirectoryPresent verifies the
// state the hook observes: originals already restored, backup directory still
// present. Guards the seam's firing point so the test above stays meaningful.
func TestRecoveryCheck_TeardownWindowSeam_RestoredAndDirectoryPresent(t *testing.T) {
	// Arrange
	base := t.TempDir()
	agentsDir, _ := newTestFullBackupState(t, base)
	hookCalls := 0
	var restoredInHook, dirPresentInHook bool

	afterRecoveryRestoringReleasedForTest = func() {
		hookCalls++
		restoredInHook = agentFileIsOriginal(t, agentsDir)
		_, statErr := os.Stat(backupDirPath(agentsDir))
		dirPresentInHook = statErr == nil
	}
	t.Cleanup(func() { afterRecoveryRestoringReleasedForTest = nil })

	// Act
	err := RecoveryCheck(agentsDir, nil)

	// Assert
	if err != nil {
		t.Fatalf("RecoveryCheck: unexpected error: %v", err)
	}
	if hookCalls != 1 {
		t.Fatalf("seam called %d times; want exactly 1", hookCalls)
	}
	if !restoredInHook {
		t.Error("originals must already be restored when the seam fires")
	}
	if !dirPresentInHook {
		t.Error("backup directory must still exist when the seam fires")
	}
}
