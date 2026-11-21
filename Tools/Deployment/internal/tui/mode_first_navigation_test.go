package tui

// mode_first_navigation_test.go verifies how the root model moves between the entry screens and
// restarts: Esc on each entry screen, the return from the final screen (previous mode and
// harness pre-highlighted), the restart after a declined plan (nothing pre-highlighted, choices
// cleared) and a harness preset at launch (mode screen first, harness screen skipped).

import (
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// newPresetEntryModel builds a root model launched with the harness already chosen, over a
// service that can run every mode and whose path screen is pre-filled.
func newPresetEntryModel(t *testing.T, presetID string) *rootModel {
	t.Helper()
	ws := t.TempDir()
	svc := newFlowSvc(ws)
	svc.harnesses = []domain.HarnessRef{entryAlpha, entryBravo}
	return newRootModel(context.Background(), svc, Options{
		Theme:          DefaultTheme(),
		InitialRequest: &app.DeployRequest{HarnessID: presetID, WorkspacePath: ws},
	})
}

// runEntryToDone drives the entry screens for mode and harnessID, starts the run and delivers
// the outcome of the service call (an error when runErr is set) so the model shows a final screen.
func runEntryToDone(t *testing.T, m *rootModel, mode domain.RunMode, harnessID string, runErr error) {
	t.Helper()
	cmd := driveEntry(t, m, mode, harnessID)
	requireScreen(t, m, screenRunning)
	if runErr != nil {
		m.Update(runErrorMsg{err: runErr})
	} else {
		m.Update(runCmd(cmd))
	}
	requireScreen(t, m, screenDone)
}

// assertModeHighlighted confirms the mode screen shows and Enter chooses want, which proves the
// cursor was on it.
func assertModeHighlighted(t *testing.T, m *rootModel, want domain.RunMode) {
	t.Helper()
	requireScreen(t, m, screenMode)
	pressEnter(m)
	if m.selections.mode != want {
		t.Errorf("Enter on the mode screen chose %q, want the pre-highlighted %q", m.selections.mode, want)
	}
}

// assertHarnessHighlighted confirms the harness screen shows and Enter chooses want.
func assertHarnessHighlighted(t *testing.T, m *rootModel, want string) {
	t.Helper()
	requireScreen(t, m, screenHarness)
	pressEnter(m)
	if m.selections.harnessID != want {
		t.Errorf("Enter on the harness screen chose %q, want the pre-highlighted %q", m.selections.harnessID, want)
	}
}

// ---------------------------------------------------------------------------
// Esc on the entry screens
// ---------------------------------------------------------------------------

func TestModeFirstNavigation_EscOnModeScreen_ExitsAlsoAfterGoingBack(t *testing.T) {
	m := newEntryModel()
	pickMode(t, m, domain.ModeDeployHooks)
	pressEsc(m) // harness -> mode

	cmd := pressEsc(m)

	if !isQuitCmd(cmd) {
		t.Error("Esc on the mode screen did not exit the TUI")
	}
}

func TestModeFirstNavigation_EscOnHarnessScreen_ReturnsToModeScreenKeepingTheMode(t *testing.T) {
	m := newEntryModel()
	pickMode(t, m, domain.ModeDeployHooks)

	cmd := pressEsc(m)

	if isQuitCmd(cmd) {
		t.Fatal("Esc on the harness screen exited the TUI")
	}
	assertModeHighlighted(t, m, domain.ModeDeployHooks)
}

func TestModeFirstNavigation_EscOnPathScreen_ReturnsToHarnessScreenKeepingTheHarness(t *testing.T) {
	m := newEntryModel()
	driveToPath(t, m, domain.ModeDeployWorkspace, entryBravo.ID)

	cmd := pressEsc(m)

	if isQuitCmd(cmd) {
		t.Fatal("Esc on the path screen exited the TUI")
	}
	assertHarnessHighlighted(t, m, entryBravo.ID)
}

func TestModeFirstNavigation_EscOnPathScreen_WithPresetHarness_ReturnsToModeScreen(t *testing.T) {
	m := newPresetEntryModel(t, entryBravo.ID)
	driveToPath(t, m, domain.ModeDeployHooks, "")

	pressEsc(m)

	if m.screen != screenMode {
		t.Fatalf("screen = %v after Esc on the path screen of a preset-harness session, want screenMode", m.screen)
	}
	if m.selections.harnessID != entryBravo.ID {
		t.Errorf("selections.harnessID = %q, want the preset %q kept", m.selections.harnessID, entryBravo.ID)
	}
	assertModeHighlighted(t, m, domain.ModeDeployHooks)
}

// ---------------------------------------------------------------------------
// Return from the final screen
// ---------------------------------------------------------------------------

func TestModeFirstNavigation_FinalScreen_ReturnsToModeScreenWithPreviousModeAndHarnessHighlighted(t *testing.T) {
	cases := []struct {
		name   string
		mode   domain.RunMode
		runErr error
	}{
		{"hooks run summary", domain.ModeDeployHooks, nil},
		{"workflows run summary", domain.ModeUpdateWorkflows, nil},
		{"failed run", domain.ModeDeployAgents, errors.New("run failed")},
		{"cancelled run", domain.ModeUpdateWorkspace, app.ErrSelectionCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newEntryFlowModel(t)
			runEntryToDone(t, m, tc.mode, entryBravo.ID, tc.runErr)

			m.Update(runeKey('x'))

			assertModeHighlighted(t, m, tc.mode)
			if isPathFirst(tc.mode) {
				requireScreen(t, m, screenWorkspace)
				pressEnter(m) // confirm the path; the harness screen follows it
			}
			assertHarnessHighlighted(t, m, entryBravo.ID)
		})
	}
}

func TestModeFirstNavigation_FinalScreen_WithPresetHarness_ReturnsToModeScreenKeepingThePreset(t *testing.T) {
	m := newPresetEntryModel(t, entryBravo.ID)
	runEntryToDone(t, m, domain.ModeDeployHooks, "", nil)

	m.Update(runeKey('x'))

	assertModeHighlighted(t, m, domain.ModeDeployHooks)
	if m.screen != screenWorkspace {
		t.Errorf("screen = %v after choosing the mode, want the path screen (the preset harness skips the harness screen)", m.screen)
	}
	if m.selections.harnessID != entryBravo.ID {
		t.Errorf("selections.harnessID = %q after the restart, want the preset %q", m.selections.harnessID, entryBravo.ID)
	}
}

// ---------------------------------------------------------------------------
// Declined plan
// ---------------------------------------------------------------------------

func TestModeFirstNavigation_DeclinedPlan_ReturnsToModeScreenWithNothingHighlighted(t *testing.T) {
	m := newEntryFlowModel(t)
	driveEntry(t, m, domain.ModeDeployHooks, entryBravo.ID)
	requireScreen(t, m, screenRunning)

	m.Update(runErrorMsg{err: app.ErrPlanNotConfirmed})

	requireScreen(t, m, screenMode)
	assertModeHighlighted(t, m, domain.ModeDeployWorkspace) // the first mode, not the declined one
	assertHarnessHighlighted(t, m, entryAlpha.ID)           // the first harness, not the declined one
}

func TestModeFirstNavigation_DeclinedPlan_ClearsTheChoices(t *testing.T) {
	m := newEntryFlowModel(t)
	driveEntry(t, m, domain.ModeDeployHooks, entryBravo.ID)

	m.Update(runErrorMsg{err: app.ErrPlanNotConfirmed})

	if m.selections != (entrySelections{}) {
		t.Errorf("selections = %+v after a declined plan, want all cleared", m.selections)
	}
}

func TestModeFirstNavigation_DeclinedPlan_WithPresetHarness_KeepsThePreset(t *testing.T) {
	m := newPresetEntryModel(t, entryBravo.ID)
	driveEntry(t, m, domain.ModeDeployHooks, "")
	requireScreen(t, m, screenRunning)

	m.Update(runErrorMsg{err: app.ErrPlanNotConfirmed})

	requireScreen(t, m, screenMode)
	pickMode(t, m, domain.ModeDeployAgents)
	if m.screen != screenWorkspace || m.selections.harnessID != entryBravo.ID {
		t.Errorf("screen = %v, harness = %q after the restart; want the path screen with the preset %q",
			m.screen, m.selections.harnessID, entryBravo.ID)
	}
}

// TestModeFirstNavigation_DeclinedPlan_TransformIsUnchanged verifies a declined transform plan
// still ends on the final screen instead of restarting.
func TestModeFirstNavigation_DeclinedPlan_TransformIsUnchanged(t *testing.T) {
	m := newEntryFlowModel(t)
	driveEntry(t, m, domain.ModeTransformHarness, entryBravo.ID)
	requireScreen(t, m, screenRunning)

	m.Update(runErrorMsg{err: app.ErrPlanNotConfirmed})

	if m.screen != screenDone {
		t.Errorf("screen = %v after a declined transform plan, want screenDone as before", m.screen)
	}
}

// ---------------------------------------------------------------------------
// Preset harness
// ---------------------------------------------------------------------------

func TestModeFirstNavigation_PresetHarness_ModeScreenFirstAndHarnessScreenSkippedInEveryMode(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			m := newPresetEntryModel(t, entryBravo.ID)
			requireScreen(t, m, screenMode)

			pickMode(t, m, mode)

			if m.screen != screenWorkspace {
				t.Errorf("screen = %v after choosing %s with a preset harness, want the path screen", m.screen, mode)
			}
			if m.selections.harnessID != entryBravo.ID {
				t.Errorf("selections.harnessID = %q, want the preset %q (the source in Promote and Transform)",
					m.selections.harnessID, entryBravo.ID)
			}
		})
	}
}
