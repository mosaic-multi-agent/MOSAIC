package cli_test

import (
	"testing"

	"mosaic-run/internal/cli"
)

// Test mode without --harness covers every CLI-backed harness, so opencode-v2
// must be part of the default list.
func TestDefaultTestHarnesses_IncludesOpenCodeV2(t *testing.T) {
	for _, id := range cli.DefaultTestHarnesses() {
		if id == "opencode-v2" {
			return
		}
	}
	t.Errorf("DefaultTestHarnesses() = %v, want it to include opencode-v2", cli.DefaultTestHarnesses())
}
