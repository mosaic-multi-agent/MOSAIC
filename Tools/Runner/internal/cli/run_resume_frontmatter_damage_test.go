package cli_test

// Resuming a run whose Orchestration.md has damaged frontmatter. With a usable
// last-good copy the run's recorded values come from that copy: the early resume
// checks pass, the run configuration carries the recorded workflow and the run
// reaches the session, which repairs it at start. Without a usable copy the
// resume is refused early, naming the damage.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

// lastGoodFixtureDir is resolved when the test binary starts, before any test
// changes the working directory.
var lastGoodFixtureDir = func() string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "damaged"))
	if err != nil {
		panic(err)
	}
	return dir
}()

// writeLastGoodCopy lays the named fixture under testdata/damaged out as the
// last-good copy of run testRunID in the current working directory's workspace.
func writeLastGoodCopy(t *testing.T, fixture string, replacements ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(lastGoodFixtureDir, fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	text := string(data)
	for i := 0; i+1 < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("fixture %s does not contain %q", fixture, replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	dir := filepath.Join(workDir, "RunnerLogs", testRunID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Orchestration.last-good.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write last-good copy: %v", err)
	}
}

func TestResume_DamagedFrontmatterWithUsableLastGoodCopy_ReachesTheSessionWithTheRecordedWorkflow(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "frontmatter_intact.md", `task: "test task"`, `task: "copy task"`)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	if !sess.called {
		t.Fatal("session.Start was not called: the run was refused before the session could repair it")
	}
	if sess.config.IsNewRun {
		t.Error("config.IsNewRun = true, want a resume")
	}
	if string(sess.config.WorkflowID) != "test-workflow" || sess.config.Task != "copy task" {
		t.Errorf("config workflow/task = %q/%q, want test-workflow/copy task from the last-good copy",
			sess.config.WorkflowID, sess.config.Task)
	}
}

func TestResume_DamagedFrontmatterWithUsableLastGoodCopy_SummaryShowsTheCopysSettings(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "frontmatter_intact.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	for _, want := range []string{
		"Recorded settings for run " + testRunID,
		"  Workflow:           test-workflow (version 1.0)",
		"  Mode:               auto",
	} {
		if !strings.Contains(sess.atStart, want) {
			t.Errorf("stdout at session start does not contain %q; got:\n%s", want, sess.atStart)
		}
	}
}

func TestResume_DamagedFrontmatterWithUsableLastGoodCopy_AnnouncesTheDamageAndTheCopysPosition(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "frontmatter_intact.md")

	_, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if !sess.called {
		t.Fatalf("session.Start was not called; stderr: %q", errOut)
	}
	if !strings.Contains(stdout, "damaged") {
		t.Errorf("stdout does not say the artifact is damaged:\n%s", stdout)
	}
	if !strings.Contains(stdout, "agent#2") {
		t.Errorf("stdout does not state the last agent the copy records (agent#2):\n%s", stdout)
	}
}

func TestResume_DamagedFrontmatterWhoseLastGoodCopyRecordsCommitsWithoutBranch_AsksForTheCommitBranch(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "commit_setup_pending.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage: the copy records a pending commit setup; stderr: %q", code, errOut)
	}
	if !strings.Contains(errOut, "--commit-branch is required") {
		t.Errorf("stderr %q does not ask for --commit-branch", errOut)
	}
	if sess.called {
		t.Error("session.Start was called without the commit branch the run needs")
	}
}

func TestResume_DamagedFrontmatterWhoseLastGoodCopyRecordsNoRunnerSettings_AsksForTheMode(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "no_runner_settings.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage: the copy records no runner settings to adopt; stderr: %q", code, errOut)
	}
	if !strings.Contains(errOut, "--mode") {
		t.Errorf("stderr %q does not ask for --mode", errOut)
	}
	if sess.called {
		t.Error("session.Start was called without the mode the run needs")
	}
}

func TestResume_DamagedFrontmatterWhoseLastGoodCopyRecordsNoRunnerSettings_StartsWhenTheModeIsSupplied(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "no_runner_settings.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--mode", "auto"})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if sess.config.Mode != domain.ExecutionModeAuto || !sess.config.Supplied.Mode {
		t.Errorf("mode = %q supplied = %v, want the supplied mode handed to the session", sess.config.Mode, sess.config.Supplied.Mode)
	}
}

func TestResume_DamagedFrontmatterWithLastGoodCopyOfAnotherRun_IsRefusedEarlyNamingTheDamage(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "frontmatter_intact.md", "run_id: "+testRunID, "run_id: 20260101T000000Z-abcd")

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut, "damaged")
}

func TestResume_DamagedFrontmatterWithLastGoodCopyRecordedAsCompleted_IsNotResumed(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")
	writeLastGoodCopy(t, "frontmatter_intact.md", "phase: EXECUTION", "phase: COMPLETED")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code == cli.ExitSuccess || sess.called {
		t.Errorf("exit code = %d, started = %v; want a run the copy records as completed not resumed", code, sess.called)
	}
	if !strings.Contains(errOut, "completed") {
		t.Errorf("stderr %q does not say the run is completed", errOut)
	}
}
