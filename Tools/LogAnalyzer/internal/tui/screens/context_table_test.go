package screens_test

// Behaviour of the Context view table: one row per invocation plus the
// orchestrator, the figures and counts of each row, the labels that mark
// heuristic and detected figures, the absent and partial markers and the
// definition of final context.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var ctxActorIDs = []string{"orchestrator", "builder#1", "coder#1", "ghost#1", "reviewer#1", "tester#1"}

func TestContext_ListsTheOrchestratorAndEveryInvocationOnce(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	for _, id := range ctxActorIDs {
		ctxRowLine(t, view, id)
	}
	for _, id := range ctxActorIDs[1:] {
		if n := countOf(view, id); n != 1 {
			t.Errorf("%s is listed %d times, want once\n%s", id, n, stripStyling(view))
		}
	}
}

func TestContext_HeaderNamesEveryColumn(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	titles := headerTitles(view, "instance")
	for _, want := range []string{"instance", "model", "API calls", "first context", "context at first write", "peak context",
		"final context", "logged", "harness", "detected only", "unapplied", "cost"} {
		if !hasTitle(titles, want) {
			t.Errorf("the header has no column titled %q; its columns are %q", want, titles)
		}
	}
}

func TestContext_ShowsTheFiguresOfEachRow(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	want := map[string]ctxCells{
		"orchestrator": {"4", "40,000", screens.AbsentMarker, "180,000", "130,000", "2", "0", "1", "0"},
		"builder#1":    {"4", "30,000", "260,000", "380,000", "360,000", "0", "0", "0", "0"},
		"coder#1":      {"5", "100,000", screens.AbsentMarker, "300,000", "80,000", "0", "0", "1", "0"},
		"reviewer#1":   {"3", "120,000", "150,000", "210,000 (partial)", "210,000 (partial)", "1", "0", "0", "0"},
		"tester#1":     {"2", "50,000", screens.AbsentMarker, "95,000", "95,000", "0", "0", "0", "0"},
	}
	for id, w := range want {
		if got := ctxRowCells(t, view, id); got != w {
			t.Errorf("%s: figures = %+v, want %+v", id, got, w)
		}
	}
}

func TestContext_RowsShowTheModel(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	assertContains(t, ctxRowLine(t, view, "builder#1"), "model-b")
	assertContains(t, ctxRowLine(t, view, "reviewer#1"), "model-r")
	assertContains(t, ctxRowLine(t, view, "tester#1"), "model-t")
}

func TestContext_AnInvocationWithoutCallsShowsTheAbsentMarkerNeverZero(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	got := ctxRowCells(t, view, "ghost#1")

	for name, cell := range map[string]string{"first": got.first, "first write": got.firstWrite, "peak": got.peak, "final": got.final} {
		if cell != screens.AbsentMarker {
			t.Errorf("ghost#1 %s = %q, want the absent marker", name, cell)
		}
	}
}

func TestContext_OnlyPartialFiguresCarryThePartialMarker(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	assertContains(t, ctxRowLine(t, view, "reviewer#1"), "(partial)")
	for _, id := range []string{"orchestrator", "builder#1", "coder#1", "ghost#1", "tester#1"} {
		assertNotContains(t, ctxRowLine(t, view, id), "(partial)")
	}
}

func TestContext_LoggedAndDetectedCompactionsAreCountedSeparately(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	orch := ctxRowCells(t, view, "orchestrator")
	if orch.logged != "2" || orch.detected != "1" {
		t.Errorf("orchestrator logged/detected = %s/%s, want 2/1", orch.logged, orch.detected)
	}
	coder := ctxRowCells(t, view, "coder#1")
	if coder.logged != "0" || coder.detected != "1" {
		t.Errorf("coder#1 logged/detected = %s/%s, want 0/1", coder.logged, coder.detected)
	}
	reviewer := ctxRowCells(t, view, "reviewer#1")
	if reviewer.logged != "1" || reviewer.detected != "0" {
		t.Errorf("reviewer#1 logged/detected = %s/%s, want 1/0", reviewer.logged, reviewer.detected)
	}
}

func TestContext_DetectedCompactionsAreLabelledAsDetected(t *testing.T) {
	titles := headerTitles(ctxView(t, contextRun(), tableW, tallH), "instance")

	if !hasTitle(titles, "logged") || !hasTitle(titles, "detected only") {
		t.Errorf("the header must name the logged and the detected compactions columns; its columns are %q", titles)
	}
}

func TestContext_FirstWriteTitleNeedsNoNoteToBeUnderstood(t *testing.T) {
	titles := headerTitles(ctxView(t, contextRun(), tableW, tallH), "instance")

	if !hasTitle(titles, "context at first write") {
		t.Errorf("the first write figure must be titled \"context at first write\"; its columns are %q", titles)
	}
	for _, title := range titles {
		if strings.Contains(title, "*") {
			t.Errorf("title %q points at a note", title)
		}
	}
}

func TestContext_OrchestratorHasNoFirstWriteFigure(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	if got := ctxRowCells(t, view, "orchestrator").firstWrite; got != screens.AbsentMarker {
		t.Errorf("orchestrator first write = %q, want the absent marker", got)
	}
}

func TestContext_StatesTheDefinitionOfFinalContext(t *testing.T) {
	// The definition lives in the column legend of the ? overlay, not under the table.
	overlay := strings.Join(overlayLines(openOverlay(ctxOpen(t), tableW, legendOverlayH)), "\n")

	assertContains(t, stripStyling(overlay), domain.FinalContextDefinition)
}
