package screens_test

// The instance name already holds the agent type, so the Dispatches, Context and
// Timeline list tables carry no type column of their own. The sorts of those tables
// keep marking the column they sort by.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// typeWidth is wide enough for every column of the instance tables to show whole.
const typeWidth = 240

// assertNoTypeColumn fails when the table whose first column is "instance" has a
// type column, or when a row repeats the agent type its instance name already holds.
func assertNoTypeColumn(t *testing.T, view string) {
	t.Helper()
	if titles := headerTitles(view, "instance"); hasTitle(titles, "type") {
		t.Errorf("the table has a type column: %q", titles)
	}
	var repeated []string
	for _, row := range tableRows(t, view, "instance") {
		id := firstCell(row)
		if !strings.Contains(id, "#") {
			continue
		}
		if strings.Count(row, id[:strings.LastIndex(id, "#")]) != 1 {
			repeated = append(repeated, id)
		}
	}
	if len(repeated) > 0 {
		t.Errorf("%d rows show their agent type besides the instance, for example %q", len(repeated), repeated[:min(3, len(repeated))])
	}
}

func TestDispatches_HaveNoTypeColumn(t *testing.T) {
	colourOff(t)

	assertNoTypeColumn(t, dispatchesView(sampleRun(t), typeWidth, layoutH))
}

func TestDispatches_RowOfALongTypeDoesNotRepeatTheType(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), typeWidth, tallH)

	assertContains(t, rowFor(t, view, longInstance), "$2.28")
	assertNotContains(t, rowFor(t, view, longInstance), longType)
}

func TestContext_HasNoTypeColumn(t *testing.T) {
	assertNoTypeColumn(t, ctxView(t, sampleRun(t), typeWidth, layoutH))
}

func TestTimelineList_HasNoTypeColumn(t *testing.T) {
	assertNoTypeColumn(t, listView(t, sampleRun(t), typeWidth, layoutH))
}

// assertSortMarksColumns presses the sort key through the cycle and fails unless the
// header marks the expected title after each press, wrapping back to the first.
func assertSortMarksColumns(t *testing.T, s screens.Screen, width, height int, titles ...string) {
	t.Helper()
	for round := 0; round < 2; round++ {
		for _, want := range titles {
			if got := markedTitle(t, s.View(width, height), "instance"); got != want {
				t.Errorf("the sort arrow is on %q, want %q", got, want)
			}
			press(s, screens.KeySort)
		}
	}
}

func TestDispatches_SortArrowStaysOnTheSortedColumnWithoutTheTypeColumn(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), typeWidth, tallH, paletteStyles())

	assertSortMarksColumns(t, s, typeWidth, tallH, "cost", "tokens", "duration", "peak context", "instance")
}

func TestContext_SortArrowStaysOnTheSortedColumnWithoutTheTypeColumn(t *testing.T) {
	colourOff(t)
	s := screens.NewContextScreen(contextRun(), typeWidth, tallH, paletteStyles())

	assertSortMarksColumns(t, s, typeWidth, tallH, "peak context", "final context", "API calls", "instance")
}

func TestTimelineList_SortArrowStaysOnTheSortedColumnWithoutTheTypeColumn(t *testing.T) {
	s := openTimeline(t, timelineRun(), typeWidth, tallH)
	press(s, screens.KeyToggleView)

	assertSortMarksColumns(t, s, typeWidth, tallH, "start", "duration", "instance")
}

func TestDispatches_SortByNameOrdersTheRowsByInstanceWithoutTheTypeColumn(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), typeWidth, tallH, paletteStyles())
	press(s, screens.KeySort, screens.KeySort, screens.KeySort, screens.KeySort) // cost, tokens, duration, peak, name

	view := s.View(typeWidth, tallH)

	assertAppearInOrder(t, view, []string{"builder#1", "builder#2", "builder#3", "reviewer#1", longInstance}, true)
}
