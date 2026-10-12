package artifact_test

// Tests that a pre-3.0 run artifact layout (an Execution Log Stage column or a
// current_state.stage key) is reported as artifact.ErrPre30Layout instead of
// being read, and that the 3.0 layout beside it is not mistaken for it.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func requirePre30Layout(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Parse of a pre-3.0 layout succeeded, want a refusal")
	}
	var refusal *domain.RefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("Parse: want a *domain.RefusalError, got %T: %v", err, err)
	}
	if !errors.Is(err, artifact.ErrPre30Layout) {
		t.Errorf("Parse error does not match artifact.ErrPre30Layout: %v", err)
	}
}

func TestParse_ExecutionLogStageColumn_IsReportedAsThePre30Layout(t *testing.T) {
	_, err := artifact.Parse(v3Document("Stage", "Test.1", "iteration", `"1.Test"`))

	requirePre30Layout(t, err)
}

func TestParse_CurrentStateStageKey_IsReportedAsThePre30Layout(t *testing.T) {
	_, err := artifact.Parse(v3Document("Iteration", "1.Test", "stage", "Test.1"))

	requirePre30Layout(t, err)
}

// The old ungrouped value form does not change the answer: the layout alone
// marks a pre-3.0 artifact.
func TestParse_StageColumnWithStageDashValue_IsReportedAsThePre30Layout(t *testing.T) {
	_, err := artifact.Parse(v3Document("Stage", "Stage-1", "stage", "Stage-1"))

	requirePre30Layout(t, err)
}

// An artifact in the 3.0 layout carrying an ungrouped ID that starts with
// "Stage-" is an ordinary 3.0 artifact: it parses, and the value is kept as
// written.
func TestParse_ThreeOhLayoutWithStageDashID_ParsesAndKeepsTheValue(t *testing.T) {
	state, err := artifact.Parse(v3Document("Iteration", "Stage-1", "iteration", `"Stage-1"`))

	if err != nil {
		t.Fatalf("Parse of a 3.0 layout: %v", err)
	}
	if got := state.ExecutionLog[0].Stage; got != "Stage-1" {
		t.Errorf("log recorded value: want %q, got %q", "Stage-1", got)
	}
	if got := state.CurrentState.Stage; got != "Stage-1" {
		t.Errorf("current_state recorded value: want %q, got %q", "Stage-1", got)
	}
}

// A column that merely contains the word is not the pre-3.0 column: the
// Summary text of a 3.0 row may talk about a stage.
func TestParse_ThreeOhLayoutWhoseSummaryMentionsStage_IsNotThePre30Layout(t *testing.T) {
	doc := strings.Replace(string(v3Document("Iteration", "1.Test", "iteration", `"1.Test"`)),
		"| done |", "| Stage column removed |", 1)

	_, err := artifact.Parse([]byte(doc))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
}
