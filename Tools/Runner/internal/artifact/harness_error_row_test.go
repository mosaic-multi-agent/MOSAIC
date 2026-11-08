package artifact_test

// Tests for HARNESS_ERROR rows in the artifact: applying one appends the
// Execution Log row but leaves current_state and the artifact registry as the
// last accepted step left them, the row renders and parses back, and the status
// parses without error in the Status column and in current_state.last_status.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const harnessErrorSummary = "timeout: harness invocation timed out"

func harnessErrorStep(seq int, agent string) domain.CompletedStep {
	step := newTestStep(seq, agent, "EXECUTION", "1.Test", domain.StatusHARNESS_ERROR,
		time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), []string{"Review.md"})
	step.Summary = harnessErrorSummary
	step.Inputs = "Plan.md"
	return step
}

func TestApply_HarnessErrorStep_LogsRowButLeavesCurrentStateAndRegistry(t *testing.T) {
	store, state := mustCreateStore(t)
	ctx := context.Background()
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), []string{"Plan.md"})
	state1, err := store.Apply(ctx, state, accepted)
	if err != nil {
		t.Fatalf("Apply accepted: %v", err)
	}

	after, err := store.Apply(ctx, state1, harnessErrorStep(2, "reviewer#2"))
	if err != nil {
		t.Fatalf("Apply HARNESS_ERROR: %v", err)
	}

	if after.CurrentState != state1.CurrentState {
		t.Errorf("current_state must stay the accepted step's: want %+v, got %+v", state1.CurrentState, after.CurrentState)
	}
	if rows := registryRowsFor(after, "Review.md"); len(rows) != 0 {
		t.Errorf("a HARNESS_ERROR step must register no artifacts, got %v", rows)
	}
	if len(after.ArtifactRegistry) != 1 {
		t.Errorf("registry must keep only the accepted step's row, got %v", after.ArtifactRegistry)
	}
	if after.GlobalSequence != 2 {
		t.Errorf("sequence must advance: want 2, got %d", after.GlobalSequence)
	}
	if len(after.ExecutionLog) != 2 {
		t.Fatalf("log rows: want 2, got %d", len(after.ExecutionLog))
	}
	assertHarnessErrorRow(t, after.ExecutionLog[1])
}

func TestApply_HarnessErrorStep_RowAndCurrentStateSurviveReload(t *testing.T) {
	store, state := mustCreateStore(t)
	ctx := context.Background()
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), nil)
	state1, err := store.Apply(ctx, state, accepted)
	if err != nil {
		t.Fatalf("Apply accepted: %v", err)
	}
	if _, err := store.Apply(ctx, state1, harnessErrorStep(2, "reviewer#2")); err != nil {
		t.Fatalf("Apply HARNESS_ERROR: %v", err)
	}

	onDisk, err := store.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if onDisk.CurrentState != state1.CurrentState {
		t.Errorf("current_state on disk: want %+v, got %+v", state1.CurrentState, onDisk.CurrentState)
	}
	if len(onDisk.ArtifactRegistry) != 0 {
		t.Errorf("registry on disk must be empty, got %v", onDisk.ArtifactRegistry)
	}
	if len(onDisk.ExecutionLog) != 2 {
		t.Fatalf("log rows on disk: want 2, got %d", len(onDisk.ExecutionLog))
	}
	assertHarnessErrorRow(t, onDisk.ExecutionLog[1])
}

// The run's very first dispatch failed: no step was accepted, so current_state
// stays empty.
func TestApply_HarnessErrorStep_FirstDispatch_LeavesCurrentStateEmpty(t *testing.T) {
	store, state := mustCreateStore(t)

	after, onDisk := applyAndRead(t, store, state, harnessErrorStep(1, "reviewer#1"))

	if after.CurrentState != state.CurrentState || onDisk.CurrentState != state.CurrentState {
		t.Errorf("current_state must stay empty: returned %+v, on disk %+v", after.CurrentState, onDisk.CurrentState)
	}
	if onDisk.CurrentState.LastAgent != "" || onDisk.CurrentState.LastStatus != "" {
		t.Errorf("current_state must name no step, got %+v", onDisk.CurrentState)
	}
	if len(onDisk.ExecutionLog) != 1 {
		t.Fatalf("log rows on disk: want 1, got %d", len(onDisk.ExecutionLog))
	}
}

func TestApply_HarnessErrorInfrastructureStep_LogsRowButLeavesCurrentState(t *testing.T) {
	store, state := mustCreateStore(t)
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), nil)
	state1, _ := applyAndRead(t, store, state, accepted)
	step := harnessErrorStep(2, "checkpoint-manager-git#2")
	step.IsInfrastructure = true

	after, onDisk := applyAndRead(t, store, state1, step)

	if after.CurrentState != state1.CurrentState || onDisk.CurrentState != state1.CurrentState {
		t.Errorf("current_state must stay the accepted step's: returned %+v, on disk %+v",
			after.CurrentState, onDisk.CurrentState)
	}
	if len(onDisk.ExecutionLog) != 2 || onDisk.ExecutionLog[1].Status != domain.StatusHARNESS_ERROR {
		t.Errorf("want the HARNESS_ERROR row logged, got %+v", onDisk.ExecutionLog)
	}
}

func assertHarnessErrorRow(t *testing.T, row domain.ExecutionLogEntry) {
	t.Helper()
	if row.Agent != "reviewer#2" || row.Seq != 2 {
		t.Errorf("row identity: got seq %d agent %q", row.Seq, row.Agent)
	}
	if row.Status != domain.StatusHARNESS_ERROR {
		t.Errorf("row status: want HARNESS_ERROR, got %v", row.Status)
	}
	if row.Summary != harnessErrorSummary {
		t.Errorf("row summary: want %q, got %q", harnessErrorSummary, row.Summary)
	}
	if row.ErrorCode != domain.ErrorNone {
		t.Errorf("a HARNESS_ERROR row carries no error code, got %q", row.ErrorCode)
	}
	if row.Inputs != "Plan.md" {
		t.Errorf("row inputs must be preserved: got %q", row.Inputs)
	}
}

func TestRender_HarnessErrorRow_HoldsStatusVerbatimAndNoErrorMarker(t *testing.T) {
	store, path, state := newMarkerStore(t)
	step := harnessErrorStep(1, "builder#1")
	step.ErrorCode = domain.ErrorNone

	applyAndRead(t, store, state, step)

	raw := readFile(t, path)
	if !strings.Contains(raw, "| HARNESS_ERROR |") {
		t.Errorf("Status cell must hold HARNESS_ERROR verbatim:\n%s", raw)
	}
	if strings.Contains(raw, "[error:") {
		t.Errorf("a HARNESS_ERROR row must carry no [error:CODE] marker:\n%s", raw)
	}
	if cell := summaryCell(t, raw, "1"); cell != harnessErrorSummary {
		t.Errorf("Summary cell: want %q, got %q", harnessErrorSummary, cell)
	}
}

func TestRenderParse_HarnessErrorRow_RoundTripsLosslessly(t *testing.T) {
	store, path, state := newMarkerStore(t)
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), nil)
	state1, _ := applyAndRead(t, store, state, accepted)
	applyAndRead(t, store, state1, harnessErrorStep(2, "reviewer#2"))
	first := []byte(readFile(t, path))

	parsed, err := artifact.Parse(first)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	second, err := artifact.Render(parsed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("render(parse(file)) must reproduce the file.\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	assertHarnessErrorRow(t, parsed.ExecutionLog[1])
	if parsed.CurrentState.LastAgent != "planner#1" {
		t.Errorf("parsed current_state must still name the accepted step, got %+v", parsed.CurrentState)
	}
}

func TestParse_HarnessErrorInStatusColumn_DoesNotFail(t *testing.T) {
	raw := strings.Replace(string(minimalArtifactWithExecutionRow("")), "| SUCCESS |", "| HARNESS_ERROR |", 1)

	state, err := artifact.Parse([]byte(raw))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := state.ExecutionLog[0].Status; got != domain.StatusHARNESS_ERROR {
		t.Errorf("row status: want HARNESS_ERROR, got %q", got)
	}
}

func TestParse_HarnessErrorInCurrentStateLastStatus_DoesNotFail(t *testing.T) {
	raw := strings.Replace(string(minimalArtifactWithExecutionRow("")), "last_status: SUCCESS", "last_status: HARNESS_ERROR", 1)

	state, err := artifact.Parse([]byte(raw))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := state.CurrentState.LastStatus; got != domain.StatusHARNESS_ERROR {
		t.Errorf("current_state.last_status: want HARNESS_ERROR, got %q", got)
	}
}
