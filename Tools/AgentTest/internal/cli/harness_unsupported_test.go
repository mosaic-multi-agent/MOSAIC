package cli_test

// Flag validation for catalog harnesses this tool knows but refuses: the
// outcome is a usage error distinguishable from the unknown-id usage error,
// wraps cli.ErrHarnessNotSupported, names the harness, and never reaches
// pre-flight or the suite runner. Assertions are on kind and naming, not wording.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/preflight"
)

// unsupportedOptions is catalogOptions plus one known-but-unsupported entry.
func unsupportedOptions(stdout, stderr *bytes.Buffer, pf cli.PreflightFunc, sr cli.SuiteRunner) cli.Options {
	o := catalogOptions(stdout, stderr, pf, sr)
	o.UnsupportedHarnesses = []commonharness.CLIHarness{
		{ID: "codex", Label: "Codex CLI"},
	}
	return o
}

func TestRunCommand_UnsupportedHarness_IsUsageErrorNamingHarness(t *testing.T) {
	forms := map[string][]string{
		"space":  {"run", "suite.yaml", "--harness", "codex"},
		"equals": {"run", "suite.yaml", "--harness=codex"},
	}
	for name, args := range forms {
		t.Run(name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			var preflightCalled bool
			pf := func(in preflight.Input) (preflight.Plan, authoring.Report) {
				preflightCalled = true
				return preflight.Plan{}, authoring.Report{}
			}
			runner := &fakeSuiteRunner{}
			opts := unsupportedOptions(stdout, stderr, pf, runner)

			got := cli.Execute(context.Background(), args, opts)

			if got != cli.ExitUsage {
				t.Errorf("exit code = %d, want ExitUsage (%d)", got, cli.ExitUsage)
			}
			if runner.wasCalled() {
				t.Error("suite runner was invoked for an unsupported --harness selection")
			}
			if preflightCalled {
				t.Error("pre-flight was invoked for an unsupported --harness selection")
			}
			if !strings.Contains(stderr.String(), "codex") {
				t.Errorf("stderr = %q, want it to name codex", stderr.String())
			}
		})
	}
}

// TestRunCommand_UnknownHarness_StillUsageErrorNamingId is regression coverage:
// an id in neither list keeps its usage error. The distinguishable-outcome
// check on the sentinel lives in harness_unsupported_internal_test.go.
func TestRunCommand_UnknownHarness_StillUsageErrorNamingId(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	opts := unsupportedOptions(stdout, stderr, scriptedPreflight(preflight.Plan{}, cleanReport(), nil), &fakeSuiteRunner{})

	got := cli.Execute(context.Background(), []string{"run", "suite.yaml", "--harness", "not-a-real-harness"}, opts)

	if got != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage (%d)", got, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "not-a-real-harness") {
		t.Errorf("stderr = %q, want it to name the id", stderr.String())
	}
}

// TestRunCommand_UnsupportedHarness_ValidatedEvenWithoutSupportedCatalog
// verifies the refusal does not depend on a supported catalog being supplied
// alongside: a harness listed only in UnsupportedHarnesses is still refused.
func TestRunCommand_UnsupportedHarness_ValidatedEvenWithoutSupportedCatalog(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	runner := &fakeSuiteRunner{}
	opts := baseOptions(stdout, stderr, scriptedPreflight(preflight.Plan{}, cleanReport(), nil), runner)
	opts.UnsupportedHarnesses = []commonharness.CLIHarness{{ID: "codex", Label: "Codex CLI"}}

	got := cli.Execute(context.Background(), []string{"run", "suite.yaml", "--harness", "codex"}, opts)

	if got != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage (%d)", got, cli.ExitUsage)
	}
	if runner.wasCalled() {
		t.Error("suite runner was invoked for an unsupported --harness selection")
	}
}
