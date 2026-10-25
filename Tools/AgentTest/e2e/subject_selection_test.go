package e2e

// A run without an explicit subject model selection is refused through the
// same entry point a user reaches, before any workspace is created or any
// subject is spawned.

import (
	"os"
	"strings"
	"testing"

	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/domain"
)

func TestSubjectSelection_RunWithoutSubjectModel_RejectedBeforeAnythingIsSpent(t *testing.T) {
	workspaceRoot := t.TempDir()
	sc := Scenario{
		Dir:       "testdata/e2e/happy-path",
		SuitePath: "happy-path.suite.yaml",
		Script: fakeOptionsFor(
			domain.HarnessCapabilities{SupportsDirectSubstitution: true},
			[]subjectStep{
				invoke("dispatch", "greeter"),
				finish(`{"agent_instance_id":"standin-subject#1","status_code":"SUCCESS","status_message":"Done."}`),
			},
		),
		WorkspaceRoot:    workspaceRoot,
		OmitSubjectModel: true,
	}

	out := RunScenario(t, sc)

	if out.ExitCode != cli.ExitPreflight {
		t.Fatalf("exit code = %d, want %d (ExitPreflight)\nstdout: %s\nstderr: %s",
			out.ExitCode, cli.ExitPreflight, out.Stdout, out.Stderr)
	}
	combined := string(out.Stdout) + string(out.Stderr)
	if !strings.Contains(combined, "missing-subject-model") {
		t.Errorf("output does not report missing-subject-model:\n%s", combined)
	}
	if !strings.Contains(combined, "--subject-model") {
		t.Errorf("output does not name the --subject-model flag:\n%s", combined)
	}
	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		t.Fatalf("reading workspace root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("workspace root holds %d entries after a rejected run, want none (nothing may be created before the guard)", len(entries))
	}
	if got := out.Result.Counts[string(domain.VerdictPass)]; got != 0 {
		t.Errorf("a rejected run reported %d passing test(s), want 0", got)
	}
}

func TestSubjectSelection_RunWithSubjectModel_ProceedsToExecution(t *testing.T) {
	sc := Scenario{
		Dir:       "testdata/e2e/happy-path",
		SuitePath: "happy-path.suite.yaml",
		Script: fakeOptionsFor(
			domain.HarnessCapabilities{SupportsDirectSubstitution: true},
			[]subjectStep{
				invoke("dispatch", "greeter"),
				finish(`{"agent_instance_id":"standin-subject#1","status_code":"SUCCESS","status_message":"Done."}`),
			},
		),
	}

	out := RunScenario(t, sc)

	if out.ExitCode != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (ExitSuccess)\nstdout: %s\nstderr: %s",
			out.ExitCode, cli.ExitSuccess, out.Stdout, out.Stderr)
	}
}
