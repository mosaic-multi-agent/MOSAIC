package artifact_test

// Tests for the OrchestrationArtifactFormat 3.0 form of the written run
// artifact: the Execution Log `Iteration` column, the `current_state.iteration`
// key and the Artifacts registry `Created In` value.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

const (
	iterationHeaderLine = "| Seq | Agent | Phase | Iteration | WorkflowRow | Status | Timestamp | Summary | Inputs | Checkpoint |"
	iterationSepLine    = "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"
)

// v3LogState builds a state whose Execution Log holds one row per recorded
// value shape: grouped numeric ID, ungrouped numeric ID, grouped non-numeric
// ID, and a row outside every block.
func v3LogState() domain.ArtifactState {
	ts := time.Date(2026, 10, 7, 17, 40, 11, 0, time.UTC)
	return domain.ArtifactState{
		RunID: testRunID,
		ExecutionLog: []domain.ExecutionLogEntry{
			{Seq: 1, Agent: "writer#1", Phase: "EXECUTION", Stage: "1.Test", WorkflowRow: 3,
				Status: domain.StatusSUCCESS, Timestamp: ts, Summary: "grouped"},
			{Seq: 2, Agent: "builder#2", Phase: "EXECUTION", Stage: "1", WorkflowRow: 4,
				Status: domain.StatusSUCCESS, Timestamp: ts, Summary: "ungrouped"},
			{Seq: 3, Agent: "builder#3", Phase: "EXECUTION", Stage: "Acme.Test", WorkflowRow: 3,
				Status: domain.StatusSUCCESS, Timestamp: ts, Summary: "named"},
			{Seq: 4, Agent: "planner#4", Phase: "PLANNING", Stage: "", WorkflowRow: 1,
				Status: domain.StatusSUCCESS, Timestamp: ts, Summary: "outside"},
			{Seq: 5, Agent: "builder#5", Phase: "EXECUTION", Stage: "Acme", WorkflowRow: 4,
				Status: domain.StatusSUCCESS, Timestamp: ts, Summary: "named ungrouped"},
		},
	}
}

// v3FrontmatterLines renders the state and returns the frontmatter lines.
func v3FrontmatterLines(t *testing.T, state domain.ArtifactState) []string {
	t.Helper()
	data, err := artifact.Render(state)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	var out []string
	dashes := 0
	for _, l := range lines {
		if l == "---" {
			dashes++
			if dashes == 2 {
				return out
			}
			continue
		}
		if dashes == 1 {
			out = append(out, l)
		}
	}
	t.Fatalf("frontmatter end not found in:\n%s", data)
	return nil
}

func TestRender_ExecutionLogHeaderNamesTheIterationColumn(t *testing.T) {
	lines := renderBodyLines(t, v3LogState())

	indexOfLine(t, lines, iterationHeaderLine)
	indexOfLine(t, lines, iterationSepLine)
	for _, l := range lines {
		if strings.HasPrefix(l, "| Seq |") && strings.Contains(l, "| Stage |") {
			t.Errorf("Execution Log header still names a Stage column: %q", l)
		}
	}
}

func TestRender_IterationCellHoldsTheRecordedValue(t *testing.T) {
	lines := renderBodyLines(t, v3LogState())

	want := []string{
		"| 1 | writer#1 | EXECUTION | 1.Test | 3 | SUCCESS | 2026-10-07T17:40:11Z | grouped | - | - |",
		"| 2 | builder#2 | EXECUTION | 1 | 4 | SUCCESS | 2026-10-07T17:40:11Z | ungrouped | - | - |",
		"| 3 | builder#3 | EXECUTION | Acme.Test | 3 | SUCCESS | 2026-10-07T17:40:11Z | named | - | - |",
		"| 5 | builder#5 | EXECUTION | Acme | 4 | SUCCESS | 2026-10-07T17:40:11Z | named ungrouped | - | - |",
	}
	for _, w := range want {
		indexOfLine(t, lines, w)
	}
}

func TestRender_IterationCellIsDashOutsideEveryBlock(t *testing.T) {
	lines := renderBodyLines(t, v3LogState())

	indexOfLine(t, lines, "| 4 | planner#4 | PLANNING | - | 1 | SUCCESS | 2026-10-07T17:40:11Z | outside | - | - |")
}

func TestRender_CurrentStateWritesQuotedIterationValue(t *testing.T) {
	state := domain.ArtifactState{RunID: testRunID}
	state.CurrentState.Phase = "EXECUTION"
	state.CurrentState.Stage = "1.Test"

	lines := v3FrontmatterLines(t, state)

	found := false
	for _, l := range lines {
		if l == `  iteration: "1.Test"` {
			found = true
		}
		if strings.HasPrefix(l, "  stage:") {
			t.Errorf("current_state still carries a stage key: %q", l)
		}
	}
	if !found {
		t.Errorf("current_state has no `iteration: \"1.Test\"` line in:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRender_CurrentStateWritesNullIterationOutsideABlock(t *testing.T) {
	state := domain.ArtifactState{RunID: testRunID}
	state.CurrentState.Phase = "PLANNING"
	state.CurrentState.Stage = ""

	lines := v3FrontmatterLines(t, state)

	found := false
	for _, l := range lines {
		if l == "  iteration: null" {
			found = true
		}
		if strings.HasPrefix(l, "  stage:") {
			t.Errorf("current_state still carries a stage key: %q", l)
		}
	}
	if !found {
		t.Errorf("current_state has no `iteration: null` line in:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRender_CurrentStateWritesUngroupedNonNumericIteration(t *testing.T) {
	state := domain.ArtifactState{RunID: testRunID}
	state.CurrentState.Phase = "EXECUTION"
	state.CurrentState.Stage = "Acme"

	lines := v3FrontmatterLines(t, state)

	found := false
	for _, l := range lines {
		if l == `  iteration: "Acme"` {
			found = true
		}
	}
	if !found {
		t.Errorf("current_state has no `iteration: \"Acme\"` line in:\n%s", strings.Join(lines, "\n"))
	}
}

func TestApply_CreatedInCarriesPhaseAndIteration(t *testing.T) {
	store, state := mustCreateStore(t)
	ts := time.Date(2026, 10, 7, 17, 40, 11, 0, time.UTC)
	step := newTestStep(1, "writer#1", "EXECUTION", "1.Test", domain.StatusSUCCESS, ts, []string{"Out.md"})

	after, err := store.Apply(context.Background(), state, step)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(after.ArtifactRegistry) != 1 {
		t.Fatalf("ArtifactRegistry: want 1 entry, got %d", len(after.ArtifactRegistry))
	}
	if got := after.ArtifactRegistry[0].CreatedIn; got != "EXECUTION.1.Test" {
		t.Errorf("CreatedIn: want %q, got %q", "EXECUTION.1.Test", got)
	}
}

func TestApply_CreatedInIsThePhaseAloneOutsideEveryBlock(t *testing.T) {
	store, state := mustCreateStore(t)
	ts := time.Date(2026, 10, 7, 17, 40, 11, 0, time.UTC)
	step := newTestStep(1, "planner#1", "PLANNING", "", domain.StatusSUCCESS, ts, []string{"Plan.md"})

	after, err := store.Apply(context.Background(), state, step)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(after.ArtifactRegistry) != 1 {
		t.Fatalf("ArtifactRegistry: want 1 entry, got %d", len(after.ArtifactRegistry))
	}
	if got := after.ArtifactRegistry[0].CreatedIn; got != "PLANNING" {
		t.Errorf("CreatedIn: want %q, got %q", "PLANNING", got)
	}
}

func TestApply_WritesTheIterationFormToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Orchestration.md")
	store := artifact.NewFileStore(path)
	ctx := context.Background()
	info := domain.WorkflowInfo{ID: "quick-fix", Version: "3.0"}
	state, err := store.Create(ctx, info, "task", domain.RunSettings{}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), testRunID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ts := time.Date(2026, 10, 7, 17, 40, 11, 0, time.UTC)
	step := newTestStep(1, "writer#1", "EXECUTION", "1.Test", domain.StatusSUCCESS, ts, nil)

	if _, err := store.Apply(ctx, state, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data := readFile(t, path)
	for _, want := range []string{"| Iteration |", `  iteration: "1.Test"`, "| EXECUTION | 1.Test |"} {
		if !strings.Contains(data, want) {
			t.Errorf("artifact on disk lacks %q:\n%s", want, data)
		}
	}
	if strings.Contains(data, "| Stage |") || strings.Contains(data, "\n  stage:") {
		t.Errorf("artifact on disk still carries the pre-3.0 Stage form:\n%s", data)
	}
}

func TestApply_WrittenFileParsesBackToTheAppliedIteration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Orchestration.md")
	store := artifact.NewFileStore(path)
	ctx := context.Background()
	info := domain.WorkflowInfo{ID: "quick-fix", Version: "3.0"}
	state, err := store.Create(ctx, info, "task", domain.RunSettings{}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), testRunID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ts := time.Date(2026, 10, 7, 17, 40, 11, 0, time.UTC)
	step := newTestStep(1, "writer#1", "EXECUTION", "2.Test", domain.StatusSUCCESS, ts, []string{"Out.md"})

	if _, err := store.Apply(ctx, state, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	parsed, err := artifact.Parse([]byte(readFile(t, path)))
	if err != nil {
		t.Fatalf("Parse written file: %v", err)
	}

	if len(parsed.ExecutionLog) != 1 || parsed.ExecutionLog[0].Stage != "2.Test" {
		t.Errorf("ExecutionLog after Apply and Parse: %+v", parsed.ExecutionLog)
	}
	if parsed.CurrentState.Stage != "2.Test" {
		t.Errorf("current_state recorded value: want %q, got %q", "2.Test", parsed.CurrentState.Stage)
	}
	if len(parsed.ArtifactRegistry) != 1 || parsed.ArtifactRegistry[0].CreatedIn != "EXECUTION.2.Test" {
		t.Errorf("ArtifactRegistry after Apply and Parse: %+v", parsed.ArtifactRegistry)
	}
}
