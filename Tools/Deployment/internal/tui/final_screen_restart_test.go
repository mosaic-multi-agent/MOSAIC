package tui

// final_screen_restart_test.go verifies the root model's behaviour on every final screen
// (error, cancelled, transform cancelled, run summary of any outcome, promote summary,
// transform summary): a key returns to the first screen with the session state cleared,
// the scroll keys of the run summary keep scrolling, ctrl+c still quits at once, and a
// second run can be configured and executed in the same session.

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// isQuitCmd reports whether cmd, when run, asks the program to quit.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// assertSessionCleared fails unless every piece of state of a finished run is gone.
func assertSessionCleared(t *testing.T, m *rootModel) {
	t.Helper()
	if m.errMsg != "" {
		t.Errorf("errMsg = %q after the return to the first screen; want empty", m.errMsg)
	}
	if m.runCancelled {
		t.Error("runCancelled is still true after the return to the first screen")
	}
	if m.statusMsg != "" {
		t.Errorf("statusMsg = %q after the return to the first screen; want empty", m.statusMsg)
	}
	if m.summaryScreen != nil || m.promoteScreen != nil || m.transformScreen != nil {
		t.Error("a summary screen of the previous run is still set after the return to the first screen")
	}
	if m.activeQuestion != nil || m.reviewOverlay != nil {
		t.Error("an overlay of the previous run is still set after the return to the first screen")
	}
	if m.stack.key != "" {
		t.Errorf("stack.key = %q after the return to the first screen; want empty", m.stack.key)
	}
	if m.ctx.Err() != nil {
		t.Errorf("ctx.Err() = %v; the session must continue after the return to the first screen", m.ctx.Err())
	}
}

// assertBackAtFirstScreen checks the model is on the harness screen with empty selections
// and that the key which led here did not quit the program.
func assertBackAtFirstScreen(t *testing.T, m *rootModel, cmd tea.Cmd) {
	t.Helper()
	if isQuitCmd(cmd) {
		t.Error("the key on the final screen quit the program; it must return to the first screen")
	}
	if m.screen != screenHarness {
		t.Errorf("screen = %v after a key on the final screen; want screenHarness", m.screen)
	}
	if m.selections != (entrySelections{}) {
		t.Errorf("selections = %+v after the return to the first screen; want all cleared", m.selections)
	}
	assertSessionCleared(t, m)
}

// finalScreenCase builds a root model that shows one kind of final screen.
type finalScreenCase struct {
	name     string
	scrolls  bool // the final screen scrolls (run summary)
	build    func(t *testing.T) *rootModel
	showText string
}

func newFinalModel(t *testing.T) *rootModel {
	t.Helper()
	ws := t.TempDir()
	m := newFlowModel(newFlowSvc(ws), ws)
	m.selections = entrySelections{harnessID: flowTestHarness.ID, mode: domain.ModeDeployWorkspace, workspacePath: ws}
	m.statusMsg = "stale-status-of-previous-run"
	return m
}

// markSessionStale puts the leftovers of the finished run into the model (a remembered stack
// key, an unanswered question and a review overlay) and checks they are set, so a test can
// verify the return to the first screen clears them.
func markSessionStale(t *testing.T, m *rootModel) {
	t.Helper()
	m.stack.key = "stale-stack-key-of-previous-run"
	m.activeQuestion = &questionMsg{kind: questionReview, reply: make(chan answerMsg, 1)}
	m.reviewOverlay = screens.NewReviewScreen(domain.Plan{}, 80, 24, stylesFromTheme(m.theme))
	if m.stack.key == "" || m.activeQuestion == nil || m.reviewOverlay == nil {
		t.Fatal("precondition: the stale session state was not set")
	}
}

func runSummaryModel(t *testing.T, outcome domain.Outcome) *rootModel {
	t.Helper()
	m := newFinalModel(t)
	summary := newFlowSvc(m.selections.workspacePath).deploy
	summary.Outcome = outcome
	m.Update(runDoneMsg{summary: summary})
	return m
}

func finalScreenCases() []finalScreenCase {
	return []finalScreenCase{
		{name: "error", showText: "Deployment failed", build: func(t *testing.T) *rootModel {
			m := newFinalModel(t)
			m.Update(runErrorMsg{err: errors.New("disk-full-in-first-run")})
			return m
		}},
		{name: "cancelled", showText: "Deployment cancelled", build: func(t *testing.T) *rootModel {
			m := newFinalModel(t)
			m.Update(runErrorMsg{err: app.ErrSelectionCancelled})
			return m
		}},
		{name: "transform cancelled", showText: "Transform cancelled", build: func(t *testing.T) *rootModel {
			m := newFinalModel(t)
			m.selections.mode = domain.ModeTransformHarness
			m.Update(runErrorMsg{err: app.ErrSelectionCancelled})
			return m
		}},
		{name: "run summary success", scrolls: true, build: func(t *testing.T) *rootModel {
			return runSummaryModel(t, domain.OutcomeSuccess)
		}},
		{name: "run summary failed outcome", scrolls: true, build: func(t *testing.T) *rootModel {
			return runSummaryModel(t, domain.OutcomeFailed)
		}},
		{name: "promote summary", build: func(t *testing.T) *rootModel {
			ws := t.TempDir()
			m := newFlowModel(newPromoteFlowSvc(ws), ws)
			m.selections = entrySelections{harnessID: flowTestHarness.ID, mode: domain.ModePromoteToGeneric, workspacePath: ws}
			m.statusMsg = "stale-status-of-previous-run"
			m.Update(runCmd(m.startService())) // the batch summary of a finished batch promote
			return m
		}},
		{name: "transform summary", build: func(t *testing.T) *rootModel {
			m := newFinalModel(t)
			m.selections.mode = domain.ModeTransformHarness
			m.Update(transformDoneMsg{result: newTransformFlowSvc(m.selections.workspacePath).transform})
			return m
		}},
	}
}

// ---------------------------------------------------------------------------
// A key returns to the first screen
// ---------------------------------------------------------------------------

func TestFinalScreen_KeyReturnsToFirstScreenWithSessionCleared(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"q": runeKey('q'), "x": runeKey('x'), "enter": {Type: tea.KeyEnter}, "esc": {Type: tea.KeyEsc},
		"space": {Type: tea.KeySpace, Runes: []rune{' '}}, "tab": {Type: tea.KeyTab},
	}
	for _, c := range finalScreenCases() {
		for keyName, key := range keys {
			t.Run(c.name+"/"+keyName, func(t *testing.T) {
				m := c.build(t)
				if m.screen != screenDone {
					t.Fatalf("precondition: screen = %v, want screenDone", m.screen)
				}
				markSessionStale(t, m)
				if c.showText != "" && !strings.Contains(m.View(), c.showText) {
					t.Fatalf("precondition: the final screen does not show %q:\n%s", c.showText, m.View())
				}

				_, cmd := m.Update(key)

				assertBackAtFirstScreen(t, m, cmd)
			})
		}
	}
}

func TestFinalScreen_AfterReturn_FirstScreenShowsNoResultOfPreviousRun(t *testing.T) {
	for _, c := range finalScreenCases() {
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Update(runeKey('x'))

			view := m.View()

			for _, stale := range []string{"disk-full-in-first-run", "Deployment failed", "Deployment cancelled",
				"Deployment complete", "stale-status-of-previous-run", "Promote complete", "Transform cancelled"} {
				if strings.Contains(view, stale) {
					t.Errorf("first screen shows %q of the previous run:\n%s", stale, view)
				}
			}
			if !strings.Contains(view, flowTestHarness.DisplayName) {
				t.Errorf("first screen is not the harness selection:\n%s", view)
			}
		})
	}
}

func TestFinalScreen_BareDoneView_WithoutResultOrError_AlsoReturns(t *testing.T) {
	m := newFinalModel(t)
	m.screen = screenDone

	_, cmd := m.Update(runeKey('q'))

	assertBackAtFirstScreen(t, m, cmd)
}

// ---------------------------------------------------------------------------
// Scroll keys on the run summary
// ---------------------------------------------------------------------------

func TestRunSummary_ScrollKeysKeepScrollingAndDoNotReturn(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"up": {Type: tea.KeyUp}, "down": {Type: tea.KeyDown}, "k": runeKey('k'), "j": runeKey('j'),
	}
	for _, outcome := range []domain.Outcome{domain.OutcomeSuccess, domain.OutcomeFailed} {
		for keyName, key := range keys {
			m := runSummaryModel(t, outcome)

			_, cmd := m.Update(key)

			if m.screen != screenDone || m.summaryScreen == nil {
				t.Errorf("%s/%s: scroll key left the run summary (screen = %v)", outcome, keyName, m.screen)
			}
			if isQuitCmd(cmd) {
				t.Errorf("%s/%s: scroll key quit the program", outcome, keyName)
			}
		}
	}
}

func TestRunSummary_ScrollKey_ThenOtherKey_Returns(t *testing.T) {
	m := runSummaryModel(t, domain.OutcomeSuccess)
	m.Update(runeKey('j'))

	_, cmd := m.Update(runeKey('x'))

	assertBackAtFirstScreen(t, m, cmd)
}

// ---------------------------------------------------------------------------
// Ctrl+C and Esc
// ---------------------------------------------------------------------------

func TestFinalScreen_CtrlC_QuitsAtOnce(t *testing.T) {
	for _, c := range finalScreenCases() {
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

			if !isQuitCmd(cmd) {
				t.Error("ctrl+c on the final screen did not quit")
			}
		})
	}
}

func TestFinalScreen_AfterReturn_EscOnFirstScreenStillExits(t *testing.T) {
	m := runSummaryModel(t, domain.OutcomeSuccess)
	_, back := m.Update(runeKey('x'))
	assertBackAtFirstScreen(t, m, back)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !isQuitCmd(cmd) {
		t.Error("Esc on the first screen after a return did not exit the tool")
	}
}

func TestFinalScreen_AfterReturn_CtrlCQuitsFromFirstScreen(t *testing.T) {
	m := runSummaryModel(t, domain.OutcomeSuccess)
	_, back := m.Update(runeKey('x'))
	assertBackAtFirstScreen(t, m, back)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !isQuitCmd(cmd) {
		t.Error("ctrl+c on the first screen after a return did not quit")
	}
}

func TestFinalScreen_NonKeyMessageDoesNotReturnOrQuit(t *testing.T) {
	for _, c := range finalScreenCases() {
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)

			_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

			if m.screen != screenDone {
				t.Errorf("screen = %v after a non-key message; want the final screen to stay", m.screen)
			}
			if isQuitCmd(cmd) {
				t.Error("a non-key message quit the program")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// A preset harness
// ---------------------------------------------------------------------------

func TestFinalScreen_PresetHarness_ReturnsToModeScreenKeepingHarness(t *testing.T) {
	ws := t.TempDir()
	m := newRootModel(context.Background(), newFlowSvc(ws), Options{
		Theme:          DefaultTheme(),
		InitialRequest: &app.DeployRequest{HarnessID: flowTestHarness.ID, WorkspacePath: ws},
	})
	if m.screen != screenMode {
		t.Fatalf("precondition: screen = %v, want screenMode for a preset harness", m.screen)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // mode -> workspace
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(runCmd(cmd))
	if m.screen != screenDone {
		t.Fatalf("precondition: screen = %v after the run, want screenDone", m.screen)
	}

	_, next := m.Update(runeKey('x'))

	if isQuitCmd(next) {
		t.Error("the key quit the program; it must return to the first screen")
	}
	if m.screen != screenMode {
		t.Errorf("screen = %v after the return; want screenMode (first screen of a preset-harness session)", m.screen)
	}
	if m.selections != (entrySelections{harnessID: flowTestHarness.ID}) {
		t.Errorf("selections = %+v; want only the preset harness kept", m.selections)
	}
	assertSessionCleared(t, m)
}

func TestFinalScreen_PresetHarness_ErrorAndCancelledEndingsReturnToModeScreen(t *testing.T) {
	for name, err := range map[string]error{"error": errors.New("preset-run-failure"), "cancelled": app.ErrSelectionCancelled} {
		t.Run(name, func(t *testing.T) {
			ws := t.TempDir()
			m := newRootModel(context.Background(), newFlowSvc(ws), Options{
				Theme:          DefaultTheme(),
				InitialRequest: &app.DeployRequest{HarnessID: flowTestHarness.ID, WorkspacePath: ws},
			})
			m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // mode -> workspace
			m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m.Update(runErrorMsg{err: err})
			if m.screen != screenDone {
				t.Fatalf("precondition: screen = %v, want screenDone", m.screen)
			}

			_, cmd := m.Update(runeKey('x'))

			if isQuitCmd(cmd) || m.screen != screenMode {
				t.Errorf("screen = %v, quit = %v; want screenMode of the preset-harness session", m.screen, isQuitCmd(cmd))
			}
			assertSessionCleared(t, m)
		})
	}
}

func TestFinalScreen_PresetHarness_SecondRunCanBeStarted(t *testing.T) {
	ws := t.TempDir()
	m := newRootModel(context.Background(), newFlowSvc(ws), Options{
		Theme:          DefaultTheme(),
		InitialRequest: &app.DeployRequest{HarnessID: flowTestHarness.ID, WorkspacePath: ws},
	})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(runCmd(cmd))
	m.Update(runeKey('x'))

	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // mode -> workspace
	_, second := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRunning || second == nil {
		t.Errorf("screen = %v, cmd nil = %v; want a second run started from the preset-harness session",
			m.screen, second == nil)
	}
}

