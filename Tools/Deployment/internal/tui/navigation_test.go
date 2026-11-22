package tui

// navigation_test.go verifies the rootModel navigation state machine: forward transitions
// between entry screens, back navigation with prior-input preservation, and cancellation.
//
// These tests are in package tui (internal) because the screenID type and rootModel fields
// are unexported. The tests drive the model through the Bubble Tea model/update cycle with
// no real terminal attached.
//
// Navigation contract exercised:
//   mode → (enter) → harness → (enter) → workspace → (enter) → running
//   esc from workspace → harness (prior harness preserved)
//   esc from harness   → mode (prior mode preserved)
//   esc from mode      → quit (nothing to go back to)
//   ctrl+c from any screen → context cancelled, quit command returned

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackservicetest"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Stub app.Service for navigation tests
// ---------------------------------------------------------------------------

// stubNavService satisfies app.Service with fixed, non-blocking responses. Navigation tests
// only need ListHarnesses; the deploy methods are never called during entry-screen testing.
type stubNavService struct {
	stackservicetest.MockStackService
	harnesses []domain.HarnessRef
}

func (s *stubNavService) ListHarnesses() []domain.HarnessRef { return s.harnesses }
func (s *stubNavService) DeployNew(_ context.Context, _ app.DeployRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}
func (s *stubNavService) Update(_ context.Context, _ app.UpdateRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}
// PromoteBatch satisfies app.Service; the batch is not exercised by these tests.
func (s *stubNavService) PromoteBatch(_ context.Context, _ app.PromoteBatchRequest) (app.PromoteBatchResult, error) {
	return app.PromoteBatchResult{}, nil
}

func (s *stubNavService) Promote(_ context.Context, _ app.PromoteRequest) (app.PromoteResult, error) {
	return app.PromoteResult{}, nil
}
func (s *stubNavService) UpdateWorkflows(_ context.Context, _ app.WorkflowUpdateRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}
func (s *stubNavService) TransformHarness(_ context.Context, _ app.TransformHarnessRequest) (app.TransformHarnessResult, error) {
	return app.TransformHarnessResult{}, nil
}

func (s *stubNavService) DeployAgents(_ context.Context, _ app.DeployAgentsRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}

func (s *stubNavService) DeployHooks(_ context.Context, _ app.DeployHooksRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}

func (s *stubNavService) RenderAgent(_ context.Context, _ app.RenderAgentRequest) (app.RenderAgentResult, error) {
	return app.RenderAgentResult{}, nil
}

func (s *stubNavService) CheckWorkflowIndex(_ context.Context) (app.IndexCheckResult, error) {
	return app.IndexCheckResult{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var navUsableHarness = domain.HarnessRef{
	ID:          "nav-harness",
	DisplayName: "Nav Harness",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// newNavModel creates a rootModel wired with a single usable harness and no reload
// capability, so mode selection leads straight to the path screen.
func newNavModel() *rootModel {
	svc := &stubNavService{harnesses: []domain.HarnessRef{navUsableHarness}}
	return newRootModel(context.Background(), svc, Options{
		Theme: DefaultTheme(),
	})
}

// sendKey drives the model with a single key message and returns the updated model and cmd.
// Since rootModel uses pointer receivers, the returned model is always the same *rootModel.
func sendKey(m *rootModel, keyType tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: keyType})
}

// sendRune drives the model with a single character key message.
func sendRune(m *rootModel, r rune) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// ---------------------------------------------------------------------------
// Initial state
// ---------------------------------------------------------------------------

// TestNavigation_InitialScreen_IsMode verifies that the root model starts on the mode
// selection screen when no harness is pre-filled.
func TestNavigation_InitialScreen_IsMode(t *testing.T) {
	// Arrange / Act
	m := newNavModel()

	// Assert
	if m.screen != screenMode {
		t.Errorf("initial screen = %v, want screenMode (%v)", m.screen, screenMode)
	}
}

// TestNavigation_PrefilledHarness_StartsOnModeScreen verifies that when a harness ID is
// pre-filled in the InitialRequest, the TUI starts on mode like any session.
func TestNavigation_PrefilledHarness_StartsOnModeScreen(t *testing.T) {
	// Arrange
	svc := &stubNavService{harnesses: []domain.HarnessRef{navUsableHarness}}
	m := newRootModel(context.Background(), svc, Options{
		Theme: DefaultTheme(),
		InitialRequest: &app.DeployRequest{
			HarnessID: "nav-harness",
		},
	})

	// Assert
	if m.screen != screenMode {
		t.Errorf("screen = %v with pre-filled harness, want screenMode (%v)", m.screen, screenMode)
	}
	if m.selections.harnessID != "nav-harness" {
		t.Errorf("selections.harnessID = %q, want %q", m.selections.harnessID, "nav-harness")
	}
}

// ---------------------------------------------------------------------------
// Forward transitions
// ---------------------------------------------------------------------------

// TestNavigation_ModeEnter_TransitionsToHarness verifies the first forward transition:
// pressing Enter on a mode advances the root model to the harness selection screen.
func TestNavigation_ModeEnter_TransitionsToHarness(t *testing.T) {
	// Arrange
	m := newNavModel()
	requireScreen(t, m, screenMode)

	// Act — Enter on the mode list (first mode is deploy-new)
	sendKey(m, tea.KeyEnter)

	// Assert
	if m.screen != screenHarness {
		t.Errorf("screen = %v after mode Enter, want screenHarness (%v)", m.screen, screenHarness)
	}
}

// TestNavigation_HarnessEnter_RecordsSelectedHarnessID verifies that the selected harness ID
// is captured in the model's selections so the service call receives the correct harness.
func TestNavigation_HarnessEnter_RecordsSelectedHarnessID(t *testing.T) {
	// Arrange
	m := newNavModel()
	sendKey(m, tea.KeyEnter) // mode -> harness

	// Act
	sendKey(m, tea.KeyEnter)

	// Assert
	if m.selections.harnessID != "nav-harness" {
		t.Errorf("selections.harnessID = %q after harness selection, want %q", m.selections.harnessID, "nav-harness")
	}
}

// TestNavigation_HarnessEnter_TransitionsToWorkspace verifies the second forward transition:
// pressing Enter on a harness advances the root model to the path screen.
func TestNavigation_HarnessEnter_TransitionsToWorkspace(t *testing.T) {
	// Arrange - advance to the harness screen first
	m := newNavModel()
	sendKey(m, tea.KeyEnter) // mode -> harness
	requireScreen(t, m, screenHarness)

	// Act
	sendKey(m, tea.KeyEnter)

	// Assert
	if m.screen != screenWorkspace {
		t.Errorf("screen = %v after harness Enter, want screenWorkspace (%v)", m.screen, screenWorkspace)
	}
}

// TestNavigation_ModeEnter_RecordsSelectedMode verifies that the chosen run mode is captured
// in the model's selections so the service call uses the correct mode.
func TestNavigation_ModeEnter_RecordsSelectedMode(t *testing.T) {
	// Arrange
	m := newNavModel()

	// Act — Enter on first mode (deploy-new)
	sendKey(m, tea.KeyEnter)

	// Assert
	if m.selections.mode != domain.ModeDeployWorkspace {
		t.Errorf("selections.mode = %q, want %q (deploy-new is the first mode item)", m.selections.mode, domain.ModeDeployWorkspace)
	}
}

// ---------------------------------------------------------------------------
// Back navigation — state preservation
// ---------------------------------------------------------------------------

// TestNavigation_ModeEsc_Quits verifies that pressing Esc on the mode screen returns
// a quit command. There is nothing to go back to from the first entry screen.
func TestNavigation_ModeEsc_Quits(t *testing.T) {
	// Arrange
	m := newNavModel()

	// Act
	cmd := pressEsc(m)

	// Assert
	if !isQuitCmd(cmd) {
		t.Error("cmd is not tea.Quit after Esc from the mode screen")
	}
}

// TestNavigation_HarnessEsc_TransitionsBackToMode verifies that pressing Esc on the harness
// screen returns the user to the mode selection screen.
func TestNavigation_HarnessEsc_TransitionsBackToMode(t *testing.T) {
	// Arrange — advance to harness screen
	m := newNavModel()
	sendKey(m, tea.KeyEnter) // mode → harness

	// Act
	cmd := pressEsc(m) // harness → mode

	// Assert
	if isQuitCmd(cmd) {
		t.Error("Esc on the harness screen quit the program; it must return to the mode screen")
	}
	if m.screen != screenMode {
		t.Errorf("screen = %v after harness Esc, want screenMode (%v)", m.screen, screenMode)
	}
}

// TestNavigation_HarnessEsc_PreservesModeSelection verifies that pressing Esc on the harness
// screen does not lose the mode that was chosen on the previous screen. The user
// should be able to go back and return without losing prior valid input.
func TestNavigation_HarnessEsc_PreservesModeSelection(t *testing.T) {
	// Arrange — select a non-default mode and advance to harness
	m := newNavModel()
	pickMode(t, m, domain.ModeDeployHooks)

	// Act — go back from harness to mode
	pressEsc(m)

	// Assert
	if m.selections.mode != domain.ModeDeployHooks {
		t.Errorf("selections.mode = %q after harness Esc; want %q (prior selection must survive back navigation)", m.selections.mode, domain.ModeDeployHooks)
	}
}

// TestNavigation_WorkspaceEsc_TransitionsBackToHarness verifies that pressing Esc on the
// workspace screen returns the user to the harness selection screen.
func TestNavigation_WorkspaceEsc_TransitionsBackToHarness(t *testing.T) {
	// Arrange — advance to workspace screen
	m := newNavModel()
	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	// Act
	pressEsc(m) // workspace → harness

	// Assert
	if m.screen != screenHarness {
		t.Errorf("screen = %v after workspace Esc, want screenHarness (%v)", m.screen, screenHarness)
	}
}

// TestNavigation_WorkspaceEsc_PreservesHarnessSelection verifies that pressing Esc on the
// workspace screen does not lose the harness that was chosen on the harness screen.
func TestNavigation_WorkspaceEsc_PreservesHarnessSelection(t *testing.T) {
	// Arrange
	m := newNavModel()
	driveToPath(t, m, domain.ModeDeployWorkspace, "")
	capturedHarness := m.selections.harnessID

	// Act
	pressEsc(m) // workspace → harness

	// Assert
	if capturedHarness == "" || m.selections.harnessID != capturedHarness {
		t.Errorf("selections.harnessID = %q after workspace Esc; want %q (prior harness must survive back navigation)", m.selections.harnessID, capturedHarness)
	}
}

// ---------------------------------------------------------------------------
// ctrl+c — global cancel from any entry screen
// ---------------------------------------------------------------------------

// TestNavigation_CtrlC_FromFirstScreen_CancelsContextAndQuits verifies that ctrl+c from the
// first (mode) screen cancels the model's context (which would cancel any running service call)
// and returns a quit command.
func TestNavigation_CtrlC_FromFirstScreen_CancelsContextAndQuits(t *testing.T) {
	// Arrange
	m := newNavModel()

	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Assert
	if m.ctx.Err() == nil {
		t.Error("ctx.Err() = nil after ctrl+c; context must be cancelled so any running service goroutine is stopped")
	}
	if cmd == nil {
		t.Error("cmd = nil after ctrl+c; want tea.Quit (non-nil command)")
	}
}

// TestNavigation_CtrlC_FromHarness_CancelsContextAndQuits verifies that ctrl+c works from the
// harness selection screen, not only from the first screen.
func TestNavigation_CtrlC_FromHarness_CancelsContextAndQuits(t *testing.T) {
	// Arrange — advance to harness screen
	m := newNavModel()
	sendKey(m, tea.KeyEnter) // mode → harness
	requireScreen(t, m, screenHarness)

	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Assert
	if m.ctx.Err() == nil {
		t.Error("ctx.Err() = nil after ctrl+c from harness screen; context must be cancelled from any screen")
	}
	if cmd == nil {
		t.Error("cmd = nil after ctrl+c from harness screen; want tea.Quit")
	}
}

// TestNavigation_CtrlC_FromWorkspace_CancelsContextAndQuits verifies that ctrl+c works from
// the workspace path entry screen.
func TestNavigation_CtrlC_FromWorkspace_CancelsContextAndQuits(t *testing.T) {
	// Arrange — advance to workspace screen
	m := newNavModel()
	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	// Act
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Assert
	if m.ctx.Err() == nil {
		t.Error("ctx.Err() = nil after ctrl+c from workspace screen; context must be cancelled from any screen")
	}
	if cmd == nil {
		t.Error("cmd = nil after ctrl+c from workspace screen; want tea.Quit")
	}
}

// ---------------------------------------------------------------------------
// View rendering — screen-specific output
// ---------------------------------------------------------------------------

// TestNavigation_HarnessScreen_ViewContainsTitleText verifies that the harness selection
// screen's view contains a title, confirming the correct screen is rendered.
func TestNavigation_HarnessScreen_ViewContainsTitleText(t *testing.T) {
	// Arrange
	m := newNavModel()
	sendKey(m, tea.KeyEnter) // mode → harness
	requireScreen(t, m, screenHarness)

	// Act
	view := m.View()

	// Assert
	if view == "" {
		t.Error("View() returned empty string on harness screen; want non-empty output")
	}
	// The harness screen title is "Select Harness" (from screens/harness.go)
	if !containsAny(view, "Select Harness", "Harness", "harness") {
		t.Errorf("harness screen view does not appear to show the harness selection title:\n%s", view)
	}
}

// TestNavigation_ModeScreen_ViewContainsTitleText verifies that the first view
// is the mode selection screen.
func TestNavigation_ModeScreen_ViewContainsTitleText(t *testing.T) {
	// Arrange
	m := newNavModel()
	requireScreen(t, m, screenMode)

	// Act
	view := m.View()

	// Assert
	if view == "" {
		t.Error("View() returned empty string on mode screen; want non-empty output")
	}
	if !containsAny(view, "Select Mode", "Mode", "deploy", "update") {
		t.Errorf("mode screen view does not appear to show the mode selection content:\n%s", view)
	}
}

// TestNavigation_WorkspaceScreen_ViewContainsTitleText verifies that after transitioning to
// the workspace screen, the view reflects the workspace path entry screen.
func TestNavigation_WorkspaceScreen_ViewContainsTitleText(t *testing.T) {
	// Arrange
	m := newNavModel()
	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	// Act
	view := m.View()

	// Assert
	if view == "" {
		t.Error("View() returned empty string on workspace screen; want non-empty output")
	}
	if !containsAny(view, "Workspace", "workspace", "path", "directory") {
		t.Errorf("workspace screen view does not appear to show the workspace entry content:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Window resize propagation
// ---------------------------------------------------------------------------

// TestNavigation_WindowResize_PropagatedToAllScreens verifies that a WindowSizeMsg updates
// the model dimensions without causing a panic. All screens must accept resize events.
func TestNavigation_WindowResize_PropagatedToAllScreens(t *testing.T) {
	// Arrange
	m := newNavModel()

	// Act — send a resize; must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("resize message caused a panic: %v", r)
		}
	}()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Assert
	if m.width != 120 || m.height != 40 {
		t.Errorf("after resize: width=%d height=%d, want 120/40", m.width, m.height)
	}
}

// ---------------------------------------------------------------------------

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
