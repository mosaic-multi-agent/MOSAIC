package screens_test

// The order in which the Context table and the per-call table of a Context
// drill-down lose columns as the terminal narrows: the sparkline first, the
// figures next by importance, and the columns that name the row (type, model,
// kind) last. Hidden figures stay reachable through the drill-down.

import "testing"

func contextTableAt(t *testing.T) func(width int) string {
	return func(width int) string { return ctxView(t, contextRun(), width, tallH) }
}

func TestContextTable_ColumnsDropFromTheSparklineThroughTheFiguresToTheNamingColumns(t *testing.T) {
	assertDropOrder(t, contextTableAt(t), "instance",
		[]string{"context"},
		[]string{"peak", "final", "calls", "logged", "harness", "det. only", "first", "first write*", "compaction cost", "unapplied", "rewrites"},
		[]string{"type", "model"},
	)
}

func TestContextTable_ModelOutlastsEveryFigureColumn(t *testing.T) {
	assertDropOrder(t, contextTableAt(t), "instance",
		[]string{"peak", "final", "calls", "logged", "harness", "det. only", "first", "first write*", "compaction cost", "unapplied", "rewrites"},
		[]string{"model"},
	)
}

func TestContextTable_SparklineIsDroppedBeforeAnyFigureColumn(t *testing.T) {
	assertDropOrder(t, contextTableAt(t), "instance",
		[]string{"context"},
		[]string{"unapplied", "rewrites", "first", "first write*", "compaction cost", "calls", "logged", "harness", "det. only", "peak", "final"},
	)
}

func TestContextDrillDown_CallTableDropsColumnsFromCacheFiguresToTheNamingColumns(t *testing.T) {
	s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)
	render := func(width int) string { return s.View(width, tallH) }

	assertDropOrder(t, render, "time",
		[]string{"input", "output", "cache read", "cache write", "mark", "context", "cost"},
		[]string{"model", "kind"},
	)
}

func TestContextDrillDown_ModelOutlastsEveryFigureColumn(t *testing.T) {
	s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)
	render := func(width int) string { return s.View(width, tallH) }

	assertDropOrder(t, render, "time",
		[]string{"input", "output", "cache read", "cache write", "mark", "context", "cost"},
		[]string{"model"},
	)
}
