package cli_test

// Resuming a run whose Orchestration.md is damaged: the early resume checks and
// the identity and settings readers do not refuse it, so the run reaches the
// session, which repairs the artifact at start. What the damaged artifact
// cannot say is left to the session; the pre-3.0 layout, run-identity refusals
// and other read errors end the resume as before.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

// writeDamagedRun lays the named fixture under testdata/damaged out as the
// artifact of run testRunID (the run_id the fixtures record) and makes the
// workspace the working directory. Each old->new pair is replaced once first.
func writeDamagedRun(t *testing.T, fixture string, replacements ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "damaged", fixture))
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
	rootDir := t.TempDir()
	folder := filepath.Join(rootDir, "Orchestration-"+testRunID)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Orchestration.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	inWorkDir(t, rootDir)
}

func TestResume_DamagedArtifactWithIntactFrontmatter_ReachesTheSession(t *testing.T) {
	writeDamagedRun(t, "frontmatter_intact.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want ExitSuccess; stderr: %q", code, errOut)
	}
	if !sess.called {
		t.Fatal("session.Start was not called: the damaged run was refused before the session could repair it")
	}
	if sess.config.IsNewRun {
		t.Error("config.IsNewRun = true, want a resume")
	}
	if string(sess.config.WorkflowID) != "test-workflow" || sess.config.Task != "test task" {
		t.Errorf("config workflow/task = %q/%q, want the frontmatter's test-workflow/test task",
			sess.config.WorkflowID, sess.config.Task)
	}
}

func TestResume_DamagedArtifactWithIntactFrontmatter_SummaryShowsTheRecordedSettings(t *testing.T) {
	writeDamagedRun(t, "frontmatter_intact.md")

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

func TestResume_DamagedArtifactWithIntactFrontmatter_AnnouncementNamesTheDamageAndThePosition(t *testing.T) {
	writeDamagedRun(t, "frontmatter_intact.md")

	_, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if !sess.called {
		t.Fatalf("session.Start was not called; stderr: %q", errOut)
	}
	if !strings.Contains(stdout, "damaged") {
		t.Errorf("stdout does not say the artifact is damaged:\n%s", stdout)
	}
	if !strings.Contains(stdout, "agent#2") {
		t.Errorf("stdout does not state the recorded last agent (agent#2):\n%s", stdout)
	}
}

func TestResume_DamagedFrontmatterWithoutLastGoodCopy_IsRefusedEarlyNamingTheDamage(t *testing.T) {
	writeDamagedRun(t, "frontmatter_broken.md")

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut, "damaged")
}

func TestResume_DamagedArtifactEnablingCommitsWithoutBranch_AsksForTheCommitBranch(t *testing.T) {
	writeDamagedRun(t, "commit_setup_pending.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage: the frontmatter records a pending commit setup", code)
	}
	if !strings.Contains(errOut, "--commit-branch is required") {
		t.Errorf("stderr %q does not ask for --commit-branch as for any run with a pending commit setup", errOut)
	}
	if sess.called {
		t.Error("session.Start was called without the commit branch the run needs")
	}
}

func TestResume_DamagedArtifactRecordingNoRunnerSettings_AsksForTheMode(t *testing.T) {
	writeDamagedRun(t, "no_runner_settings.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage: the frontmatter records no runner settings to adopt", code)
	}
	if !strings.Contains(errOut, "--mode") {
		t.Errorf("stderr %q does not ask for --mode as for any run without recorded runner settings", errOut)
	}
	if sess.called {
		t.Error("session.Start was called without the mode the run needs")
	}
}

func TestResume_DamagedArtifactRecordingNoRunnerSettings_StartsWhenTheModeIsSupplied(t *testing.T) {
	writeDamagedRun(t, "no_runner_settings.md")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID, "--mode", "auto"})

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v; want a start; stderr: %q", code, sess.called, errOut)
	}
	if sess.config.Mode != domain.ExecutionModeAuto || !sess.config.Supplied.Mode {
		t.Errorf("mode = %q supplied = %v, want the supplied mode handed to the session", sess.config.Mode, sess.config.Supplied.Mode)
	}
}

func TestResume_Pre30LayoutWithDamage_IsStillRefusedAsPre30(t *testing.T) {
	writeDamagedRun(t, "pre30_layout_and_damage.md")

	code, sess, stdout, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	assertRefusedBeforeAnything(t, cli.ExitRefused, code, sess, stdout, errOut, "pre-3.0")
}

func TestResume_DamagedArtifactWithMalformedRunID_IsStillRefusedForItsIdentity(t *testing.T) {
	writeDamagedRun(t, "frontmatter_intact.md", "run_id: "+testRunID, "run_id: not-a-run-id")

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code == cli.ExitSuccess || sess.called {
		t.Errorf("exit code = %d, started = %v; want the run-identity refusal kept for a damaged artifact", code, sess.called)
	}
	if !strings.Contains(errOut, "cannot be resumed") {
		t.Errorf("stderr %q does not refuse the resume for the run identity", errOut)
	}
}

func TestResume_UnreadableArtifact_IsStillRefusedAsBefore(t *testing.T) {
	rootDir := t.TempDir()
	folder := filepath.Join(rootDir, "Orchestration-"+testRunID)
	// A directory where the artifact should be: reading it fails on any platform.
	if err := os.MkdirAll(filepath.Join(folder, "Orchestration.md"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	inWorkDir(t, rootDir)

	code, sess, _, errOut := runSnapshotCLI(t, []string{"run", "--run", testRunID})

	if code == cli.ExitSuccess || sess.called {
		t.Errorf("exit code = %d, started = %v; want the read error to end the resume", code, sess.called)
	}
	if strings.Contains(errOut, "damaged") {
		t.Errorf("stderr %q reports damage for a read error", errOut)
	}
}
