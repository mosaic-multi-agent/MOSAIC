package runflow

// progress_guard_test.go verifies the progress screen's anti-loop guard key:
// what a press records, when it is ignored, and the permanent guard-off
// indicator that is independent of the help bar.
//
// The screen is drawn at 140 columns so the indicator and the help bar do not
// wrap.

import (
	"strings"
	"testing"
)

const (
	guardOffIndicator = "LOOP GUARD OFF for this session - same-agent dispatches are not limited (g to turn it back on)"
	helpBarWithGuard  = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  a artifact-view  ctrl+c force-quit"
)

func newGuardScreen() *ProgressScreen {
	return NewProgressScreen(140, 24, progressStyles())
}

func TestProgressScreen_GuardKey_RecordsOneEventWithTheGuardOnAtThePress(t *testing.T) {
	for _, key := range []string{"g", "G"} {
		t.Run(key, func(t *testing.T) {
			s := newGuardScreen()

			pressProgressKey(s, key)
			events := s.TakeGuardKeyEvents()

			if len(events) != 1 {
				t.Fatalf("TakeGuardKeyEvents() returned %d events after %q, want 1", len(events), key)
			}
			if events[0].Key != key {
				t.Errorf("event Key = %q, want %q verbatim", events[0].Key, key)
			}
			if !events[0].Waive {
				t.Error("event Waive = false on a screen showing the guard on, want true")
			}
		})
	}
}

func TestProgressScreen_GuardKey_WhenShownOff_RecordsASwitchBackOn(t *testing.T) {
	s := newGuardScreen()
	s.SetGuardWaived(true)

	pressProgressKey(s, "g")
	events := s.TakeGuardKeyEvents()

	if len(events) != 1 || events[0].Waive {
		t.Fatalf("events = %+v, want one event with Waive false: a press while the guard is shown off switches it on", events)
	}
}

func TestProgressScreen_GuardKey_DoesNotChangeItsOwnDisplay(t *testing.T) {
	s := newGuardScreen()

	pressProgressKey(s, "g")

	if s.GuardWaived() {
		t.Error("GuardWaived() = true after a press: the root model owns the waiver and sets the display")
	}
}

func TestProgressScreen_GuardKey_TakeDrainsTheEvents(t *testing.T) {
	s := newGuardScreen()
	pressProgressKey(s, "g")
	if len(s.TakeGuardKeyEvents()) != 1 {
		t.Fatal("precondition: the press recorded no event")
	}

	if events := s.TakeGuardKeyEvents(); len(events) != 0 {
		t.Errorf("second TakeGuardKeyEvents() returned %d events, want 0", len(events))
	}
}

func TestProgressScreen_GuardKey_IgnoredWhileTheStopGateIsPending(t *testing.T) {
	s := newGuardScreen()
	pressProgressKey(s, "s")

	pressProgressKey(s, "g")

	if events := s.TakeGuardKeyEvents(); len(events) != 0 {
		t.Errorf("recorded %+v while the stop gate was pending, want nothing", events)
	}
}

func TestProgressScreen_GuardKey_IgnoredAfterAStopIsConfirmed(t *testing.T) {
	s := newGuardScreen()
	pressProgressKey(s, "s")
	pressProgressKey(s, "y")

	pressProgressKey(s, "g")

	if events := s.TakeGuardKeyEvents(); len(events) != 0 {
		t.Errorf("recorded %+v after a stop was confirmed, want nothing", events)
	}
}

func TestProgressScreen_GuardKey_DoesNotDisturbOtherKeys(t *testing.T) {
	s := newGuardScreen()

	pressProgressKey(s, "g")

	if s.ArtifactViewRequested() || s.GracefulStop() || s.ConfirmPending() {
		t.Error("the guard key raised an artifact view, a stop or the stop gate")
	}
	if events := s.TakeRequestKeyEvents(); len(events) != 0 {
		t.Errorf("the guard key recorded in-run request events %+v", events)
	}
}

func TestProgressScreen_View_GuardOff_ShowsThePermanentIndicator(t *testing.T) {
	s := newGuardScreen()
	s.SetGuardWaived(true)

	view := s.View()

	if !strings.Contains(view, guardOffIndicator) {
		t.Errorf("View() lacks the guard-off indicator %q:\n%s", guardOffIndicator, view)
	}
}

func TestProgressScreen_View_GuardOn_ShowsNoIndicator(t *testing.T) {
	s := newGuardScreen()

	if view := s.View(); strings.Contains(view, "LOOP GUARD OFF") {
		t.Errorf("View() shows the guard-off indicator with the guard on:\n%s", view)
	}
}

func TestProgressScreen_View_GuardSwitchedBackOn_RemovesTheIndicator(t *testing.T) {
	s := newGuardScreen()
	s.SetGuardWaived(true)
	if !strings.Contains(s.View(), guardOffIndicator) {
		t.Fatal("precondition: the indicator is not shown while the guard is off")
	}

	s.SetGuardWaived(false)

	if view := s.View(); strings.Contains(view, "LOOP GUARD OFF") {
		t.Errorf("View() still shows the indicator after the guard was switched on:\n%s", view)
	}
}

// The indicator is its own line above the help bar, so it does not depend on
// the help bar being drawn or on the other notices.
func TestProgressScreen_View_GuardOff_IndicatorIsItsOwnLineAboveTheHelpBar(t *testing.T) {
	s := newGuardScreen()
	s.SetGuardWaived(true)

	lines := strings.Split(s.View(), "\n")
	indicatorAt, helpAt := -1, -1
	for i, line := range lines {
		if strings.Contains(line, guardOffIndicator) {
			indicatorAt = i
			if strings.Contains(line, "graceful-stop") {
				t.Errorf("the indicator shares a line with the help bar: %q", line)
			}
		}
		if strings.Contains(line, "ctrl+c force-quit") {
			helpAt = i
		}
	}
	if indicatorAt < 0 || helpAt < 0 {
		t.Fatalf("indicator line %d, help bar line %d: both must be rendered", indicatorAt, helpAt)
	}
	if indicatorAt >= helpAt {
		t.Errorf("the indicator is on line %d, below or on the help bar line %d", indicatorAt, helpAt)
	}
}

func TestProgressScreen_View_GuardOff_IndicatorSurvivesRowsStatusAndStopNotices(t *testing.T) {
	s := newGuardScreen()
	s.SetGuardWaived(true)
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Phase: "PLANNING"})
	s.CompleteRow("SUCCESS")
	s.SetStatus("some status", false)
	s.SetPendingRequests(PendingRequests{ManualDispatch: true})

	pressProgressKey(s, "s")
	if view := s.View(); !strings.Contains(view, guardOffIndicator) {
		t.Errorf("the indicator is gone while the stop prompt is shown:\n%s", view)
	}
	pressProgressKey(s, "y")
	if view := s.View(); !strings.Contains(view, guardOffIndicator) {
		t.Errorf("the indicator is gone while the run is stopping:\n%s", view)
	}
}

func TestProgressScreen_View_HelpBarListsTheGuardKey(t *testing.T) {
	s := newGuardScreen()

	if view := s.View(); !strings.Contains(view, helpBarWithGuard) {
		t.Errorf("View() lacks the help bar %q:\n%s", helpBarWithGuard, view)
	}
}
