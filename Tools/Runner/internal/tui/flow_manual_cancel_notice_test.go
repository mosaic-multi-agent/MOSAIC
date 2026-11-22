package tui

// The progress screen shows the notice the session sends when the user cancels
// an in-run manual dispatch request, without mistaking it for a step status.

import (
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

func TestFlow_ManualDispatchCancelNotice_ShowsOnTheStatusLineWithoutTouchingTheRows(t *testing.T) {
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	m.progressScreen.AppendRow(progressRow{AgentInstance: "agent-a#1", Phase: "PLANNING", Status: "running"})
	rowsBefore := len(m.progressScreen.Rows())
	statusBefore := m.progressScreen.Rows()[0].Status

	m.Update(questionMsg{
		kind: questionNotice,
		notice: interaction.Notice{
			Level:   interaction.NoticeInfo,
			Title:   "Manual dispatch",
			Message: "Manual dispatch request cancelled; the run continues as if it had not been requested.",
		},
	})

	view := m.progressScreen.View()
	if !containsStr(view, "cancelled") {
		t.Errorf("want the cancel shown on the progress screen, got:\n%s", view)
	}
	rows := m.progressScreen.Rows()
	if len(rows) != rowsBefore {
		t.Errorf("want no row added by the notice, got %d rows (was %d)", len(rows), rowsBefore)
	}
	if rows[0].Status != statusBefore {
		t.Errorf("want the running row status untouched (%q), got %q", statusBefore, rows[0].Status)
	}
}
