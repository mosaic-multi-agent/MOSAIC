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
		return screens.NewTimelineScreen(r, wideW, tallH, paletteStyles())
	}}
}

func TestTimelineView_IdentifiesItself(t *testing.T) {
	if got := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles()).ID(); got != screens.ScreenTimeline {
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

func TestTimelineView_HelpHintsAtTheOverviewListingEveryOtherViewKeyAndBack(t *testing.T) {
	s := timelinePerRunView().make(timelineRun())
	var others []string
	for _, to := range allPerRunViews() {
		others = append(others, to.key)
	}

	if help := s.Help(); !strings.Contains(help, screens.KeyHelp) {
		t.Errorf("help %q must hint at the ? overlay", help)
	}
	assertOverlayListsViewKeys(t, s, others...)
}

func TestRunViews_OverlayListsTheTimelineKey(t *testing.T) {
	for _, from := range allPerRunViews() {
		assertOverlayListsViewKeys(t, from.make(timelineRun()), screens.KeyViewTimeline)
	}
}
