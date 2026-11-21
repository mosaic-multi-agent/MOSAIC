package tui

// flow_test.go verifies that both the deploy-new and update flows are completable by
// keyboard alone, from the mode selection screen through to summary dismissal.
// Tests drive the rootModel through the model/update cycle with no real terminal attached.
//
// Navigation contract verified:
//
//	mode screen → (Enter for deploy-new, Down+Enter for update) → harness screen → (Enter) →
//	workspace screen → (Enter with pre-filled path) → screenRunning →
//	(update and workflows-only ask the path first: mode → workspace → harness → screenRunning)
//	(service completes) → screenDone with SummaryScreen → (any key) → first screen
//
// A pre-filled workspace path is used so the workspace screen does not require character-by-
// character keyboard input in tests — the workspace path must be a real writable directory and
// is provided via t.TempDir(). All other navigation is pure keyboard.

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackservicetest"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Stub service
// ---------------------------------------------------------------------------

// stubFlowService is a minimal app.Service that returns a fixed RunSummary synchronously
// without consulting Interaction. It lets flow tests verify keyboard navigation without
// exercising the question overlay system, which is covered by interaction_routing_test.go.
type stubFlowService struct {
	stackservicetest.MockStackService
	harnesses       []domain.HarnessRef
	deploy          domain.RunSummary
	update          domain.RunSummary
	workflows       domain.RunSummary

	promote         app.PromoteResult
	transform       app.TransformHarnessResult
	deployErr       error
	updateErr       error
	workflowsErr    error

	promoteErr      error
	transformErr    error

	// Batch promote: the result and error PromoteBatch returns, the last request it received
	// and how often Promote (single file) and PromoteBatch were called.
	promoteBatch      app.PromoteBatchResult
	promoteBatchErr   error
	promoteBatchReq   *app.PromoteBatchRequest
	promoteBatchCalls int
	promoteCalls      int

	// transformReq is the last request TransformHarness received.
	transformReq *app.TransformHarnessRequest
}

func (s *stubFlowService) ListHarnesses() []domain.HarnessRef { return s.harnesses }
func (s *stubFlowService) DeployNew(_ context.Context, _ app.DeployRequest) (domain.RunSummary, error) {
	return s.deploy, s.deployErr
}
func (s *stubFlowService) Update(_ context.Context, _ app.UpdateRequest) (domain.RunSummary, error) {
	return s.update, s.updateErr
}
func (s *stubFlowService) UpdateWorkflows(_ context.Context, _ app.WorkflowUpdateRequest) (domain.RunSummary, error) {
	return s.workflows, s.workflowsErr
}
func (s *stubFlowService) PromoteBatch(_ context.Context, req app.PromoteBatchRequest) (app.PromoteBatchResult, error) {
	s.promoteBatchCalls++
	s.promoteBatchReq = &req
	return s.promoteBatch, s.promoteBatchErr
}

func (s *stubFlowService) Promote(_ context.Context, _ app.PromoteRequest) (app.PromoteResult, error) {
	s.promoteCalls++
	return s.promote, s.promoteErr
}
func (s *stubFlowService) TransformHarness(_ context.Context, req app.TransformHarnessRequest) (app.TransformHarnessResult, error) {
	s.transformReq = &req
	return s.transform, s.transformErr
}

func (s *stubFlowService) DeployAgents(_ context.Context, _ app.DeployAgentsRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}

func (s *stubFlowService) DeployHooks(_ context.Context, _ app.DeployHooksRequest) (domain.RunSummary, error) {
	return domain.RunSummary{}, nil
}

func (s *stubFlowService) RenderAgent(_ context.Context, _ app.RenderAgentRequest) (app.RenderAgentResult, error) {
	return app.RenderAgentResult{}, nil
}

func (s *stubFlowService) CheckWorkflowIndex(_ context.Context) (app.IndexCheckResult, error) {
	return app.IndexCheckResult{}, nil
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

var flowTestHarness = domain.HarnessRef{
	ID:          "flow-harness",
	DisplayName: "Flow Test Harness",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// newFlowSvc returns a stubFlowService pre-configured with one usable harness and clean
// RunSummary values for both deploy-new and update modes.
func newFlowSvc(workspace string) *stubFlowService {
	return &stubFlowService{
		harnesses: []domain.HarnessRef{flowTestHarness},
		deploy: domain.RunSummary{
			Mode:           domain.ModeDeployWorkspace,
			Harness:        flowTestHarness,
			WorkspacePath:  workspace,
			DeploymentRoot: workspace + "/.ai",
			Fallback:       domain.FallbackNone,
			Outcome:        domain.OutcomeSuccess,
			Actions: []domain.ActionRecord{
				{
					Ref:    domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "test-runner"},
					Taken:  domain.TakenCreated,
				},
			},
		},
		update: domain.RunSummary{
			Mode:           domain.ModeUpdateWorkspace,
			Harness:        flowTestHarness,
			WorkspacePath:  workspace,
			DeploymentRoot: workspace + "/.ai",
			Fallback:       domain.FallbackNone,
			Outcome:        domain.OutcomeSuccess,
			Actions: []domain.ActionRecord{
				{
					Ref:   domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "test-runner"},
					Taken: domain.TakenUpdated,
				},
			},
		},
	}
}

// newFlowModel creates a rootModel with the given service and a pre-filled workspace path.
// The workspace path is set in the InitialRequest so the workspace screen accepts Enter
// without requiring the test to type the path character-by-character.
func newFlowModel(svc app.Service, workspace string) *rootModel {
	return newRootModel(context.Background(), svc, Options{
		Theme: DefaultTheme(),
		InitialRequest: &app.DeployRequest{
			WorkspacePath: workspace,
		},
	})
}

// runCmd executes a tea.Cmd and returns the message it would dispatch to the model. This is
// the test equivalent of the Bubble Tea runtime's command dispatch loop.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ---------------------------------------------------------------------------
// Deploy-new keyboard-only flow
// ---------------------------------------------------------------------------

// TestFullKeyboardFlow_DeployNew_FromLaunchToExit verifies that the complete deploy-new
// flow — mode selection, harness selection, workspace confirmation, service run, and summary
// dismissal — is achievable by keyboard input alone, with no mouse interaction.
func TestFullKeyboardFlow_DeployNew_FromLaunchToExit(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	if m.screen != screenMode {
		t.Fatalf("precondition: initial screen = %v, want screenMode", m.screen)
	}

	// Step 1: Mode screen — Enter selects deploy-new, which is the first listed mode.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenHarness {
		t.Fatalf("screen = %v after mode Enter; want screenHarness", m.screen)
	}
	if m.selections.mode != domain.ModeDeployWorkspace {
		t.Fatalf("selections.mode = %q; want ModeDeployWorkspace", m.selections.mode)
	}

	// Step 2: Harness screen — Enter selects the first (and only) usable harness.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenWorkspace {
		t.Fatalf("screen = %v after harness Enter; want screenWorkspace", m.screen)
	}

	// Step 3: Workspace screen — Enter confirms the pre-filled path and starts the service.
	// The model transitions to screenRunning and returns a command that calls svc.DeployNew.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning {
		t.Fatalf("screen = %v after workspace Enter; want screenRunning", m.screen)
	}
	if cmd == nil {
		t.Fatal("cmd = nil after workspace confirmation; want a service command")
	}

	// Step 4: Execute the service command (synchronous with our stub) and deliver the result.
	_, _ = m.Update(runCmd(cmd))

	// Assert: model transitioned to summary screen.
	if m.screen != screenDone {
		t.Fatalf("screen = %v after run completion; want screenDone", m.screen)
	}
	if m.summaryScreen == nil {
		t.Fatal("summaryScreen is nil after deploy completion; want a SummaryScreen")
	}

	// Step 5: Summary screen — a key returns to the first screen.
	_, next := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assertBackAtFirstScreen(t, m, next)
}

// TestFullKeyboardFlow_DeployNew_SummaryAcceptsEnterToDismiss verifies that Enter is also
// accepted as an exit key on the summary screen, offering an alternative keyboard path.
func TestFullKeyboardFlow_DeployNew_SummaryAcceptsEnterToDismiss(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	// Navigate to screenDone by keyboard.
	cmd := driveEntry(t, m, domain.ModeDeployWorkspace, "")
	_, _ = m.Update(runCmd(cmd)) // run

	if m.screen != screenDone {
		t.Fatalf("precondition: screen = %v, want screenDone", m.screen)
	}

	// Act: Enter should dismiss the summary screen and return to the first screen.
	_, next := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert
	assertBackAtFirstScreen(t, m, next)
}

// TestFullKeyboardFlow_DeployNew_SummaryViewIsNonEmpty verifies that the summary screen
// rendered at the end of a deploy-new run contains visible content — not an empty string.
func TestFullKeyboardFlow_DeployNew_SummaryViewIsNonEmpty(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	cmd := driveEntry(t, m, domain.ModeDeployWorkspace, "")
	_, _ = m.Update(runCmd(cmd))

	if m.summaryScreen == nil {
		t.Fatal("summaryScreen is nil; nothing to render")
	}

	// Act
	view := m.View()

	// Assert
	if view == "" {
		t.Error("View() returned empty string on screenDone after deploy-new run; want summary content")
	}
}

// ---------------------------------------------------------------------------
// Update keyboard-only flow
// ---------------------------------------------------------------------------

// TestFullKeyboardFlow_Update_FromLaunchToExit verifies that the update flow is completable
// by keyboard alone. The update mode is the second item on the mode screen and requires one
// Down key before pressing Enter to select it.
func TestFullKeyboardFlow_Update_FromLaunchToExit(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	// Step 1: Mode screen — Down moves cursor from deploy-new to update; Enter confirms.
	requireScreen(t, m, screenMode)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // cursor: deploy-new → update
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select update → workspace

	if m.screen != screenWorkspace {
		t.Fatalf("screen = %v after Down+Enter on mode; want screenWorkspace (update asks the path first)", m.screen)
	}
	if m.selections.mode != domain.ModeUpdateWorkspace {
		t.Fatalf("selections.mode = %q after Down+Enter; want ModeUpdateWorkspace", m.selections.mode)
	}

	// Step 2: Workspace screen. The manifest records no harness, so the full list follows.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenHarness {
		t.Fatalf("screen = %v after workspace Enter; want screenHarness", m.screen)
	}

	// Step 3: Harness screen.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning {
		t.Fatalf("screen = %v after harness Enter; want screenRunning", m.screen)
	}

	// Step 4: Run completion.
	_, _ = m.Update(runCmd(cmd))

	if m.screen != screenDone {
		t.Fatalf("screen = %v after update run; want screenDone", m.screen)
	}
	if m.summaryScreen == nil {
		t.Fatal("summaryScreen is nil after update completion; want a SummaryScreen")
	}

	// Step 5: A key returns to the first screen.
	_, next := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assertBackAtFirstScreen(t, m, next)
}

// TestFullKeyboardFlow_Update_BackAndReselect verifies that back navigation during the
// update flow does not break the flow: the user can go back from workspace to mode,
// re-confirm the mode selection, and proceed to a successful completion.
func TestFullKeyboardFlow_Update_BackAndReselect(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	// Navigate to workspace with update selected.
	driveToPath(t, m, domain.ModeUpdateWorkspace, "")

	if m.selections.mode != domain.ModeUpdateWorkspace {
		t.Fatalf("precondition: mode = %q, want ModeUpdateWorkspace", m.selections.mode)
	}

	// Go back from workspace to the mode screen (update asks the path before the harness).
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenMode {
		t.Fatalf("screen = %v after workspace Esc; want screenMode", m.screen)
	}

	// Re-select update and proceed. After back-navigation the cursor remains on the
	// previously selected item (update = index 1); no Down press is needed.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // mode → workspace (cursor already on update)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // workspace → harness

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // harness → running
	_, _ = m.Update(runCmd(cmd))                        // run → done

	// Assert: flow completed with the update mode.
	if m.screen != screenDone {
		t.Fatalf("screen = %v after back-and-reselect completion; want screenDone", m.screen)
	}
	if m.selections.mode != domain.ModeUpdateWorkspace {
		t.Errorf("selections.mode = %q after back-and-reselect; want ModeUpdateWorkspace", m.selections.mode)
	}
}

// ---------------------------------------------------------------------------
// Error path keyboard dismissibility
// ---------------------------------------------------------------------------

// TestFullKeyboardFlow_ServiceError_IsKeyboardDismissible verifies that when the service
// returns an error, the error state on screenDone is dismissible by keyboard alone. The
// keyboard-only contract must hold even when the run fails.
func TestFullKeyboardFlow_ServiceError_IsKeyboardDismissible(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	svc := newFlowSvc(workspace)
	svc.deployErr = errors.New("deployment cancelled by user")
	m := newFlowModel(svc, workspace)

	// Navigate to screenRunning.
	cmd := driveEntry(t, m, domain.ModeDeployWorkspace, "")

	// Deliver the error result (runErrorMsg, not runDoneMsg).
	_, _ = m.Update(runCmd(cmd))

	if m.screen != screenDone {
		t.Fatalf("screen = %v after service error; want screenDone", m.screen)
	}
	if m.errMsg == "" {
		t.Error("errMsg is empty after service error; want a non-empty error message")
	}

	// Act: 'q' must dismiss the error state and return to the first screen.
	_, next := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Assert
	assertBackAtFirstScreen(t, m, next)
}

// ---------------------------------------------------------------------------
// Workflows-only keyboard flow
// ---------------------------------------------------------------------------

// TestFullKeyboardFlow_WorkflowsOnly_StartService_CallsUpdateWorkflows verifies that when the
// mode screen's third item (workflows-only) is selected and the workspace is confirmed,
// startService dispatches to svc.UpdateWorkflows rather than DeployNew or Update.
func TestFullKeyboardFlow_WorkflowsOnly_StartService_CallsUpdateWorkflows(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	svc := newFlowSvc(workspace)
	svc.workflows = domain.RunSummary{
		Mode:           domain.ModeUpdateWorkflows,
		WorkspacePath:  workspace,
		DeploymentRoot: workspace + "/.ai",
		Outcome:        domain.OutcomeSuccess,
	}
	m := newFlowModel(svc, workspace)

	// Step 1: Mode screen — Down twice moves to the third item (workflows-only); Enter confirms.
	requireScreen(t, m, screenMode)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // deploy-new → update
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // update → workflows-only
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select

	if m.screen != screenWorkspace {
		t.Fatalf("screen = %v after two Down+Enter on mode; want screenWorkspace (workflows asks the path first)", m.screen)
	}
	if m.selections.mode != domain.ModeUpdateWorkflows {
		t.Fatalf("selections.mode = %q; want ModeUpdateWorkflows", m.selections.mode)
	}

	// Step 2: Workspace screen — Enter confirms; the manifest records no harness, so the full list follows.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenHarness {
		t.Fatalf("screen = %v after workspace Enter; want screenHarness", m.screen)
	}

	// Step 3: Harness screen.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning {
		t.Fatalf("screen = %v after harness Enter; want screenRunning", m.screen)
	}

	// Step 4: Execute the service command; the stub returns a successful RunSummary.
	msg := runCmd(cmd)
	if msg == nil {
		t.Fatal("startService returned nil message; want runDoneMsg or runErrorMsg")
	}
	_, _ = m.Update(msg)

	// Assert: flow reached summary screen — UpdateWorkflows was dispatched correctly.
	if m.screen != screenDone {
		if m.errMsg != "" {
			t.Fatalf("screen = %v with error %q; want screenDone (UpdateWorkflows must be called, not errored)", m.screen, m.errMsg)
		}
		t.Fatalf("screen = %v after workflows-only run; want screenDone", m.screen)
	}
}

// TestModeScreen_HasThreeItems verifies that the mode screen presents three mode entries so
// that the workflows-only mode is discoverable through the TUI.
func TestModeScreen_HasThreeItems(t *testing.T) {
	// Arrange — navigate to the mode screen.
	workspace := t.TempDir()
	m := newFlowModel(newFlowSvc(workspace), workspace)

	requireScreen(t, m, screenMode)

	// Move the cursor through all three items (Down twice) and verify none of the presses
	// immediately commits the selection (i.e., the list has a third item rather than
	// wrapping or stopping at the second).
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // deploy-new → update
	if m.screen != screenMode {
		t.Fatal("Down from first mode item left screenMode; expected second item")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // update → workflows-only
	if m.screen != screenMode {
		t.Fatal("Down from second mode item left screenMode; expected third item")
	}

	// Confirm the third item and verify the mode is set to ModeUpdateWorkflows.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.selections.mode != domain.ModeUpdateWorkflows {
		t.Errorf("selections.mode = %q after selecting third mode item; want ModeUpdateWorkflows — mode screen may be missing the third entry", m.selections.mode)
	}
}

// TestFullKeyboardFlow_CtrlC_CancelsFromAnyEntryScreen verifies that ctrl+c aborts the flow
// from any of the three entry screens. These are the only keyboard paths that exit without
// going through the normal flow.
func TestFullKeyboardFlow_CtrlC_CancelsFromAnyEntryScreen(t *testing.T) {
	screens := []struct {
		name    string
		advance func(*rootModel)
	}{
		{"mode", func(m *rootModel) {}},
		{"harness", func(m *rootModel) {
			m.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck
		}},
		{"workspace", func(m *rootModel) {
			m.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck
			m.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck
		}},
	}

	for _, tc := range screens {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			m := newFlowModel(newFlowSvc(workspace), workspace)
			tc.advance(m)

			_, quit := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

			if m.ctx.Err() == nil {
				t.Errorf("ctx.Err() = nil after ctrl+c on %s screen; want context cancelled", tc.name)
			}
			if quit == nil {
				t.Errorf("cmd = nil after ctrl+c on %s screen; want tea.Quit", tc.name)
			}
		})
	}
}
