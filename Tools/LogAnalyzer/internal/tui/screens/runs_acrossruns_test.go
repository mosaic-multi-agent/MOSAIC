package screens_test

// The runs list advertises the key that opens the across-runs view.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func TestRunsScreen_HelpListsTheAcrossRunsKey(t *testing.T) {
	help := screens.NewRunsScreen(acrossReport(), wideW, tallH, plain()).Help()

	if !strings.Contains(help, screens.KeyAcrossRuns+" ") || !strings.Contains(strings.ToLower(help), "across") {
		t.Errorf("runs help %q must list key %q with the across-runs view", help, screens.KeyAcrossRuns)
	}
}
