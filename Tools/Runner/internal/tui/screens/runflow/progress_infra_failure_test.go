package runflow

// Tests of how the progress screen shows a failed infrastructure step: as
// failed, with status, error code and a short reason, and clearly different
// from a successful one. Other rows render as before.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func infraRow(status, code, reason string) ProgressRow {
	return ProgressRow{
		AgentInstance: "commit-manager-git#2", Phase: "EXECUTION", Status: status,
		Infrastructure: true, ErrorCode: code, Reason: reason,
	}
}

func progressViewOf(width int, rows ...ProgressRow) string {
	s := NewProgressScreen(width, 30, progressStyles())
	s.SetHistory(rows)
	return s.View()
}

func TestProgressScreen_FailedInfrastructureRow_ShowsAgentStatusErrorCodeAndReason(t *testing.T) {
	view := progressViewOf(100, infraRow("BLOCKED", "E101", "nothing to commit against"))

	want := "commit-manager-git#2  BLOCKED E101  infrastructure failed: nothing to commit against"
	if !strings.Contains(view, want) {
		t.Errorf("View() lacks %q:\n%s", want, view)
	}
}

func TestProgressScreen_FailedInfrastructureRow_WithoutErrorCode_OmitsTheCodePart(t *testing.T) {
	view := progressViewOf(100, infraRow("PARTIALLY_DONE", "", "half a checkpoint"))

	want := "commit-manager-git#2  PARTIALLY_DONE  infrastructure failed: half a checkpoint"
	if !strings.Contains(view, want) {
		t.Errorf("View() lacks %q:\n%s", want, view)
	}
}

func TestProgressScreen_FailedInfrastructureRow_EveryNonSuccessStatusIsShownAsFailed(t *testing.T) {
	for _, status := range []string{"BLOCKED", "CAPABILITY_EXCEEDED", "NEEDS_CLARIFICATION", "PARTIALLY_DONE", "COMPLETED_NEEDS_ACTION"} {
		t.Run(status, func(t *testing.T) {
			view := progressViewOf(100, infraRow(status, "", "the reason"))

			if !strings.Contains(view, "infrastructure failed: the reason") {
				t.Errorf("View() does not show a %s infrastructure row as failed:\n%s", status, view)
			}
		})
	}
}

func TestProgressScreen_SuccessfulInfrastructureRow_IsNotShownAsFailed(t *testing.T) {
	view := progressViewOf(100, infraRow("SUCCESS", "", "committed 3 files"))

	if strings.Contains(view, "infrastructure failed") {
		t.Errorf("View() shows a successful infrastructure step as failed:\n%s", view)
	}
	if strings.Contains(view, "committed 3 files") {
		t.Errorf("View() shows the reason of a successful step:\n%s", view)
	}
	if !strings.Contains(view, "commit-manager-git#2  SUCCESS") {
		t.Errorf("View() lacks the plain success row:\n%s", view)
	}
}

func TestProgressScreen_FailedWorkflowRow_IsNotShownAsInfrastructureFailure(t *testing.T) {
	row := ProgressRow{AgentInstance: "builder#2", Phase: "EXECUTION", Status: "BLOCKED", ErrorCode: "E101", Reason: "workflow reason"}

	view := progressViewOf(100, row)

	if strings.Contains(view, "infrastructure failed") || strings.Contains(view, "workflow reason") {
		t.Errorf("View() shows a workflow row as an infrastructure failure:\n%s", view)
	}
	if !strings.Contains(view, "builder#2  BLOCKED") {
		t.Errorf("View() lacks the workflow row as before:\n%s", view)
	}
}

func TestProgressScreen_RunningInfrastructureRow_IsNotShownAsFailed(t *testing.T) {
	view := progressViewOf(100, infraRow("running", "", "stale reason"))

	if strings.Contains(view, "infrastructure failed") {
		t.Errorf("View() shows a running infrastructure step as failed:\n%s", view)
	}
}

func TestProgressScreen_FailedInfrastructureRow_LongReasonIsTruncatedToOneScreenLine(t *testing.T) {
	// Wide enough for the row prefix plus the start of the reason, far narrower
	// than the whole reason.
	const width = 90
	reason := strings.Repeat("long reason words ", 20) + "TAILMARKER"

	view := progressViewOf(width, infraRow("BLOCKED", "E501", reason))

	var rowLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "commit-manager-git#2") {
			rowLine = line
		}
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line of width %d exceeds the screen width %d: %q", w, width, line)
		}
	}
	if !strings.Contains(rowLine, "infrastructure failed: long reason") {
		t.Errorf("the row line lacks the start of the reason: %q", rowLine)
	}
	if strings.Contains(view, "TAILMARKER") {
		t.Errorf("View() shows the end of an over-long reason; want it truncated:\n%s", view)
	}
}
