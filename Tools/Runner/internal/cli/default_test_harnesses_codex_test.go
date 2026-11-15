package cli_test

import (
	"testing"

	"mosaic-run/internal/cli"
)

// Test mode without --harness covers every CLI-backed harness, so codex must be
// part of the default list.
func TestDefaultTestHarnesses_IncludesCodex(t *testing.T) {
	for _, id := range cli.DefaultTestHarnesses() {
		if id == "codex" {
			return
		}
	}
	t.Errorf("DefaultTestHarnesses() = %v, want it to include codex", cli.DefaultTestHarnesses())
}
