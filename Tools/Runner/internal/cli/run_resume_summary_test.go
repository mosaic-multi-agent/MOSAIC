package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

const (
	recordedWorkflowID = "test-workflow"
	recordedTask       = "test task"
)

// snapshotSession is a session double that records what the CLI had written to
// stdout at the moment the session started, which is the moment of the first
// dispatch.
type snapshotSession struct {
	out     *bytes.Buffer
	outcome domain.RunOutcome
	called  bool
	atStart string
	config  domain.RunConfig
}

func (s *snapshotSession) Start(_ context.Context, config domain.RunConfig) (domain.RunOutcome, error) {
	s.called = true
	s.config = config
	s.atStart = s.out.String()
	return s.outcome, nil
}

// recordedRun describes the frontmatter settings of a resumable run fixture.
type recordedRun struct {
	runnerSettings string // runner_* lines, "" for a run that records none
	extra          string // further frontmatter lines (checkpoints, commits, ...)
}

// writeRecordedRun creates a resumable run folder whose artifact records the
// given settings. Artifact keys written are a subset of the artifact format.
func writeRecordedRun(t *testing.T, rootDir, runID string, rec recordedRun) {
	t.Helper()
	folder := filepath.Join(rootDir, "Orchestration-"+runID)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatalf("writeRecordedRun: %v", err)
	}
	content := "---\n" +
		"type: orchestration-artifact\n" +
		"run_id: " + runID + "\n" +
		"workflow: " + recordedWorkflowID + "\n" +
		"workflow_version: \"1.0\"\n" +
		"task: \"" + recordedTask + "\"\n" +
		"started: 2026-01-01T00:00:00Z\n" +
		"last_updated: 2026-01-01T00:00:00Z\n" +
		"global_sequence: 1\n" +
		rec.extra +
		rec.runnerSettings +
		"current_state:\n" +
		"  phase: EXECUTION\n" +
		"  iteration: null\n" +
		"  last_status: SUCCESS\n" +
		"  last_agent: \"agent#1\"\n" +
		"  error_code: null\n" +
		"---\n\n" +
		"<ExecutionLog type=\"core\">\n" +
		"| Seq | Agent   | Phase     | Iteration | Status  | Timestamp            | Summary | Checkpoint |\n" +
		"| --- | ------- | --------- | ----- | ------- | -------------------- | ------- | ---------- |\n" +
		"| 1   | agent#1 | EXECUTION | -     | SUCCESS | 2026-01-01T00:00:00Z | done    | -          |\n" +
		"</ExecutionLog>\n\n" +
		"<Artifacts type=\"core\">\n" +
		"| Artifact | Created In | Created By |\n" +
		"| -------- | ---------- | ---------- |\n" +
		"</Artifacts>\n"
	if err := os.WriteFile(filepath.Join(folder, "Orchestration.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("writeRecordedRun: %v", err)
	}
}

const autoSettings = "runner_mode: auto\nrunner_pre_consultation: enabled\nrunner_manual_resolution: disabled\n"

// inWorkDir changes into dir for the test and restores the previous directory.
func inWorkDir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// runSnapshotCLI runs the CLI with a snapshotSession and returns what it wrote.
func runSnapshotCLI(t *testing.T, args []string) (code int, sess *snapshotSession, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	sess = &snapshotSession{out: &out, outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	code = cli.Run(context.Background(), args, &spyStore{}, nil, sess, &out, &errOut)
	return code, sess, out.String(), errOut.String()
}

func TestResume_NoSettingsFlags_StartsFromRecordedWorkflowAndTask(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	if !sess.called {
		t.Fatal("session.Start was not called")
	}
	if string(sess.config.WorkflowID) != recordedWorkflowID {
		t.Errorf("config.WorkflowID = %q, want the recorded %q", sess.config.WorkflowID, recordedWorkflowID)
	}
	if sess.config.Task != recordedTask {
		t.Errorf("config.Task = %q, want the recorded %q", sess.config.Task, recordedTask)
	}
	if sess.config.IsNewRun {
		t.Error("config.IsNewRun = true, want a resume")
	}
}

func TestResume_NoSettingsFlags_PrintsSummaryBeforeFirstDispatch(t *testing.T) {
	rootDir := t.TempDir()
	extra := "checkpoints: disabled\ncommits: enabled\ncommit_branch: mosaic/run/" + testRunID + "\nreview_loop_limit: 3\n"
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings, extra: extra})
	inWorkDir(t, rootDir)

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	for _, want := range []string{
		"Recorded settings for run " + testRunID,
		"  Workflow:           " + recordedWorkflowID + " (version 1.0)",
		"  Mode:               auto",
		"  Pre-consultation:   on",
		"  Manual resolution:  off",
		"  Checkpoints:        off",
		"  Commits:            on (branch mosaic/run/" + testRunID + ", mosaic-owned)",
		"  Review loop limit:  3",
	} {
		if !strings.Contains(sess.atStart, want) {
			t.Errorf("stdout at session start does not contain %q; got:\n%s", want, sess.atStart)
		}
	}
	announce := strings.Index(stdout, testRunID)
	summary := strings.Index(stdout, "Recorded settings for run")
	if announce < 0 || summary < 0 || announce > summary {
		t.Errorf("want the run announcement before the summary; announcement at %d, summary at %d in:\n%s", announce, summary, stdout)
	}
}

func TestResume_CheckpointsAndCommitsRecordedOn_NoFlags_StartsWithoutRefusal(t *testing.T) {
	rootDir := t.TempDir()
	extra := "checkpoints: enabled\ncommits: enabled\ncommit_branch: mosaic/run/" + testRunID + "\n"
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings, extra: extra})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
}

func TestResume_SuppliedFlagsAreHandedToTheSessionForReconciliation(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{
		"run", "--run", testRunID,
		"--mode", "orchestrated", "--review-loop-limit", "7", "--workflow", "other-workflow",
	})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	if sess.config.Mode != domain.ExecutionModeOrchestrated || !sess.config.Supplied.Mode {
		t.Errorf("mode = %q supplied = %v, want the supplied value flagged as supplied", sess.config.Mode, sess.config.Supplied.Mode)
	}
	if sess.config.ReviewLoopLimit != 7 || !sess.config.Supplied.ReviewLoopLimit {
		t.Errorf("review loop limit = %d supplied = %v, want 7 flagged as supplied", sess.config.ReviewLoopLimit, sess.config.Supplied.ReviewLoopLimit)
	}
	if string(sess.config.WorkflowID) != "other-workflow" {
		t.Errorf("config.WorkflowID = %q, want the supplied workflow to win over the record", sess.config.WorkflowID)
	}
}

func TestResume_InvalidSuppliedModeValue_StillRefused(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--mode", "quick"})

	if code != cli.ExitUsage || sess.called {
		t.Errorf("exit code = %d, started = %v; want ExitUsage and no start; stderr: %q", code, sess.called, errOut)
	}
	if !strings.Contains(errOut, "quick") {
		t.Errorf("stderr %q does not name the offending value", errOut)
	}
}

func TestResume_InputWithRun_StillRefusedWithoutWorkflowOrTask(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--input", "a.md"})

	if code != cli.ExitUsage || sess.called {
		t.Errorf("exit code = %d, started = %v; want ExitUsage and no start", code, sess.called)
	}
	if !strings.Contains(errOut, "--input") {
		t.Errorf("stderr %q does not name the conflict with --input", errOut)
	}
}

func TestResume_PendingCommitSetup_StillRequiresCommitBranch(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings, extra: "commits: enabled\n"})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage || sess.called {
		t.Fatalf("exit code = %d, started = %v; want ExitUsage and no start", code, sess.called)
	}
	if !strings.Contains(errOut, "--commit-branch") {
		t.Errorf("stderr %q does not ask for --commit-branch", errOut)
	}
	if strings.Contains(errOut, "--workflow") || strings.Contains(errOut, "--task") {
		t.Errorf("stderr %q asks for a recorded setting flag", errOut)
	}
}

func TestResume_PendingCommitSetup_WithCommitBranch_StartsWithSuppliedVariant(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{runnerSettings: autoSettings, extra: "commits: enabled\n"})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--commit-branch", "user-own"})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if sess.config.CommitBranchVariant != domain.CommitBranchUserOwn || !sess.config.Supplied.CommitBranchVariant {
		t.Errorf("variant = %q supplied = %v, want user-own flagged as supplied", sess.config.CommitBranchVariant, sess.config.Supplied.CommitBranchVariant)
	}
}

func TestResume_Adoption_WithoutMode_RefusesNamingModeOnly(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage || sess.called {
		t.Fatalf("exit code = %d, started = %v; want ExitUsage and no start", code, sess.called)
	}
	if !strings.Contains(errOut, "--mode") {
		t.Errorf("stderr %q does not ask for --mode", errOut)
	}
	if strings.Contains(errOut, "--workflow") || strings.Contains(errOut, "--task") {
		t.Errorf("stderr %q asks for a recorded setting flag", errOut)
	}
}

func TestResume_Adoption_TakesSettingsFromFlagsAndSummarisesThem(t *testing.T) {
	rootDir := t.TempDir()
	writeRecordedRun(t, rootDir, testRunID, recordedRun{})
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{
		"run", "--run", testRunID, "--mode", "auto-review", "--pre-consult", "--manual-resolution",
	})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if sess.config.Mode != domain.ExecutionModeAutoReview || !sess.config.PreConsultation || !sess.config.ManualResolution {
		t.Errorf("config settings = %q/%v/%v, want auto-review/true/true", sess.config.Mode, sess.config.PreConsultation, sess.config.ManualResolution)
	}
	for _, want := range []string{"  Mode:               auto-review", "  Pre-consultation:   on", "  Manual resolution:  on"} {
		if !strings.Contains(sess.atStart, want) {
			t.Errorf("stdout at session start does not contain the adopted value line %q; got:\n%s", want, sess.atStart)
		}
	}
}

func TestNewRun_RequiresWorkflowAndTask_AndPrintsNoRecordedSummary(t *testing.T) {
	for _, missing := range []string{"--workflow", "--task"} {
		t.Run(missing, func(t *testing.T) {
			args := []string{"run", "--mode", "auto", "--new-run", "--review-loop-limit", "3"}
			if missing != "--workflow" {
				args = append(args, "--workflow", "w1")
			} else {
				args = append(args, "--task", "do work")
			}

			code, sess, _, errOut := runSnapshotCLI(t, args)

			if code != cli.ExitUsage || sess.called {
				t.Errorf("exit code = %d, started = %v; want ExitUsage and no start", code, sess.called)
			}
			if !strings.Contains(errOut, missing+" is required") {
				t.Errorf("stderr %q does not say %s is required", errOut, missing)
			}
		})
	}
}

func TestNewRun_PrintsNoRecordedSettingsSummary(t *testing.T) {
	code, sess, _, errOut := runSnapshotCLI(t, newStage7BaseArgs())

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if strings.Contains(sess.atStart, "Recorded settings") {
		t.Errorf("a new run printed a recorded-settings summary:\n%s", sess.atStart)
	}
}
