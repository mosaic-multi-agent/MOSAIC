package runflow

// Tests of the done screen's list of infrastructure-agent failures: every
// failure of the run is listed below the outcome message, also when the run
// ended with an error, and nothing is listed for a run without failures.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func doneInfraFailures() []domain.InfraFailure {
	return []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusBLOCKED, ErrorCode: domain.ErrorINPUT_NOT_FOUND, Summary: "nothing to commit against"},
		{AgentInstance: "checkpoint-manager-git#5", Status: domain.StatusPARTIALLY_DONE, Summary: "half a checkpoint"},
	}
}

func requireDoneFailureList(t *testing.T, view string) {
	t.Helper()
	pos := 0
	for _, want := range []string{
		"Infrastructure failures in this run (2):",
		"  commit-manager-git#2  BLOCKED E101  nothing to commit against",
		"  checkpoint-manager-git#5  PARTIALLY_DONE  half a checkpoint",
	} {
		i := strings.Index(view[pos:], want)
		if i < 0 {
			t.Errorf("View() lacks %q (in order):\n%s", want, view)
			return
		}
		pos += i + len(want)
	}
}

func TestDoneScreen_View_ListsEveryInfrastructureFailureBelowTheMessage(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunCompleted, domain.RunStopped, domain.RunFailed, domain.RunDeviationUnresolved} {
		t.Run(string(status), func(t *testing.T) {
			outcome := domain.RunOutcome{Status: status, Message: "outcome message text", InfraFailures: doneInfraFailures()}
			s := NewDoneScreen(outcome, "", 100, 40, progressStyles())

			view := s.View()

			requireDoneFailureList(t, view)
			if strings.Index(view, "outcome message text") > strings.Index(view, "Infrastructure failures in this run") {
				t.Errorf("the failure list must come below the outcome message:\n%s", view)
			}
		})
	}
}

func TestDoneScreen_View_StartError_ListsTheReturnedOutcomesFailuresBelowTheError(t *testing.T) {
	outcome := domain.RunOutcome{Status: domain.RunFailed, InfraFailures: doneInfraFailures()}
	s := NewDoneScreen(outcome, "artifact write failed", 100, 40, progressStyles())

	view := s.View()

	if !strings.Contains(view, "artifact write failed") {
		t.Errorf("View() lacks the error text:\n%s", view)
	}
	requireDoneFailureList(t, view)
}

func TestDoneScreen_View_NoInfrastructureFailures_ListsNothing(t *testing.T) {
	cases := map[string]*DoneScreen{
		"outcome":  NewDoneScreen(domain.RunOutcome{Status: domain.RunCompleted, Message: "all good"}, "", 100, 40, progressStyles()),
		"error":    NewDoneScreen(domain.RunOutcome{}, "panic recovered", 100, 40, progressStyles()),
		"explicit": NewDoneScreen(domain.RunOutcome{Status: domain.RunStopped, InfraFailures: []domain.InfraFailure{}}, "", 100, 40, progressStyles()),
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if view := s.View(); strings.Contains(view, "Infrastructure failures") {
				t.Errorf("View() mentions infrastructure failures for a run without any:\n%s", view)
			}
		})
	}
}
