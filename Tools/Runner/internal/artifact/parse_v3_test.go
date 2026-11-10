package artifact_test

// Tests that a rendered 3.0 run artifact parses back to the same state and
// that a pre-3.0 artifact (Stage column, current_state.stage) is reported as
// the pre-3.0 layout.

import (
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// v3Document builds a run artifact with one Execution Log row. logColumn is
// the header of the recorded-value column ("Iteration" or "Stage"), stateKey is
// the current_state key ("iteration" or "stage"), and logCell and stateValue
// are the raw cell and the raw frontmatter value.
func v3Document(logColumn, logCell, stateKey, stateValue string) []byte {
	return []byte("---\n" +
		"type: orchestration-artifact\n" +
		"run_id: " + testRunID + "\n" +
		"workflow: test\n" +
		"workflow_version: \"1.0\"\n" +
		"task: \"test\"\n" +
		"started: 2026-01-01T00:00:00Z\n" +
		"last_updated: 2026-01-01T01:00:00Z\n" +
		"global_sequence: 1\n" +
		"checkpoints: disabled\n" +
		"current_state:\n" +
		"  phase: EXECUTION\n" +
		"  " + stateKey + ": " + stateValue + "\n" +
		"  last_status: SUCCESS\n" +
		"  last_agent: \"writer#1\"\n" +
		"  error_code: null\n" +
		"---\n" +
		"\n" +
		"<ExecutionLog type=\"core\">\n" +
		"\n" +
		"| Seq | Agent | Phase | " + logColumn + " | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |\n" +
		"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n" +
		"| 1 | writer#1 | EXECUTION | " + logCell + " | 3 | SUCCESS | 2026-01-01T01:00:00Z | done | - | - |\n" +
		"\n" +
		"</ExecutionLog>\n" +
		"\n" +
		"<Artifacts type=\"core\">\n" +
		"\n" +
		"| Artifact | Created In | Created By |\n" +
		"| --- | --- | --- |\n" +
		"\n" +
		"</Artifacts>\n" +
		"\n" +
		"<WorkflowNotes type=\"core\">\n" +
		"\n" +
		"| Seq | Note |\n" +
		"| --- | --- |\n" +
		"\n" +
		"</WorkflowNotes>\n")
}

func TestParse_IterationColumnAndKey_ReadAsTheRecordedValue(t *testing.T) {
	state, err := artifact.Parse(v3Document("Iteration", "1.Test", "iteration", `"1.Test"`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(state.ExecutionLog) != 1 {
		t.Fatalf("ExecutionLog: want 1 row, got %d", len(state.ExecutionLog))
	}
	if got := state.ExecutionLog[0].Stage; got != "1.Test" {
		t.Errorf("log recorded value: want %q, got %q", "1.Test", got)
	}
	if got := state.ExecutionLog[0].WorkflowRow; got != 3 {
		t.Errorf("log WorkflowRow: want 3, got %d", got)
	}
	if got := state.CurrentState.Stage; got != "1.Test" {
		t.Errorf("current_state recorded value: want %q, got %q", "1.Test", got)
	}
}

// The Iteration column must be read for a dash row to be told apart from a
// valued row: the valued row sits beside the dash row in the same document.
func TestParse_IterationDashCell_ReadAsOutsideEveryBlockBesideAValuedRow(t *testing.T) {
	doc := string(v3Document("Iteration", "-", "iteration", `"1.Test"`))
	dashRow := "| 1 | writer#1 | EXECUTION | - | 3 | SUCCESS | 2026-01-01T01:00:00Z | done | - | - |\n"
	valuedRow := "| 2 | writer#1 | EXECUTION | 1.Test | 3 | SUCCESS | 2026-01-01T01:00:00Z | done | - | - |\n"
	if !strings.Contains(doc, dashRow) {
		t.Fatalf("fixture row not found in:\n%s", doc)
	}
	doc = strings.Replace(doc, dashRow, dashRow+valuedRow, 1)

	state, err := artifact.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(state.ExecutionLog) != 2 {
		t.Fatalf("ExecutionLog: want 2 rows, got %d", len(state.ExecutionLog))
	}
	if got := state.ExecutionLog[0].Stage; got != "" {
		t.Errorf("log recorded value for %q: want empty, got %q", "-", got)
	}
	if got := state.ExecutionLog[1].Stage; got != "1.Test" {
		t.Errorf("log recorded value beside the dash row: want %q, got %q", "1.Test", got)
	}
}

// Support coverage: a null iteration key is the empty value.
func TestParse_IterationNull_ReadAsOutsideEveryBlock(t *testing.T) {
	state, err := artifact.Parse(v3Document("Iteration", "-", "iteration", "null"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := state.CurrentState.Stage; got != "" {
		t.Errorf("current_state recorded value for null: want empty, got %q", got)
	}
}

func TestParse_PreIterationStageColumnAndKey_ReportedAsThePre30Layout(t *testing.T) {
	_, err := artifact.Parse(v3Document("Stage", "Test.1", "stage", "Test.1"))

	requirePre30Layout(t, err)
}

func TestParse_PreIterationStageNull_ReportedAsThePre30Layout(t *testing.T) {
	_, err := artifact.Parse(v3Document("Stage", "-", "stage", "null"))

	requirePre30Layout(t, err)
}

func TestRoundTrip_RenderedIterationForm_ParsesBackToTheSameState(t *testing.T) {
	original := v3LogState()
	original.Type = "orchestration-artifact"
	original.Workflow = "test"
	original.CurrentState.Phase = "EXECUTION"
	original.CurrentState.Stage = "Acme.Test"
	original.ArtifactRegistry = []domain.ArtifactRegistryEntry{
		{Artifact: "Plan.md", CreatedIn: "PLANNING", CreatedBy: "planner#4"},
		{Artifact: "Out.md", CreatedIn: "EXECUTION.1.Test", CreatedBy: "writer#1"},
	}

	rendered, err := artifact.Render(original)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(rendered), "| Iteration |") {
		t.Fatalf("rendered artifact has no Iteration column:\n%s", rendered)
	}
	parsed, err := artifact.Parse(rendered)
	if err != nil {
		t.Fatalf("Parse rendered bytes: %v", err)
	}

	if len(parsed.ExecutionLog) != len(original.ExecutionLog) {
		t.Fatalf("ExecutionLog rows: want %d, got %d", len(original.ExecutionLog), len(parsed.ExecutionLog))
	}
	for i, want := range original.ExecutionLog {
		got := parsed.ExecutionLog[i]
		if got.Stage != want.Stage || got.WorkflowRow != want.WorkflowRow || got.Phase != want.Phase {
			t.Errorf("row %d: want recorded value %q phase %q row %d, got %q %q %d",
				i+1, want.Stage, want.Phase, want.WorkflowRow, got.Stage, got.Phase, got.WorkflowRow)
		}
	}
	if parsed.CurrentState.Stage != "Acme.Test" {
		t.Errorf("current_state recorded value: want %q, got %q", "Acme.Test", parsed.CurrentState.Stage)
	}
	if len(parsed.ArtifactRegistry) != 2 || parsed.ArtifactRegistry[1].CreatedIn != "EXECUTION.1.Test" {
		t.Errorf("ArtifactRegistry after round trip: %+v", parsed.ArtifactRegistry)
	}
}

func TestRoundTrip_RenderedOutsideEveryBlock_ParsesBackToEmpty(t *testing.T) {
	original := domain.ArtifactState{RunID: testRunID, Type: "orchestration-artifact", Workflow: "test"}
	original.CurrentState.Phase = "PLANNING"

	rendered, err := artifact.Render(original)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(rendered), "iteration: null") {
		t.Fatalf("rendered artifact has no `iteration: null`:\n%s", rendered)
	}
	parsed, err := artifact.Parse(rendered)
	if err != nil {
		t.Fatalf("Parse rendered bytes: %v", err)
	}

	if parsed.CurrentState.Stage != "" {
		t.Errorf("current_state recorded value: want empty, got %q", parsed.CurrentState.Stage)
	}
}
