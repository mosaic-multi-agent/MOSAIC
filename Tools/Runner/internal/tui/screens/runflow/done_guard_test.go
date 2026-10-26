package runflow

// done_guard_test.go verifies the ending screen's resume with the anti-loop
// guard off: it is offered only for an outcome the guard caused.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

const guardOffHelp = "g  continue with loop guard off"

func newGuardDoneScreen(status domain.RunStatus, antiLoopStop bool) *DoneScreen {
	outcome := domain.RunOutcome{Status: status, Message: "test outcome", AntiLoopStop: antiLoopStop}
	return NewDoneScreen(outcome, "", 120, 24, progressStyles())
}

func TestDoneScreen_GuardStop_GKeyChoosesResumeWithTheGuardOff(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunStopped, domain.RunDeviationUnresolved} {
		for _, key := range []string{"g", "G"} {
			t.Run(string(status)+"/"+key, func(t *testing.T) {
				s := newGuardDoneScreen(status, true)

				pressDoneKey(s, key)

				if !s.ContinueWithGuardOff() {
					t.Errorf("ContinueWithGuardOff() = false after %q on a guard stop (%s), want true", key, status)
				}
				if s.Done() {
					t.Error("Done() = true after the guard-off choice: it is a restart, not an exit")
				}
			})
		}
	}
}

func TestDoneScreen_GuardStop_ShowsTheChoiceInTheHelpLine(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunStopped, domain.RunDeviationUnresolved} {
		t.Run(string(status), func(t *testing.T) {
			s := newGuardDoneScreen(status, true)

			if view := s.View(); !strings.Contains(view, guardOffHelp) {
				t.Errorf("View() lacks %q:\n%s", guardOffHelp, view)
			}
		})
	}
}

func TestDoneScreen_GuardStop_ContinueIsStillOfferedForStoppedOnly(t *testing.T) {
	stopped := newGuardDoneScreen(domain.RunStopped, true)
	pressDoneKey(stopped, "c")
	if !stopped.Continue() {
		t.Error("Continue() = false on a guard-stopped run, want true")
	}
	if stopped.ContinueWithGuardOff() {
		t.Error("a plain Continue also chose the guard-off resume")
	}

	deviation := newGuardDoneScreen(domain.RunDeviationUnresolved, true)
	pressDoneKey(deviation, "c")
	if deviation.Continue() {
		t.Error("Continue() = true for RunDeviationUnresolved, want false (unchanged)")
	}
}

// Regression coverage: the key is inert unless the outcome carries the marker.
func TestDoneScreen_NotAGuardStop_OffersNoGuardOffChoice(t *testing.T) {
	for _, status := range []domain.RunStatus{
		domain.RunStopped, domain.RunDeviationUnresolved, domain.RunCompleted,
		domain.RunFailed, domain.RunRefused, domain.RunStartFailed, domain.RunStoppedByConsultant,
	} {
		t.Run(string(status), func(t *testing.T) {
			s := newGuardDoneScreen(status, false)

			pressDoneKey(s, "g")

			if s.ContinueWithGuardOff() {
				t.Errorf("ContinueWithGuardOff() = true for %s without AntiLoopStop, want false", status)
			}
			if strings.Contains(s.View(), guardOffHelp) {
				t.Errorf("View() offers the guard-off resume for %s without AntiLoopStop", status)
			}
		})
	}
}

func TestDoneScreen_GuardOffChoice_IsFalseUntilChosen(t *testing.T) {
	s := newGuardDoneScreen(domain.RunStopped, true)

	if s.ContinueWithGuardOff() {
		t.Error("ContinueWithGuardOff() = true on a freshly constructed screen, want false")
	}
}
