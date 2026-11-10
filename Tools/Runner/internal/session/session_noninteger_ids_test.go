package session_test

// Session-level test that a run whose seeded Plan.md uses non-numeric
// iteration IDs dispatches every iteration, in schedule order.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

func TestSession_Start_NonNumericIterationIDs_DispatchesEveryIterationInScheduleOrder(t *testing.T) {
	dir := scopedTempDir(t)
	orchPath := copyOrchestratorFile(t, dir, "staged-orch.md")
	writeAgentFile(t, dir, "implementation-tdd")
	writeAgentFile(t, dir, "implementation-review")
	plan := `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL |
|-------|------|------|------------|:----:|
| A | Alpha | First | - | FALSE |
| b-2 | Beta | Second | A | FALSE |
| C | Gamma | Third | b-2 | FALSE |
`
	if err := os.WriteFile(filepath.Join(dir, "Plan.md"), []byte(plan), 0600); err != nil {
		t.Fatalf("write Plan.md: %v", err)
	}

	f := harness.NewMockAdapter()
	store := &memStore{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    store,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	// Six dispatches: implementation-tdd then implementation-review per iteration.
	seq := 1
	for i := 0; i < 3; i++ {
		queueSuccess(f, "implementation-tdd", "implementation-tdd#"+strconv.Itoa(seq))
		queueSuccess(f, "implementation-review", "implementation-review#"+strconv.Itoa(seq+1))
		seq += 2
	}
	cfg := domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "staged",
		Task:                 "task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		RunFolder:            dir,
	}

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	steps := workflowSteps(store)
	wantIDs := []string{"A", "A", "b-2", "b-2", "C", "C"}
	wantAgents := []string{"implementation-tdd", "implementation-review"}
	if len(steps) != len(wantIDs) {
		t.Fatalf("want %d workflow steps (two rows per iteration), got %d: %+v", len(wantIDs), len(steps), steps)
	}
	for i, step := range steps {
		if step.Stage != wantIDs[i] {
			t.Errorf("step %d (%s): want recorded iteration %q, got %q", i, step.AgentInstance, wantIDs[i], step.Stage)
		}
		if want := wantAgents[i%2]; len(step.AgentInstance) < len(want) || step.AgentInstance[:len(want)] != want {
			t.Errorf("step %d: want agent %q, got %q", i, want, step.AgentInstance)
		}
	}
}
