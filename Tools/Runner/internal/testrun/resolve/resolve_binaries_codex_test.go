package resolve_test

// Test-mode binary resolution for the Codex harness: the default binary name
// looked up on PATH is "codex", and the resolved path is returned under the
// harness id.

import (
	"testing"

	"mosaic-run/internal/testrun/resolve"
)

func TestResolveHarnessBinaries_Codex_CallsLookPathWithCodex(t *testing.T) {
	rec := &recordingLookPath{result: "/usr/local/bin/codex"}

	result, err := resolve.ResolveHarnessBinaries([]string{"codex"}, rec.fn())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.called) != 1 || rec.called[0] != "codex" {
		t.Errorf("lookPath called with %v, want exactly [codex]", rec.called)
	}
	if result["codex"] != "/usr/local/bin/codex" {
		t.Errorf("result[codex] = %q, want %q", result["codex"], "/usr/local/bin/codex")
	}
}

func TestHarnessDisplayOrder_Codex_IsKeptAsARealHarness(t *testing.T) {
	got := resolve.HarnessDisplayOrder([]string{"codex", "fake", "claude-code"})

	if len(got) != 2 || got[0] != "codex" || got[1] != "claude-code" {
		t.Errorf("HarnessDisplayOrder = %v, want [codex claude-code]", got)
	}
}
