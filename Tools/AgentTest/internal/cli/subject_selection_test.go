package cli_test

// Tests that the command surface refuses a run without an explicit subject
// model selection, through the real pre-flight, before the suite factory is
// reached (so no workspace is created and no subject is spawned).

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/preflight"
)

// guardSuitePath returns the absolute path of the tool-local fixture suite
// used to drive the real pre-flight.
func guardSuitePath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "model_guard", "suite", "s.suite.yaml"))
	if err != nil {
		t.Fatalf("resolving fixture suite path: %v", err)
	}
	return p
}

// realPreflightOptions wires the real preflight.Validate and a counting
// suite factory.
func realPreflightOptions(t *testing.T, stdout, stderr *bytes.Buffer) (cli.Options, *fakeSuiteFactory) {
	t.Helper()
	runner := &fakeSuiteRunner{result: passingResult("s")}
	factory := &fakeSuiteFactory{runner: runner}
	opts := baseOptionsWithFactory(stdout, stderr, preflight.Validate, factory.build)
	opts.FixtureRoot = filepath.Join(filepath.Dir(guardSuitePath(t)), "fixtures")
	return opts, factory
}

// TestRunCommand_NoSubjectModelFlag_RejectedBeforeSuiteIsBuilt asserts the
// rejection, its exit code, that it names the flag, and that nothing was spent.
func TestRunCommand_NoSubjectModelFlag_RejectedBeforeSuiteIsBuilt(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts, factory := realPreflightOptions(t, stdout, stderr)

	got := cli.Execute(context.Background(), []string{"run", guardSuitePath(t)}, opts)

	if got != cli.ExitPreflight {
		t.Errorf("exit code = %d, want ExitPreflight (%d)\nstdout: %s\nstderr: %s", got, cli.ExitPreflight, stdout, stderr)
	}
	output := stdout.String() + stderr.String()
	for _, want := range []string{"missing-subject-model", "--subject-model"} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not mention %q:\n%s", want, output)
		}
	}
	if factory.callCount() != 0 {
		t.Errorf("suite factory called %d time(s) for a run with no subject model, want 0", factory.callCount())
	}
	if factory.runner.(*fakeSuiteRunner).wasCalled() {
		t.Error("suite runner invoked for a run with no subject model")
	}
}

// TestRunCommand_StubModelFlagOnly_StillRejected asserts a stub selection does
// not stand in for the subject selection.
func TestRunCommand_StubModelFlagOnly_StillRejected(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts, factory := realPreflightOptions(t, stdout, stderr)

	got := cli.Execute(context.Background(), []string{"run", guardSuitePath(t), "--stub-model", "some-stub"}, opts)

	if got != cli.ExitPreflight {
		t.Errorf("exit code = %d, want ExitPreflight (%d)", got, cli.ExitPreflight)
	}
	if factory.callCount() != 0 {
		t.Errorf("suite factory called %d time(s), want 0", factory.callCount())
	}
}

// TestRunCommand_SubjectModelOnly_RunsAndStubFollowsSubject asserts the
// accepted case: a subject selection alone starts the run, and the plan the
// suite receives resolves the stub model to the subject selection.
func TestRunCommand_SubjectModelOnly_RunsAndStubFollowsSubject(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts, factory := realPreflightOptions(t, stdout, stderr)

	got := cli.Execute(context.Background(), []string{"run", guardSuitePath(t), "--subject-model", "chosen-subject"}, opts)

	if got == cli.ExitPreflight {
		t.Fatalf("a run with a subject model was rejected by pre-flight\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	runner := factory.runner.(*fakeSuiteRunner)
	if !runner.wasCalled() {
		t.Fatal("suite runner was not invoked for a run with a subject model")
	}
	if len(runner.gotPlan.Tests) == 0 {
		t.Fatal("plan handed to the suite has no tests")
	}
	models := runner.gotPlan.Tests[0].Models
	if models.Subject != "chosen-subject" {
		t.Errorf("plan Models.Subject = %q, want %q", models.Subject, "chosen-subject")
	}
	if got := models.EffectiveStub(); got != "chosen-subject" {
		t.Errorf("plan Models.EffectiveStub() = %q, want the subject selection %q", got, "chosen-subject")
	}
}

// TestValidateCommand_NoSubjectModelFlag_ExitsPreflight asserts validate
// applies the same rule, so "no errors found" keeps meaning a run with the
// same arguments would pass pre-flight.
func TestValidateCommand_NoSubjectModelFlag_ExitsPreflight(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts, _ := realPreflightOptions(t, stdout, stderr)

	got := cli.Execute(context.Background(), []string{"validate", guardSuitePath(t)}, opts)

	if got != cli.ExitPreflight {
		t.Errorf("exit code = %d, want ExitPreflight (%d)\nstdout: %s\nstderr: %s", got, cli.ExitPreflight, stdout, stderr)
	}
	if output := stdout.String() + stderr.String(); !strings.Contains(output, "missing-subject-model") {
		t.Errorf("validate output does not report missing-subject-model:\n%s", output)
	}
}

// TestValidateCommand_WithSubjectModelFlag_Passes is the mirror case.
func TestValidateCommand_WithSubjectModelFlag_Passes(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts, _ := realPreflightOptions(t, stdout, stderr)

	got := cli.Execute(context.Background(), []string{"validate", guardSuitePath(t), "--subject-model", "chosen-subject"}, opts)

	if got != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)\nstdout: %s\nstderr: %s", got, cli.ExitSuccess, stdout, stderr)
	}
}

// TestValidateHelp_StatesSubjectModelIsRequired asserts the usage text for
// validate tells the user the flag is required.
func TestValidateHelp_StatesSubjectModelIsRequired(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts := baseOptions(stdout, stderr, scriptedPreflight(preflight.Plan{}, cleanReport(), nil), &fakeSuiteRunner{})

	cli.Execute(context.Background(), []string{"validate", "--help"}, opts)

	output := strings.ToLower(stdout.String() + stderr.String())
	idx := strings.Index(output, "--subject-model")
	if idx < 0 {
		t.Fatalf("validate usage does not mention --subject-model:\n%s", output)
	}
	if !strings.Contains(output[idx:min(len(output), idx+200)], "required") {
		t.Errorf("validate usage does not say --subject-model is required:\n%s", output)
	}
}
