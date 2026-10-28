package main

// usageingest_callsites_test.go drives the two production composition paths (the
// non-interactive CLI frontend and the interactive TUI frontend) with the real
// OTel ingest, to show that each of them applies the GHCP CLI usage ingest to the
// session it builds. A resumed run is observable from outside: its catch-up
// ingest leaves an ingest state in the run's log folder before any harness
// process could run.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/runconfig"
)

const uiResumedRunID = "20261003T185044Z-07e9"

// uiWriteOTelLines creates an OTel file holding n complete JSONL lines.
func uiWriteOTelLines(t *testing.T, n int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "otel.jsonl")
	if err := os.WriteFile(p, []byte(strings.Repeat("{}\n", n)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// uiIngestStateOf is the ingest state file of a run log folder.
func uiIngestStateOf(workspace, runID string) string {
	return filepath.Join(workspace, "OrchestrationLogs", runID, ".otel-ingest", "state.json")
}

func TestCLIWiring_ResumedGHCPCLIRunCatchesUpUsageBeforeItsSessionRuns(t *testing.T) {
	ws := t.TempDir()
	t.Chdir(ws)
	mlDeployEmptyOrchestrator(t, ws, "ghcp-cli")
	writeRunArtifactRecordingWorkflow(t, ws, uiResumedRunID, "greenfield-tdd")
	uiSetEnv(t, uiWriteOTelLines(t, 3))
	args := []string{
		"run", "--harness", "ghcp-cli", "--ghcp-permission-mode", "blanket",
		"--executable-path", filepath.Join(ws, "no-such-copilot"), "--run", uiResumedRunID, "--mode", "orchestrated",
	}

	_ = runCLIMode(args, args)

	if _, err := os.Stat(uiIngestStateOf(ws, uiResumedRunID)); err != nil {
		t.Fatalf("no ingest state after resuming a GHCP CLI run through the CLI path: %v", err)
	}
}

func TestCLIWiring_ResumedRunOnAnotherHarnessRunsNoCatchUp(t *testing.T) {
	ws := t.TempDir()
	t.Chdir(ws)
	mlDeployEmptyOrchestrator(t, ws, "opencode")
	writeRunArtifactRecordingWorkflow(t, ws, uiResumedRunID, "greenfield-tdd")
	uiSetEnv(t, uiWriteOTelLines(t, 3))
	args := []string{
		"run", "--harness", "opencode", "--executable-path", filepath.Join(ws, "no-such-opencode"), "--run", uiResumedRunID, "--mode", "orchestrated",
	}

	_ = runCLIMode(args, args)

	if _, err := os.Stat(uiIngestStateOf(ws, uiResumedRunID)); err == nil {
		t.Fatal("an ingest state was written for a run on a harness other than GHCP CLI")
	}
}

func TestInteractiveWiring_ResumedGHCPCLIRunCatchesUpUsageBeforeItsSessionRuns(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	w := buildInteractiveWiring(in)
	runFolder := testRunFolder(t, uiResumedRunID)
	ws := filepath.Dir(runFolder)
	uiSetEnv(t, uiWriteOTelLines(t, 3))
	cfg := runconfig.ConfigSelection{
		Harness:        "ghcp-cli",
		GHCPCLIMode:    "blanket",
		ExecutablePath: filepath.Join(ws, "no-such-copilot"),
	}

	sess := w.Options.SessionFactory(runFolder, false, "", cfg)
	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if _, err := os.Stat(uiIngestStateOf(ws, uiResumedRunID)); err != nil {
		t.Fatalf("no ingest state after resuming a GHCP CLI run through the interactive path: %v", err)
	}
}

func TestInteractiveWiring_ResumedRunOnAnotherHarnessRunsNoCatchUp(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	w := buildInteractiveWiring(in)
	runFolder := testRunFolder(t, uiResumedRunID)
	ws := filepath.Dir(runFolder)
	uiSetEnv(t, uiWriteOTelLines(t, 3))
	cfg := runconfig.ConfigSelection{Harness: "claude-code", ExecutablePath: filepath.Join(ws, "no-such-claude")}

	sess := w.Options.SessionFactory(runFolder, false, "", cfg)
	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if _, err := os.Stat(uiIngestStateOf(ws, uiResumedRunID)); err == nil {
		t.Fatal("an ingest state was written for a run on a harness other than GHCP CLI")
	}
}
