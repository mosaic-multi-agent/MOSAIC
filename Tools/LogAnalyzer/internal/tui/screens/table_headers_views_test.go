package screens_test

// The table views the header tests walk: each opens one table of the sample run
// (or a small fixture for the views the sample run does not fill) wide enough that
// no column is dropped, so every title of the table is on screen.

import (
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	headerW = 320 // wide enough for every column of every table to show
	headerH = 100 // tall enough for every row of the sample run
)

// headerView is one table of one view.
type headerView struct {
	name   string
	anchor string // the first column's title; "a|b" allows either name
	screen func(t *testing.T, w, h int) screens.Screen
}

// view draws the screen on a terminal of w by h cells with colour off.
func (v headerView) view(t *testing.T, w, h int) string {
	t.Helper()
	colourOff(t)
	return v.screen(t, w, h).View(w, h)
}

// headerViews are all the table views of the LogAnalyzer TUI, first table of each
// unless the name says otherwise.
func headerViews() []headerView {
	st := paletteStyles
	return []headerView{
		{"dispatches", "instance", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewRunDetailScreen(sampleRun(t), w, h, st())
		}},
		{"by type", "type", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewAgentTypesScreen(sampleRun(t), w, h, st())
		}},
		{"by type drill-down", "instance", func(t *testing.T, w, h int) screens.Screen {
			return press(screens.NewAgentTypesScreen(sampleRun(t), w, h, st()), "enter")
		}},
		{"by status", "status", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewStatusScreen(sampleRun(t), w, h, st())
		}},
		{"tools", "tool", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewToolsScreen(sampleRun(t), w, h, st())
		}},
		{"context", "instance", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewContextScreen(sampleRun(t), w, h, st())
		}},
		{"overview models", "model", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewOverviewScreen(sampleRun(t), w, h, st())
		}},
		{"compare", "type", func(t *testing.T, w, h int) screens.Screen {
			return newCompare(compareFixture(), w, h)
		}},
		{"across runs, runs", "run", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewAcrossRunsScreen(acrossReport(), w, h, st())
		}},
		{"across runs, agent types", "type", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewAcrossRunsScreen(acrossReport(), w, h, st())
		}},
		{"runs list", "compare|cmp", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewRunsScreen(runsReport(), w, h, st())
		}},
		{"runs list with marked runs", "compare|cmp", func(t *testing.T, w, h int) screens.Screen {
			s := screens.NewRunsScreen(runsReport(), w, h, st())
			press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun)
			return s
		}},
		{"pricing", "model", func(t *testing.T, w, h int) screens.Screen {
			return screens.NewPricingScreen(pricingModels(), "", w, h, st())
		}},
		{"timeline invocations", "instance", func(t *testing.T, w, h int) screens.Screen {
			return timelineListScreen(w, h)
		}},
		{"timeline compactions", "time", func(t *testing.T, w, h int) screens.Screen {
			return timelineListScreen(w, h)
		}},
	}
}

// timelineListScreen is the Timeline in its list version.
func timelineListScreen(w, h int) screens.Screen {
	s := screens.NewTimelineScreen(timelineRun(), w, h, paletteStyles())
	press(s, screens.KeyToggleView)
	return s
}

// namedView is the table view with the name.
func namedView(t *testing.T, name string) headerView {
	t.Helper()
	for _, v := range headerViews() {
		if v.name == name {
			return v
		}
	}
	t.Fatalf("no table view is named %q", name)
	return headerView{}
}

// viewNamed renders the table view with the name on the wide terminal.
func viewNamed(t *testing.T, name string) string {
	t.Helper()
	return namedView(t, name).view(t, headerW, headerH)
}

// viewAnchor is the anchor of the table view with the name.
func viewAnchor(t *testing.T, name string) string {
	t.Helper()
	return namedView(t, name).anchor
}

// contextHeaderView is the Context view.
func contextHeaderView(t *testing.T) string { return viewNamed(t, "context") }

// narrowView renders the table view with the name on a terminal of the width.
func narrowView(t *testing.T, name string, w int) string {
	t.Helper()
	return namedView(t, name).view(t, w, headerH)
}
