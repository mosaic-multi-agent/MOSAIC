package session_test

// ITERATION_END and PHASE_END decided from routing. The step after which a
// boundary trigger fires is the one whose On Success ends its iteration or
// leaves its phase, not the one that happens to be last by table position. In
// Modes 2 and 3 the engine's routing decides it; in Mode 1 the table's declared
// routing does, whatever the consultant chooses next.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const (
	routePhaseAgent      = "phase-committer"
	routeCommitAgent     = "commit-manager-git"
	routeCheckpointAgent = "checkpoint-manager-git"
)

// routeSession builds a session over the named orchestrator fixture. Agent
// files are written for every name in agents, and plan, when not empty, is
// written as Plan.md next to the orchestrator file. The runs of routeConfig
// enable commits, so the fixture's first commit-class agent, commitAgent,
// answers commit setup first.
func routeSession(t *testing.T, fixture, plan, commitAgent string, consultant domain.RoutingConsultant, agents ...string) (
	ses session.Session, f *harness.MockAdapter, orchPath string,
) {
	t.Helper()
	dir := t.TempDir()
	orchPath = copyOrchestratorFile(t, dir, fixture)
	for _, a := range agents {
		writeAgentFile(t, dir, a)
	}
	if plan != "" {
		if err := os.WriteFile(filepath.Join(dir, "Plan.md"), []byte(plan), 0600); err != nil {
			t.Fatalf("write Plan.md: %v", err)
		}
	}
	f = harness.NewMockAdapter()
	f.Queue(commitAgent, commitSetupEntry())
	ses = session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Routing:  consultant,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	return
}

func routeConfig(orchPath, workflowID string, mode domain.ExecutionMode) domain.RunConfig {
	cfg := condConfig(orchPath, mode)
	cfg.WorkflowID = domain.WorkflowID(workflowID)
	return cfg
}

// ===== Modes 2 and 3 =====

// EXECUTION holds the Stage block and, after it, a run-scope row. The commit
// agent fires at the end of each iteration; the phase agent fires only after
// the run-scope row, not after the last row of the last iteration.
func TestSession_Start_PhaseEnd_BlockFollowedByRunScopeRowInSamePhase_FiresAfterTrailingRow(t *testing.T) {
	forEachSessionAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		ses, f, orchPath := routeSession(t, "phase-end-block-then-run-scope-orch.md", condTwoStagePlan, routeCommitAgent, nil,
			"implementation-tdd", "implementation-review", "wrap-up", routeCommitAgent, routeCheckpointAgent)
		for i := 0; i < 2; i++ {
			queueCond(f, "implementation-tdd", domain.StatusSUCCESS)
			queueCond(f, "implementation-review", domain.StatusSUCCESS)
			queueCond(f, routeCommitAgent, domain.StatusSUCCESS)
		}
		queueCond(f, "wrap-up", domain.StatusSUCCESS)
		queueCond(f, routeCheckpointAgent, domain.StatusSUCCESS)
		cfg := routeConfig(orchPath, "staged", mode)
		cfg.Checkpoints = true

		got, err := ses.Start(context.Background(), cfg)

		requireRunStatus(t, got, err, domain.RunCompleted)
		requireAgentSequence(t, invokedAgents(f), []string{
			"implementation-tdd", "implementation-review", routeCommitAgent,
			"implementation-tdd", "implementation-review", routeCommitAgent,
			"wrap-up", routeCheckpointAgent,
		})
	})
}

func forEachSessionAutoMode(t *testing.T, fn func(t *testing.T, mode domain.ExecutionMode)) {
	t.Helper()
	for _, mode := range []domain.ExecutionMode{domain.ExecutionModeAuto, domain.ExecutionModeAutoReview} {
		t.Run(string(mode), func(t *testing.T) { fn(t, mode) })
	}
}

// ===== Mode 1 =====

// The join's On Success leaves RESEARCH, so the phase agent fires after it,
// although the table carries one more RESEARCH row further down. The fork and
// the branch rows route inside RESEARCH and fire nothing.
func TestSession_Start_PhaseEndMode1_JoinLeavingPhase_FiresAfterJoinOnly(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	for i, agent := range []string{"scout", "research-a", "research-b", "merge", "reporter", "cleanup"} {
		consultant.queueDispatch(agent, "task", i)
	}
	consultant.queueStop("done")
	ses, f, orchPath := routeSession(t, "phase-end-fork-join-leaves-orch.md", "", routePhaseAgent, consultant,
		"scout", "research-a", "research-b", "merge", "reporter", "cleanup", routePhaseAgent)
	for _, agent := range []string{"scout", "research-a", "research-b", "merge"} {
		queueCond(f, agent, domain.StatusSUCCESS)
	}
	queueCond(f, routePhaseAgent, domain.StatusSUCCESS) // after merge
	queueCond(f, "reporter", domain.StatusSUCCESS)
	queueCond(f, routePhaseAgent, domain.StatusSUCCESS) // after reporter: routes to RESEARCH again
	queueCond(f, "cleanup", domain.StatusSUCCESS)
	queueCond(f, routePhaseAgent, domain.StatusSUCCESS) // after cleanup: COMPLETE

	got, err := ses.Start(context.Background(), routeConfig(orchPath, "fork-join-leaves", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

	requireAgentSequence(t, invokedAgents(f), []string{
		"scout", "research-a", "research-b", "merge", routePhaseAgent,
		"reporter", routePhaseAgent, "cleanup", routePhaseAgent,
	})
}

// The worker declares COMPLETE and so ends its iteration, although the checker
// is the last row of the scope by position.
func TestSession_Start_IterationEndMode1_DeclaredCompleteOnFirstRow_FiresAfterIt(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStagedDispatch("worker", "task", 0, "1")
	consultant.queueStop("done")
	ses, f, orchPath := routeSession(t, "iteration-end-declared-complete-orch.md", condOneStagePlan, routeCommitAgent, consultant,
		"worker", "checker", "reporter", routeCommitAgent)
	queueCond(f, "worker", domain.StatusSUCCESS)
	queueCond(f, routeCommitAgent, domain.StatusSUCCESS)

	got, err := ses.Start(context.Background(), routeConfig(orchPath, "declared-complete", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireAgentSequence(t, invokedAgents(f), []string{"worker", routeCommitAgent})
}

// The consultant sends the run on to the checker although the worker declared
// COMPLETE. ITERATION_END fired after the worker by its declared routing, and the
// checker, the last row of the scope with NEXT, ends the iteration once more.
func TestSession_Start_IterationEndMode1_ConsultedRouteDiffersFromDeclared_TriggersFollowDeclared(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStagedDispatch("worker", "task", 0, "1")
	consultant.queueStagedDispatch("checker", "task", 1, "1")
	consultant.queueStop("done")
	ses, f, orchPath := routeSession(t, "iteration-end-declared-complete-orch.md", condOneStagePlan, routeCommitAgent, consultant,
		"worker", "checker", "reporter", routeCommitAgent)
	queueCond(f, "worker", domain.StatusSUCCESS)
	queueCond(f, routeCommitAgent, domain.StatusSUCCESS)
	queueCond(f, "checker", domain.StatusSUCCESS)
	queueCond(f, routeCommitAgent, domain.StatusSUCCESS)

	got, err := ses.Start(context.Background(), routeConfig(orchPath, "declared-complete", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireAgentSequence(t, invokedAgents(f), []string{"worker", routeCommitAgent, "checker", routeCommitAgent})
}
