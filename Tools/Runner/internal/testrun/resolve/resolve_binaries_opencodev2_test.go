package resolve_test

// Test-mode binary resolution for the opencode-v2 harness: its default binary
// name looked up on PATH is "opencode", and the resolved path is returned under
// the harness id.

import (
	"testing"

	"mosaic-run/internal/testrun/resolve"
)

func TestResolveHarnessBinaries_OpenCodeV2_CallsLookPathWithOpenCode(t *testing.T) {
	rec := &recordingLookPath{result: "/usr/local/bin/opencode"}

	result, err := resolve.ResolveHarnessBinaries([]string{"opencode-v2"}, rec.fn())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.called) != 1 || rec.called[0] != "opencode" {
		t.Errorf("lookPath called with %v, want exactly [opencode]", rec.called)
	}
	if result["opencode-v2"] != "/usr/local/bin/opencode" {
		t.Errorf("result[opencode-v2] = %q, want %q", result["opencode-v2"], "/usr/local/bin/opencode")
	}
}

func TestHarnessDisplayOrder_OpenCodeV2_IsKeptAsARealHarness(t *testing.T) {
	got := resolve.HarnessDisplayOrder([]string{"opencode-v2", "fake", "claude-code"})

	if len(got) != 2 || got[0] != "opencode-v2" || got[1] != "claude-code" {
		t.Errorf("HarnessDisplayOrder = %v, want [opencode-v2 claude-code]", got)
	}
}
