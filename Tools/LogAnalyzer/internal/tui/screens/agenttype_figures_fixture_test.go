package screens_test

// Fixture for the by-agent-type figures that are new to the table: peak context,
// cache, repeat spend and likely rework per type. Every figure is set directly and
// differs from every other figure of the fixture.

import (
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func contextFigure(tokens int64) domain.ContextFigure {
	return domain.ContextFigure{Tokens: domain.Tokens(tokens), Band: domain.BandOf(tokens)}
}

func peakStats(basis int, average, max int64) domain.PeakContextStats {
	return domain.PeakContextStats{Basis: basis, Average: domain.Tokens(average), Max: contextFigure(max)}
}

func reworkOf(repeats int, repeatCents int64, likely int, likelyCents int64) domain.ReworkFigures {
	return domain.ReworkFigures{
		Repeats:      domain.GroupFigures{Invocations: repeats, Totals: moneyTotals(repeatCents)},
		LikelyRework: domain.GroupFigures{Invocations: likely, Totals: moneyTotals(likelyCents)},
	}
}

// typesFiguresRun is agentTypesRun on a $100.00 run, with these figures added:
//
//	         avg peak  max peak  hit    cache write       repeat spend  likely rework
//	builder  111,111   150,000   75.0%  23,456 / $0.88    11 / $4.44    10 / $1.33
//	reviewer 262,626   380,380   12.5%  65,432 / $5.43    14 / $2.52    13 / $1.78
//	unknown  150,150   150,150   absent absent            absent        absent
func typesFiguresRun() domain.RunReport {
	run := agentTypesRun()
	run.Totals.Money.Total = known(10000)
	run.Overview.Totals.Money.Total = known(10000)
	b, r, u := &run.ByAgentType[0], &run.ByAgentType[1], &run.ByAgentType[2]
	b.PeakContext, r.PeakContext, u.PeakContext = peakStats(12, 111_111, 150_000), peakStats(15, 262_626, 380_380), peakStats(1, 150_150, 150_150)
	b.Cache = cacheOf(3, 4, 23_456, 88, 222, 5_001, 41)
	r.Cache = cacheOf(1, 8, 65_432, 543, 333, 6_002, 52)
	b.Rework, r.Rework = reworkOf(11, 444, 10, 133), reworkOf(14, 252, 13, 178)
	return run
}

func typesFiguresView(run domain.RunReport, width, height int) string {
	return screens.NewAgentTypesScreen(run, width, height, paletteStyles()).View(width, height)
}

// typesSortAscending are the types from the lowest to the highest by each sort
// key of the by-type list (the unknown type is left out of the name order).
func typesSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost:        {screens.UnknownTypeLabel, "reviewer", "builder"},
		screens.SortInvocations: {"builder", "reviewer", screens.UnknownTypeLabel},
		screens.SortPeakContext: {"builder", screens.UnknownTypeLabel, "reviewer"},
		screens.SortName:        {"builder", "reviewer"},
	}
}
