package screens_test

// The Context view for actors that made helper calls (compaction, auxiliary).
//
// Rules under test:
//   - The Context table's model column and the context sparkline are built from
//     main calls only.
//   - The drill-down per-call table lists every call, with a kind column that is
//     empty for main calls and reads "compaction" or "auxiliary" for the rest,
//     followed by " (<label>)" when the call has a label.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const (
	kindMainModel = "model-main"
	kindAuxModel  = "model-aux"
	kindCompModel = "model-comp"
)

func kindedCall(sec int, context int64, model string, kind domain.CallKind, label string) domain.ContextCall {
	c := ctxCallOf(sec, context, 10, 10, model)
	c.Kind = kind
	c.Label = label
	return c
}

// kindMixedActor makes two main calls around helper calls on other models.
func kindMixedActor() domain.ActorContext {
	calls := []domain.ContextCall{
		kindedCall(10, 100_000, kindMainModel, domain.CallMain, ""),
		kindedCall(40, 300, kindAuxModel, domain.CallAuxiliary, "title"),
		kindedCall(70, 200_000, kindMainModel, domain.CallMain, ""),
		kindedCall(100, 150_000, kindCompModel, domain.CallCompaction, "summarizeConversationHistory"),
		kindedCall(130, 500, kindAuxModel, domain.CallAuxiliary, ""),
	}
	return domain.ActorContext{
		Actor: ctxAgent("mixed#1", "mixed"), Calls: calls,
		First: ctxFig(100_000, false), Peak: ctxFig(200_000, false), Final: ctxFig(200_000, false),
	}
}

// kindPlainActor makes only the two main calls of kindMixedActor.
func kindPlainActor() domain.ActorContext {
	calls := []domain.ContextCall{
		kindedCall(10, 100_000, kindMainModel, domain.CallMain, ""),
		kindedCall(70, 200_000, kindMainModel, domain.CallMain, ""),
	}
	return domain.ActorContext{
		Actor: ctxAgent("plain#1", "plain"), Calls: calls,
		First: ctxFig(100_000, false), Peak: ctxFig(200_000, false), Final: ctxFig(200_000, false),
	}
}

func kindRun() domain.RunReport {
	run := overviewRun()
	run.Context = domain.RunContext{
		Orchestrator: ctxOrchestrator(),
		Invocations:  []domain.ActorContext{kindMixedActor(), kindPlainActor()},
	}
	return run
}

// kindRowOf is the table row of an actor, found by its instance id.
func kindRowOf(t *testing.T, view, id string) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if strings.Contains(l, id) && !strings.Contains(l, "sort:") {
			return l
		}
	}
	t.Fatalf("no table row for %s\n%s", id, stripStyling(view))
	return ""
}

func TestContext_ModelColumn_NamesTheModelsOfMainCallsOnly(t *testing.T) {
	view := ctxView(t, kindRun(), tableW, tallH)

	row := kindRowOf(t, view, "mixed#1")

	assertContains(t, row, kindMainModel)
	assertNotContains(t, row, kindAuxModel, kindCompModel)
}

func TestContext_Sparkline_DrawsMainCallsOnly(t *testing.T) {
	view := ctxView(t, kindRun(), tableW, tallH)

	mixed := lastToken(kindRowOf(t, view, "mixed#1"))
	plain := lastToken(kindRowOf(t, view, "plain#1"))

	if plain == "" || len([]rune(plain)) != 2 {
		t.Fatalf("plain sparkline = %q, want two points", plain)
	}
	if mixed != plain {
		t.Errorf("mixed sparkline = %q, want %q (helper calls draw no point)", mixed, plain)
	}
}

func lastToken(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func TestContextDrill_CallTableHasAKindColumn(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)))

	if lineWithAll(view, "time", "model", "kind", "context", "mark") == "" {
		t.Errorf("no header line names time, model, kind, context and mark\n%s", view)
	}
}

func TestContextDrill_ListsEveryCallOfAnActorWithHelperCalls(t *testing.T) {
	view := ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)

	assertAppearInOrder(t, view, ctxCallLabels(10, 40, 70, 100, 130), true)
}

func TestContextDrill_HelperCallsShowTheirKindAndLabel(t *testing.T) {
	view := ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)

	assertContains(t, ctxCallRow(t, view, 40), kindAuxModel, "auxiliary (title)")
	assertContains(t, ctxCallRow(t, view, 100), kindCompModel, "compaction (summarizeConversationHistory)")
}

func TestContextDrill_HelperCallWithoutALabel_ShowsOnlyItsKind(t *testing.T) {
	view := ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)

	row := ctxCallRow(t, view, 130)

	assertContains(t, row, "auxiliary")
	assertNotContains(t, row, "auxiliary (", "()")
}

func TestContextDrill_MainCallsShowNoKind(t *testing.T) {
	view := ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)

	for _, sec := range []int{10, 70} {
		assertContains(t, ctxCallRow(t, view, sec), kindMainModel)
		assertNotContains(t, ctxCallRow(t, view, sec), "auxiliary", "compaction")
	}
}

func TestContextDrill_Curve_PlotsMainCallsOnly(t *testing.T) {
	mixed := ctxDrill(t, kindRun(), "mixed#1", tableW, tallH).View(tableW, tallH)
	plain := ctxDrill(t, kindRun(), "plain#1", tableW, tallH).View(tableW, tallH)

	want := strings.Count(stripStyling(plain), ctxPointGlyph)
	if want != 2 {
		t.Fatalf("plain curve has %d points, want 2", want)
	}
	if got := strings.Count(stripStyling(mixed), ctxPointGlyph); got != want {
		t.Errorf("mixed curve has %d points, want %d (helper calls draw none)", got, want)
	}
}
