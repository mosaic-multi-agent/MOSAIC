package workflow_test

// Routing-cell parsing of the strict parser on whole tables: the 1.x "-" in On
// Success, and "-" in Output.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

func TestParse_OnSuccessDash_RefusedByStrictParser(t *testing.T) {
	// On Success "-" is 1.x notation. The strict parser refuses it (T5) naming
	// the row and the cell.
	info := domain.WorkflowInfo{ID: "on-success-dash", Version: "1.0"}

	_, err := workflow.Parse([]byte(onSuccessDashContent), info)

	re := asRefusalError(t, err)
	if re.Rule != "T5" {
		t.Errorf("Rule: want T5, got %q", re.Rule)
	}
	if re.Row != 1 {
		t.Errorf("Row: want 1 (1-based), got %d", re.Row)
	}
	if re.Token != "-" {
		t.Errorf("Token: want the offending cell %q, got %q", "-", re.Token)
	}
	if !strings.Contains(re.Resource, "on-success-dash") {
		t.Errorf("Resource must name the workflow id, got %q", re.Resource)
	}
}

func TestParse_OutputDash_ProducesEmptyArtifacts(t *testing.T) {
	// Output column value "-" must produce an empty OutputArtifacts slice,
	// symmetric with the Input column behavior.
	table := mustParseFixture(t, outputDashContent, "output-dash", "1.0")

	if len(table.Rows[0].OutputArtifacts) != 0 {
		t.Errorf("Rows[0].OutputArtifacts: want empty (Output is \"-\"), got %v", table.Rows[0].OutputArtifacts)
	}
}
