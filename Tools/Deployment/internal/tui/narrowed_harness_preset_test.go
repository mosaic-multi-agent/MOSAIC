package tui

// narrowed_harness_preset_test.go verifies a harness preset at launch in the modes that ask the
// path first (Update workspace, Change workflows, Transform harness): the harness screen is
// skipped; a preset the manifest at the entered path does not record is refused on the path screen
// with a message naming the recorded harnesses; a path whose manifest records nothing accepts it.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestNarrowedHarnessPreset_RecordedPreset_SkipsTheHarnessScreen(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID}, withPreset(entryBravo.ID))
			pickMode(t, f.m, mode)

			pressEnter(f.m)

			if f.m.screen == screenHarness {
				t.Fatalf("the harness screen is shown although a harness was preset:\n%s", f.m.View())
			}
			if !f.onStack() {
				t.Errorf("the stack screen does not follow the path screen:\n%s", f.m.View())
			}
			f.confirmStack(t)
			if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
				t.Errorf("injections checked for %v, want the preset %q", f.reloaded.checked, entryBravo.ID)
			}
		})
	}
}

func TestNarrowedHarnessPreset_PresetNotRecorded_IsRefusedOnThePathScreenNamingTheRecordedHarnesses(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID, entryCharlie.ID}, withPreset(entryAlpha.ID))
			pickMode(t, f.m, mode)

			pressEnter(f.m)

			requireScreen(t, f.m, screenWorkspace)
			if f.onStack() {
				t.Fatal("the stack screen is shown although the preset is not recorded")
			}
			requireMessageOnScreen(t, f.m, entryAlpha.ID, "not recorded", entryBravo.ID+", "+entryCharlie.ID)
		})
	}
}

func TestNarrowedHarnessPreset_Refused_NoRunStartsAndTheUserCanGoBack(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
	pickMode(t, f.m, f.mode)

	cmd := pressEnter(f.m)
	runStarted(cmd)

	requireScreen(t, f.m, screenWorkspace)
	if len(f.log.entries) != 0 || len(f.rec.calls) != 0 {
		t.Errorf("a run started although the preset was refused: reloads %d, calls %v", len(f.rec.calls), f.log.entries)
	}
	pressEsc(f.m)
	requireScreen(t, f.m, screenMode)
}

func TestNarrowedHarnessPreset_MissingOrEmptyManifest_AcceptsThePreset(t *testing.T) {
	cases := []struct {
		name     string
		recorded []string
	}{
		{"no manifest, or one that cannot be read", nil},
		{"manifest recording no harness", []string{}},
	}
	for _, tc := range cases {
		for _, mode := range stackAfterHarnessModes {
			t.Run(tc.name+"/"+string(mode), func(t *testing.T) {
				f := newNarrowFlow(t, mode, tc.recorded, withPreset(entryAlpha.ID))
				pickMode(t, f.m, mode)

				pressEnter(f.m)

				if !f.onStack() {
					t.Fatalf("the preset was not accepted (screen %v):\n%s", f.m.screen, f.m.View())
				}
				if strings.Contains(strings.ToLower(f.m.View()), "not recorded") {
					t.Errorf("a refusal message is shown:\n%s", f.m.View())
				}
				if f.m.selections.harnessID != entryAlpha.ID {
					t.Errorf("harness = %q, want the preset %q", f.m.selections.harnessID, entryAlpha.ID)
				}
			})
		}
	}
}

func TestNarrowedHarnessPreset_PathBelowAWorkspace_AcceptsThePreset(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
	f.m.wsScreen.SetPrefilledPath(subDir(t, f.path, "below"))
	pickMode(t, f.m, f.mode)

	pressEnter(f.m)

	if !f.onStack() {
		t.Fatalf("the preset was refused for a path without a manifest of its own:\n%s", f.m.View())
	}
}

// TestNarrowedHarnessPreset_HarnessFirstModes_AreNotChecked verifies the refusal does not apply
// to the modes that ask the harness before the path.
func TestNarrowedHarnessPreset_HarnessFirstModes_AreNotChecked(t *testing.T) {
	for _, mode := range []domain.RunMode{domain.ModeDeployWorkspace, domain.ModeDeployAgents, domain.ModeDeployHooks} {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID}, withPreset(entryAlpha.ID))
			pickMode(t, f.m, mode)

			pressEnter(f.m)

			if !f.onStack() {
				t.Fatalf("the preset was refused in %s:\n%s", mode, f.m.View())
			}
		})
	}
}
