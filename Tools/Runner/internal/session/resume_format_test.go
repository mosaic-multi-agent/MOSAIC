package session_test

// Resume of a run artifact in the pre-3.0 form. An artifact with an Execution
// Log Stage column or a current_state.stage key, and a 3.0-layout artifact
// whose grouped row records the pre-3.0 "{Group}.{ID}" value, are refused
// before any dispatch, naming the run. A 3.0 artifact resumes, including one
// that records an ungrouped schedule ID such as "Stage-1" verbatim.
//
// The artifacts are written as text and read through the real artifact file
// store, so the layout is judged by what the parser reports.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const (
	rfGroupedWorkflow   = "notice-iteration-grouped"
	rfUngroupedWorkflow = "seeded-plan-first-block"
)

// rfLogRow is one Execution Log row of a resume artifact.
type rfLogRow struct {
	seq    int
	agent  string
	phase  string
	value  string // the recorded value cell, verbatim
	wfRow  int
	status string
}

// rfArtifact renders a run artifact of workflow with the given recorded-value
// column header ("Iteration" is the 3.0 layout, "Stage" the pre-3.0 one) and
// current_state key ("iteration" or "stage").
func rfArtifact(workflowID, column, stateKey, stateValue string, rows ...rfLogRow) string {
	last := rows[len(rows)-1]
	var b strings.Builder
	b.WriteString("---\n" +
		"type: orchestration-artifact\n" +
		"run_id: " + testRunID + "\n" +
		"workflow: " + workflowID + "\n" +
		"workflow_version: \"1.0\"\n" +
		"task: \"test task\"\n" +
		"started: 2026-01-01T00:00:00Z\n" +
		"last_updated: 2026-01-01T00:00:00Z\n" +
		fmt.Sprintf("global_sequence: %d\n", last.seq) +
		"runner_mode: auto\n" +
		"runner_pre_consultation: disabled\n" +
		"runner_manual_resolution: disabled\n" +
		"checkpoints: disabled\n" +
		"current_state:\n" +
		"  phase: " + last.phase + "\n" +
		"  " + stateKey + ": " + stateValue + "\n" +
		"  last_status: " + last.status + "\n" +
		"  last_agent: \"" + last.agent + "\"\n" +
		"  error_code: null\n" +
		"---\n\n" +
		"<ExecutionLog type=\"core\">\n\n" +
		"| Seq | Agent | Phase | " + column + " | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |\n" +
		"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %s | 2026-01-01T00:00:00Z | done | - | - |\n",
			r.seq, r.agent, r.phase, r.value, r.wfRow, r.status)
	}
	b.WriteString("\n</ExecutionLog>\n\n" +
		"<Artifacts type=\"core\">\n\n| Artifact | Created In | Created By |\n| --- | --- | --- |\n\n</Artifacts>\n\n" +
		"<WorkflowNotes type=\"core\">\n\n| Seq | Note |\n| --- | --- |\n\n</WorkflowNotes>\n")
	return b.String()
}

// rfUseFileStore replaces the rig's session by one on the real artifact file
// store at the run folder, and writes text as the run artifact.
func (r *blockRig) rfUseFileStore(t *testing.T, text string) {
	t.Helper()
	path := filepath.Join(r.runFolder, "Orchestration.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write run artifact: %v", err)
	}
	r.ses = session.New(session.Deps{
		Harness:  r.adapter,
		Store:    artifact.NewFileStore(path),
		Routing:  r.consult,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
}

// rfGroupedRig is the grouped-block fixture over an artifact text; Plan.md
// holds the iterations 1 and 2.
func rfGroupedRig(t *testing.T, text string) *blockRig {
	t.Helper()
	rig := newBlockRig(t, "notice-iteration-grouped-orch.md", nil, "test-writer", "tests-review", "reporter")
	plan := "# Plan\n\n## Stages\n\n| Stage | Name | Goal | HITL | Approach |\n|---|---|---|:---:|---|\n" +
		"| 1 | One | Goal | FALSE | Tests-Only |\n| 2 | Two | Goal | FALSE | Tests-Only |\n"
	rig.writeRunFile(t, "Plan.md", plan)
	rig.rfUseFileStore(t, text)
	return rig
}

// rfUngroupedRig is the ungrouped-block fixture over an artifact text; Plan.md
// holds the single iteration id.
func rfUngroupedRig(t *testing.T, id, text string) *blockRig {
	t.Helper()
	rig := newBlockRig(t, "seeded-plan-first-block-orch.md", nil, "implementer", "reviewer", "test-runner")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids(id)))
	rig.rfUseFileStore(t, text)
	return rig
}

// rfRefusedResume resumes the rig and requires the refusal of the resume: a
// refused outcome, no dispatch, and a message naming the run. It returns the
// outcome and the refusal carried as its cause.
func rfRefusedResume(t *testing.T, rig *blockRig, workflowID string) (domain.RunOutcome, *domain.RefusalError) {
	t.Helper()
	// Queue replies so that a dispatch would not fail for lack of a script.
	for _, a := range []string{"test-writer", "tests-review", "reporter", "implementer", "reviewer", "test-runner"} {
		rig.queueOKTimes(a, 2)
	}

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(workflowID, domain.ExecutionModeAuto))

	msg := requireRefused(t, got, err)
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	if !strings.Contains(msg, testRunID) {
		t.Errorf("want the refusal to name the run %q, got %q", testRunID, msg)
	}
	var refusal *domain.RefusalError
	if !errors.As(got.Cause, &refusal) {
		t.Fatalf("want the outcome cause to be a *domain.RefusalError, got %T: %v", got.Cause, got.Cause)
	}
	return got, refusal
}

func rfRow(seq int, agent, phase, value string, wfRow int) rfLogRow {
	return rfLogRow{seq: seq, agent: agent, phase: phase, value: value, wfRow: wfRow, status: "SUCCESS"}
}

func TestResume_ExecutionLogStageColumn_IsRefusedBeforeAnyDispatch(t *testing.T) {
	text := rfArtifact(rfUngroupedWorkflow, "Stage", "stage", "Stage-1",
		rfRow(1, "implementer#1", "EXECUTION", "Stage-1", 1))
	rig := rfUngroupedRig(t, "1", text)

	got, refusal := rfRefusedResume(t, rig, rfUngroupedWorkflow)

	if !errors.Is(got.Cause, artifact.ErrPre30Layout) {
		t.Errorf("want the refusal to be caused by artifact.ErrPre30Layout, got %v", got.Cause)
	}
	if refusal.Row != domain.NoWorkflowRow || refusal.Token != "" {
		t.Errorf("a layout refusal is about no row and no token: got row %d token %q", refusal.Row, refusal.Token)
	}
}

func TestResume_ExecutionLogStageColumnWithGroupedValues_IsRefusedBeforeAnyDispatch(t *testing.T) {
	text := rfArtifact(rfGroupedWorkflow, "Stage", "stage", "Test.1",
		rfRow(1, "test-writer#1", "EXECUTION", "Test.1", 1))
	rig := rfGroupedRig(t, text)

	got, _ := rfRefusedResume(t, rig, rfGroupedWorkflow)

	if !errors.Is(got.Cause, artifact.ErrPre30Layout) {
		t.Errorf("want the refusal to be caused by artifact.ErrPre30Layout, got %v", got.Cause)
	}
}

func TestResume_CurrentStateStageKeyBesideIterationColumn_IsRefusedBeforeAnyDispatch(t *testing.T) {
	text := rfArtifact(rfGroupedWorkflow, "Iteration", "stage", "1.Test",
		rfRow(1, "test-writer#1", "EXECUTION", "1.Test", 1))
	rig := rfGroupedRig(t, text)

	got, refusal := rfRefusedResume(t, rig, rfGroupedWorkflow)

	if !errors.Is(got.Cause, artifact.ErrPre30Layout) {
		t.Errorf("want the refusal to be caused by artifact.ErrPre30Layout, got %v", got.Cause)
	}
	if refusal.Row != domain.NoWorkflowRow || refusal.Token != "" {
		t.Errorf("a layout refusal is about no row and no token: got row %d token %q", refusal.Row, refusal.Token)
	}
}

func TestResume_GroupedRowRecordedGroupDotID_IsRefusedBeforeAnyDispatch(t *testing.T) {
	text := rfArtifact(rfGroupedWorkflow, "Iteration", "iteration", `"Test.1"`,
		rfRow(1, "test-writer#1", "EXECUTION", "Test.1", 1))
	rig := rfGroupedRig(t, text)

	got, refusal := rfRefusedResume(t, rig, rfGroupedWorkflow)

	var valueErr *domain.IterationValueError
	if !errors.As(got.Cause, &valueErr) {
		t.Fatalf("want the refusal caused by a *domain.IterationValueError, got %v", got.Cause)
	}
	if valueErr.Kind != domain.IterationValueMalformed || valueErr.Recorded != "Test.1" {
		t.Errorf("want a malformed %q, got kind %q recorded %q", "Test.1", valueErr.Kind, valueErr.Recorded)
	}
	if refusal.Row != domain.WorkflowRowFromIndex(0) {
		t.Errorf("want the refusal to name the recorded row 1, got row %d", refusal.Row)
	}
}

// The refusal is not about the last entry only: an earlier entry with a value
// its row does not accept refuses the resume as well.
func TestResume_EarlierEntryRecordedGroupDotID_IsRefusedBeforeAnyDispatch(t *testing.T) {
	text := rfArtifact(rfGroupedWorkflow, "Iteration", "iteration", `"1.Test"`,
		rfRow(1, "test-writer#1", "EXECUTION", "Test.1", 1),
		rfRow(2, "test-writer#2", "EXECUTION", "1.Test", 1))
	rig := rfGroupedRig(t, text)

	got, refusal := rfRefusedResume(t, rig, rfGroupedWorkflow)

	var valueErr *domain.IterationValueError
	if !errors.As(got.Cause, &valueErr) {
		t.Fatalf("want the refusal caused by a *domain.IterationValueError, got %v", got.Cause)
	}
	if valueErr.Recorded != "Test.1" {
		t.Errorf("want the refusal to carry the recorded value %q, got %q", "Test.1", valueErr.Recorded)
	}
	if refusal.Row != domain.WorkflowRowFromIndex(0) {
		t.Errorf("want the refusal to name the recorded row 1, got row %d", refusal.Row)
	}
}

func TestResume_ThreeOhArtifactGroupedRow_ResumesAfterTheRecordedRow(t *testing.T) {
	text := rfArtifact(rfGroupedWorkflow, "Iteration", "iteration", `"1.Test"`,
		rfRow(1, "test-writer#1", "EXECUTION", "1.Test", 1))
	rig := rfGroupedRig(t, text)
	// Iteration 1 continues with its review row; iteration 2 runs both rows.
	rig.queueOKTimes("tests-review", 2)
	rig.queueOK("test-writer")
	rig.queueOK("reporter")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(rfGroupedWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := rig.adapter.Invocations()
	if len(invs) == 0 || invs[0].Agent.Identifier != "tests-review" {
		t.Errorf("want the resume to continue with tests-review, got %v", invs)
	}
	if n := rig.invocationsOf("test-writer"); n != 1 {
		t.Errorf("want the recorded test-writer row of iteration 1 not dispatched again (iteration 2 only), got %d dispatches", n)
	}
}

// "Stage-1" is the schedule ID of an ungrouped block, recorded verbatim by the
// 3.0 layout: it resumes, at the iteration "Stage-1" and not at "1".
func TestResume_ThreeOhArtifactUngroupedStageDashID_ResumesAtThatIteration(t *testing.T) {
	text := rfArtifact(rfUngroupedWorkflow, "Iteration", "iteration", `"Stage-1"`,
		rfRow(1, "implementer#1", "EXECUTION", "Stage-1", 1))
	rig := rfUngroupedRig(t, "Stage-1", text)
	rig.queueOK("reviewer")
	rig.queueOK("test-runner")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(rfUngroupedWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := rig.invocationsOf("implementer"); n != 0 {
		t.Errorf("want the recorded implementer row not dispatched again, got %d dispatches", n)
	}
	requireSequence(t, "iterations the reviewer ran at",
		rig.dispatchedIterations("reviewer", "Stage-", "/review.md"), []string{"Stage-1"})
}
