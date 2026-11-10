package runflow

// What the progress screen shows for a repair consultation: the damage note on
// the running row, the repair outcome lines that outlive the row, and the
// unchanged rendering of consultations without damage.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

var repairRouting = domain.ConsultationActivity{Context: domain.ConsultContextRouting, RepairAttempt: 1}

func TestProgressScreen_View_RepairConsultationRowNotesTheDamage(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.StartConsultation(repairRouting)

	view := s.View()
	if !strings.Contains(view, "consultation: routing") {
		t.Fatalf("View() lacks the consultation row:\n%s", view)
	}
	if !strings.Contains(view, "damaged") {
		t.Errorf("View() does not note that the artifact was damaged:\n%s", view)
	}
}

func TestProgressScreen_View_RepairConsultationRowShowsTheAttemptNumber(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.StartConsultation(domain.ConsultationActivity{Context: domain.ConsultContextRouting, RepairAttempt: 2})

	if view := s.View(); !strings.Contains(view, "repair 2") {
		t.Errorf("View() does not show repair attempt 2:\n%s", view)
	}
}

func TestProgressScreen_View_ConsultationWithoutDamage_RendersAsBefore(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.StartConsultation(routingHarnessKd)

	view := s.View()
	if !strings.Contains(view, "▶ consultation: routing (harness-error)") {
		t.Errorf("View() changed the ordinary consultation row:\n%s", view)
	}
	for _, word := range []string{"damaged", "repair"} {
		if strings.Contains(view, word) {
			t.Errorf("View() mentions %q for a consultation without damage:\n%s", word, view)
		}
	}
}

func TestProgressScreen_View_AcceptedRepairOutcome_StaysVisibleAfterTheConsultationEnds(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.StartConsultation(repairRouting)
	s.EndConsultation()

	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Accepted: true})

	view := s.View()
	if strings.Contains(view, "consultation") {
		t.Errorf("View() still shows the ended consultation row:\n%s", view)
	}
	if !strings.Contains(view, "accepted") {
		t.Errorf("View() does not show the accepted repair after the consultation ended:\n%s", view)
	}
	if strings.Contains(view, "not accepted") {
		t.Errorf("View() shows an accepted repair as not accepted:\n%s", view)
	}
}

func TestProgressScreen_View_RejectedRepairOutcome_ShowsNotAcceptedAndRemainingProblems(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Accepted: false, Problems: 7})

	view := s.View()
	if !strings.Contains(view, "not accepted") {
		t.Errorf("View() does not show that the repair was not accepted:\n%s", view)
	}
	if !strings.Contains(view, "7 problems") {
		t.Errorf("View() does not show the number of remaining problems:\n%s", view)
	}
}

func TestProgressScreen_View_RepairOutcomes_ShownInOrderRecorded(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())

	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Accepted: false, Problems: 2})
	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 2, Accepted: true})

	view := s.View()
	first := strings.Index(view, "attempt 1 of 2")
	second := strings.Index(view, "attempt 2 of 2")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("want the attempt 1 line before the attempt 2 line (at %d, %d):\n%s", first, second, view)
	}
	if !strings.Contains(view[first:second], "not accepted") {
		t.Errorf("want attempt 1 shown as not accepted:\n%s", view)
	}
	if strings.Contains(view[second:], "not accepted") {
		t.Errorf("want attempt 2 shown as accepted:\n%s", view)
	}
}

func TestProgressScreen_RepairOutcomes_ReturnsRecordedOutcomesInOrder(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	first := domain.RepairOutcome{Attempt: 1, Problems: 3}
	second := domain.RepairOutcome{Attempt: 2, Accepted: true}

	s.RecordRepairOutcome(first)
	s.RecordRepairOutcome(second)

	got := s.RepairOutcomes()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Errorf("RepairOutcomes() = %+v, want [%+v %+v]", got, first, second)
	}
}

func TestProgressScreen_View_RepairOutcomeLinesSitBetweenStepRowsAndTheConsultationRow(t *testing.T) {
	s := NewProgressScreen(120, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-b#2", Phase: "EXECUTION", Status: "SUCCESS"})
	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Problems: 1})

	s.StartConsultation(repairRouting)

	view := s.View()
	step, outcome, consult := strings.Index(view, "agent-b#2"), strings.Index(view, "not accepted"), strings.Index(view, "consultation: routing")
	if step < 0 || outcome < 0 || consult < 0 || !(step < outcome && outcome < consult) {
		t.Errorf("want step row, outcome line, consultation row in that order (%d, %d, %d):\n%s", step, outcome, consult, view)
	}
}
