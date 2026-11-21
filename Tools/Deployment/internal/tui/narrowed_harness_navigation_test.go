package tui

// narrowed_harness_navigation_test.go verifies Esc and the restart in the modes that ask the path
// first (Update workspace, Change workflows, Transform harness): Esc on the path screen returns
// to the mode screen, Esc on the harness screen to the path screen, Esc on the stack screen to the
// harness screen (or to the path screen when the harness was chosen without being shown), and the
// final-screen restart highlights the previous mode and, when it is offered again, the previous
// harness. Choices are kept.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// pathTail returns the end of path, which a width-bounded input always shows.
func pathTail(path string) string {
	if len(path) > 12 {
		return path[len(path)-12:]
	}
	return path
}

func TestNarrowedHarnessNavigation_EscOnPathScreen_ReturnsToModeScreen(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			pickMode(t, f.m, mode)
			requireScreen(t, f.m, screenWorkspace)

			cmd := pressEsc(f.m)

			if isQuitCmd(cmd) {
				t.Fatal("Esc on the path screen exited the TUI")
			}
			assertModeHighlighted(t, f.m, mode)
		})
	}
}

func TestNarrowedHarnessNavigation_EscOnHarnessScreen_ReturnsToPathScreenKeepingThePath(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			f.toHarnessScreen(t)

			cmd := pressEsc(f.m)

			if isQuitCmd(cmd) {
				t.Fatal("Esc on the harness screen exited the TUI")
			}
			requireScreen(t, f.m, screenWorkspace)
			if !strings.Contains(f.m.View(), pathTail(f.path)) {
				t.Errorf("the path screen lost the entered path %q:\n%s", f.path, f.m.View())
			}
			pressEnter(f.m)
			requireScreen(t, f.m, screenHarness)
		})
	}
}

func TestNarrowedHarnessNavigation_EscOnStackScreen_ReturnsToHarnessScreenWithTheChoiceHighlighted(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			f.toHarnessScreen(t)
			pickHarness(t, f.m, entryBravo.ID)
			if !f.onStack() {
				t.Fatalf("precondition: not on the stack screen:\n%s", f.m.View())
			}

			pressEsc(f.m)

			requireScreen(t, f.m, screenHarness)
			requireOffered(t, f.m, entryAlpha, entryBravo)
			assertHarnessHighlighted(t, f.m, entryBravo.ID)
		})
	}
}

func TestNarrowedHarnessNavigation_EscOnStackScreen_AfterAutomaticChoice_ReturnsToPathScreen(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID})
			pickMode(t, f.m, mode)
			pressEnter(f.m)
			if !f.onStack() {
				t.Fatalf("precondition: not on the stack screen:\n%s", f.m.View())
			}

			pressEsc(f.m)

			requireScreen(t, f.m, screenWorkspace)
			if !strings.Contains(f.m.View(), pathTail(f.path)) {
				t.Errorf("the path screen lost the entered path %q:\n%s", f.path, f.m.View())
			}
			pressEnter(f.m)
			if !f.onStack() || f.m.selections.harnessID != entryBravo.ID {
				t.Errorf("confirming the path again did not choose %q automatically and show the stack screen:\n%s",
					entryBravo.ID, f.m.View())
			}
		})
	}
}

func TestNarrowedHarnessNavigation_FullBackChain_StackToHarnessToPathToModeToExit(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryAlpha.ID, entryBravo.ID})
	f.toHarnessScreen(t)
	pickHarness(t, f.m, entryAlpha.ID)

	pressEsc(f.m)
	requireScreen(t, f.m, screenHarness)
	pressEsc(f.m)
	requireScreen(t, f.m, screenWorkspace)
	pressEsc(f.m)
	requireScreen(t, f.m, screenMode)
	cmd := pressEsc(f.m)

	if !isQuitCmd(cmd) {
		t.Error("Esc on the mode screen did not exit the TUI")
	}
}

func TestNarrowedHarnessNavigation_EscOnStackScreen_WithPresetHarness_ReturnsToPathScreen(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID}, withPreset(entryBravo.ID))
			pickMode(t, f.m, mode)
			pressEnter(f.m)
			if !f.onStack() {
				t.Fatalf("precondition: not on the stack screen:\n%s", f.m.View())
			}

			pressEsc(f.m)

			requireScreen(t, f.m, screenWorkspace)
			pressEsc(f.m)
			requireScreen(t, f.m, screenMode)
		})
	}
}

func TestNarrowedHarnessNavigation_HarnessFirstModes_KeepTheirEscTargets(t *testing.T) {
	for _, mode := range harnessFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, nil)
			driveToPath(t, f.m, mode, entryBravo.ID)

			pressEsc(f.m) // path -> harness
			requireScreen(t, f.m, screenHarness)
			pressEsc(f.m) // harness -> mode
			requireScreen(t, f.m, screenMode)
		})
	}
}

// ---------------------------------------------------------------------------
// Restart from the final screen
// ---------------------------------------------------------------------------

// finishRunWithError drives mode to a finished run that ended in an error and presses a key on the
// final screen, so the model shows the mode screen again.
func finishRunWithError(t *testing.T, f *narrowFlow, harnessID string) {
	t.Helper()
	f.toHarnessScreen(t)
	pickHarness(t, f.m, harnessID)
	pickTargetIfAsked(t, f.m)
	f.confirmStack(t)
	requireScreen(t, f.m, screenRunning)
	f.m.Update(runErrorMsg{err: errors.New("run failed")})
	requireScreen(t, f.m, screenDone)
	f.m.Update(runeKey('x'))
	requireScreen(t, f.m, screenMode)
}

func TestNarrowedHarnessNavigation_Restart_HighlightsPreviousModeAndPreviousHarness(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			finishRunWithError(t, f, entryBravo.ID)

			assertModeHighlighted(t, f.m, mode)
			requireScreen(t, f.m, screenWorkspace)
			pressEnter(f.m)

			requireOffered(t, f.m, entryAlpha, entryBravo)
			assertHarnessHighlighted(t, f.m, entryBravo.ID)
		})
	}
}

func TestNarrowedHarnessNavigation_Restart_PreviousHarnessNoLongerOffered_HighlightsFirstEnabled(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryAlpha.ID, entryBravo.ID})
	finishRunWithError(t, f, entryBravo.ID)
	f.svc.record(f.path, entryAlpha.ID, entryCharlie.ID) // the manifest changed during the run

	pickMode(t, f.m, domain.ModeUpdateWorkspace)
	pressEnter(f.m)

	requireOffered(t, f.m, entryAlpha, entryCharlie)
	assertHarnessHighlighted(t, f.m, entryAlpha.ID)
}

func TestNarrowedHarnessNavigation_Restart_AfterAutomaticChoice_ChoosesAgainWithoutTheScreen(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkflows, []string{entryBravo.ID})
	pickMode(t, f.m, f.mode)
	pressEnter(f.m)
	f.confirmStack(t)
	f.m.Update(runErrorMsg{err: errors.New("run failed")})
	requireScreen(t, f.m, screenDone)
	f.m.Update(runeKey('x'))

	assertModeHighlighted(t, f.m, f.mode)
	pressEnter(f.m) // confirm the path

	if f.m.screen == screenHarness {
		t.Fatalf("the harness screen is shown again although one usable harness is recorded:\n%s", f.m.View())
	}
	if f.m.selections.harnessID != entryBravo.ID {
		t.Errorf("harness = %q after the restart, want %q chosen automatically", f.m.selections.harnessID, entryBravo.ID)
	}
}
