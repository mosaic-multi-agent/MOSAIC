package tui

// transform_target_flow_test.go verifies the Transform harness entry order: after the source
// harness screen (or the path screen when the source was chosen automatically or preset) the
// target harness is asked on its own entry screen. It is worded as the harness to transform to,
// names the source, offers the full harness list with the source disabled, and its choice reaches
// the service request, so no target question appears after the stack screen.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// entryDelta is a fourth harness, usable, listed after the unusable charlie.
var entryDelta = domain.HarnessRef{ID: "delta", DisplayName: "Delta Harness", Tier: domain.TierBuiltin, Usable: true}

// newTargetFlow builds a Transform root model over a service that lists alpha, bravo, the unusable
// charlie and delta, whose manifest at the pre-filled path records recordedIDs.
func newTargetFlow(t *testing.T, recordedIDs []string, opts ...flowOption) *narrowFlow {
	t.Helper()
	f := newNarrowFlow(t, domain.ModeTransformHarness, recordedIDs, opts...)
	all := []domain.HarnessRef{entryAlpha, entryBravo, entryCharlie, entryDelta}
	f.orig.harnesses = all
	f.reloaded.harnesses = all
	return f
}

// toTargetScreen chooses the Transform mode, confirms the path and chooses source on the source
// screen. It requires the target screen.
func (f *narrowFlow) toTargetScreen(t *testing.T, source string) {
	t.Helper()
	f.toHarnessScreen(t)
	pickHarness(t, f.m, source)
	if !onTargetScreen(f.m) {
		t.Fatalf("the target screen does not follow the source screen:\n%s", f.m.View())
	}
}

// transformRequest returns the request the Transform run reached the service with.
func (f *narrowFlow) transformRequest(t *testing.T) app.TransformHarnessRequest {
	t.Helper()
	for _, s := range []*stackFlowSvc{f.reloaded, f.orig} {
		if s.transformReq != nil {
			return *s.transformReq
		}
	}
	t.Fatal("the service received no transform request")
	return app.TransformHarnessRequest{}
}

// requireNeverChosen walks the cursor through the whole target list in both directions and
// requires it never to rest on the disabled harness id, then accepts the highlighted row and
// requires the run to reach the service with a different target. The list skips disabled rows,
// so the disabled harness cannot be highlighted and therefore cannot be chosen.
func (f *narrowFlow) requireNeverChosen(t *testing.T, id string) {
	t.Helper()
	steps := []tea.KeyType{tea.KeyDown, tea.KeyUp}
	for _, key := range steps {
		for i := 0; i < 6; i++ {
			pressKey(f.m, key)
			if got := highlightedHarness(f.m); got == id {
				t.Fatalf("the cursor rests on the disabled harness %q:\n%s", id, f.m.View())
			}
		}
	}

	pressEnter(f.m)
	if !onTargetScreen(f.m) && !f.onStack() {
		t.Fatalf("Enter on the target screen reached neither the stack screen nor stayed:\n%s", f.m.View())
	}
	if !f.onStack() {
		return // Enter was refused; the disabled harness was not chosen
	}
	runStarted(f.confirmStack(t))
	if got := f.transformRequest(t); got.TargetHarnessID == id {
		t.Errorf("the disabled harness %q was chosen as the target", id)
	}
}

func TestTransformTarget_FollowsTheSourceScreen(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toHarnessScreen(t)

	pickHarness(t, f.m, entryBravo.ID)

	if !onTargetScreen(f.m) {
		t.Errorf("the target screen does not follow the source screen (screen %v):\n%s", f.m.screen, f.m.View())
	}
	if f.onStack() {
		t.Error("the stack screen follows the source screen directly")
	}
}

func TestTransformTarget_FollowsThePathScreen_WhenTheSourceWasChosenAutomatically(t *testing.T) {
	f := newTargetFlow(t, []string{entryBravo.ID})
	pickMode(t, f.m, f.mode)

	pressEnter(f.m)

	if !onTargetScreen(f.m) {
		t.Errorf("the target screen does not follow the path screen (screen %v):\n%s", f.m.screen, f.m.View())
	}
	if f.m.selections.harnessID != entryBravo.ID {
		t.Errorf("source = %q, want %q chosen automatically", f.m.selections.harnessID, entryBravo.ID)
	}
}

func TestTransformTarget_FollowsThePathScreen_WhenTheSourceIsPreset(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID}, withPreset(entryBravo.ID))
	pickMode(t, f.m, f.mode)

	pressEnter(f.m)

	if !onTargetScreen(f.m) {
		t.Fatalf("the target screen is not shown after the path with a preset source (screen %v):\n%s", f.m.screen, f.m.View())
	}
	pickTarget(t, f.m, entryAlpha.ID)
	runStarted(f.confirmStack(t))
	if got := f.transformRequest(t); got.SourceHarnessID != entryBravo.ID || got.TargetHarnessID != entryAlpha.ID {
		t.Errorf("request source/target = %q/%q, want the preset %q and the chosen %q",
			got.SourceHarnessID, got.TargetHarnessID, entryBravo.ID, entryAlpha.ID)
	}
}

func TestTransformTarget_ScreenIsWordedToHarnessAndNamesTheSource(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})

	f.toTargetScreen(t, entryBravo.ID)

	requireMessageOnScreen(t, f.m, "harness to transform to", "from "+entryBravo.DisplayName)
}

func TestTransformTarget_OffersTheFullListNotNarrowedByTheManifest(t *testing.T) {
	t.Run("source chosen on the narrowed screen", func(t *testing.T) {
		f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})

		f.toTargetScreen(t, entryBravo.ID)

		requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
		requireMessageOnScreen(t, f.m, entryDelta.DisplayName)
	})
	t.Run("source chosen automatically", func(t *testing.T) {
		f := newTargetFlow(t, []string{entryBravo.ID})
		pickMode(t, f.m, f.mode)

		pressEnter(f.m)

		requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
		requireMessageOnScreen(t, f.m, entryDelta.DisplayName)
	})
}

func TestTransformTarget_SourceIsDisabledWithItsReasonAndNeverChosen(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	requireMessageOnScreen(t, f.m, "already the transform source")

	f.requireNeverChosen(t, entryBravo.ID)
}

func TestTransformTarget_UnusableHarnessIsDisabledWithItsReasonAndNeverChosen(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryAlpha.ID)
	requireMessageOnScreen(t, f.m, entryCharlie.UnusableReason)

	f.requireNeverChosen(t, entryCharlie.ID)
}

func TestTransformTarget_ChosenTargetReachesTheServiceRequest(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	pickTarget(t, f.m, entryDelta.ID)
	if !f.onStack() {
		t.Fatalf("the stack screen does not follow the target screen:\n%s", f.m.View())
	}

	cmd := f.confirmStack(t)
	requireScreen(t, f.m, screenRunning)
	runStarted(cmd)

	got := f.transformRequest(t)
	if got.SourceHarnessID != entryBravo.ID || got.TargetHarnessID != entryDelta.ID {
		t.Errorf("request source/target = %q/%q, want %q/%q",
			got.SourceHarnessID, got.TargetHarnessID, entryBravo.ID, entryDelta.ID)
	}
	if got.Workspace != f.path {
		t.Errorf("request workspace = %q, want %q", got.Workspace, f.path)
	}
}

func TestTransformTarget_FirstEnabledHarnessIsHighlighted_NeverTheSource(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryAlpha.ID)

	pressEnter(f.m) // accept the highlighted harness
	runStarted(f.confirmStack(t))

	if got := f.transformRequest(t); got.TargetHarnessID != entryBravo.ID {
		t.Errorf("target = %q, want %q (the first enabled harness; the source alpha is disabled)",
			got.TargetHarnessID, entryBravo.ID)
	}
}

func TestTransformTarget_StackScreenChecksInjectionsOfTheSource(t *testing.T) {
	f := newTargetFlow(t, []string{entryAlpha.ID, entryBravo.ID})
	f.toTargetScreen(t, entryBravo.ID)
	pickTarget(t, f.m, entryAlpha.ID)

	f.confirmStack(t)

	if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
		t.Errorf("injections checked for %v, want only the source %q", f.reloaded.checked, entryBravo.ID)
	}
}
