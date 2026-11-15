package main

// The run-lifecycle decorator writes run_start and run_end for a Codex run like
// for every harness, and nothing writes invocation events for Codex.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

func TestRunLifecycle_Codex_WritesRunStartAndRunEndWithHarnessCodex(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	sess := newRunLifecycleSession(&mlSession{}, mlLogConfig(runFolder, "codex"))

	_, _ = sess.Start(context.Background(), domain.RunConfig{})

	events := mlReadEvents(t, mlRunLogPath(ws))
	mlAssertPairs(t, events, 1)
	for _, ev := range events {
		if ev["harness"] != "codex" || ev["run_id"] != mlRunID {
			t.Errorf("envelope wrong for a codex run: %v", ev)
		}
	}
	if events[0]["cwd"] != ws {
		t.Errorf("run_start.cwd = %v, want %q", events[0]["cwd"], ws)
	}
}

func TestBuildAdapter_Codex_WritesNoInvocationEvents(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	events := mlInvokeFailingHarness(t, runFolder, "codex")

	if evs := mlReadEvents(t, events); len(evs) != 0 {
		t.Fatalf("invocation events = %v, want none for Codex", evs)
	}
	if matches, _ := filepath.Glob(filepath.Join(ws, "OrchestrationLogs", "*", "*", "03_events.jsonl")); len(matches) != 0 {
		t.Errorf("invocation event files = %v, want none for Codex", matches)
	}
}
