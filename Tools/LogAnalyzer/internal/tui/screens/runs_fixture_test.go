package screens_test

// Fixture for the runs list: five runs whose figures are all distinct. The runs
// are listed in an order that is none of the sort orders, so a test that sees
// them sorted knows the screen sorted them.

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	runIDOne   = "20260101T120000Z-aaaa" // $12.34, 58 invocations, 1m30s
	runIDTwo   = "20260102T120000Z-bbbb" // $5.67, 86 invocations, 30s, in progress
	runIDThree = "20260103T120000Z-cccc" // unpriced, 31 invocations, 3m0s
	runIDFour  = "20260104T120000Z-dddd" // $7.89 partial, 74 invocations, 45s
	// runIDLong is 47 characters: far longer than a run id normally is.
	runIDLong = "20260105T120000Z-averyveryverylongrunidentifier" // no cost, no duration, 0 invocations
)

func listedRun(id string, money domain.CategoryMoney, invocations, seconds int) domain.RunReport {
	run := domain.RunReport{
		Run:          domain.NamedRun(id),
		Totals:       domain.Totals{Money: money},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
	}
	run.Overview.Subagents.Invocations = invocations
	if seconds > 0 {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		run.Timeline = domain.Timeline{
			Axis:    domain.TimeInterval{Start: start, End: start.Add(time.Duration(seconds) * time.Second)},
			HasAxis: true,
		}
	}
	return run
}

func completeMoney(cents int64) domain.CategoryMoney {
	return domain.CategoryMoney{Total: known(cents), Complete: true}
}

// runsReport lists the five runs in the order long, three, two, four, one.
func runsReport() domain.Report {
	two := listedRun(runIDTwo, completeMoney(567), 86, 30)
	two.Provisional = true
	three := listedRun(runIDThree, domain.CategoryMoney{Total: domain.UnpricedMoney()}, 31, 180)
	three.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	four := listedRun(runIDFour, domain.CategoryMoney{Total: known(789), Complete: false}, 74, 45)
	long := listedRun(runIDLong, domain.CategoryMoney{Total: domain.NoMoneyData()}, 0, 0)
	return domain.Report{
		Runs:     []domain.RunReport{long, three, two, four, listedRun(runIDOne, completeMoney(1234), 58, 90)},
		Currency: domain.Currency,
		Quality:  domain.NewQualitySummary(nil),
	}
}

func runsView(report domain.Report, width, height int) string {
	return screens.NewRunsScreen(report, width, height, paletteStyles()).View(width, height)
}
