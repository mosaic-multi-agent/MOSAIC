package screens_test

// Fixture and helpers of the Context view tests. The run holds an orchestrator
// and five invocations with distinct context figures, so a figure found on a row
// can only have come from that actor:
//
//	orchestrator  40k -> 180k -> 92k -> 130k   2 logged, 1 detected, no first write
//	builder#1     30k -> 260k -> 380k -> 360k  first write on the 2nd call, no compaction
//	coder#1       100k -> 200k -> 300k -> 90k -> 80k   1 detected, no first write
//	reviewer#1    120k -> 150k -> 210k (partial)       1 logged, first write on the 2nd call
//	tester#1      50k -> 95k                           unknown type, no marks
//	ghost#1       no per-call records

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var ctxBase = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func ctxAt(sec int) time.Time { return ctxBase.Add(time.Duration(sec) * time.Second) }

// ctxFig is a present context figure with the band of its size.
func ctxFig(tokens int64, partial bool) domain.ContextFigure {
	return domain.ContextFigure{Tokens: domain.Tokens(tokens), Partial: partial, Band: domain.BandOf(tokens)}
}

// ctxCallOf is a complete call whose context is the given size: cache read is a
// quarter of it, cache write is always 1,001 and input is the rest.
func ctxCallOf(sec int, context, output, cents int64, model string) domain.ContextCall {
	cacheRead := context / 4
	const cacheWrite = int64(1001)
	return domain.ContextCall{
		At:       ctxAt(sec),
		RecordID: fmt.Sprintf("rec-%d", sec),
		Model:    domain.ModelID(model),
		Usage: domain.TokenUsage{
			Input:         domain.Tokens(context - cacheRead - cacheWrite),
			CacheRead:     domain.Tokens(cacheRead),
			CacheCreation: domain.Tokens(cacheWrite),
			Output:        domain.Tokens(output),
		},
		Context: domain.Tokens(context),
		Cost:    known(cents),
	}
}

func ctxAgent(id, typ string) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: typ}
}

func ctxOrchestrator() domain.ActorContext {
	return domain.ActorContext{
		Actor: domain.Orchestrator(),
		Calls: []domain.ContextCall{
			ctxCallOf(0, 40_000, 100, 10, "model-o"),
			ctxCallOf(60, 180_000, 100, 20, "model-o"),
			ctxCallOf(120, 92_000, 100, 30, "model-o"),
			ctxCallOf(180, 130_000, 100, 40, "model-o"),
		},
		First: ctxFig(40_000, false), Peak: ctxFig(180_000, false), Final: ctxFig(130_000, false),
		Compactions: []domain.CompactionMark{
			{Kind: domain.CompactionLogged, At: ctxAt(100), Before: 1, After: 2},
			{Kind: domain.CompactionDetected, At: ctxAt(120), Before: 1, After: 2, BeforeTokens: 180_000, AfterTokens: 92_000},
			{Kind: domain.CompactionLogged, At: ctxAt(170), Before: 2, After: 3},
		},
	}
}

func ctxBuilder() domain.ActorContext {
	calls := []domain.ContextCall{
		ctxCallOf(10, 30_000, 111, 50, "model-b"),
		ctxCallOf(70, 260_000, 222, 22, "model-b"),
		ctxCallOf(130, 380_000, 333, 33, "model-b"),
		ctxCallOf(190, 360_000, 444, 44, "model-b"),
	}
	calls[1].FirstWrite = true
	return domain.ActorContext{
		Actor: ctxAgent("builder#1", "builder"), Calls: calls,
		First: ctxFig(30_000, false), Peak: ctxFig(380_000, false), Final: ctxFig(360_000, false),
		FirstWrite: ctxFig(260_000, false), FirstWriteAt: ctxAt(65),
	}
}

func ctxCoder() domain.ActorContext {
	return domain.ActorContext{
		Actor: ctxAgent("coder#1", "coder"),
		Calls: []domain.ContextCall{
			ctxCallOf(30, 100_000, 11, 61, "model-c"),
			ctxCallOf(90, 200_000, 12, 62, "model-c"),
			ctxCallOf(150, 300_000, 13, 63, "model-c"),
			ctxCallOf(210, 90_000, 14, 64, "model-c"),
			ctxCallOf(270, 80_000, 15, 65, "model-c"),
		},
		First: ctxFig(100_000, false), Peak: ctxFig(300_000, false), Final: ctxFig(80_000, false),
		Compactions: []domain.CompactionMark{
			{Kind: domain.CompactionDetected, At: ctxAt(210), Before: 2, After: 3, BeforeTokens: 300_000, AfterTokens: 90_000},
		},
	}
}

// ctxReviewer ends with a partial call: it reports no cache write.
func ctxReviewer() domain.ActorContext {
	last := ctxCallOf(140, 210_000, 33, 33, "model-r")
	last.Usage.Input, last.Usage.CacheRead = domain.Tokens(200_031), domain.Tokens(9_969)
	last.Usage.CacheCreation = domain.AbsentTokens()
	last.Partial = true
	calls := []domain.ContextCall{ctxCallOf(20, 120_000, 11, 31, "model-r"), ctxCallOf(80, 150_000, 22, 32, "model-r"), last}
	calls[1].FirstWrite = true
	return domain.ActorContext{
		Actor: ctxAgent("reviewer#1", "reviewer"), Calls: calls,
		First: ctxFig(120_000, false), Peak: ctxFig(210_000, true), Final: ctxFig(210_000, true),
		FirstWrite: ctxFig(150_000, false), FirstWriteAt: ctxAt(85),
		Compactions: []domain.CompactionMark{{Kind: domain.CompactionLogged, At: ctxAt(50), Before: 0, After: 1}},
	}
}

func ctxTester() domain.ActorContext {
	return domain.ActorContext{
		Actor: ctxAgent("tester#1", ""),
		Calls: []domain.ContextCall{ctxCallOf(40, 50_000, 5, 71, "model-t"), ctxCallOf(100, 95_000, 6, 72, "model-t")},
		First: ctxFig(50_000, false), Peak: ctxFig(95_000, false), Final: ctxFig(95_000, false),
	}
}

// contextRun is a run whose Context holds the actors above; the ghost invocation
// has no per-call record, so every one of its figures is absent.
func contextRun() domain.RunReport {
	run := overviewRun()
	run.Context = domain.RunContext{
		Orchestrator: ctxOrchestrator(),
		Invocations: []domain.ActorContext{
			ctxBuilder(), ctxCoder(),
			{Actor: ctxAgent("ghost#1", "ghost")},
			ctxReviewer(), ctxTester(),
		},
	}
	return run
}

// ctxView renders the Context view with colour off.
func ctxView(t *testing.T, run domain.RunReport, w, h int) string {
	t.Helper()
	colourOff(t)
	return screens.NewContextScreen(run, w, h, paletteStyles()).View(w, h)
}

// ctxOpen opens the Context view of the standard run with colour off.
func ctxOpen(t *testing.T) *screens.ContextScreen {
	t.Helper()
	colourOff(t)
	return screens.NewContextScreen(contextRun(), tableW, tallH, paletteStyles())
}

// ctxModels names the model each actor's row carries, which tells its row from
// any other line that mentions the actor.
var ctxModels = map[string]string{
	"orchestrator": "model-o", "builder#1": "model-b", "coder#1": "model-c",
	"reviewer#1": "model-r", "tester#1": "model-t", "ghost#1": "",
}

// ctxRowLine is the styling-free table row of an actor.
func ctxRowLine(t *testing.T, view, id string) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if strings.Contains(l, id) && strings.Contains(l, ctxModels[id]) && !strings.Contains(l, "sort:") {
			return l
		}
	}
	t.Fatalf("no table row for %s\n%s", id, stripStyling(view))
	return ""
}

// ctxCells are the figures of a row, read from its right end: the sparkline, when
// the row has one, is dropped first, so rows without calls parse too.
type ctxCells struct{ calls, first, firstWrite, peak, final, logged, detected string }

func ctxRowCells(t *testing.T, view, id string) ctxCells {
	t.Helper()
	var merged []string
	for _, tok := range strings.Fields(ctxRowLine(t, view, id)) {
		if tok == "(partial)" && len(merged) > 0 {
			merged[len(merged)-1] += " (partial)"
			continue
		}
		merged = append(merged, tok)
	}
	if n := len(merged); n > 0 && strings.Trim(merged[n-1], "▁▂▃▄▅▆▇█┃│┊") == "" {
		merged = merged[:n-1]
	}
	if len(merged) < 7 {
		t.Fatalf("row of %s has fewer than 7 figures: %q", id, merged)
	}
	c := merged[len(merged)-7:]
	return ctxCells{c[0], c[1], c[2], c[3], c[4], c[5], c[6]}
}

// ctxSparkline is the sparkline token ending a row, or "" when it has none.
func ctxSparkline(t *testing.T, view, id string) string {
	t.Helper()
	fields := strings.Fields(ctxRowLine(t, view, id))
	last := fields[len(fields)-1]
	if strings.Trim(last, "▁▂▃▄▅▆▇█┃│┊") != "" {
		return ""
	}
	return last
}

// ctxDrill opens the context view of the standard run and drills into an actor:
// it moves the cursor down until the actor's row is selected, then presses enter.
func ctxDrill(t *testing.T, run domain.RunReport, id string, w, h int) *screens.ContextScreen {
	t.Helper()
	colourOff(t)
	s := screens.NewContextScreen(run, w, h, paletteStyles())
	for i := 0; i < 12; i++ {
		row := ctxRowLine(t, s.View(w, h), id)
		if strings.HasPrefix(strings.TrimSpace(row), ">") {
			press(s, "enter")
			if after := stripStyling(s.View(w, h)); strings.Contains(after, "reviewer#1") && strings.Contains(after, "coder#1") {
				t.Fatalf("enter on %s did not open a drill-down; the table is still shown\n%s", id, after)
			}
			return s
		}
		press(s, "down")
	}
	t.Fatalf("the cursor never reached the row of %s", id)
	return nil
}

// ctxCallRow is the per-call table row of the call made at the given second.
func ctxCallRow(t *testing.T, view string, sec int) string {
	t.Helper()
	label := ctxAt(sec).Format("15:04:05")
	for _, l := range viewLines(view) {
		if strings.Contains(l, label) {
			return l
		}
	}
	t.Fatalf("no per-call row at %s\n%s", label, stripStyling(view))
	return ""
}

// ctxCallLabels are the clock labels of the calls made at the given seconds.
func ctxCallLabels(secs ...int) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		out[i] = ctxAt(s).Format("15:04:05")
	}
	return out
}
