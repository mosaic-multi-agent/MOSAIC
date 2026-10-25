package screens_test

// View switching, back, identity and help text of the timeline view and of the
// other per-run views towards it.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var _ screens.RunView = (*screens.TimelineScreen)(nil)

func timelinePerRunView() perRunView {
	return perRunView{screens.ScreenTimeline, screens.KeyViewTimeline, func(r domain.RunReport) screens.RunView {
		return screens.NewTimelineScreen(r, wideW, tallH, plain())
	}}
}

func TestTimelineView_IdentifiesItself(t *testing.T) {
	if got := screens.NewTimelineScreen(timelineRun(), wideW, tallH, plain()).ID(); got != screens.ScreenTimeline {
		t.Errorf("timeline screen ID = %q, want %q", got, screens.ScreenTimeline)
	}
}

func TestTimelineView_EveryOtherViewKeyRequestsTheMatchingSibling(t *testing.T) {
	tl := timelinePerRunView()
	for _, to := range allPerRunViews() {
		s := tl.make(timelineRun())
		press(s, to.key)

		got, ok := s.Requested()
		if !ok || got != to.id {
			t.Errorf("key %q on the timeline: Requested() = (%q, %v), want (%q, true)", to.key, got, ok, to.id)
		}
		s.Reset()
		if _, ok := s.Requested(); ok {
			t.Errorf("Reset must clear the request for key %q", to.key)
		}
	}
}

func TestTimelineView_EveryOtherViewReachesTheTimelineWithItsKey(t *testing.T) {
	for _, from := range allPerRunViews() {
		s := from.make(timelineRun())
		press(s, screens.KeyViewTimeline)

		got, ok := s.Requested()
		if !ok || got != screens.ScreenTimeline {
			t.Errorf("%s: key %q: Requested() = (%q, %v), want (%q, true)", from.id, screens.KeyViewTimeline, got, ok, screens.ScreenTimeline)
		}
	}
}

func TestTimelineView_OwnKeyRequestsNothing(t *testing.T) {
	s := timelinePerRunView().make(timelineRun())
	press(s, screens.KeyViewTimeline)

	if got, ok := s.Requested(); ok {
		t.Errorf("the timeline's own key must not request a switch, got %q", got)
	}
}

func TestTimelineView_EscAndQAskToLeave(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		s := timelinePerRunView().make(timelineRun())
		press(s, k)

		if !s.Back() {
			t.Errorf("key %q must set Back()", k)
		}
		s.Reset()
		if s.Back() {
			t.Errorf("Reset must clear Back after %q", k)
		}
	}
}

func TestTimelineView_HelpListsEveryOtherViewKeyAndBack(t *testing.T) {
	help := timelinePerRunView().make(timelineRun()).Help()

	for _, to := range allPerRunViews() {
		if !strings.Contains(help, to.key+" ") {
			t.Errorf("help %q must list key %q for the %s view", help, to.key, to.id)
		}
	}
	if !strings.Contains(help, "esc") {
		t.Errorf("help %q must list esc", help)
	}
}

func TestRunViews_HelpListsTheTimelineKey(t *testing.T) {
	for _, from := range allPerRunViews() {
		help := from.make(timelineRun()).Help()

		if !strings.Contains(help, screens.KeyViewTimeline+" ") {
			t.Errorf("%s: help %q must list key %q for the timeline", from.id, help, screens.KeyViewTimeline)
		}
	}
}
