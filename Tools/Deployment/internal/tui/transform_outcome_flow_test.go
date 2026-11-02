package tui

// transform_outcome_flow_test.go covers what the transform flow shows at its end: the
// workspace root passed to the service, the cancelled outcome, the summary with the
// folder transform looked in when no agents were found, a rejected regular file at the
// path screen, and the custom model entry on the transform model question.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// runTransformToDone drives a transform flow over workspace with svc to the end of the
// service call and returns the model.
func runTransformToDone(t *testing.T, svc *stubFlowService, workspace string) *rootModel {
	t.Helper()
	m := newFlowModel(svc, workspace)
	navigateToTransformMode(m)
	if m.screen != screenWorkspace {
		t.Fatalf("screen = %v, want screenWorkspace", m.screen)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning {
		t.Fatalf("screen = %v after confirming the workspace, want screenRunning", m.screen)
	}
	_, _ = m.Update(runCmd(cmd))
	if m.screen != screenDone {
		t.Fatalf("screen = %v after the service call, want screenDone", m.screen)
	}
	return m
}

func TestTransformMode_PathScreen_RejectsRegularFile(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "agent.md")
	if err := os.WriteFile(file, []byte("# agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newFlowModel(newTransformFlowSvc(workspace), file)
	navigateToTransformMode(m)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen == screenRunning {
		t.Error("the transform path screen accepted a regular file; it takes the workspace root")
	}
}

func TestTransformMode_PathScreen_DoesNotOfferFileOrFolderInput(t *testing.T) {
	workspace := t.TempDir()
	m := newFlowModel(newTransformFlowSvc(workspace), workspace)
	navigateToTransformMode(m)

	view := strings.ToLower(m.View())

	if strings.Contains(view, "agent file") || strings.Contains(view, "non-recursive") {
		t.Errorf("transform path screen still offers file or folder input:\n%s", view)
	}
	if !strings.Contains(view, "workspace") {
		t.Errorf("transform path screen does not ask for the workspace:\n%s", view)
	}
}

func TestTransformMode_ServiceCall_PassesWorkspaceRootAndAsksTargetHarness(t *testing.T) {
	workspace := t.TempDir()
	spy := &capturingTransformService{inner: newTransformFlowSvc(workspace)}
	m := newFlowModel(spy, workspace)
	navigateToTransformMode(m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = m.Update(runCmd(cmd))

	if spy.lastTransformReq == nil {
		t.Fatal("TransformHarness was not called")
	}
	if spy.lastTransformReq.Workspace != workspace {
		t.Errorf("Workspace = %q, want %q", spy.lastTransformReq.Workspace, workspace)
	}
	if spy.lastTransformReq.TargetHarnessID != "" {
		t.Errorf("TargetHarnessID = %q, want empty so the target harness is asked", spy.lastTransformReq.TargetHarnessID)
	}
}

func TestTransformMode_CancelledRun_ShowsTransformCancelledNothingWritten(t *testing.T) {
	// Esc at a transform question and declining the plan both end the run with a sentinel
	// error; each must read as a cancelled transform.
	for _, cancelErr := range []error{app.ErrModelSelectionCancelled, app.ErrSelectionCancelled,
		app.ErrPlanNotConfirmed} {
		workspace := t.TempDir()
		svc := newTransformFlowSvc(workspace)
		svc.transformErr = cancelErr

		m := runTransformToDone(t, svc, workspace)

		view := strings.ToLower(m.View())
		if !strings.Contains(view, "cancelled") {
			t.Errorf("%v: the done screen does not show a cancelled outcome:\n%s", cancelErr, view)
		}
		if !strings.Contains(view, "transform") || !strings.Contains(view, "nothing was written") {
			t.Errorf("%v: the cancelled outcome must say the transform wrote nothing:\n%s", cancelErr, view)
		}
		if strings.Contains(view, "deployment cancelled") || strings.Contains(view, "failed") {
			t.Errorf("%v: the cancelled transform is shown as a deployment or a failure:\n%s", cancelErr, view)
		}
	}
}

func TestTransformMode_NoAgentsFound_SummaryNamesFolderTransformLookedIn(t *testing.T) {
	workspace := t.TempDir()
	agentsDir := "/ws/.claude/agents"
	svc := newTransformFlowSvc(workspace)
	svc.transform = app.TransformHarnessResult{
		SourceHarnessID: flowTestHarness.ID, TargetHarnessID: "target-harness",
		Workspace: workspace, SourceAgentsDir: agentsDir,
		Notes: []app.TransformNote{{Kind: app.NoteNoAgentsFound, Message: "no MOSAIC agents were found",
			Paths: []string{agentsDir}}},
	}

	m := runTransformToDone(t, svc, workspace)

	if view := m.View(); !strings.Contains(strings.Join(strings.Fields(view), " "), agentsDir) {
		t.Errorf("the summary does not name the folder transform looked in (%s):\n%s", agentsDir, view)
	}
}

// Regression coverage: the existing summary already prints source, destination and reasons.
func TestTransformMode_Summary_ShowsPlanListingAndReasons(t *testing.T) {
	workspace := t.TempDir()
	svc := newTransformFlowSvc(workspace)
	svc.transform = app.TransformHarnessResult{
		SourceHarnessID: flowTestHarness.ID, TargetHarnessID: "target-harness", Workspace: workspace,
		Transformed: 1, Skipped: 1,
		Files: []app.TransformFileOutcome{
			{Kind: domain.ArtifactAgent, Key: "a", SourcePath: "/src/a.md", DestinationPath: "/dst/a.md",
				Action: app.PlanWriteCreate, Status: app.StatusTransformed},
			{Kind: domain.ArtifactAgent, Key: "b", SourcePath: "/src/b.md", Action: app.PlanSkip,
				Status: app.StatusSkipped, Skip: app.SkipNotAgent, Reason: "no version stamp in frontmatter"},
		},
	}

	m := runTransformToDone(t, svc, workspace)

	view := strings.Join(strings.Fields(m.View()), " ")
	for _, want := range []string{"/src/a.md", "/dst/a.md", "/src/b.md", "no version stamp in frontmatter"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary lacks %q:\n%s", want, view)
		}
	}
}

// The two tests below are regression coverage: the model screen already renders the
// custom-entry row and the transform service already marks the question as allowing it.

func TestInteractionRouting_QTransformTargetModel_OffersCustomEntryWhenAllowed(t *testing.T) {
	m := newRoutingModel()
	qMsg := buildSelectOneMsg(domain.QTransformTargetModel, []domain.Option{{ID: "gpt-4o", Label: "GPT-4o"}})
	qMsg.choiceQ.AllowCustom = true

	m.Update(qMsg)

	if !strings.Contains(strings.ToLower(m.View()), "custom") {
		t.Errorf("the transform model question does not offer custom model entry:\n%s", m.View())
	}
}

func TestInteractionRouting_QTransformTargetModel_EscCancelsTheQuestion(t *testing.T) {
	m := newRoutingModel()
	qMsg := buildSelectOneMsg(domain.QTransformTargetModel, []domain.Option{{ID: "gpt-4o", Label: "GPT-4o"}})
	qMsg.choiceQ.AllowCustom = true
	m.Update(qMsg)

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	select {
	case ans := <-qMsg.reply:
		if ans.choiceAns.Status != domain.Cancelled {
			t.Errorf("answer status = %v, want Cancelled", ans.choiceAns.Status)
		}
	default:
		t.Fatal("Esc at the transform model question gave no answer; the service would wait forever")
	}
}
