package runflow

// What the progress screen draws for the consultation row: its label, its
// running indicator, its position and its difference from a subagent row.
// Drawn at 120 columns so no line wraps.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestProgressScreen_View_ConsultationRowLabels(t *testing.T) {
	cases := []struct {
		name     string
		activity domain.ConsultationActivity
		want     string
	}{
		{"routing without kind", routingNoKind, "consultation: routing"},
		{"routing with kind", routingHarnessKd, "consultation: routing (harness-error)"},
		{"schedule defect kind", domain.ConsultationActivity{Context: domain.ConsultContextRouting, Kind: domain.DeviationScheduleDefect},
			"consultation: routing (schedule-defect)"},
		{"pre-consultation", preConsultation, "consultation: pre-consultation"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewProgressScreen(120, 24, progressStyles())

			s.StartConsultation(c.activity)

			if view := s.View(); !strings.Contains(view, "\u25b6 "+c.want) {
				t.Errorf("View() lacks the running consultation row %q:\n%s", "\u25b6 "+c.want, view)
			}
		})
	}
}

func TestProgressScreen_View_RoutingWithoutKind_ShowsNoKindParentheses(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.StartConsultation(routingNoKind)

	view := s.View()
	if !strings.Contains(view, "consultation: routing") {
		t.Fatalf("View() shows no consultation row:\n%s", view)
	}
	if strings.Contains(view, "consultation: routing (") {
		t.Errorf("View() shows a kind for a consultation without one:\n%s", view)
	}
}

func TestProgressScreen_View_ConsultationRowIsTheLastRowAfterStepRows(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "done#1", Phase: "PLANNING", Status: "SUCCESS"})
	s.AppendRow(ProgressRow{AgentInstance: "agent-b#2", Phase: "EXECUTION", Status: "running"})

	s.StartConsultation(routingNoKind)

	view := s.View()
	step, consult := strings.Index(view, "agent-b#2"), strings.Index(view, "consultation: routing")
	if step < 0 || consult < 0 || consult < step {
		t.Errorf("want the consultation row after the step rows (step at %d, consultation at %d):\n%s", step, consult, view)
	}
}

func TestProgressScreen_View_SubagentRunningRowIsNotLabelledAsConsultation(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "orchestrator#2", Phase: "EXECUTION", Status: "running"})

	view := s.View()

	if strings.Contains(view, "consultation") {
		t.Errorf("View() labels a subagent row as a consultation:\n%s", view)
	}
}

func TestProgressScreen_View_EndedConsultationLeavesNoTrace(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(routingHarnessKd)
	if !strings.Contains(s.View(), "consultation") {
		t.Fatal("scenario did not run as intended: the consultation row was never drawn")
	}

	s.EndConsultation()

	if view := s.View(); strings.Contains(view, "consultation") {
		t.Errorf("View() still shows the ended consultation:\n%s", view)
	}
}

func TestProgressScreen_View_ConsultationRowCountsTowardTheVisibleTail(t *testing.T) {
	s := NewProgressScreen(120, 10, progressStyles())
	for i := 1; i <= 20; i++ {
		s.AppendRow(ProgressRow{AgentInstance: fmt.Sprintf("agent#%d", i), Phase: "EXECUTION", Status: "SUCCESS"})
		s.CompleteRow("SUCCESS")
	}

	s.StartConsultation(routingNoKind)

	view := s.View()
	if !strings.Contains(view, "consultation: routing") {
		t.Errorf("View() hides the consultation row behind older rows:\n%s", view)
	}
	if !strings.Contains(view, "agent#20 ") {
		t.Errorf("View() lost the newest step row:\n%s", view)
	}
	// Visible rows = height-6 = 4: three step rows and the consultation row.
	if strings.Contains(view, "agent#17 ") {
		t.Errorf("View() shows a fourth step row although the consultation row takes tail space:\n%s", view)
	}
}
