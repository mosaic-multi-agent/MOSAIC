package screens_test

// The per-call table below the curve of a drill-down: its columns, call order,
// the figures of a call, the cost and context sorts, partial calls, and the marks
// on the first write call and on the calls around compactions.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	markFirstWrite = "first write"
	markLogged     = "logged compaction"
	markDetected   = "detected compaction"
)

func TestContextDrill_CallTableHasTheListedColumns(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrillView(t, "builder#1")))

	header := lineWithAll(view, "time", "model", "context", "input", "cache read", "cache write", "output", "cost", "mark")
	if header == "" {
		t.Errorf("no header line names time, model, context, input, cache read, cache write, output, cost and mark\n%s", view)
	}
}

func TestContextDrill_CallsAreListedInCallOrderByDefault(t *testing.T) {
	view := ctxDrillView(t, "builder#1")

	key, _ := activeSort(t, view)
	if key != screens.SortCallOrder {
		t.Errorf("default call sort = %q, want call order", key)
	}
	assertAppearInOrder(t, view, ctxCallLabels(10, 70, 130, 190), true)
}

func TestContextDrill_ACallRowShowsItsFigures(t *testing.T) {
	view := ctxDrillView(t, "builder#1")

	row := ctxCallRow(t, view, 70)

	assertContains(t, row, "model-b")
	assertFields(t, row, "260,000", "193,999", "65,000", "1,001", "222", "$0.22")
	assertNoFields(t, row, "380,000", "$0.33", "$0.50")
}

func TestContextDrill_CallSortsCycleThroughCallOrderCostAndContext(t *testing.T) {
	s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)
	ascending := map[screens.SortKey][]string{
		screens.SortCallOrder: ctxCallLabels(10, 70, 130, 190),
		screens.SortCost:      ctxCallLabels(70, 130, 190, 10),
		screens.SortContext:   ctxCallLabels(10, 70, 190, 130),
	}

	seen := walkSorts(t, s, tableW, tallH, ascending, screens.SortCallOrder, screens.SortCost, screens.SortContext)

	if len(seen) == 0 || seen[0] != screens.SortCallOrder {
		t.Errorf("the cycle must start at call order; offered %v", seen)
	}
}

func TestContextDrill_CostAndContextSortsStartWithTheLargest(t *testing.T) {
	s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)

	press(s, screens.KeySort)
	byCost := s.View(tableW, tallH)
	press(s, screens.KeySort)
	byContext := s.View(tableW, tallH)

	if key, desc := activeSort(t, byCost); key != screens.SortCost || !desc {
		t.Errorf("first sort after call order = %q desc=%v, want cost descending", key, desc)
	}
	assertAppearInOrder(t, byCost, ctxCallLabels(10, 190, 130, 70), true)
	if key, desc := activeSort(t, byContext); key != screens.SortContext || !desc {
		t.Errorf("second sort after call order = %q desc=%v, want context descending", key, desc)
	}
	assertAppearInOrder(t, byContext, ctxCallLabels(130, 190, 70, 10), true)
}

func TestContextDrill_APartialCallIsMarkedAndShowsTheAbsentCacheWrite(t *testing.T) {
	view := ctxDrillView(t, "reviewer#1")

	partial := ctxCallRow(t, view, 140)

	assertContains(t, partial, "(partial)", screens.AbsentMarker)
	assertFields(t, partial, "210,000", "200,031", "9,969", "33")
	complete := ctxCallRow(t, view, 20)
	assertNotContains(t, complete, "(partial)")
}

func TestContextDrill_MarksTheFirstWriteCall(t *testing.T) {
	view := ctxDrillView(t, "builder#1")

	assertContains(t, ctxCallRow(t, view, 70), markFirstWrite)
	for _, sec := range []int{10, 130, 190} {
		assertNotContains(t, ctxCallRow(t, view, sec), markFirstWrite, markLogged, markDetected)
	}
}

func TestContextDrill_MarksTheCallsAroundADetectedCompaction(t *testing.T) {
	view := ctxDrillView(t, "coder#1")

	for _, sec := range []int{150, 210} {
		assertContains(t, ctxCallRow(t, view, sec), markDetected)
	}
	for _, sec := range []int{30, 90, 270} {
		assertNotContains(t, ctxCallRow(t, view, sec), markDetected)
	}
	for _, sec := range []int{30, 90, 150, 210, 270} {
		assertNotContains(t, ctxCallRow(t, view, sec), markLogged, markFirstWrite)
	}
}

func TestContextDrill_MarksTheCallsAroundALoggedCompactionAndTheFirstWriteTogether(t *testing.T) {
	view := ctxDrillView(t, "reviewer#1")

	assertContains(t, ctxCallRow(t, view, 20), markLogged)
	assertNotContains(t, ctxCallRow(t, view, 20), markFirstWrite, markDetected)
	assertContains(t, ctxCallRow(t, view, 80), markLogged, markFirstWrite)
	assertNotContains(t, ctxCallRow(t, view, 140), markLogged, markFirstWrite, markDetected)
}

func TestContextDrill_LoggedAndDetectedMarksShareACallWhenTheirCallsCoincide(t *testing.T) {
	view := ctxDrillView(t, "orchestrator")

	assertNotContains(t, ctxCallRow(t, view, 0), markLogged, markDetected, markFirstWrite)
	assertContains(t, ctxCallRow(t, view, 60), markLogged, markDetected)
	assertContains(t, ctxCallRow(t, view, 120), markLogged, markDetected)
	assertContains(t, ctxCallRow(t, view, 180), markLogged)
	assertNotContains(t, ctxCallRow(t, view, 180), markDetected)
}

func TestContextDrill_OrchestratorCallsAreNeverMarkedAsFirstWrite(t *testing.T) {
	view := ctxDrillView(t, "orchestrator")

	for _, sec := range []int{0, 60, 120, 180} {
		assertNotContains(t, ctxCallRow(t, view, sec), markFirstWrite)
	}
}
