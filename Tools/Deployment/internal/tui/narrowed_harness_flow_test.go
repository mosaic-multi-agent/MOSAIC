package tui

// narrowed_harness_flow_test.go verifies the entry order and the harness screen of the modes that
// ask the path first: Update workspace, Change workflows and Transform harness. After the path is
// confirmed, the harness screen offers only the harnesses the manifest at that path records; one
// usable recorded harness is chosen without showing the screen; a missing, empty or unreadable
// manifest (the service reports none) and a path below a workspace show the full list. The other
// modes keep harness before path with the full list.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestNarrowedHarness_PathScreenComesBeforeTheHarnessScreen(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})

			pickMode(t, f.m, mode)

			requireScreen(t, f.m, screenWorkspace)
			pressEnter(f.m)
			requireScreen(t, f.m, screenHarness)
		})
	}
}

func TestNarrowedHarness_OtherModes_KeepHarnessBeforePathWithTheFullList(t *testing.T) {
	for _, mode := range harnessFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})

			pickMode(t, f.m, mode)

			requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
			pickHarness(t, f.m, entryBravo.ID)
			requireScreen(t, f.m, screenWorkspace)
		})
	}
}

func TestNarrowedHarness_TwoRecordedHarnesses_OffersExactlyThose(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID, entryBravo.ID})

			f.toHarnessScreen(t)

			requireOffered(t, f.m, entryAlpha, entryBravo)
		})
	}
}

func TestNarrowedHarness_AsksTheManifestAtTheEnteredPath(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryAlpha.ID, entryBravo.ID})

	f.toHarnessScreen(t)

	found := false
	for _, p := range f.svc.asked {
		if p == f.path {
			found = true
		}
	}
	if !found {
		t.Errorf("the manifest was asked for %v, want the entered path %q", f.svc.asked, f.path)
	}
}

func TestNarrowedHarness_ChoosingOneOfTheOfferedHarnesses_LeadsToTheStackScreen(t *testing.T) {
	f := newNarrowFlow(t, domain.ModeUpdateWorkflows, []string{entryAlpha.ID, entryBravo.ID})
	f.toHarnessScreen(t)

	pickHarness(t, f.m, entryBravo.ID)

	if f.m.selections.harnessID != entryBravo.ID {
		t.Errorf("harness = %q, want %q", f.m.selections.harnessID, entryBravo.ID)
	}
	if !f.onStack() {
		t.Errorf("the stack screen does not follow the harness screen:\n%s", f.m.View())
	}
}

func TestNarrowedHarness_RecordedUnusableHarness_IsShownDisabledWithItsReasonAndNeverChosen(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID, entryCharlie.ID})
			f.toHarnessScreen(t)

			requireOffered(t, f.m, entryBravo, entryCharlie)
			requireMessageOnScreen(t, f.m, entryCharlie.UnusableReason)
			// The offered entries follow the registry order (bravo, charlie), so index 1 is the
			// unusable charlie; requireOffered above guarantees exactly these two rows exist.
			moveListTo(f.m, 1)
			pressEnter(f.m)

			if f.m.selections.harnessID == entryCharlie.ID {
				t.Errorf("the unusable harness was chosen")
			}
		})
	}
}

func TestNarrowedHarness_SingleRecordedUsableHarness_IsChosenWithoutShowingTheScreen(t *testing.T) {
	for _, mode := range stackAfterHarnessModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryBravo.ID})
			pickMode(t, f.m, mode)

			pressEnter(f.m) // confirm the path

			if f.m.screen == screenHarness {
				t.Fatalf("the harness screen is shown although exactly one usable harness is recorded:\n%s", f.m.View())
			}
			if !f.onStack() {
				t.Errorf("the stack screen does not follow the path screen after the automatic choice:\n%s", f.m.View())
			}
			if f.m.selections.harnessID != entryBravo.ID {
				t.Errorf("harness = %q, want the recorded %q", f.m.selections.harnessID, entryBravo.ID)
			}
		})
	}
}

func TestNarrowedHarness_SingleRecordedUnusableHarness_ShowsTheScreenAndIsNotChosen(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryCharlie.ID})

			f.toHarnessScreen(t)

			requireOffered(t, f.m, entryCharlie)
			requireMessageOnScreen(t, f.m, entryCharlie.UnusableReason)
			pressEnter(f.m)
			requireScreen(t, f.m, screenHarness)
			if f.m.selections.harnessID != "" {
				t.Errorf("harness = %q chosen, want none (the only offered harness is unusable)", f.m.selections.harnessID)
			}
		})
	}
}

// TestNarrowedHarness_RecordedHarnessMissingFromTheRegistry_IsOfferedDisabled verifies a harness
// the manifest records but the session does not list is shown as unavailable and never chosen,
// not even automatically.
func TestNarrowedHarness_RecordedHarnessMissingFromTheRegistry_IsOfferedDisabled(t *testing.T) {
	t.Run("together with a usable one", func(t *testing.T) {
		f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryAlpha.ID, ghostHarnessID})

		f.toHarnessScreen(t)

		requireOffered(t, f.m, entryAlpha, domain.HarnessRef{ID: ghostHarnessID})
		requireMessageOnScreen(t, f.m, "not available")
	})
	t.Run("alone", func(t *testing.T) {
		f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{ghostHarnessID})

		f.toHarnessScreen(t)

		requireOffered(t, f.m, domain.HarnessRef{ID: ghostHarnessID})
		pressEnter(f.m)
		requireScreen(t, f.m, screenHarness)
		if f.m.selections.harnessID != "" {
			t.Errorf("harness = %q chosen, want none", f.m.selections.harnessID)
		}
	})
}

func TestNarrowedHarness_NothingRecorded_ShowsTheFullList(t *testing.T) {
	cases := []struct {
		name     string
		recorded []string
	}{
		{"no manifest, or one that cannot be read", nil},
		{"manifest recording no harness", []string{}},
	}
	for _, tc := range cases {
		for _, mode := range pathFirstModes {
			t.Run(tc.name+"/"+string(mode), func(t *testing.T) {
				f := newNarrowFlow(t, mode, tc.recorded)

				f.toHarnessScreen(t)

				requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
			})
		}
	}
}

func TestNarrowedHarness_PathBelowAWorkspace_ShowsTheFullList(t *testing.T) {
	for _, mode := range pathFirstModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newNarrowFlow(t, mode, []string{entryAlpha.ID})
			f.m.wsScreen.SetPrefilledPath(subDir(t, f.path, "below"))

			f.toHarnessScreen(t)

			requireOffered(t, f.m, entryAlpha, entryBravo, entryCharlie)
		})
	}
}

func TestNarrowedHarness_StackScreenChecksInjectionsOfTheChosenHarness(t *testing.T) {
	t.Run("chosen on the narrowed screen", func(t *testing.T) {
		f := newNarrowFlow(t, domain.ModeUpdateWorkspace, []string{entryAlpha.ID, entryBravo.ID})
		f.toHarnessScreen(t)
		pickHarness(t, f.m, entryBravo.ID)

		f.confirmStack(t)

		if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
			t.Errorf("injections checked for %v, want only %q", f.reloaded.checked, entryBravo.ID)
		}
	})
	t.Run("chosen automatically", func(t *testing.T) {
		f := newNarrowFlow(t, domain.ModeUpdateWorkflows, []string{entryBravo.ID})
		pickMode(t, f.m, f.mode)
		pressEnter(f.m)

		f.confirmStack(t)

		if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
			t.Errorf("injections checked for %v, want only %q", f.reloaded.checked, entryBravo.ID)
		}
	})
	t.Run("transform source", func(t *testing.T) {
		f := newNarrowFlow(t, domain.ModeTransformHarness, []string{entryBravo.ID})
		pickMode(t, f.m, f.mode)
		pressEnter(f.m)
		pickTargetIfAsked(t, f.m)

		f.confirmStack(t)

		if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != entryBravo.ID {
			t.Errorf("injections checked for %v, want only the source %q", f.reloaded.checked, entryBravo.ID)
		}
	})
}
