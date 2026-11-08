package integration_test

// Resume after harness failures, through the full stack with the real
// file-based artifact store: a log that ends in a HARNESS_ERROR row resumes by
// re-dispatching the same assignment and a later failure is bounded by the
// budget the log holds; a log written before HARNESS_ERROR existed (synthetic
// BLOCKED rows with a "harness:" Summary and the [error:E501] marker) still
// resumes, its rows count against the E501 budget and are not rewritten.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
)

const (
	// resumeStateNone is a current_state that no accepted step has moved.
	resumeStateNone = "  phase: null\n  iteration: null\n  last_status: null\n  last_agent: null\n  error_code: null\n"

	// resumeStateOldSynthetic is the current_state an old synthetic BLOCKED/E501
	// row left behind.
	resumeStateOldSynthetic = "  phase: PLANNING\n  iteration: null\n  last_status: BLOCKED\n  last_agent: \"agent-a#2\"\n  error_code: E501\n"

	resumeTimestamp = "2026-01-01T00:00:00Z"
)

var errHarnessDown = errors.New("harness down")

// resumeRow renders one Execution Log row for agent-a at workflow row 1.
func resumeRow(seq int, status, summary string) string {
	return "| " + strconv.Itoa(seq) + " | agent-a#" + strconv.Itoa(seq) + " | PLANNING | - | 1 | " + status +
		" | " + resumeTimestamp + " | " + summary + " | - | - |\n"
}

// harnessResumeArtifact renders a resumable Orchestration.md of the linear
// workflow in auto mode with the given current_state block and log rows.
func harnessResumeArtifact(currentState string, rows ...string) string {
	return "---\ntype: orchestration-artifact\nrun_id: " + integrationRunID + "\nworkflow: linear\n" +
		"workflow_version: \"1.0\"\ntask: \"resumed task\"\nstarted: " + resumeTimestamp + "\n" +
		"last_updated: " + resumeTimestamp + "\nglobal_sequence: " + strconv.Itoa(len(rows)) + "\n" +
		"runner_mode: auto\nrunner_pre_consultation: disabled\nrunner_manual_resolution: disabled\n" +
		"checkpoints: disabled\ncommits: disabled\ncurrent_state:\n" + currentState + "---\n\n" +
		"<ExecutionLog type=\"core\">\n\n" +
		"| Seq | Agent | Phase | Iteration | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |\n" +
		"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n" + strings.Join(rows, "") +
		"\n</ExecutionLog>\n\n" +
		"<Artifacts type=\"core\">\n\n| Artifact | Created In | Created By |\n| --- | --- | --- |\n\n</Artifacts>\n\n" +
		"<WorkflowNotes type=\"core\">\n\n| Seq | Note |\n| --- | --- |\n\n</WorkflowNotes>\n"
}

// resumeRig is a resumable run of the linear workflow.
type resumeRig struct {
	adapter      *harness.MockAdapter
	cfg          domain.RunConfig
	artifactPath string
}

func newResumeRig(t *testing.T, artifactText string) *resumeRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md", filepath.Join(sessionTestdataDir, "linear-orch.md"))
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	runFolder := scopedRunFolder(t, dir)
	artifactPath := filepath.Join(runFolder, "Orchestration.md")
	if err := os.WriteFile(artifactPath, []byte(artifactText), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return &resumeRig{
		adapter:      harness.NewMockAdapter(),
		artifactPath: artifactPath,
		cfg: domain.RunConfig{
			RunID:                integrationRunID,
			OrchestratorFilePath: orchPath,
			WorkflowID:           "linear",
			Task:                 "resumed task",
			IsNewRun:             false,
			RunFolder:            runFolder,
		},
	}
}

func (r *resumeRig) logAfterRun(t *testing.T) []domain.ExecutionLogEntry {
	t.Helper()
	return readLog(t, r.artifactPath)
}

// requireStoppedAfterOneConsultation asserts the run ended through the
// consultant's stop after exactly one consultation, and returns its request.
func requireStoppedAfterOneConsultation(t *testing.T, got domain.RunOutcome, err error, c *intScriptedRoutingConsultant) domain.ConsultationRequest {
	t.Helper()
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if c.CallCount != 1 {
		t.Fatalf("want exactly one consultation once the budget is used up, got %d", c.CallCount)
	}
	return c.Requests[0]
}

func TestIntegration_Resume_TrailingHarnessErrorRow_RedispatchesSameAssignment(t *testing.T) {
	r := newResumeRig(t, harnessResumeArtifact(resumeStateNone,
		resumeRow(1, "HARNESS_ERROR", "timeout: subprocess timed out")))
	r.adapter.Queue("agent-a", okEntry("planned"))
	r.adapter.Queue("agent-b", okEntry("done"))

	got, err := newSession(r.adapter, r.artifactPath).Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.adapter.Invocations()
	if len(invs) != 2 || invs[0].Agent.Identifier != "agent-a" || invs[1].Agent.Identifier != "agent-b" {
		t.Fatalf("want agent-a re-dispatched, then agent-b, got %d invocations: %+v", len(invs), invs)
	}
	if id := invs[0].Request.AgentInstanceID; id != "agent-a#2" {
		t.Errorf("want the re-dispatch under the next sequence (agent-a#2), got %q", id)
	}
	log := r.logAfterRun(t)
	if len(log) < 2 || log[0].Status != domain.StatusHARNESS_ERROR || log[0].Agent != "agent-a#1" {
		t.Fatalf("want the recorded HARNESS_ERROR row kept first in the log, got %+v", log)
	}
	if log[1].Agent != "agent-a#2" || log[1].Status != domain.StatusSUCCESS {
		t.Errorf("want the re-dispatch appended as agent-a#2 SUCCESS, got %+v", log[1])
	}
}

// The budget is read from the whole log: the pre-resume HARNESS_ERROR row spends
// one of the three attempts, so the resume re-dispatch and one mechanical
// re-dispatch are all that remain before the deviation.
func TestIntegration_Resume_TrailingHarnessErrorRow_FurtherFailuresBoundedByLoggedBudget(t *testing.T) {
	r := newResumeRig(t, harnessResumeArtifact(resumeStateNone,
		resumeRow(1, "HARNESS_ERROR", "timeout: subprocess timed out")))
	queueFailures(r.adapter, "agent-a", 6)
	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("budget used up")

	got, err := newSessionWithRouting(r.adapter, r.artifactPath, consultant).Start(context.Background(), r.cfg)

	req := requireStoppedAfterOneConsultation(t, got, err, consultant)
	if want := engine.HarnessErrorAttemptLimit - 1; len(r.adapter.Invocations()) != want {
		t.Errorf("want %d agent-a dispatches after the resume (one attempt was already logged), got %d",
			want, len(r.adapter.Invocations()))
	}
	if req.LastStatus == nil || *req.LastStatus != domain.StatusHARNESS_ERROR {
		t.Errorf("want the consultation to report HARNESS_ERROR, got %v", req.LastStatus)
	}
	if req.LastErrorReason == nil || !strings.Contains(*req.LastErrorReason, "harness down") {
		t.Errorf("want the failure description in last_error_reason, got %v", req.LastErrorReason)
	}
}

// With the budget already spent in the log, the resume still re-dispatches the
// failed assignment once; its failure goes straight to the consultation.
func TestIntegration_Resume_TrailingHarnessErrorRowsBudgetSpent_RedispatchesOnceThenConsults(t *testing.T) {
	r := newResumeRig(t, harnessResumeArtifact(resumeStateNone,
		resumeRow(1, "HARNESS_ERROR", "timeout: subprocess timed out"),
		resumeRow(2, "HARNESS_ERROR", "timeout: subprocess timed out"),
		resumeRow(3, "HARNESS_ERROR", "timeout: subprocess timed out")))
	queueFailures(r.adapter, "agent-a", 6)
	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("budget used up")

	got, err := newSessionWithRouting(r.adapter, r.artifactPath, consultant).Start(context.Background(), r.cfg)

	req := requireStoppedAfterOneConsultation(t, got, err, consultant)
	if n := len(r.adapter.Invocations()); n != 1 {
		t.Errorf("want exactly one agent-a dispatch after the resume, got %d", n)
	}
	if req.LastStatus == nil || *req.LastStatus != domain.StatusHARNESS_ERROR {
		t.Errorf("want the consultation to report HARNESS_ERROR, got %v", req.LastStatus)
	}
}

// A log from before HARNESS_ERROR existed resumes; the two synthetic rows spend
// two of the three E501 attempts, and they stay exactly as written.
func TestIntegration_Resume_OldSyntheticE501Rows_CountAgainstE501BudgetAndStayUnrewritten(t *testing.T) {
	const synthetic = "harness: subprocess timed out [error:E501]"
	r := newResumeRig(t, harnessResumeArtifact(resumeStateOldSynthetic,
		resumeRow(1, "BLOCKED", synthetic),
		resumeRow(2, "BLOCKED", synthetic)))
	for i := 0; i < 6; i++ {
		r.adapter.Queue("agent-a", e501ScriptedEntry())
	}
	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("budget used up")

	got, err := newSessionWithRouting(r.adapter, r.artifactPath, consultant).Start(context.Background(), r.cfg)

	req := requireStoppedAfterOneConsultation(t, got, err, consultant)
	if want := engine.E501AttemptLimit - 2; len(r.adapter.Invocations()) != want {
		t.Errorf("want %d agent-a dispatches after the resume (two E501 attempts were already logged), got %d",
			want, len(r.adapter.Invocations()))
	}
	if req.LastStatus == nil || *req.LastStatus != domain.StatusBLOCKED {
		t.Errorf("want the consultation to report BLOCKED (the agent's own E501), got %v", req.LastStatus)
	}
	state, readErr := artifact.NewFileStore(r.artifactPath).Read(context.Background())
	if readErr != nil {
		t.Fatalf("read artifact: %v", readErr)
	}
	for i := 0; i < 2; i++ {
		e := state.ExecutionLog[i]
		if e.Status != domain.StatusBLOCKED || e.ErrorCode != domain.ErrorTOOL_UNAVAILABLE ||
			!strings.HasPrefix(e.Summary, "harness:") {
			t.Errorf("want old row %d left as BLOCKED/E501 with its harness: Summary, got %+v", i+1, e)
		}
	}
}

// queueFailures scripts n harness failures for agent.
func queueFailures(f *harness.MockAdapter, agent string, n int) {
	for i := 0; i < n; i++ {
		f.Queue(agent, harness.ScriptedEntry{Err: errHarnessDown})
	}
}

// e501ScriptedEntry is a subagent-returned BLOCKED response with error code E501.
func e501ScriptedEntry() harness.ScriptedEntry {
	return harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		StatusCode:    domain.StatusBLOCKED,
		ErrorCode:     domain.ErrorTOOL_UNAVAILABLE,
		StatusMessage: "tool unavailable",
		ErrorReason:   "tool unavailable",
	}}
}
