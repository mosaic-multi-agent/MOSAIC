package cli_test

// output_kept_workflow_warnings_test.go specifies the console output of update for workflows
// that are kept as deployed because the catalog stack does not supply them: the human output
// prints one "warning: " line per kept workflow, with the warning text in full.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	keptWarningX = "custom-x is not in the catalog stack; kept as deployed, not updated or validated."
	keptWarningY = "custom-y is not in the catalog stack; kept as deployed, not updated or validated."
)

func TestHumanOutput_Update_PrintsOneWarningLinePerKeptWorkflow(t *testing.T) {
	summary := regionSummary("update", domain.OutcomeSuccess)
	summary.Todos = []domain.TodoItem{
		{Category: domain.TodoManual, Subject: "custom-x", Detail: keptWarningX, Kind: domain.GapWorkflowKept},
		{Category: domain.TodoManual, Subject: "custom-y", Detail: keptWarningY, Kind: domain.GapWorkflowKept},
		{Category: domain.TodoModels, Subject: "test-runner", Detail: "no model resolved", Kind: domain.GapNoModel},
	}

	out, _ := runRegionCommand(t, "update", summary)

	lines := warningLines(out)
	if len(lines) != 2 {
		t.Fatalf("warning lines = %d, want 2 (one per kept workflow); output:\n%s", len(lines), out)
	}
	for i, want := range []string{"warning: " + keptWarningX, "warning: " + keptWarningY} {
		if lines[i] != want {
			t.Errorf("warning line %d = %q, want %q", i, lines[i], want)
		}
	}
}
