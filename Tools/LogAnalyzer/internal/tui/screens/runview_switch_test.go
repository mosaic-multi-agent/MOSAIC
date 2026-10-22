package screens_test

// View switching, back and help text shared by every per-run view, now that
// the by-status and tools views exist.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// Compile-time proof that the new screens satisfy RunView.
var (
	_ screens.RunView = (*screens.StatusScreen)(nil)
	_ screens.RunView = (*screens.ToolsScreen)(nil)
)

type perRunView struct {
	id   screens.ScreenID
	key  string
	make func(run domain.RunReport) screens.RunView
}

func allPerRunViews() []perRunView {
	return []perRunView{
		{screens.ScreenRunDetail, screens.KeyViewDispatches, func(r domain.RunReport) screens.RunView {
			return screens.NewRunDetailScreen(r, wideW, tallH, plain())
		}},
		{screens.ScreenOverview, screens.KeyViewOverview, func(r domain.RunReport) screens.RunView {
			return screens.NewOverviewScreen(r, wideW, tallH, plain())
		}},
		{screens.ScreenAgentTypes, screens.KeyViewAgentTypes, func(r domain.RunReport) screens.RunView {
			return screens.NewAgentTypesScreen(r, wideW, tallH, plain())
		}},
		{screens.ScreenStatus, screens.KeyViewStatus, func(r domain.RunReport) screens.RunView {
			return screens.NewStatusScreen(r, wideW, tallH, plain())
		}},
		{screens.ScreenTools, screens.KeyViewTools, func(r domain.RunReport) screens.RunView {
			return screens.NewToolsScreen(r, wideW, tallH, plain())
		}},
	}
}

func TestRunViews_EveryViewKeyRequestsTheMatchingSibling(t *testing.T) {
	for _, from := range allPerRunViews() {
		for _, to := range allPerRunViews() {
			if from.id == to.id {
				continue
			}
			s := from.make(statusRun())
			press(s, to.key)

			got, ok := s.Requested()
			if !ok || got != to.id {
				t.Errorf("%s: key %q: Requested() = (%q, %v), want (%q, true)", from.id, to.key, got, ok, to.id)
			}
			s.Reset()
			if _, ok := s.Requested(); ok {
				t.Errorf("%s: Reset must clear the request for key %q", from.id, to.key)
			}
		}
	}
}

func TestRunViews_OwnViewKeyRequestsNothing(t *testing.T) {
	for _, v := range allPerRunViews() {
		s := v.make(statusRun())
		press(s, v.key)

		if got, ok := s.Requested(); ok {
			t.Errorf("%s: its own key %q must not request a switch, got %q", v.id, v.key, got)
		}
	}
}

func TestStatusAndToolsViews_EscAndQAskToLeave(t *testing.T) {
	for _, v := range allPerRunViews() {
		if v.id != screens.ScreenStatus && v.id != screens.ScreenTools {
			continue
		}
		for _, k := range []string{"esc", "q"} {
			s := v.make(statusRun())
			press(s, k)

			if !s.Back() {
				t.Errorf("%s: key %q must set Back()", v.id, k)
			}
			s.Reset()
			if s.Back() {
				t.Errorf("%s: Reset must clear Back after %q", v.id, k)
			}
		}
	}
}

func TestStatusAndToolsViews_IdentifyThemselves(t *testing.T) {
	if got := screens.NewStatusScreen(statusRun(), wideW, tallH, plain()).ID(); got != screens.ScreenStatus {
		t.Errorf("status screen ID = %q, want %q", got, screens.ScreenStatus)
	}
	if got := screens.NewToolsScreen(toolsRun(), wideW, tallH, plain()).ID(); got != screens.ScreenTools {
		t.Errorf("tools screen ID = %q, want %q", got, screens.ScreenTools)
	}
}

func TestRunViews_HelpListsEveryOtherViewKeyAndBack(t *testing.T) {
	for _, from := range allPerRunViews() {
		help := from.make(statusRun()).Help()

		for _, to := range allPerRunViews() {
			if to.id == from.id {
				continue
			}
			if !strings.Contains(help, to.key+" ") {
				t.Errorf("%s: help %q must list key %q for the %s view", from.id, help, to.key, to.id)
			}
		}
		if !strings.Contains(help, "esc") {
			t.Errorf("%s: help %q must list esc", from.id, help)
		}
	}
}
