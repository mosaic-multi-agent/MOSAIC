package integration_test

// Golden case of the harness-error incident: a subagent response with a stray
// `,"` before the closing brace is not valid protocol JSON. The Runner records
// HARNESS_ERROR for it, never BLOCKED, through the full stack with the real
// file-based artifact store and the real classification of a malformed
// response.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// incidentResponse is the response of the incident run: the agent claims
// COMPLETED_NEEDS_ACTION, but a stray `,"` before the closing brace makes the
// payload invalid JSON.
const incidentResponse = `{"agent_instance_id":"contracts-designer#22","run_id":"20261107T211627Z-8fe7",` +
	`"status_code":"COMPLETED_NEEDS_ACTION","status_message":"Contracts design written. human_approved is true.","}`

const harnessErrorOrch = `<Workflow type="core" name="harness-error" version="1.0">
## Harness Error Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | agent-a | FALSE | agent-b | - | - | plan.md |
| PLANNING | agent-b | FALSE | COMPLETE | - | plan.md | result.md |
</Workflow>
`

func harnessErrorConfig(orchPath string) domain.RunConfig {
	return domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "harness-error",
		Task:                 "task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
	}
}

func okEntry(msg string) harness.ScriptedEntry {
	return harness.ScriptedEntry{Response: &domain.ProtocolResponse{StatusCode: domain.StatusSUCCESS, StatusMessage: msg}}
}

// readLog returns the Execution Log read back from the artifact file.
func readLog(t *testing.T, artifactPath string) []domain.ExecutionLogEntry {
	t.Helper()
	state, err := artifact.NewFileStore(artifactPath).Read(context.Background())
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	return state.ExecutionLog
}

func TestIntegration_HarnessError_IncidentResponse_IsRecordedAsHarnessErrorNotBlocked(t *testing.T) {
	dir := t.TempDir()
	orchPath := writeOrchFile(t, dir, "orchestrator.md", harnessErrorOrch)
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	artifactPath := filepath.Join(dir, "Orchestration.md")
	f := harness.NewMockAdapter()
	// The malformed payload goes through the adapter's own classification. The
	// raw-text bypass re-dispatches and the agent answers properly.
	f.Queue("agent-a", harness.ScriptedEntry{RawJSON: []byte(incidentResponse)}, okEntry("planned"))
	f.Queue("agent-b", okEntry("done"))
	sess := newSessionWithRouting(f, artifactPath, &intScriptedRoutingConsultant{})

	got, err := sess.Start(context.Background(), harnessErrorConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	log := readLog(t, artifactPath)
	if len(log) < 1 {
		t.Fatal("want an Execution Log with the failed attempt, got none")
	}
	first := log[0]
	if first.Status != domain.StatusHARNESS_ERROR {
		t.Errorf("want the incident response recorded as HARNESS_ERROR, got %s", first.Status)
	}
	for _, e := range log {
		if e.Status == domain.StatusBLOCKED {
			t.Errorf("want no BLOCKED row for a malformed response, got %+v", e)
		}
	}
	if !strings.HasPrefix(first.Summary, "unparseable-response") {
		t.Errorf("want the Summary to name the unparseable response, got %q", first.Summary)
	}
	if strings.Contains(first.Summary, "COMPLETED_NEEDS_ACTION") {
		t.Errorf("want the agent's claimed status kept out of the Summary, got %q", first.Summary)
	}
}

// With every attempt malformed the run ends in a deviation, and the artifact
// holds only HARNESS_ERROR rows for the step: the workflow position never moved.
func TestIntegration_HarnessError_IncidentResponseOnEveryAttempt_NeverMovesCurrentState(t *testing.T) {
	dir := t.TempDir()
	orchPath := writeOrchFile(t, dir, "orchestrator.md", harnessErrorOrch)
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	artifactPath := filepath.Join(dir, "Orchestration.md")
	f := harness.NewMockAdapter()
	for i := 0; i < 6; i++ {
		f.Queue("agent-a", harness.ScriptedEntry{RawJSON: []byte(incidentResponse)})
	}
	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("stop after the failures")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	got, err := sess.Start(context.Background(), harnessErrorConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	state, readErr := artifact.NewFileStore(artifactPath).Read(context.Background())
	if readErr != nil {
		t.Fatalf("read artifact: %v", readErr)
	}
	if len(state.ExecutionLog) == 0 {
		t.Fatal("want HARNESS_ERROR rows in the Execution Log, got none")
	}
	for _, e := range state.ExecutionLog {
		if e.Status != domain.StatusHARNESS_ERROR {
			t.Errorf("want only HARNESS_ERROR rows, got %s for %s", e.Status, e.Agent)
		}
	}
	if cs := state.CurrentState; cs.LastAgent != "" || cs.LastStatus != "" {
		t.Errorf("want current_state untouched, got last_agent=%q last_status=%q", cs.LastAgent, cs.LastStatus)
	}
	if consultant.CallCount != 1 {
		t.Fatalf("want one consultation once the budget is used up, got %d", consultant.CallCount)
	}
	if dev := consultant.Requests[0].Deviation; dev == nil || dev.Response.StatusCode != domain.StatusHARNESS_ERROR {
		t.Errorf("want the consultation to carry a HARNESS_ERROR deviation, got %+v", dev)
	}
}
