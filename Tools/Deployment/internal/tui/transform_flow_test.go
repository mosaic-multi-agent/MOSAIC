package tui

// transform_flow_test.go covers the TUI surface for the "transform-harness" mode: mode
// screen entry, keyboard flow from harness selection through the workspace-root path
// screen, running state and service dispatch. The path screen takes the workspace root
// (a directory), as deploy does; a regular file is rejected.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackservicetest"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Transform flow fixtures
// ---------------------------------------------------------------------------

// newTransformFlowSvc returns a stubFlowService pre-configured for transform-mode tests.
// The transform field carries a minimal TransformHarnessResult that the TUI can render
// at screenDone without panicking.
func newTransformFlowSvc(workspace string) *stubFlowService {
	svc := newFlowSvc(workspace)
	svc.transform = app.TransformHarnessResult{
		SourceHarnessID: flowTestHarness.ID,
		TargetHarnessID: "target-harness",
		Transformed:     1,
		Files: []app.TransformFileOutcome{
			{
				SourcePath:      workspace + "/my-agent.src.md",
				DestinationPath: workspace + "/.ai/agents/my-agent.tgt.md",
				Status:          app.StatusTransformed,
			},
		},
	}
	return svc
}

// navigateToTransformMode drives the model from the harness screen to the workspace
// screen with ModeTransformHarness selected. It performs:
//  1. Enter to confirm the single harness.
//  2. Down x6 to move the mode cursor from deploy-workspace to transform-harness (7th item).
//  3. Enter to confirm the mode.
//
// Returns the model at screenWorkspace (or whatever screen the model ends up on).
func navigateToTransformMode(m *rootModel) {
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck — harness → mode
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — deploy-workspace → update-workspace
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — update-workspace → update-workflows
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — update-workflows → deploy-agents
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — deploy-agents → deploy-hooks
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — deploy-hooks → promote-to-generic
	m.Update(tea.KeyMsg{Type: tea.KeyDown})  //nolint:errcheck — promote-to-generic → transform-harness
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck — select transform → workspace
}

// ---------------------------------------------------------------------------
// Mode screen item
// ---------------------------------------------------------------------------

// TestModeScreen_IncludesTransformHarnessEntry verifies that the mode screen contains a
// ModeTransformHarness entry as its fifth item, reachable by pressing Down four times
// from the first item and then Enter.
func TestModeScreen_IncludesTransformHarnessEntry(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // harness → mode

	if m.screen != screenMode {
		t.Fatalf("precondition: screen = %v, want screenMode", m.screen)
	}

	// Navigate Down x6 to the 7th item (transform-harness).
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → update-workspace
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → update-workflows
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → deploy-agents
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → deploy-hooks
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → promote-to-generic
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → transform-harness (only if it exists)

	// Confirm whatever item the cursor is on.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert: the selected mode must be ModeTransformHarness.
	if m.selections.mode != domain.ModeTransformHarness {
		t.Errorf("selections.mode = %q after Down x6 + Enter; want ModeTransformHarness (%q); "+
			"the mode screen must include a ModeTransformHarness entry as the seventh item",
			m.selections.mode, domain.ModeTransformHarness)
	}
}

// TestModeScreen_HasSevenItems verifies that the mode screen has at least seven items,
// with transform-harness at position 6 (the seventh item).
func TestModeScreen_HasSevenItems(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // harness → mode

	if m.screen != screenMode {
		t.Fatalf("precondition: screen = %v, want screenMode", m.screen)
	}

	// Press Down six times to walk through items 2-7 without committing.
	for i := 0; i < 6; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		if m.screen != screenMode {
			t.Fatalf("Down %d left screenMode; mode screen must have at least %d items", i+1, i+2)
		}
	}

	// Confirm the 7th item — must be ModeTransformHarness.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.selections.mode != domain.ModeTransformHarness {
		t.Errorf("7th mode item selection = %q; want ModeTransformHarness", m.selections.mode)
	}
}

// ---------------------------------------------------------------------------
// Workspace screen in transform mode takes the workspace root
// ---------------------------------------------------------------------------

// TestTransformMode_PathScreen_AcceptsExistingDirectory verifies that in transform mode
// the path screen accepts an existing readable directory (the workspace root).
func TestTransformMode_PathScreen_AcceptsExistingDirectory(t *testing.T) {
	// Arrange: pre-fill with an existing directory (the workspace itself).
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	// Navigate to transform workspace screen.
	navigateToTransformMode(m)

	if m.screen != screenWorkspace {
		t.Fatalf("precondition: screen = %v, want screenWorkspace", m.screen)
	}

	// Act: Enter on the directory path.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert: directory accepted → advances to screenRunning.
	if m.screen != screenRunning {
		t.Errorf("screen = %v after Enter on directory in transform mode; want screenRunning", m.screen)
	}
	if cmd == nil {
		t.Error("cmd = nil after directory accepted in transform mode; want a transform service command")
	}
}

// ---------------------------------------------------------------------------
// Full keyboard flow — harness → mode → workspace → running → done
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Service dispatch wiring
// ---------------------------------------------------------------------------

// capturingTransformService wraps a stubFlowService and records the last
// TransformHarnessRequest received by TransformHarness. It is used to verify that the TUI
// wires the already-selected harness id into the transform request as SourceHarnessID.
type capturingTransformService struct {
	stackservicetest.MockStackService
	inner            *stubFlowService
	lastTransformReq *app.TransformHarnessRequest
}

func (s *capturingTransformService) ListHarnesses() []domain.HarnessRef {
	return s.inner.ListHarnesses()
}
func (s *capturingTransformService) DeployNew(ctx context.Context, req app.DeployRequest) (domain.RunSummary, error) {
	return s.inner.DeployNew(ctx, req)
}
func (s *capturingTransformService) Update(ctx context.Context, req app.UpdateRequest) (domain.RunSummary, error) {
	return s.inner.Update(ctx, req)
}
func (s *capturingTransformService) UpdateWorkflows(ctx context.Context, req app.WorkflowUpdateRequest) (domain.RunSummary, error) {
	return s.inner.UpdateWorkflows(ctx, req)
}
func (s *capturingTransformService) Promote(ctx context.Context, req app.PromoteRequest) (app.PromoteResult, error) {
	return s.inner.Promote(ctx, req)
}
func (s *capturingTransformService) TransformHarness(ctx context.Context, req app.TransformHarnessRequest) (app.TransformHarnessResult, error) {
	s.lastTransformReq = &req
	return s.inner.TransformHarness(ctx, req)
}

func (s *capturingTransformService) DeployAgents(ctx context.Context, req app.DeployAgentsRequest) (domain.RunSummary, error) {
	return s.inner.DeployAgents(ctx, req)
}

func (s *capturingTransformService) DeployHooks(ctx context.Context, req app.DeployHooksRequest) (domain.RunSummary, error) {
	return s.inner.DeployHooks(ctx, req)
}

func (s *capturingTransformService) RenderAgent(ctx context.Context, req app.RenderAgentRequest) (app.RenderAgentResult, error) {
	return s.inner.RenderAgent(ctx, req)
}

func (s *capturingTransformService) CheckWorkflowIndex(ctx context.Context) (app.IndexCheckResult, error) {
	return s.inner.CheckWorkflowIndex(ctx)
}

// TestTransformMode_SuppliesSelectedHarnessIDInRequest verifies that when the TUI executes
// a transform run, the TransformHarnessRequest carries the harness id selected on the
// harness screen as SourceHarnessID. The TUI must not present any additional harness
// question during the transform flow.
func TestTransformMode_SuppliesSelectedHarnessIDInRequest(t *testing.T) {
	// Arrange
	workspace := t.TempDir()

	inner := newTransformFlowSvc(workspace)
	spy := &capturingTransformService{inner: inner}
	m := newFlowModel(spy, workspace)

	// Step 1: Harness screen — select the single harness.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.selections.harnessID != flowTestHarness.ID {
		t.Fatalf("precondition: harnessID = %q after harness selection, want %q",
			m.selections.harnessID, flowTestHarness.ID)
	}

	// Step 2: Navigate to ModeTransformHarness (Down x6) and select it.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.selections.mode != domain.ModeTransformHarness {
		t.Fatalf("precondition: mode = %q, want ModeTransformHarness; navigate mode screen first", m.selections.mode)
	}

	// Step 3: Confirm the workspace path.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning {
		t.Fatalf("precondition: screen = %v after workspace confirmation, want screenRunning", m.screen)
	}

	// Execute the transform command so the spy captures the request.
	_, _ = m.Update(runCmd(cmd))

	// Assert: TransformHarness must have been called.
	if spy.lastTransformReq == nil {
		t.Fatal("svc.TransformHarness was not called; startService must dispatch to TransformHarness for ModeTransformHarness")
	}

	// Assert: SourceHarnessID must match the harness selected on the harness screen.
	if spy.lastTransformReq.SourceHarnessID != flowTestHarness.ID {
		t.Errorf("SourceHarnessID = %q, want %q; "+
			"the TUI transform branch must supply the already-selected harness id as SourceHarnessID without asking again",
			spy.lastTransformReq.SourceHarnessID, flowTestHarness.ID)
	}
}

// ---------------------------------------------------------------------------
// Running status text
// ---------------------------------------------------------------------------

// TestTransformMode_RunningStatus_IsTransformSpecific verifies that when the model
// transitions to screenRunning after a transform path confirmation, the in-flight status
// message is transform-specific and does not use the deployment wording.
func TestTransformMode_RunningStatus_IsTransformSpecific(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	// Navigate to transform mode workspace screen.
	navigateToTransformMode(m)

	// Confirm the workspace path — transitions to screenRunning.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRunning {
		t.Fatalf("screen = %v after transform path confirmation; want screenRunning", m.screen)
	}

	// Assert: statusMsg must not be the generic deployment wording.
	if m.statusMsg == "Running deployment…" {
		t.Errorf("statusMsg = %q during transform run; want transform-specific wording, not 'Running deployment…'",
			m.statusMsg)
	}
	if m.statusMsg == "" {
		t.Error("statusMsg is empty during transform run; want transform-specific text")
	}
	// The message should reference the transform operation.
	if !strings.Contains(strings.ToLower(m.statusMsg), "transform") {
		t.Errorf("statusMsg = %q during transform run; want text containing 'transform'", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// Target-harness selection step between mode and workspace
// ---------------------------------------------------------------------------

// TestTransformMode_AfterModeSelection_AdvancesToWorkspaceScreen verifies that selecting
// ModeTransformHarness advances directly to screenWorkspace. The target-harness and
// target-model selection steps are handled via the domain.Interaction port overlay during
// service execution — they are not pre-workspace TUI screens.
//
// This is the contracted design: QTransformTargetHarness routes through the existing
// selectOverlay path (verified by TestInteractionRouting_QTransformTargetHarness_ShowsSelectionOverlay
// in interaction_routing_test.go) and QTransformTargetModel routes through modelOverlay
// (verified by TestInteractionRouting_QTransformTargetModel_ShowsModelSelectScreen).
// The TUI's updateMode handler sets PathKindDirectory and advances to screenWorkspace,
// matching the pattern every other mode follows: mode selection → workspace.
func TestTransformMode_AfterModeSelection_AdvancesToWorkspaceScreen(t *testing.T) {
	// Arrange
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // harness → mode
	if m.screen != screenMode {
		t.Fatalf("precondition: screen = %v, want screenMode", m.screen)
	}

	// Navigate to ModeTransformHarness (Down x6) and confirm.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → update-workspace
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → update-workflows
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → deploy-agents
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → deploy-hooks
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → promote-to-generic
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})  // → transform-harness
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm mode

	if m.selections.mode != domain.ModeTransformHarness {
		t.Fatalf("precondition: selections.mode = %q; ModeTransformHarness item not present; "+
			"TestModeScreen_IncludesTransformHarnessEntry covers this separately", m.selections.mode)
	}

	// Assert: mode selection advances directly to screenWorkspace. Target-harness and
	// target-model selection are overlay interactions during service execution, not
	// pre-workspace TUI screens.
	if m.screen != screenWorkspace {
		t.Errorf("screen = %v after selecting ModeTransformHarness; want screenWorkspace; "+
			"the contracted design routes QTransformTargetHarness and QTransformTargetModel "+
			"through the domain.Interaction port overlay, not through pre-workspace TUI screens; "+
			"updateMode must advance to screenWorkspace immediately after mode confirmation",
			m.screen)
	}
}

// ---------------------------------------------------------------------------
// Regression: non-transform modes keep their path kind
// ---------------------------------------------------------------------------

// TestDeployMode_AfterTransformSelection_StillAcceptsDirectory verifies that after
// navigating back from the transform mode workspace screen and re-selecting deploy-new,
// the path screen switches back to directory kind and accepts a workspace directory.
// This guards against the transform path kind persisting into subsequent mode selections.
func TestDeployMode_AfterTransformSelection_StillAcceptsDirectory(t *testing.T) {
	// Arrange: the transform leg takes the workspace root, as deploy-new does.
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)

	// Navigate to transform workspace screen.
	navigateToTransformMode(m)

	if m.screen != screenWorkspace {
		t.Fatalf("precondition: screen = %v, want screenWorkspace (transform)", m.screen)
	}

	// Esc back to mode screen.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenMode {
		t.Fatalf("screen = %v after Esc from transform workspace; want screenMode", m.screen)
	}

	// Navigate to deploy-workspace (Up x6 from transform-harness index 6) and select it.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // transform-harness → promote-to-generic
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // promote-to-generic → deploy-hooks
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // deploy-hooks → deploy-agents
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // deploy-agents → update-workflows
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // update-workflows → update-workspace
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // update-workspace → deploy-workspace
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenWorkspace {
		t.Fatalf("screen = %v after selecting deploy-new; want screenWorkspace", m.screen)
	}

	// Switch pre-fill to a workspace directory for the deploy-new leg.
	m.wsScreen.SetPrefilledPath(workspace)

	// Act: Enter on the workspace directory.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Assert: directory accepted in deploy-new (directory kind) after back from transform.
	if m.screen != screenRunning {
		t.Errorf("screen = %v after Enter on directory in deploy-new mode following transform selection; "+
			"want screenRunning; switching from transform to deploy-new must re-apply directory-kind validation",
			m.screen)
	}
}
