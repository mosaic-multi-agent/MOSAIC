package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== Orchestrated mode - every routing decision from consultant =====

// TestIntegration_OrchestratedMode_AllRoutingFromConsultant verifies that a
// run in orchestrated mode routes every step through the routing consultant
// and never auto-routes via the engine.
//
// In orchestrated mode the engine never produces a routing decision: the
// consultant is invoked before every dispatch (including the very first step)
// and again after every completion to get the next instruction. A two-agent
// linear workflow therefore results in three consultant calls: initial,
// after agent-a, and after agent-b (which returns the final stop). Every call
// proves routing was consultant-driven.
func TestIntegration_OrchestratedMode_AllRoutingFromConsultant(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "linear-orch.md"))
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")

	f := harness.NewMockAdapter()
	f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-a#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "planning done",
	}})
	f.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#2",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "done",
	}})

	consultant := &intScriptedRoutingConsultant{}
	// linear-orch.md: row 0 = agent-a, row 1 = agent-b.
	// The consultant directs all three routing decisions:
	//   call 1 (initial): dispatch agent-a
	//   call 2 (after agent-a): dispatch agent-b
	//   call 3 (after agent-b): stop - the orchestrator signals all work is done
	consultant.queueDispatch("agent-a", "Proceed.", 0)
	consultant.queueDispatch("agent-b", "Proceed.", 1)
	consultant.queueStop("all workflow steps completed")

	artifactPath := filepath.Join(dir, "Orchestration.md")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	cfg := domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "linear",
		Task:                 "orchestrated task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	}

	got, err := sess.Start(context.Background(), cfg)

	// In orchestrated mode the consultant signals termination via a stop
	// instruction; the terminal status is RunStoppedByConsultant.
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

	// Every routing decision - including the initial one and the final stop -
	// must have come from the consultant: three calls for two workflow steps.
	if consultant.CallCount != 3 {
		t.Errorf("want 3 consultant calls (initial + after agent-a + after agent-b), got %d",
			consultant.CallCount)
	}

	// Consultations are visible through the consultant (three calls above) and
	// the diagnostic logs, not in the artifact. Read back the produced artifact
	// and confirm the Execution Log holds only the two workflow agents, with
	// consecutive Seq values and instance suffixes equal to their Seq: the
	// consultations consume no sequence slot and write no row.
	artifactData, readErr := os.ReadFile(artifactPath)
	if readErr != nil {
		t.Fatalf("read produced artifact: %v", readErr)
	}
	state, parseErr := artifact.Parse(artifactData)
	if parseErr != nil {
		t.Fatalf("parse produced artifact: %v", parseErr)
	}

	wantLog := []struct {
		agent string
		seq   int
	}{
		{"agent-a#1", 1}, {"agent-b#2", 2},
	}
	if len(state.ExecutionLog) != len(wantLog) {
		t.Fatalf("want exactly %d execution log entries (workflow agents only), got %d: %v",
			len(wantLog), len(state.ExecutionLog), state.ExecutionLog)
	}
	for i, w := range wantLog {
		e := state.ExecutionLog[i]
		if e.Agent != w.agent || e.Seq != w.seq {
			t.Errorf("execution log[%d]: want %s with Seq %d, got %q Seq %d", i, w.agent, w.seq, e.Agent, e.Seq)
		}
	}
	if state.GlobalSequence != 2 {
		t.Errorf("want global_sequence 2 (workflow agents only), got %d", state.GlobalSequence)
	}
	if state.CurrentState.LastAgent != "agent-b#2" {
		t.Errorf("want current_state.last_agent agent-b#2, got %q", state.CurrentState.LastAgent)
	}

	// Both workflow agents must have been dispatched, in order.
	invs := f.Invocations()
	if len(invs) != 2 {
		t.Fatalf("want 2 harness invocations (agent-a + agent-b), got %d", len(invs))
	}
	if invs[0].Agent.Identifier != "agent-a" {
		t.Errorf("want first dispatch to agent-a, got %q", invs[0].Agent.Identifier)
	}
	if invs[1].Agent.Identifier != "agent-b" {
		t.Errorf("want second dispatch to agent-b, got %q", invs[1].Agent.Identifier)
	}
}
