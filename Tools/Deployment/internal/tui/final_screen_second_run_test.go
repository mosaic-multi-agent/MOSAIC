package tui

// final_screen_second_run_test.go verifies that a second run can be configured and executed
// in the same session after any final screen, and the help lines of the final screens.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// A second run
// ---------------------------------------------------------------------------

// runDeployToDone drives the full entry flow from the mode screen to the end of the
// service call and returns the model on its final screen.
func runDeployToDone(t *testing.T, m *rootModel) {
	t.Helper()
	cmd := advanceToRunning(t, m)
	if m.screen != screenRunning || cmd == nil {
		t.Fatalf("screen = %v, cmd nil = %v; want a running second run", m.screen, cmd == nil)
	}
	m.Update(runCmd(cmd))
	if m.screen != screenDone {
		t.Fatalf("screen = %v after the service call; want screenDone", m.screen)
	}
}

func TestFinalScreen_SecondRun_AfterFailure_ShowsOnlyItsOwnResult(t *testing.T) {
	ws := t.TempDir()
	svc := newFlowSvc(ws)
	svc.deployErr = errors.New("first-run-failure-text")
	m := newFlowModel(svc, ws)
	runDeployToDone(t, m)
	m.Update(runeKey('q'))

	svc.deployErr = nil
	runDeployToDone(t, m)

	view := m.View()
	if m.summaryScreen == nil || m.errMsg != "" {
		t.Errorf("second run shows no summary or still carries the first run's error (errMsg = %q)", m.errMsg)
	}
	if strings.Contains(view, "first-run-failure-text") || strings.Contains(view, "Deployment failed") {
		t.Errorf("second run's final screen shows the first run's failure:\n%s", view)
	}
}

func TestFinalScreen_SecondRun_AfterCancellation_ShowsItsOwnFailure(t *testing.T) {
	ws := t.TempDir()
	svc := newFlowSvc(ws)
	svc.deployErr = app.ErrSelectionCancelled
	m := newFlowModel(svc, ws)
	runDeployToDone(t, m)
	m.Update(runeKey('q'))

	svc.deployErr = errors.New("second-run-failure-text")
	runDeployToDone(t, m)

	view := m.View()
	if !strings.Contains(view, "second-run-failure-text") {
		t.Errorf("second run's final screen does not show its own failure:\n%s", view)
	}
	if strings.Contains(strings.ToLower(view), "cancelled") {
		t.Errorf("second run's final screen still reads as cancelled:\n%s", view)
	}
}

func TestFinalScreen_SecondRun_AfterSummary_ShowsItsOwnFailureNotOldSummary(t *testing.T) {
	ws := t.TempDir()
	svc := newFlowSvc(ws)
	m := newFlowModel(svc, ws)
	runDeployToDone(t, m)
	if m.summaryScreen == nil {
		t.Fatal("precondition: first run has no summary")
	}
	m.Update(runeKey('q'))

	svc.deployErr = errors.New("second-run-failure-text")
	runDeployToDone(t, m)

	if m.summaryScreen != nil {
		t.Error("the first run's summary screen is still set during the second run's error")
	}
	if !strings.Contains(m.View(), "second-run-failure-text") {
		t.Errorf("second run's final screen does not show its own failure:\n%s", m.View())
	}
}

func TestFinalScreen_SecondRun_AfterTransformCancel_IsNotShownAsCancelled(t *testing.T) {
	ws := t.TempDir()
	svc := newTransformFlowSvc(ws)
	svc.transformErr = app.ErrSelectionCancelled
	m := runTransformToDone(t, svc, ws)
	m.Update(runeKey('q'))

	svc.transformErr = nil
	runDeployToDone(t, m)

	if view := m.View(); strings.Contains(strings.ToLower(view), "cancelled") {
		t.Errorf("second run's final screen still reads as cancelled:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Help lines of the bare done views and the run summary
// ---------------------------------------------------------------------------

func TestFinalScreen_BareDoneViews_HelpLineSaysAnyKeyGoesBackToStart(t *testing.T) {
	for _, c := range finalScreenCases() {
		if c.scrolls || c.showText == "" {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			view := strings.Join(strings.Fields(c.build(t).View()), " ")

			if !strings.Contains(view, "any key: back to start") || !strings.Contains(view, "ctrl+c: quit") {
				t.Errorf("help line lacks 'any key: back to start  ctrl+c: quit':\n%s", view)
			}
			if strings.Contains(view, "press q or enter to exit") {
				t.Errorf("view still tells the user to press q or enter to exit:\n%s", view)
			}
		})
	}
}

func TestFinalScreen_DoneWithoutResultOrError_HelpLineSaysAnyKeyGoesBackToStart(t *testing.T) {
	m := newFinalModel(t)
	m.screen = screenDone

	view := strings.Join(strings.Fields(m.View()), " ")

	if !strings.Contains(view, "any key: back to start") {
		t.Errorf("help line lacks 'any key: back to start':\n%s", view)
	}
}

func TestFinalScreen_RunSummaryThroughRootModel_HelpLineNamesScrollAndReturnKeys(t *testing.T) {
	view := strings.Join(strings.Fields(runSummaryModel(t, domain.OutcomeSuccess).View()), " ")

	if !strings.Contains(view, "other keys: back to start") || !strings.Contains(view, "ctrl+c: quit") {
		t.Errorf("run summary help line lacks 'other keys: back to start  ctrl+c: quit':\n%s", view)
	}
}
