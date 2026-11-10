package testrun_test

import (
	"context"
	"testing"

	"mosaic-run/internal/testcatalog"
	"mosaic-run/internal/testrun"
)

// TestRun_ChildStderr_ReachesCheckerInput verifies that the child's stderr is
// handed to the checker, so a stderr assertion in the expected outcome can be
// evaluated.
func TestRun_ChildStderr_ReachesCheckerInput(t *testing.T) {
	entry := testcatalog.CatalogEntry{
		WorkflowID:  "preconsult-stop",
		Mode:        "auto",
		FixturePath: "/f/preconsult-stop",
	}
	cat := &MockCatalog{
		smokeSet:    []testcatalog.CatalogEntry{entry},
		workflowIDs: []string{"preconsult-stop"},
	}
	wantStderr := "consultant stop: PRECONSULT-STOP-MARKER\n"
	inv := &MockRunInvoker{
		results: []invokeResult{
			{
				exitCode:        6,
				runFolder:       "/workspace/Orchestration-run-1",
				dispatchLogPath: "/workspace/RunnerLogs/run-1/run-1-dispatch.log",
				childStderr:     []byte(wantStderr),
			},
		},
	}
	chk := &MockChecker{}

	o := newOrchestrator(cat, &MockDeployer{}, inv, chk, &MockReporter{})
	_, err := o.Run(context.Background(), testrun.TestConfig{
		Scope:     testrun.ScopeSmoke,
		Harnesses: []string{"auto"},
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(chk.checkInputs) != 1 {
		t.Fatalf("Check called %d times, want 1", len(chk.checkInputs))
	}
	if got := chk.checkInputs[0].Stderr; got != wantStderr {
		t.Errorf("CheckInput.Stderr = %q, want %q", got, wantStderr)
	}
	if got := chk.checkInputs[0].ExitCode; got != 6 {
		t.Errorf("CheckInput.ExitCode = %d, want 6", got)
	}
}
