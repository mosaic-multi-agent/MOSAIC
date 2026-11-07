package artifact_test

// Tests for Apply of a force-stopped step on the real file store: the attempt
// is logged, but current_state and the artifact registry stay as the last
// accepted step left them.

import (
	"context"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

func TestApply_ForceStoppedStep_LogsRowButLeavesCurrentStateAndRegistry(t *testing.T) {
	store, state := mustCreateStore(t)
	ctx := context.Background()
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), []string{"Plan.md"})
	state1, err := store.Apply(ctx, state, accepted)
	if err != nil {
		t.Fatalf("Apply accepted: %v", err)
	}

	stopped := newTestStep(2, "reviewer#2", "EXECUTION", "1.Test", domain.StatusPARTIALLY_DONE, time.Now(),
		[]string{"Review.md"})
	stopped.Summary = domain.ForceStoppedSummary
	stopped.Inputs = "Plan.md"
	stopped.ForceStopped = true
	after, err := store.Apply(ctx, state1, stopped)
	if err != nil {
		t.Fatalf("Apply force-stopped: %v", err)
	}

	if after.CurrentState != state1.CurrentState {
		t.Errorf("current_state must stay the accepted step's: want %+v, got %+v",
			state1.CurrentState, after.CurrentState)
	}
	if after.CurrentState.LastAgent != "planner#1" || after.CurrentState.Phase != "PLANNING" ||
		after.CurrentState.LastStatus != domain.StatusSUCCESS {
		t.Errorf("current_state not the accepted step's: %+v", after.CurrentState)
	}
	if rows := registryRowsFor(after, "Review.md"); len(rows) != 0 {
		t.Errorf("a force-stopped step must register no artifacts, got %v", rows)
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
	assertForceStoppedRow(t, after.ExecutionLog[1])
}

func TestApply_ForceStoppedStep_RowAndCurrentStateSurviveReload(t *testing.T) {
	store, state := mustCreateStore(t)
	ctx := context.Background()
	accepted := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, time.Now(), nil)
	state1, err := store.Apply(ctx, state, accepted)
	if err != nil {
		t.Fatalf("Apply accepted: %v", err)
	}
	stopped := newTestStep(2, "reviewer#2", "EXECUTION", "1.Test", domain.StatusPARTIALLY_DONE, time.Now(),
		[]string{"Review.md"})
	stopped.Summary = domain.ForceStoppedSummary
	stopped.Inputs = "Plan.md"
	stopped.ForceStopped = true
	if _, err := store.Apply(ctx, state1, stopped); err != nil {
		t.Fatalf("Apply force-stopped: %v", err)
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
	assertForceStoppedRow(t, onDisk.ExecutionLog[1])
}

func assertForceStoppedRow(t *testing.T, row domain.ExecutionLogEntry) {
	t.Helper()
	if row.Agent != "reviewer#2" || row.Seq != 2 {
		t.Errorf("row identity: got seq %d agent %q", row.Seq, row.Agent)
	}
	if row.Status != domain.StatusPARTIALLY_DONE {
		t.Errorf("row status: want PARTIALLY_DONE, got %v", row.Status)
	}
	if row.Summary != domain.ForceStoppedSummary {
		t.Errorf("row summary: want %q, got %q", domain.ForceStoppedSummary, row.Summary)
	}
	if !row.IsForceStopped() {
		t.Errorf("row must be recognised as force-stopped: %+v", row)
	}
	if row.Inputs != "Plan.md" {
		t.Errorf("row inputs must be preserved: got %q", row.Inputs)
	}
}
