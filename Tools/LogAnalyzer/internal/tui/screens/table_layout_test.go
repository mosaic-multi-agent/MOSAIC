package screens_test

// Layout of the tables on the sample run: tables stacked in one view line up and
// show the same columns, and every row of the instance tables keeps the #N that
// tells it from the others of its type.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// layoutWidths are the terminal widths the layout is checked at.
var layoutWidths = []int{120, 180}

// layoutH is tall enough for every row of the sample run's views.
const layoutH = 200

// assertTablesShareLayout fails unless every table of the view has the column
// layout of the first: the same titles at the same positions and the same columns
// hidden.
func assertTablesShareLayout(t *testing.T, view, anchor string, minTables int) {
	t.Helper()
	tables := stackedTables(t, view, anchor)
	if len(tables) < minTables {
		t.Fatalf("the view has %d tables, want at least %d\n%s", len(tables), minTables, stripStyling(view))
	}
	want := fullLayout(tables[0])
	for _, tb := range tables[1:] {
		if got := fullLayout(tb); got != want {
			t.Errorf("table %q lays its columns out as %s, but table %q as %s", tb.label, got, tables[0].label, want)
		}
	}
}

func TestStatusTables_ShareOneColumnLayoutAtEveryWidth(t *testing.T) {
	colourOff(t)
	run := sampleRun(t)
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := screens.NewStatusScreen(run, w, layoutH, paletteStyles()).View(w, layoutH)

			assertTablesShareLayout(t, view, "status", 10)
		})
	}
}

func TestToolsTables_ShareOneColumnLayoutAtEveryWidth(t *testing.T) {
	colourOff(t)
	run := sampleRun(t)
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := screens.NewToolsScreen(run, w, layoutH, paletteStyles()).View(w, layoutH)

			assertTablesShareLayout(t, view, "tool", 10)
		})
	}
}

func TestStatusTables_ShareOneColumnLayoutWhenRowsDiffer(t *testing.T) {
	colourOff(t)
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := screens.NewStatusScreen(statusRun(), w, layoutH, paletteStyles()).View(w, layoutH)

			assertTablesShareLayout(t, view, "status", 4)
		})
	}
}

func TestToolsTables_ShareOneColumnLayoutWhenRowsDiffer(t *testing.T) {
	colourOff(t)
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := screens.NewToolsScreen(toolsRun(), w, layoutH, paletteStyles()).View(w, layoutH)

			assertTablesShareLayout(t, view, "tool", 4)
		})
	}
}

// assertEveryInstanceShowsItsNumber fails unless the table whose first column is
// "instance" has one row for each id and every row names its instance with the
// #N that ends the id.
func assertEveryInstanceShowsItsNumber(t *testing.T, view string, ids []string) {
	t.Helper()
	cells := firstCells(tableRows(t, view, "instance"))
	var lost []string
	for _, id := range ids {
		if !identityShown(cells, id) {
			lost = append(lost, id)
		}
	}
	if len(lost) > 0 {
		t.Errorf("%d of %d rows lost their #N, for example %q; first cells: %q", len(lost), len(ids), lost[:min(3, len(lost))], cells[:min(8, len(cells))])
	}
	for _, c := range cells {
		if !strings.Contains(c, "#") && !strings.HasPrefix("orchestrator", strings.TrimSuffix(c, "…")) {
			t.Errorf("the row %q has no #N", c)
		}
	}
}

// dispatchIDsOf are the instance ids of the run's dispatches.
func dispatchIDsOf(run domain.RunReport) []string {
	var ids []string
	for _, g := range run.ByAgentType {
		for _, d := range g.Dispatches {
			ids = append(ids, d.Invocation.Actor.Label())
		}
	}
	return ids
}

func TestDispatches_EveryRowOfTheSampleRunShowsItsNumber(t *testing.T) {
	colourOff(t)
	run := sampleRun(t)
	ids := dispatchIDsOf(run)
	if len(ids) != 65 {
		t.Fatalf("the sample run has %d dispatches, want 65", len(ids))
	}
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := dispatchesView(run, w, layoutH)

			assertEveryInstanceShowsItsNumber(t, view, ids)
		})
	}
}

func TestContext_EveryRowOfTheSampleRunShowsItsNumber(t *testing.T) {
	colourOff(t)
	run := sampleRun(t)
	var ids []string
	for _, a := range run.Context.Invocations {
		ids = append(ids, a.Actor.Label())
	}
	if len(ids) != 65 {
		t.Fatalf("the sample run has %d context rows, want 65", len(ids))
	}
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := ctxView(t, run, w, layoutH)

			assertEveryInstanceShowsItsNumber(t, view, ids)
		})
	}
}

func TestTimelineList_EveryRowOfTheSampleRunShowsItsNumber(t *testing.T) {
	run := sampleRun(t)
	var ids []string
	for _, e := range run.Timeline.Entries {
		ids = append(ids, e.Invocation.Actor.Label())
	}
	if len(ids) != 65 {
		t.Fatalf("the sample run has %d timeline rows, want 65", len(ids))
	}
	for _, w := range layoutWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := listView(t, run, w, layoutH)

			assertEveryInstanceShowsItsNumber(t, view, ids)
		})
	}
}
