package screens_test

// Display of the per-session first-write figures: the Context table cell, the
// sparkline marker, the Session screen's figure lines, the curve markers, the
// per-call marks and the legend. An actor with at most one figure looks as it
// always did.

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// fwMultiActor has two sessions, "a1" (30k, 100k) and "b2" (40k, 380k); each
// wrote once. The figures are the 100k call of the first and the 380k call of
// the second.
//
//	10s 30k | 20s 100k (first write) || 30s 40k | 40s 380k (first write)
func fwMultiActor(secondPartial bool) domain.ActorContext {
	calls := []domain.ContextCall{
		ctxCallOf(10, 30_000, 1, 1, "model-m"),
		ctxCallOf(20, 100_000, 2, 2, "model-m"),
		ctxCallOf(30, 40_000, 3, 3, "model-m"),
		ctxCallOf(40, 380_000, 4, 4, "model-m"),
	}
	for i, id := range []string{"a1", "a1", "b2", "b2"} {
		calls[i].SessionID = id
	}
	calls[1].FirstWrite, calls[3].FirstWrite = true, true
	return domain.ActorContext{
		Actor: ctxAgent("multi#1", "multi"), Calls: calls,
		First: ctxFig(30_000, false), Peak: ctxFig(380_000, false), Final: ctxFig(380_000, false),
		FirstWrite: ctxFig(100_000, false), FirstWriteAt: ctxAt(15),
		FirstWrites: []domain.SessionFirstWrite{
			{Session: 1, SessionID: "a1", At: ctxAt(15), Call: 1, Figure: ctxFig(100_000, false)},
			{Session: 2, SessionID: "b2", At: ctxAt(35), Call: 3, Figure: ctxFig(380_000, secondPartial)},
		},
	}
}

// fwBuilderWithEntry is the standard builder with its single figure also listed
// as a per-session entry, as the analysis delivers it.
func fwBuilderWithEntry() domain.ActorContext {
	a := ctxBuilder()
	a.FirstWrites = []domain.SessionFirstWrite{{Session: 1, At: ctxAt(65), Call: 1, Figure: ctxFig(260_000, false)}}
	return a
}

func fwRun(actors ...domain.ActorContext) domain.RunReport {
	run := contextRun()
	run.Context.Invocations = append(run.Context.Invocations, actors...)
	return run
}

// fwLoneLaterActor is fwMultiActor whose first session did not write: its only
// figure is the second session's.
func fwLoneLaterActor() domain.ActorContext {
	a := fwMultiActor(false)
	a.Calls[1].FirstWrite = false
	a.FirstWrite, a.FirstWriteAt = ctxFig(380_000, false), ctxAt(35)
	a.FirstWrites = a.FirstWrites[1:]
	return a
}

// fwThreeActor has three sessions whose figures are 100k, 380k and 150k.
func fwThreeActor() domain.ActorContext {
	a := fwMultiActor(false)
	extra := []domain.ContextCall{
		ctxCallOf(50, 20_000, 5, 5, "model-m"),
		ctxCallOf(60, 150_000, 6, 6, "model-m"),
	}
	extra[0].SessionID, extra[1].SessionID = "c3", "c3"
	extra[1].FirstWrite = true
	a.Calls = append(a.Calls, extra...)
	a.FirstWrites = append(a.FirstWrites,
		domain.SessionFirstWrite{Session: 3, SessionID: "c3", At: ctxAt(55), Call: 5, Figure: ctxFig(150_000, false)})
	return a
}

func fwLastField(line string) string {
	fields := strings.Fields(line)
	return fields[len(fields)-1]
}

// fwRowLine is the table row of an actor of the run, found by its label.
func fwRowLine(t *testing.T, view, id string) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if strings.Contains(l, id) && !strings.Contains(l, "sort:") {
			return l
		}
	}
	t.Fatalf("no table row for %s\n%s", id, stripStyling(view))
	return ""
}

// fwMarkerColumns are the distinct text columns that carry the curve's
// first-write marker glyph; a marker may span several rows of one column.
func fwMarkerColumns(view string) int {
	cols := map[int]bool{}
	for _, l := range viewLines(view) {
		col := 0
		for _, r := range l {
			if string(r) == ctxFirstWriteGlyph {
				cols[col] = true
			}
			col++
		}
	}
	return len(cols)
}

func TestContext_SeveralFirstWrites_AreListedInSessionOrderInOneCell(t *testing.T) {
	view := ctxView(t, fwRun(fwMultiActor(false)), tableW, tallH)

	assertContains(t, fwRowLine(t, view, "multi#1"), "100,000 / 380,000")
}

func TestContext_SeveralFirstWrites_EachFigureCarriesItsOwnPartialSuffix(t *testing.T) {
	view := ctxView(t, fwRun(fwMultiActor(true)), tableW, tallH)

	assertContains(t, fwRowLine(t, view, "multi#1"), "100,000 / 380,000 (partial)")
}

func TestContext_SingleFirstWriteEntry_IsTheCellAsBefore(t *testing.T) {
	run := contextRun()
	run.Context.Invocations[0] = fwBuilderWithEntry()
	view := ctxView(t, run, tableW, tallH)

	got := ctxRowCells(t, view, "builder#1")

	if got.firstWrite != "260,000" {
		t.Errorf("builder#1 first write = %q, want 260,000", got.firstWrite)
	}
	assertNotContains(t, ctxRowLine(t, view, "builder#1"), " / ")
}

func TestContext_SeveralFirstWrites_SparklineKeepsOneMarkerAtTheEarliestFirstWrite(t *testing.T) {
	view := ctxView(t, fwRun(fwMultiActor(false)), tableW, tallH)

	spark := []rune(fwLastField(fwRowLine(t, view, "multi#1")))

	if n := strings.Count(string(spark), ctxFirstWriteGlyph); n != 1 {
		t.Fatalf("sparkline %q has %d markers, want 1", string(spark), n)
	}
	if len(spark) < 2 || string(spark[1]) != ctxFirstWriteGlyph {
		t.Errorf("sparkline %q: the marker must sit on the second call, the earliest first write", string(spark))
	}
}

func TestContext_SeveralFirstWrites_ShownColumnKeepsEveryFigureInFull(t *testing.T) {
	view := ctxView(t, fwRun(fwMultiActor(true)), tableW, tallH)

	if !hasTitle(headerTitles(view, "instance"), "context at first write") {
		t.Fatalf("the column must be shown at width %d\n%s", tableW, stripStyling(view))
	}
	assertContains(t, fwRowLine(t, view, "multi#1"), "100,000 / 380,000 (partial)")
}

func TestContext_SeveralFirstWrites_NarrowTableNeverClipsTheCell(t *testing.T) {
	for _, w := range []int{100, 80} {
		view := ctxView(t, fwRun(fwMultiActor(true)), w, tallH)

		for _, l := range viewLines(view) {
			if n := utf8.RuneCountInString(l); n > w {
				t.Fatalf("width %d: line is %d wide: %q", w, n, l)
			}
		}
		// The column is either hidden or shows every figure; never a cut-off list.
		if hasTitle(headerTitles(view, "instance"), "context at first write") {
			assertContains(t, fwRowLine(t, view, "multi#1"), "100,000 / 380,000 (partial)")
		}
	}
}

func TestContext_FirstFigurePartial_SuffixSitsOnThatFigureOnly(t *testing.T) {
	a := fwMultiActor(false)
	a.FirstWrites[0].Figure = ctxFig(100_000, true)
	view := ctxView(t, fwRun(a), tableW, tallH)

	row := fwRowLine(t, view, "multi#1")

	assertContains(t, row, "100,000 (partial) / 380,000")
	assertNotContains(t, row, "380,000 (partial)")
}

func TestContext_ThreeFirstWrites_AreListedInSessionOrderInOneCell(t *testing.T) {
	view := ctxView(t, fwRun(fwThreeActor()), tableW, tallH)

	assertContains(t, fwRowLine(t, view, "multi#1"), "100,000 / 380,000 / 150,000")
}

func TestSessionScreen_ThreeFirstWrites_GetOneNumberedLineEach(t *testing.T) {
	view := stripStyling(ctxDrill(t, fwRun(fwThreeActor()), "multi#1", tableW, tallH).View(tableW, tallH))

	assertContains(t, view,
		"First write context* (session 1): 100,000",
		"First write context* (session 2): 380,000",
		"First write context* (session 3): 150,000")
}

func TestContext_OrchestratorRowStillShowsNoFirstWriteWhenOthersHaveSeveral(t *testing.T) {
	view := ctxView(t, fwRun(fwMultiActor(false)), tableW, tallH)

	if got := ctxRowCells(t, view, "orchestrator").firstWrite; got != screens.AbsentMarker {
		t.Errorf("orchestrator first write = %q, want the absent marker", got)
	}
	assertNotContains(t, ctxRowLine(t, view, "orchestrator"), " / ")
}

func TestSessionScreen_SeveralFirstWrites_GetOneFigureLinePerSession(t *testing.T) {
	view := stripStyling(ctxDrill(t, fwRun(fwMultiActor(false)), "multi#1", tableW, tallH).View(tableW, tallH))

	assertContains(t, view,
		"First write context* (session 1): 100,000 ("+domain.BandOf(100_000).Label()+")",
		"First write context* (session 2): 380,000 ("+domain.BandOf(380_000).Label()+")")
}

func TestContext_LoneLaterSessionFigure_IsAPlainSingleFigureCell(t *testing.T) {
	view := ctxView(t, fwRun(fwLoneLaterActor()), tableW, tallH)

	row := fwRowLine(t, view, "multi#1")

	assertContains(t, row, "380,000")
	assertNotContains(t, row, " / ")
	assertNotContains(t, row, "session")
}

func TestSessionScreen_LoneLaterSessionFigure_KeepsItsSessionNumber(t *testing.T) {
	view := stripStyling(ctxDrill(t, fwRun(fwLoneLaterActor()), "multi#1", tableW, tallH).View(tableW, tallH))

	assertContains(t, view, "First write context* (session 2): 380,000 ("+domain.BandOf(380_000).Label()+")")
	assertNotContains(t, view, "First write context*:")
}

func TestSessionScreen_SingleFirstWrite_KeepsTheUnnumberedLine(t *testing.T) {
	run := contextRun()
	run.Context.Invocations[0] = fwBuilderWithEntry()
	view := stripStyling(ctxDrill(t, run, "builder#1", tableW, tallH).View(tableW, tallH))

	assertContains(t, view, "First write context*: 260,000 ("+domain.BandOf(260_000).Label()+")")
	assertNotContains(t, view, "(session 1)")
}

func TestSessionScreen_SeveralFirstWrites_CurveMarksEachSessionsCall(t *testing.T) {
	multi := ctxDrill(t, fwRun(fwMultiActor(false)), "multi#1", tableW, tallH).View(tableW, tallH)
	single := ctxDrillView(t, "builder#1")

	if got := fwMarkerColumns(multi); got != 2 {
		t.Errorf("multi-session curve has first-write markers in %d columns, want 2\n%s", got, stripStyling(multi))
	}
	if got := fwMarkerColumns(single); got != 1 {
		t.Errorf("single-session curve has first-write markers in %d columns, want 1", got)
	}
}

func TestSessionScreen_SeveralFirstWrites_EachSourceCallCarriesTheMark(t *testing.T) {
	view := ctxDrill(t, fwRun(fwMultiActor(false)), "multi#1", tableW, tallH).View(tableW, tallH)

	assertContains(t, ctxCallRow(t, view, 20), markFirstWrite)
	assertContains(t, ctxCallRow(t, view, 40), markFirstWrite)
	assertNotContains(t, ctxCallRow(t, view, 10), markFirstWrite)
	assertNotContains(t, ctxCallRow(t, view, 30), markFirstWrite)
}

func TestContextLegend_FirstWriteIsPerSessionAndTheSparklineMarkIsTheEarliest(t *testing.T) {
	overlay := strings.Join(overlayLines(openOverlay(ctxOpen(t), tableW, legendOverlayH)), "\n")
	text := strings.ToLower(strings.Join(strings.Fields(stripStyling(overlay)), " "))

	// Anchor to the legend entry of the first-write column.
	i := strings.Index(text, "context at first write")
	if i < 0 {
		t.Fatalf("legend has no entry for the context at first write:\n%s", text)
	}
	const notComputed = "not computed for the orchestrator"
	end := strings.Index(text[i:], notComputed)
	if end < 0 {
		t.Fatalf("the first-write entry does not say it is %s:\n%s", notComputed, text[i:])
	}
	entry := text[i : i+end+len(notComputed)]
	assertContains(t, entry, "per session")
	// The earliest-first-write wording belongs to the sparkline entry, not the first-write entry.
	assertContains(t, text[:i]+text[i+end+len(notComputed):], "earliest first write")
}
