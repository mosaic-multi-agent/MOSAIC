package tui

// promote_flow_test.go covers the TUI entry of the promote mode, which is a batch promote:
// the mode screen entry, the path step asking for an agents folder (a file is rejected, a
// readable folder is accepted), the batch operation receiving the harness and the folder, the
// running status, the batch summary after the run, and the error and cancel outcomes.
//
// The questions of the batch are covered in promote_batch_routing_test.go, and the stack step
// of promote in stack_promote_agents_folder_test.go.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// createAgentFile creates a regular file named "agent.md" in dir and returns its absolute
// path. It is the wrong kind of input for the agents folder step.
func createAgentFile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(p, []byte("# agent\n"), 0o644); err != nil {
		t.Fatalf("createAgentFile: failed to write %q: %v", p, err)
	}
	return p
}

// createAgentsFolder creates a folder holding one agent file below parent and returns the
// folder path.
func createAgentsFolder(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("createAgentsFolder: %v", err)
	}
	createAgentFile(t, dir)
	return dir
}

// newPromoteFlowSvc returns a stubFlowService pre-configured for promote-mode tests: the batch
// returns a result for two agents, one promoted and one skipped.
func newPromoteFlowSvc(workspace string) *stubFlowService {
	svc := newFlowSvc(workspace)
	svc.promoteBatch = app.PromoteBatchResult{
		HarnessID:       flowTestHarness.ID,
		AgentsDir:       workspace,
		TargetCatalogID: "team",
		TargetLayer:     "Team Layer (team)",
		Agents: []app.PromoteBatchAgentResult{
			{
				SourcePath: filepath.Join(workspace, "alpha-agent.md"), Key: "alpha-agent", Status: app.PromoteBatchPromoted,
				Result: app.PromoteResult{DestinationPath: "/team/Subagents/Dev/alpha-agent.md", NumericID: "42", Key: "alpha-agent"},
			},
			{
				SourcePath: filepath.Join(workspace, "beta-agent.md"), Key: "beta-agent", Status: app.PromoteBatchSkipped,
				Reason: "kept the existing agent in Team Layer (team)",
			},
		},
	}
	return svc
}

// selectPromoteMode moves from the mode screen through the harness screen to the path screen of
// the promote mode.
func selectPromoteMode(t *testing.T, m *rootModel) {
	t.Helper()
	driveToPath(t, m, domain.ModePromoteToGeneric, "")
}

// runPromote confirms the pre-filled agents folder and delivers the result of the batch run to
// the model.
func runPromote(t *testing.T, m *rootModel) {
	t.Helper()
	selectPromoteMode(t, m)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRunning || cmd == nil {
		t.Fatalf("screen = %v, cmd nil = %v after confirming the agents folder; want screenRunning and a command",
			m.screen, cmd == nil)
	}
	_, _ = m.Update(runCmd(cmd))
}

// ---------------------------------------------------------------------------
// Mode screen
// ---------------------------------------------------------------------------

func TestModeScreen_IncludesPromoteEntry(t *testing.T) {
	workspace := t.TempDir()
	m := newFlowModel(newPromoteFlowSvc(workspace), workspace)

	selectPromoteMode(t, m)
}

// ---------------------------------------------------------------------------
// Path step: an agents folder
// ---------------------------------------------------------------------------

func TestPromoteMode_PathStep_AsksForAnAgentsFolder(t *testing.T) {
	workspace := t.TempDir()
	m := newFlowModel(newPromoteFlowSvc(workspace), workspace)
	selectPromoteMode(t, m)

	lower := strings.ToLower(collapseSpaces(m.View()))

	if !strings.Contains(lower, "agents folder") {
		t.Errorf("path step does not ask for an agents folder:\n%s", m.View())
	}
	if strings.Contains(lower, "workspace") {
		t.Errorf("path step uses workspace wording in promote mode:\n%s", m.View())
	}
	if strings.Contains(lower, "agent file path") {
		t.Errorf("path step still asks for a single agent file:\n%s", m.View())
	}
}

func TestPromoteMode_PathStep_AcceptsAReadableFolder(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(folder), folder)
	selectPromoteMode(t, m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRunning {
		t.Errorf("screen = %v after confirming a folder; want screenRunning", m.screen)
	}
	if cmd == nil {
		t.Error("cmd = nil after confirming a folder; want the batch promote command")
	}
}

func TestPromoteMode_PathStep_RejectsAFile(t *testing.T) {
	file := createAgentFile(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(filepath.Dir(file)), file)
	selectPromoteMode(t, m)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenWorkspace {
		t.Errorf("screen = %v after confirming a file; want the path step to stay", m.screen)
	}
}

func TestBackNavigation_PromoteThenDeployNew_PathStepUsesWorkspaceWording(t *testing.T) {
	workspace := t.TempDir()
	m := newFlowModel(newPromoteFlowSvc(workspace), workspace)
	selectPromoteMode(t, m)
	pressEsc(m) // back to the harness screen
	pressEsc(m) // back to the mode screen, cursor on promote

	driveToPath(t, m, domain.ModeDeployWorkspace, "")

	lower := strings.ToLower(m.View())
	if m.screen != screenWorkspace || !strings.Contains(lower, "workspace") || strings.Contains(lower, "agents folder") {
		t.Errorf("path step after switching to deploy shows agents-folder wording:\n%s", m.View())
	}
}

func TestBackNavigation_DeployNewThenPromote_PathStepAsksForAnAgentsFolder(t *testing.T) {
	workspace := t.TempDir()
	m := newFlowModel(newPromoteFlowSvc(workspace), workspace)
	driveToPath(t, m, domain.ModeDeployWorkspace, "")
	pressEsc(m) // back to the harness screen
	pressEsc(m) // back to the mode screen

	driveToPath(t, m, domain.ModePromoteToGeneric, "")

	if m.screen != screenWorkspace || !strings.Contains(strings.ToLower(collapseSpaces(m.View())), "agents folder") {
		t.Errorf("path step after switching to promote does not ask for an agents folder:\n%s", m.View())
	}
}

// ---------------------------------------------------------------------------
// The batch operation
// ---------------------------------------------------------------------------

func TestPromoteMode_Run_CallsPromoteBatchWithHarnessAndFolder(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	if svc.promoteBatchReq == nil {
		t.Fatal("PromoteBatch was not called after the agents folder was confirmed")
	}
	if svc.promoteBatchReq.HarnessID != flowTestHarness.ID {
		t.Errorf("PromoteBatchRequest.HarnessID = %q; want the harness selected on the harness screen %q",
			svc.promoteBatchReq.HarnessID, flowTestHarness.ID)
	}
	if filepath.Clean(svc.promoteBatchReq.AgentsDir) != filepath.Clean(folder) {
		t.Errorf("PromoteBatchRequest.AgentsDir = %q; want the confirmed folder %q", svc.promoteBatchReq.AgentsDir, folder)
	}
}

func TestPromoteMode_Run_NeverCallsSinglePromote(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	if svc.promoteCalls != 0 {
		t.Errorf("Promote (single file) was called %d time(s); the TUI promote is the batch operation", svc.promoteCalls)
	}
	if svc.promoteBatchCalls != 1 {
		t.Errorf("PromoteBatch was called %d time(s); want exactly once", svc.promoteBatchCalls)
	}
}

func TestPromoteMode_RunningStatus_IsPromoteSpecific(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(folder), folder)
	selectPromoteMode(t, m)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRunning {
		t.Fatalf("screen = %v after confirming the agents folder; want screenRunning", m.screen)
	}
	if !strings.Contains(strings.ToLower(m.statusMsg), "promot") {
		t.Errorf("statusMsg = %q during the promote run; want text naming the promote", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// Outcome: batch summary, errors, cancel
// ---------------------------------------------------------------------------

func TestPromoteMode_Done_ShowsTheBatchSummaryOfEverySelectedAgent(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(folder), folder)

	runPromote(t, m)

	if m.screen != screenDone {
		t.Fatalf("screen = %v after the batch finished; want screenDone", m.screen)
	}
	view := collapseSpaces(m.View())
	for _, want := range []string{"Promote complete", "alpha-agent", "beta-agent", "promoted", "skipped",
		"kept the existing agent in Team Layer (team)", "1 promoted, 1 skipped, 0 failed, 0 not processed"} {
		if !strings.Contains(view, want) {
			t.Errorf("batch summary does not contain %q:\n%s", want, m.View())
		}
	}
}

func TestPromoteMode_Done_AbortedBatchShowsAbortedTitleAndNotProcessedAgents(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatch.Aborted = true
	svc.promoteBatch.Agents[1] = app.PromoteBatchAgentResult{
		SourcePath: filepath.Join(folder, "beta-agent.md"), Key: "beta-agent", Status: app.PromoteBatchNotProcessed,
	}
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	view := collapseSpaces(m.View())
	for _, want := range []string{"Promote aborted", "beta-agent", "not processed"} {
		if !strings.Contains(view, want) {
			t.Errorf("aborted batch summary does not contain %q:\n%s", want, m.View())
		}
	}
}

func TestPromoteMode_Done_OneAgentSelectionShowsThatAgentsResult(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatch.Agents = svc.promoteBatch.Agents[:1]
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	view := collapseSpaces(m.View())
	for _, want := range []string{"Promote complete", "alpha-agent", "/team/Subagents/Dev/alpha-agent.md", "42",
		"1 promoted, 0 skipped, 0 failed, 0 not processed"} {
		if !strings.Contains(view, want) {
			t.Errorf("summary of a one-agent selection does not contain %q:\n%s", want, m.View())
		}
	}
	if strings.Contains(view, "beta-agent") {
		t.Errorf("summary shows an agent that was not selected:\n%s", m.View())
	}
}

func TestPromoteMode_Done_AKeyReturnsToTheFirstScreenWithSelectionsCleared(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(folder), folder)
	runPromote(t, m)

	_, next := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	assertBackAtFirstScreen(t, m, next)
	if strings.Contains(m.View(), "alpha-agent") {
		t.Errorf("first screen still shows the previous batch result:\n%s", m.View())
	}
}

func TestPromoteMode_Done_ScrollKeyDoesNotLeaveALongSummary(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatch.Agents = nil
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf("agent-%02d", i)
		svc.promoteBatch.Agents = append(svc.promoteBatch.Agents, app.PromoteBatchAgentResult{
			SourcePath: filepath.Join(folder, key+".md"), Key: key, Status: app.PromoteBatchSkipped, Reason: "kept",
		})
	}
	m := newFlowModel(svc, folder)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	runPromote(t, m)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if m.screen != screenDone {
		t.Errorf("screen = %v after scroll keys on the batch summary; want screenDone", m.screen)
	}
	if n := strings.Count(m.View(), "\n") + 1; n > 14 {
		t.Errorf("summary view has %d lines on a terminal 14 high", n)
	}
}

func TestPromoteMode_Done_CtrlCQuits(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	m := newFlowModel(newPromoteFlowSvc(folder), folder)
	runPromote(t, m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !isQuitCmd(cmd) {
		t.Error("ctrl+c on the batch summary did not quit")
	}
}

func TestPromoteMode_PreRunError_ShowsTheErrorView(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatchErr = fmt.Errorf("%w in %s", app.ErrPromoteNoPromotableAgents, folder)
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	if m.screen != screenDone || m.runCancelled {
		t.Fatalf("screen = %v, runCancelled = %v; want the error view on screenDone", m.screen, m.runCancelled)
	}
	if !strings.Contains(m.View(), "no promotable agents found") {
		t.Errorf("error view does not show the error:\n%s", m.View())
	}
	_, next := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assertBackAtFirstScreen(t, m, next)
}

func TestPromoteMode_SelectionCancelled_ShowsTheCancelledView(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatchErr = fmt.Errorf("agent selection: %w", app.ErrSelectionCancelled)
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	if m.screen != screenDone || !m.runCancelled {
		t.Errorf("screen = %v, runCancelled = %v; want the cancelled view on screenDone", m.screen, m.runCancelled)
	}
	if m.errMsg != "" {
		t.Errorf("errMsg = %q; a cancelled selection must not be shown as a failure", m.errMsg)
	}
}

func TestPromoteMode_OtherErrorsAreNotShownAsCancelled(t *testing.T) {
	folder := createAgentsFolder(t, t.TempDir())
	svc := newPromoteFlowSvc(folder)
	svc.promoteBatchErr = errors.New("boom-from-the-batch")
	m := newFlowModel(svc, folder)

	runPromote(t, m)

	if m.runCancelled || !strings.Contains(m.View(), "boom-from-the-batch") {
		t.Errorf("runCancelled = %v; want the error view naming the error:\n%s", m.runCancelled, m.View())
	}
}

