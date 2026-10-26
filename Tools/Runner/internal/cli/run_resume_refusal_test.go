package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/cli"
)

// orchestratorDeclaring returns minimal orchestrator file content declaring one
// workflow and no infrastructure agents.
func orchestratorDeclaring(workflowID string) string {
	return "<Workflow type=\"core\" name=\"" + workflowID + "\" version=\"1.0\">\n" +
		"## Declared Workflow\n\n" +
		"| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
		"|-------|----------|:----:|------------|-------------|-------|--------|\n" +
		"| PLANNING | agent-a | FALSE | COMPLETE | - | - | plan.md |\n" +
		"</Workflow>\n"
}

// inDeployedWorkDir creates a working directory holding a deployed orchestrator
// file for the claude-code harness with the given content, makes it the current
// directory and returns it. The fixture lives in the temp dir of the test.
func inDeployedWorkDir(t *testing.T, orchestratorContent string) string {
	t.Helper()
	entry, ok := commonharness.LookupCLIHarness("claude-code")
	if !ok {
		t.Skip("claude-code is not a CLI harness")
	}
	workDir := t.TempDir()
	agentsDir := filepath.Join(workDir, entry.AgentsDir)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", agentsDir, err)
	}
	orchPath := filepath.Join(agentsDir, "orchestrator-script.md")
	if err := os.WriteFile(orchPath, []byte(orchestratorContent), 0o644); err != nil {
		t.Fatalf("write %q: %v", orchPath, err)
	}
	inWorkDir(t, workDir)
	return workDir
}

// assertRefusedBeforeAnything checks the shared outcome of an early refusal: the
// given exit code, the text on stderr, and neither announcement, summary nor
// session start.
func assertRefusedBeforeAnything(t *testing.T, wantCode, code int, sess *snapshotSession, stdout, errOut, wantText string) {
	t.Helper()
	if code != wantCode {
		t.Errorf("exit code = %d, want %d; stderr: %q", code, wantCode, errOut)
	}
	if !strings.Contains(errOut, wantText) {
		t.Errorf("stderr %q does not contain %q", errOut, wantText)
	}
	if sess.called {
		t.Error("session.Start was called after an early refusal")
	}
	if strings.Contains(stdout, "Recorded settings") {
		t.Errorf("a summary was printed for a refused resume:\n%s", stdout)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing before the early checks pass", stdout)
	}
}

func TestResume_ArtifactRecordsNoWorkflow_RefusedWithoutSummary(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	artifactPath := filepath.Join(rootDir, "Orchestration-"+testRunID, "Orchestration.md")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	stripped := strings.Replace(string(data), "workflow: "+recordedWorkflowID+"\n", "", 1)
	if err := os.WriteFile(artifactPath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	inWorkDir(t, rootDir)

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut,
		"run "+testRunID+" records no workflow, so there is nothing to resume it as")
}

func TestResume_MalformedArtifact_RefusedWithoutSummary(t *testing.T) {
	rootDir := t.TempDir()
	folder := filepath.Join(rootDir, "Orchestration-"+testRunID)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Orchestration.md"), []byte("this is not a run artifact\n"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	inWorkDir(t, rootDir)

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut, "missing frontmatter")
}

func TestResume_RecordedWorkflowNoLongerDeclared_RefusedByDeployedTier(t *testing.T) {
	workDir := inDeployedWorkDir(t, orchestratorDeclaring("some-other-workflow"))
	writeRecordedRun(t, workDir, testRunID, recordedRun{runnerSettings: autoSettings})

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--harness", "claude-code"})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut,
		"workflow \""+recordedWorkflowID+"\" for run "+testRunID+" is not declared in the current orchestrator file; it may have been renamed or removed")
}

func TestResume_CommitsRecordedButNoCommitAgentDeclared_RefusedByDeployedTier(t *testing.T) {
	workDir := inDeployedWorkDir(t, orchestratorDeclaring(recordedWorkflowID))
	extra := "commits: enabled\ncommit_branch: mosaic/run/" + testRunID + "\n"
	writeRecordedRun(t, workDir, testRunID, recordedRun{runnerSettings: autoSettings, extra: extra})

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--harness", "claude-code"})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut,
		"commits enabled but no commit-class infrastructure agent is declared")
}

func TestResume_DeployedFilesSatisfyTheRecord_StartsAndPrintsSummary(t *testing.T) {
	workDir := inDeployedWorkDir(t, orchestratorDeclaring(recordedWorkflowID))
	writeRecordedRun(t, workDir, testRunID, recordedRun{runnerSettings: autoSettings})

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--harness", "claude-code"})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if !strings.Contains(sess.atStart, "Recorded settings for run "+testRunID) {
		t.Errorf("stdout at session start has no summary:\n%s", sess.atStart)
	}
}

func TestResume_NoInfrastructureRecorded_SummaryShowsInfrastructureAndOverridesLines(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	for _, want := range []string{"  Infrastructure:     none recorded", "  Overrides:          none"} {
		if !strings.Contains(sess.atStart, want) {
			t.Errorf("stdout at session start does not contain %q; got:\n%s", want, sess.atStart)
		}
	}
}
