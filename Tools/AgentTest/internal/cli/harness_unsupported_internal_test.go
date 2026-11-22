package cli

// White-box tests for the flag-validation outcome of a --harness value naming
// a catalog harness this tool knows but refuses: the returned error wraps
// ErrHarnessNotSupported, while an unknown id does not.

import (
	"errors"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
)

func resolveWithHarness(t *testing.T, id string, o Options) error {
	t.Helper()
	inv, err := parseInvocation([]string{"run", "suite.yaml", "--harness", id})
	if err != nil {
		t.Fatalf("parseInvocation: %v", err)
	}
	_, err = resolveInput(inv, o)
	return err
}

func TestResolveInput_UnsupportedHarness_WrapsErrHarnessNotSupported(t *testing.T) {
	o := Options{
		Harnesses:            []commonharness.CLIHarness{{ID: "claude-code", Label: "Claude Code"}},
		UnsupportedHarnesses: []commonharness.CLIHarness{{ID: "codex", Label: "Codex CLI"}},
	}

	err := resolveWithHarness(t, "codex", o)

	if err == nil {
		t.Fatal("resolveInput accepted an unsupported harness")
	}
	if !errors.Is(err, ErrHarnessNotSupported) {
		t.Errorf("error = %v, want it to wrap ErrHarnessNotSupported", err)
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("error = %q, want it to name codex", err.Error())
	}
}

func TestResolveInput_UnsupportedHarness_RefusedWithoutSupportedCatalog(t *testing.T) {
	o := Options{UnsupportedHarnesses: []commonharness.CLIHarness{{ID: "codex", Label: "Codex CLI"}}}

	err := resolveWithHarness(t, "codex", o)

	if !errors.Is(err, ErrHarnessNotSupported) {
		t.Errorf("error = %v, want it to wrap ErrHarnessNotSupported", err)
	}
}

func TestResolveInput_UnknownHarness_DoesNotWrapErrHarnessNotSupported(t *testing.T) {
	o := Options{
		Harnesses:            []commonharness.CLIHarness{{ID: "claude-code", Label: "Claude Code"}},
		UnsupportedHarnesses: []commonharness.CLIHarness{{ID: "codex", Label: "Codex CLI"}},
	}

	err := resolveWithHarness(t, "not-a-real-harness", o)

	if err == nil {
		t.Fatal("resolveInput accepted an unknown harness")
	}
	if errors.Is(err, ErrHarnessNotSupported) {
		t.Errorf("unknown-id error = %v, must not wrap ErrHarnessNotSupported", err)
	}
	if !strings.Contains(err.Error(), "not-a-real-harness") {
		t.Errorf("unknown-id error = %q, want it to name the id", err.Error())
	}
}
