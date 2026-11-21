package tui

// narrowed_harness_repath_test.go verifies that the manifest is consulted again each time the path
// is confirmed in the modes that ask the path first (Update workspace, Change workflows, Transform
// harness): after going back and confirming a different path, the harness screen reflects the
// manifest at the new path, and a refused preset is cleared once a path that records it is
// confirmed.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// confirmOtherPath replaces the entered path with a new folder whose manifest records ids (none =
// no record) and confirms it. It returns the new path.
func confirmOtherPath(t *testing.T, f *narrowFlow, ids ...string) string {
	t.Helper()
	other := subDir(t, f.path, "other")
	f.svc.record(other, ids...)
	f.m.wsScreen.SetPrefilledPath(other)
	pressEnter(f.m)
	return other
}

func TestNarrowedHarnessRepath_DifferentPathWithOtherRecordedHarnesses_OffersTheNewSet(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			f.toHarnessScreen(t)
			pressEsc(f.m)
			requireScreen(t, f.m, screenWorkspace)

			confirmOtherPath(t, f, entryBravo.ID, entryCharlie.ID)

			requireOffered(t, f.m, entryBravo, entryCharlie)
		})
	}
}

func TestNarrowedHarnessRepath_DifferentPathWithNothingRecorded_OffersTheFullList(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			f.toHarnessScreen(t)
			pressEsc(f.m)

			confirmOtherPath(t, f)

			requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
		})
	}
}

func TestNarrowedHarnessRepath_AutomaticChoiceThenDifferentPathRecordingTwo_ShowsTheNarrowedScreen(t *testing.T) {
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

			confirmOtherPath(t, f, entryAlpha.ID, entryBravo.ID)

			requireOffered(t, f.m, entryAlpha, entryBravo)
		})
	}
}

func TestNarrowedHarnessRepath_TwoRecordedThenDifferentPathRecordingOne_ChoosesThatOneAutomatically(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})
			f.toHarnessScreen(t)
			pressEsc(f.m)

			confirmOtherPath(t, f, entryBravo.ID)

			if f.m.screen == screenHarness {
				t.Fatalf("the harness screen is shown although the new path records one usable harness:\n%s", f.m.View())
			}
			if !f.onStack() || f.m.selections.harnessID != entryBravo.ID {
				t.Errorf("harness = %q on the stack screen = %v, want %q chosen automatically",
					f.m.selections.harnessID, f.onStack(), entryBravo.ID)
			}
		})
	}
}

func TestNarrowedHarnessRepath_RefusedPresetThenPathRecordingIt_IsAcceptedWithoutTheMessage(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
			pickMode(t, f.m, mode)
			pressEnter(f.m)
			requireScreen(t, f.m, screenWorkspace)
			requireMessageOnScreen(t, f.m, "not recorded")

			confirmOtherPath(t, f, entryAlpha.ID, entryBravo.ID)

			if !f.onStack() {
				t.Fatalf("the preset was not accepted for the path that records it:\n%s", f.m.View())
			}
			if f.m.selections.harnessID != entryAlpha.ID {
				t.Errorf("harness = %q, want the preset %q", f.m.selections.harnessID, entryAlpha.ID)
			}
			if strings.Contains(strings.ToLower(f.m.View()), "not recorded") {
				t.Errorf("the refusal message lingers:\n%s", f.m.View())
			}
		})
	}
}

func TestNarrowedHarnessRepath_RefusedPresetThenPathWithNoRecord_IsAccepted(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
	pickMode(t, f.m, f.mode)
	pressEnter(f.m)
	requireScreen(t, f.m, screenWorkspace)

	confirmOtherPath(t, f)

	if !f.onStack() {
		t.Fatalf("the preset was not accepted for a path whose manifest records nothing:\n%s", f.m.View())
	}
}

func TestNarrowedHarnessRepath_PresetRefusedWithOneRecordedHarness_NamesThatHarness(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
			pickMode(t, f.m, mode)

			pressEnter(f.m)

			requireScreen(t, f.m, screenWorkspace)
			requireMessageOnScreen(t, f.m, entryAlpha.ID, "not recorded", entryBravo.ID)
		})
	}
}
