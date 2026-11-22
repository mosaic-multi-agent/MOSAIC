package tui

// transform_target_navigation_test.go verifies Esc and the restart around the Transform target
// harness screen: Esc on the target screen returns to the source screen, or to the path screen
// when the source was chosen automatically or preset; Esc on the stack screen returns to the
// target screen; choices are kept; the final-screen restart highlights the previous source and
// target.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// requireSourceScreen fails the test unless the narrowed source screen is shown.
func requireSourceScreen(t *testing.T, m *rootModel) {
	t.Helper()
	requireScreen(t, m, screenHarness)
	if onTargetScreen(m) {
		t.Fatalf("the target screen is shown, want the source screen:\n%s", m.View())
	}
	requireMessageOnScreen(t, m, "harness to transform from")
}

func TestTransformTargetNavigation_EscOnTargetScreen_ReturnsToSourceScreenWithTheSourceHighlighted(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)

	cmd := pressEsc(f.m)

	if isQuitCmd(cmd) {
		t.Fatal("Esc on the target screen exited the TUI")
	}
	requireSourceScreen(t, f.m)
	requireOffered(t, f.m, entryAlpha, entryBravo)
	if got := highlightedHarness(f.m); got != entryBravo.ID {
		t.Errorf("highlighted source = %q, want the chosen %q", got, entryBravo.ID)
	}
}

func TestTransformTargetNavigation_EscOnTargetScreen_AfterAutomaticSource_ReturnsToPathScreen(t *testing.T) {
	f := newTargetFlow(t, []string{entryBravo.ID})
	pickMode(t, f.m, f.mode)
	pressEnter(f.m)
	if !onTargetScreen(f.m) {
		t.Fatalf("precondition: the target screen is not shown:\n%s", f.m.View())
	}

	cmd := pressEsc(f.m)

	if isQuitCmd(cmd) {
		t.Fatal("Esc on the target screen exited the TUI")
	}
	requireScreen(t, f.m, screenWorkspace)
	if !strings.Contains(f.m.View(), pathTail(f.path)) {
		t.Errorf("the path screen lost the entered path %q:\n%s", f.path, f.m.View())
	}
	pressEnter(f.m)
	if !onTargetScreen(f.m) {
		t.Errorf("confirming the path again did not choose the source automatically and show the target screen:\n%s", f.m.View())
	}
}

func TestTransformTargetNavigation_EscOnTargetScreen_WithPresetSource_ReturnsToPathScreen(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID}, withPreset(entryBravo.ID))
	pickMode(t, f.m, f.mode)
	pressEnter(f.m)
	if !onTargetScreen(f.m) {
		t.Fatalf("precondition: the target screen is not shown:\n%s", f.m.View())
	}

	pressEsc(f.m)

	requireScreen(t, f.m, screenWorkspace)
	pressEsc(f.m)
	requireScreen(t, f.m, screenMode)
}

func TestTransformTargetNavigation_EscOnStackScreen_ReturnsToTargetScreenWithTheTargetHighlighted(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	pickTarget(t, f.m, entryDelta.ID)
	if !f.onStack() {
		t.Fatalf("precondition: not on the stack screen:\n%s", f.m.View())
	}

	pressEsc(f.m)

	if !onTargetScreen(f.m) {
		t.Fatalf("Esc on the stack screen did not return to the target screen:\n%s", f.m.View())
	}
	if got := highlightedHarness(f.m); got != entryDelta.ID {
		t.Errorf("highlighted target = %q, want the chosen %q", got, entryDelta.ID)
	}
}

func TestTransformTargetNavigation_EscOnStackScreen_AfterAutomaticSource_ReturnsToTargetThenPath(t *testing.T) {
	f := newTargetFlow(t, []string{entryBravo.ID})
	pickMode(t, f.m, f.mode)
	pressEnter(f.m)
	pickTarget(t, f.m, entryDelta.ID)
	if !f.onStack() {
		t.Fatalf("precondition: not on the stack screen:\n%s", f.m.View())
	}

	pressEsc(f.m)
	if !onTargetScreen(f.m) {
		t.Fatalf("Esc on the stack screen did not return to the target screen:\n%s", f.m.View())
	}
	pressEsc(f.m)

	requireScreen(t, f.m, screenWorkspace)
}

func TestTransformTargetNavigation_FullBackChain_StackToTargetToSourceToPathToModeToExit(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryAlpha.ID)
	pickTarget(t, f.m, entryBravo.ID)

	pressEsc(f.m)
	if !onTargetScreen(f.m) {
		t.Fatalf("stack -> target failed:\n%s", f.m.View())
	}
	pressEsc(f.m)
	requireSourceScreen(t, f.m)
	pressEsc(f.m)
	requireScreen(t, f.m, screenWorkspace)
	pressEsc(f.m)
	requireScreen(t, f.m, screenMode)
	cmd := pressEsc(f.m)

	if !isQuitCmd(cmd) {
		t.Error("Esc on the mode screen did not exit the TUI")
	}
}

func TestTransformTargetNavigation_ChangingTheSourceAfterGoingBack_DisablesItAsTarget(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	pickTarget(t, f.m, entryAlpha.ID)
	pressEsc(f.m) // stack -> target
	pressEsc(f.m) // target -> source

	pickHarness(t, f.m, entryAlpha.ID) // the former target becomes the source

	if !onTargetScreen(f.m) {
		t.Fatalf("the target screen does not follow the changed source:\n%s", f.m.View())
	}
	requireMessageOnScreen(t, f.m, "from "+entryAlpha.DisplayName)
	pressEnter(f.m) // accept the highlighted harness: it must not be the source
	runStarted(f.confirmStack(t))
	got := f.transformRequest(t)
	if got.SourceHarnessID != entryAlpha.ID || got.TargetHarnessID == entryAlpha.ID || got.TargetHarnessID == "" {
		t.Errorf("request source/target = %q/%q, want source %q and a different target",
			got.SourceHarnessID, got.TargetHarnessID, entryAlpha.ID)
	}
}

func TestTransformTargetNavigation_Restart_HighlightsPreviousSourceAndTarget(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	pickTarget(t, f.m, entryDelta.ID)
	f.confirmStack(t)
	requireScreen(t, f.m, screenRunning)
	f.m.Update(runErrorMsg{err: errors.New("run failed")})
	requireScreen(t, f.m, screenDone)
	f.m.Update(runeKey('x'))
	requireScreen(t, f.m, screenMode)

	assertModeHighlighted(t, f.m, domain.ModeTransformHarness)
	requireScreen(t, f.m, screenWorkspace)
	pressEnter(f.m)
	requireSourceScreen(t, f.m)
	if got := highlightedHarness(f.m); got != entryBravo.ID {
		t.Errorf("highlighted source = %q after the restart, want %q", got, entryBravo.ID)
	}
	pressEnter(f.m)

	if !onTargetScreen(f.m) {
		t.Fatalf("the target screen is not shown again after the restart:\n%s", f.m.View())
	}
	if got := highlightedHarness(f.m); got != entryDelta.ID {
		t.Errorf("highlighted target = %q after the restart, want %q", got, entryDelta.ID)
	}
}
