package screens_test

// The Context view of compaction kinds: the table counts logged, harness (OTel)
// and detected-only compactions per actor with the compaction-call cost, the
// drill-down marks compaction calls and the calls around each kind of
// compaction, and no figure is labelled "below optimal".

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const (
	markHarness       = "harness compaction (OTel)"
	markCompactCall   = "compaction call"
	markUnappliedCall = "unapplied compaction call"
)

// ccCall turns a call into a compaction call of the given cost.
func ccCall(sec int, cents int64, unapplied bool) domain.ContextCall {
	c := ctxCallOf(sec, 2_000, 10, cents, "model-x")
	c.Kind, c.Label, c.Unapplied = domain.CallCompaction, "summarizeConversationHistory", unapplied
	return c
}

// ccHarnessActor: a 100k -> 30k drop after two compaction calls costing 4c and 6c.
func ccHarnessActor() domain.ActorContext {
	return domain.ActorContext{
		Actor: ctxAgent("harn#1", "harn"),
		Calls: []domain.ContextCall{
			ctxCallOf(10, 100_000, 5, 51, "model-h"), ccCall(20, 4, false), ccCall(30, 6, false),
			ctxCallOf(40, 30_000, 5, 52, "model-h"),
		},
		First: ctxFig(100_000, false), Peak: ctxFig(100_000, false), Final: ctxFig(30_000, false),
		Compactions: []domain.CompactionMark{{
			Kind: domain.CompactionDetected, At: ctxAt(40), Before: 0, After: 3,
			BeforeTokens: 100_000, AfterTokens: 30_000, Harness: true, CallCount: 2, CallCost: known(10),
		}},
		CompactionCalls: domain.CompactionCallFigures{Count: 2, Cost: known(10)},
	}
}

// ccMixedActor: one applied compaction call (4c) and two unapplied ones (9c, 11c).
func ccMixedActor() domain.ActorContext {
	calls := []domain.ContextCall{
		ctxCallOf(10, 100_000, 5, 81, "model-m"), ccCall(20, 4, false), ctxCallOf(30, 30_000, 5, 82, "model-m"),
		ctxCallOf(40, 90_000, 5, 83, "model-m"), ccCall(50, 9, true), ccCall(55, 11, true),
		ctxCallOf(60, 95_000, 5, 84, "model-m"),
	}
	for _, i := range []int{1, 4, 5} {
		calls[i].Model = "model-m"
	}
	return domain.ActorContext{
		Actor: ctxAgent("mixed#1", "mixed"), Calls: calls,
		First: ctxFig(100_000, false), Peak: ctxFig(100_000, false), Final: ctxFig(95_000, false),
		Compactions: []domain.CompactionMark{{
			Kind: domain.CompactionDetected, At: ctxAt(30), Before: 0, After: 2,
			BeforeTokens: 100_000, AfterTokens: 30_000, Harness: true, CallCount: 1, CallCost: known(4),
		}},
		CompactionCalls: domain.CompactionCallFigures{
			Count: 3, Cost: known(24), UnappliedCount: 2, UnappliedCost: known(20),
		},
	}
}

// ccLoggedActor: a drop matched to a logged compaction.
func ccLoggedActor() domain.ActorContext {
	return domain.ActorContext{
		Actor: ctxAgent("logm#1", "logm"),
		Calls: []domain.ContextCall{ctxCallOf(10, 100_000, 5, 91, "model-l"), ctxCallOf(40, 30_000, 5, 92, "model-l")},
		First: ctxFig(100_000, false), Peak: ctxFig(100_000, false), Final: ctxFig(30_000, false),
		Compactions: []domain.CompactionMark{
			{Kind: domain.CompactionLogged, At: ctxAt(35), Before: 0, After: 1},
			{Kind: domain.CompactionDetected, At: ctxAt(40), Before: 0, After: 1,
				BeforeTokens: 100_000, AfterTokens: 30_000, MatchedLogged: true},
		},
	}
}

func compactionRun() domain.RunReport {
	run := contextRun()
	run.Context.Invocations = append(run.Context.Invocations, ccHarnessActor(), ccMixedActor(), ccLoggedActor())
	return run
}

// ccCell is the cell of the actor's row under the named header column.
func ccCell(t *testing.T, view, id, model, column string) string {
	t.Helper()
	block := readHeader(t, view, "instance")
	col, ok := block.col(column)
	if !ok {
		t.Fatalf("no column %q in %q", column, block.titles())
	}
	for _, row := range block.rows {
		if strings.Contains(row, id) && strings.Contains(row, model) {
			return fragment(runesOf(row), col.start, col.end)
		}
	}
	t.Fatalf("no row of %s with model %s\n%s", id, model, stripStyling(view))
	return ""
}

func ccAssertCell(t *testing.T, view, id, model, column, want string) {
	t.Helper()
	if got := ccCell(t, view, id, model, column); got != want {
		t.Errorf("%s %s = %q, want %q", id, column, got, want)
	}
}

func TestContext_HarnessCompactionsAreCountedInTheirOwnColumn(t *testing.T) {
	view := ctxView(t, compactionRun(), tableW, tallH)

	ccAssertCell(t, view, "harn#1", "model-h", "harness", "1")
	ccAssertCell(t, view, "harn#1", "model-h", "detected only", "0")
	ccAssertCell(t, view, "harn#1", "model-h", "logged", "0")
	ccAssertCell(t, view, "mixed#1", "model-m", "harness", "1")
}

func TestContext_DropMatchedToALoggedCompaction_IsNotCountedAsDetectedOnlyOrHarness(t *testing.T) {
	view := ctxView(t, compactionRun(), tableW, tallH)

	ccAssertCell(t, view, "logm#1", "model-l", "logged", "1")
	ccAssertCell(t, view, "logm#1", "model-l", "harness", "0")
	ccAssertCell(t, view, "logm#1", "model-l", "detected only", "0")
}

func TestContext_CompactionCostColumnShowsTheCostOfTheActorsCompactionCalls(t *testing.T) {
	view := ctxView(t, compactionRun(), tableW, tallH)

	ccAssertCell(t, view, "harn#1", "model-h", "cost", "$0.10")
	ccAssertCell(t, view, "mixed#1", "model-m", "cost", "$0.24")
}

func TestContext_UnappliedColumnCountsTheCompactionCallsNoCompactionFollowed(t *testing.T) {
	view := ctxView(t, compactionRun(), tableW, tallH)

	ccAssertCell(t, view, "harn#1", "model-h", "unapplied", "0")
	got := strings.Fields(ccCell(t, view, "mixed#1", "model-m", "unapplied"))
	if len(got) == 0 || got[0] != "2" {
		t.Errorf("mixed#1 unapplied = %q, want the count token 2 first", got)
	}
}

func TestContextDrill_HarnessCompactionShowsItsOwnCallCountAndCost(t *testing.T) {
	// mixed#1: the harness compaction took 1 call costing $0.04, while the actor's
	// compaction calls in total are 3 costing $0.24.
	mixed := strings.ToLower(stripStyling(ctxDrill(t, compactionRun(), "mixed#1", tableW, tallH).View(tableW, tallH)))
	if lineWithAll(mixed, "harness", "1", "$0.04") == "" {
		t.Errorf("no line gives the harness compaction its own 1 call and $0.04\n%s", mixed)
	}

	// harn#1: the harness compaction took 2 calls costing $0.10.
	harn := strings.ToLower(stripStyling(ctxDrill(t, compactionRun(), "harn#1", tableW, tallH).View(tableW, tallH)))
	if lineWithAll(harn, "harness", "2", "$0.10") == "" {
		t.Errorf("no line gives the harness compaction its 2 calls and $0.10\n%s", harn)
	}
}

func TestContextDrill_MarksCompactionCallsAndTheCallsAroundAHarnessCompaction(t *testing.T) {
	view := stripStyling(ctxDrill(t, compactionRun(), "harn#1", tableW, tallH).View(tableW, tallH))

	for _, sec := range []int{10, 40} {
		assertContains(t, ctxCallRow(t, view, sec), markHarness)
		assertNotContains(t, ctxCallRow(t, view, sec), markDetected, markLogged, markCompactCall)
	}
	for _, sec := range []int{20, 30} {
		assertContains(t, ctxCallRow(t, view, sec), markCompactCall)
		assertNotContains(t, ctxCallRow(t, view, sec), "unapplied", markHarness)
	}
}

func TestContextDrill_MarksUnappliedCompactionCallsApartFromAppliedOnes(t *testing.T) {
	view := stripStyling(ctxDrill(t, compactionRun(), "mixed#1", tableW, tallH).View(tableW, tallH))

	assertNotContains(t, ctxCallRow(t, view, 20), "unapplied")
	assertContains(t, ctxCallRow(t, view, 20), markCompactCall)
	for _, sec := range []int{50, 55} {
		assertContains(t, ctxCallRow(t, view, sec), markUnappliedCall)
	}
	for _, sec := range []int{40, 60} {
		assertNotContains(t, ctxCallRow(t, view, sec), markHarness, markCompactCall)
	}
}

func TestContextDrill_StatesTheUnappliedAttemptsWithCountAndCost(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrill(t, compactionRun(), "mixed#1", tableW, tallH).View(tableW, tallH)))

	if lineWithAll(view, "unapplied", "2", "$0.20") == "" {
		t.Errorf("no line states the 2 unapplied compaction calls and their cost $0.20\n%s", view)
	}
}

func TestContextDrill_DropMatchedToALoggedCompaction_ShowsOnlyTheLoggedMark(t *testing.T) {
	view := stripStyling(ctxDrill(t, compactionRun(), "logm#1", tableW, tallH).View(tableW, tallH))

	for _, sec := range []int{10, 40} {
		assertContains(t, ctxCallRow(t, view, sec), markLogged)
		assertNotContains(t, ctxCallRow(t, view, sec), markHarness, markDetected)
	}
}

func TestContext_NoFigureIsLabelledBelowOptimal(t *testing.T) {
	table := strings.ToLower(stripStyling(ctxView(t, compactionRun(), tableW, tallH)))
	assertNotContains(t, table, "below optimal", "below_optimal")

	for _, id := range []string{"tester#1", "coder#1", "builder#1", "orchestrator"} {
		drill := strings.ToLower(stripStyling(ctxDrill(t, compactionRun(), id, tableW, tallH).View(tableW, tallH)))
		assertNotContains(t, drill, "below optimal", "below_optimal")
	}
}

func TestContextDrill_FiguresUnder250000AreNamedOptimal(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrillView(t, "tester#1")))

	if lineWithAll(view, "final", "(optimal)") == "" {
		t.Errorf("the final context 95,000 must be named optimal\n%s", view)
	}
	assertContains(t, view, "optimal")
}
