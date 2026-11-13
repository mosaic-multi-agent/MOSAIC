package screens_test

// The per-run statistics of the runs-list head: which runs count, and the
// average, median and nearest-rank p90 of cost and duration.

import (
	"fmt"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var statsEpoch = time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

// statsRun is a finished run with a known complete cost of nanos and an axis of d.
func statsRun(id string, nanos int64, d time.Duration) domain.RunReport {
	run := listedRun(id, domain.CategoryMoney{Total: domain.KnownMoney(domain.Money(nanos)), Complete: true}, 1, 0)
	run.Timeline = domain.Timeline{Axis: domain.TimeInterval{Start: statsEpoch, End: statsEpoch.Add(d)}, HasAxis: true}
	return run
}

// statsReport lists runs costing the given cents, each lasting one minute.
func statsReport(cents ...int64) domain.Report {
	var runs []domain.RunReport
	for i, c := range cents {
		runs = append(runs, statsRun(fmt.Sprintf("run-%02d", i), usd(c).Nanos(), time.Minute))
	}
	return domain.Report{Runs: runs}
}

// statsReportTimed lists runs costing one cent each with the given durations.
func statsReportTimed(durations ...time.Duration) domain.Report {
	var runs []domain.RunReport
	for i, d := range durations {
		runs = append(runs, statsRun(fmt.Sprintf("run-%02d", i), usd(1).Nanos(), d))
	}
	return domain.Report{Runs: runs}
}

func TestRunsHeadStats_CostAverageAndMedianOfAnOddCount(t *testing.T) {
	stats := screens.RunsHeadStatsOf(statsReport(10, 60, 20))

	if stats.Cost.Count != 3 || stats.Cost.Avg != usd(30) || stats.Cost.Median != usd(20) {
		t.Errorf("cost stats = %+v, want count 3, avg %v, median %v", stats.Cost, usd(30), usd(20))
	}
}

func TestRunsHeadStats_CostMedianOfAnEvenCountIsTheMeanOfTheMiddleTwo(t *testing.T) {
	stats := screens.RunsHeadStatsOf(statsReport(50, 10, 30, 20))

	if stats.Cost.Median != usd(25) {
		t.Errorf("median = %v, want %v", stats.Cost.Median, usd(25))
	}
	if stats.Cost.Avg != usd(27)+usd(1)/2 {
		t.Errorf("avg = %v, want %v", stats.Cost.Avg, usd(27)+usd(1)/2)
	}
}

func TestRunsHeadStats_CostAverageAndMedianRoundHalfUpToANanodollar(t *testing.T) {
	report := domain.Report{Runs: []domain.RunReport{
		statsRun("a", 1, time.Minute), statsRun("b", 2, time.Minute),
	}}

	stats := screens.RunsHeadStatsOf(report)

	if stats.Cost.Avg != 2 || stats.Cost.Median != 2 {
		t.Errorf("avg %d, median %d of 1 and 2 nanodollars, want both 2", stats.Cost.Avg, stats.Cost.Median)
	}
}

func TestRunsHeadStats_P90IsOnlyAvailableFromTenRuns(t *testing.T) {
	nine := screens.RunsHeadStatsOf(statsReport(1, 2, 3, 4, 5, 6, 7, 8, 9))
	ten := screens.RunsHeadStatsOf(statsReport(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))

	if nine.Cost.HasP90 {
		t.Errorf("p90 of 9 runs is available, want it omitted")
	}
	if !ten.Cost.HasP90 {
		t.Errorf("p90 of 10 runs is not available, want it present")
	}
}

func TestRunsHeadStats_P90IsTheNearestRankValue(t *testing.T) {
	cases := []struct {
		runs int
		want int64 // cents; rank ceil(0.9*runs) of the values 1..runs
	}{
		{10, 9},
		{11, 10},
		{20, 18}, // 0.9*20 is not exact in floating point; the rank is still 18
		{100, 90},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d runs", c.runs), func(t *testing.T) {
			cents := make([]int64, c.runs)
			for i := range cents {
				cents[i] = int64(c.runs - i) // listed descending: sorting is the code's job
			}

			stats := screens.RunsHeadStatsOf(statsReport(cents...))

			if !stats.Cost.HasP90 || stats.Cost.P90 != usd(c.want) {
				t.Errorf("p90 = %v (available %v), want %v", stats.Cost.P90, stats.Cost.HasP90, usd(c.want))
			}
		})
	}
}

func TestRunsHeadStats_CostLeavesOutProvisionalAndIncompleteRuns(t *testing.T) {
	report := statsReport(10, 20)
	provisional := statsRun("prov", usd(900).Nanos(), time.Minute)
	provisional.Provisional = true
	unpriced := statsRun("unpriced", 0, time.Minute)
	unpriced.Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	partial := statsRun("partial", usd(800).Nanos(), time.Minute)
	partial.Totals.Money.Complete = false
	noData := statsRun("nodata", 0, time.Minute)
	noData.Totals.Money = domain.CategoryMoney{Total: domain.NoMoneyData(), Complete: true}
	report.Runs = append(report.Runs, provisional, unpriced, partial, noData)

	stats := screens.RunsHeadStatsOf(report)

	if stats.Runs != 6 || stats.Cost.Count != 2 || stats.Cost.Avg != usd(15) {
		t.Errorf("runs %d, cost count %d, avg %v, want 6 runs, 2 counted, avg %v", stats.Runs, stats.Cost.Count, stats.Cost.Avg, usd(15))
	}
}

func TestRunsHeadStats_UnattributableUsageNeverCounts(t *testing.T) {
	report := statsReport(10, 20)
	unattributable := statsRun("unattributed", usd(5000).Nanos(), 10*time.Hour)
	unattributable.Run = domain.UnattributableRun()
	report.Unattributable = &unattributable

	stats := screens.RunsHeadStatsOf(report)

	if stats.Runs != 2 || stats.Cost.Count != 2 || stats.Time.Count != 2 || stats.Cost.Avg != usd(15) {
		t.Errorf("stats = %+v, want only the two named runs counted", stats)
	}
}

func TestRunsHeadStats_DurationAverageAndMedian(t *testing.T) {
	odd := screens.RunsHeadStatsOf(statsReportTimed(time.Minute, 10*time.Minute, 4*time.Minute))
	even := screens.RunsHeadStatsOf(statsReportTimed(time.Minute, 2*time.Minute, 9*time.Minute, 4*time.Minute))

	if odd.Time.Count != 3 || odd.Time.Avg != 5*time.Minute || odd.Time.Median != 4*time.Minute {
		t.Errorf("odd time stats = %+v, want avg 5m, median 4m", odd.Time)
	}
	if even.Time.Median != 3*time.Minute || even.Time.Avg != 4*time.Minute {
		t.Errorf("even time stats = %+v, want median 3m (mean of 2m and 4m), avg 4m", even.Time)
	}
}

func TestRunsHeadStats_DurationRoundsHalfUpToANanosecond(t *testing.T) {
	stats := screens.RunsHeadStatsOf(statsReportTimed(1, 2))

	if stats.Time.Avg != 2 || stats.Time.Median != 2 {
		t.Errorf("avg %d, median %d of 1ns and 2ns, want both 2", stats.Time.Avg, stats.Time.Median)
	}
}

func TestRunsHeadStats_DurationLeavesOutProvisionalAndAxislessRuns(t *testing.T) {
	report := statsReportTimed(time.Minute, 3*time.Minute)
	provisional := statsRun("prov", usd(1).Nanos(), time.Hour)
	provisional.Provisional = true
	axisless := statsRun("axisless", usd(1).Nanos(), 0)
	axisless.Timeline = domain.Timeline{}
	report.Runs = append(report.Runs, provisional, axisless)

	stats := screens.RunsHeadStatsOf(report)

	if stats.Time.Count != 2 || stats.Time.Avg != 2*time.Minute {
		t.Errorf("time stats = %+v, want 2 runs counted with avg 2m", stats.Time)
	}
	if stats.Cost.Count != 3 {
		t.Errorf("cost count = %d, want 3: an axisless run still has a cost", stats.Cost.Count)
	}
}

func TestRunsHeadStats_NoCountedRunsLeavesTheCountsAtZero(t *testing.T) {
	provisional := statsRun("prov", usd(1).Nanos(), time.Minute)
	provisional.Provisional = true

	stats := screens.RunsHeadStatsOf(domain.Report{Runs: []domain.RunReport{provisional}})

	if stats.Runs != 1 || stats.InProgress != 1 || stats.Cost.Count != 0 || stats.Time.Count != 0 {
		t.Errorf("stats = %+v, want 1 run in progress and nothing counted", stats)
	}
}

// twentyThreeRuns is the reference fixture: 23 runs, one in progress, one other
// with an unpriced model, and every run that is not in progress has an axis.
func twentyThreeRuns() domain.Report {
	var runs []domain.RunReport
	for i := 0; i < 23; i++ {
		runs = append(runs, statsRun(fmt.Sprintf("run-%02d", i), usd(int64(100+i)).Nanos(), time.Duration(60+i)*time.Minute))
	}
	runs[3].Provisional = true
	runs[7].Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	runs[7].UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	return domain.Report{Runs: runs}
}

func TestRunsHeadStats_TwentyThreeRunFixtureCoverage(t *testing.T) {
	stats := screens.RunsHeadStatsOf(twentyThreeRuns())

	if stats.Runs != 23 || stats.InProgress != 1 {
		t.Errorf("runs %d, in progress %d, want 23 and 1", stats.Runs, stats.InProgress)
	}
	if stats.Cost.Count != 21 {
		t.Errorf("cost count = %d, want 21", stats.Cost.Count)
	}
	if stats.Time.Count != 22 {
		t.Errorf("time count = %d, want 22", stats.Time.Count)
	}
	if !stats.Cost.HasP90 {
		t.Errorf("p90 of 21 runs is not available, want it present")
	}
}

func TestRunsHeadStats_DateRangeSpansEarliestStartToLatestEndIncludingProvisional(t *testing.T) {
	early := statsRun("early", usd(1).Nanos(), time.Hour)
	late := statsRun("late", usd(1).Nanos(), time.Hour)
	late.Timeline.Axis = domain.TimeInterval{Start: statsEpoch.Add(48 * time.Hour), End: statsEpoch.Add(50 * time.Hour)}
	lastEnding := statsRun("provisional", usd(1).Nanos(), time.Hour)
	lastEnding.Provisional = true
	lastEnding.Timeline.Axis = domain.TimeInterval{Start: statsEpoch.Add(24 * time.Hour), End: statsEpoch.Add(100 * time.Hour)}
	axisless := statsRun("axisless", usd(1).Nanos(), 0)
	axisless.Timeline = domain.Timeline{}

	stats := screens.RunsHeadStatsOf(domain.Report{Runs: []domain.RunReport{late, axisless, lastEnding, early}})

	if !stats.HasRange || !stats.First.Equal(statsEpoch) || !stats.Last.Equal(statsEpoch.Add(100*time.Hour)) {
		t.Errorf("range = %v to %v (has %v), want %v to %v", stats.First, stats.Last, stats.HasRange, statsEpoch, statsEpoch.Add(100*time.Hour))
	}
}

func TestRunsHeadStats_NoAxisAnywhereMeansNoRange(t *testing.T) {
	run := statsRun("axisless", usd(1).Nanos(), 0)
	run.Timeline = domain.Timeline{}

	stats := screens.RunsHeadStatsOf(domain.Report{Runs: []domain.RunReport{run}})

	if stats.HasRange {
		t.Errorf("HasRange is true for runs without an axis")
	}
}

func TestRunsHeadStats_TotalsComeFromAllRunsAndTokensAreTheBillableSum(t *testing.T) {
	report := statsReport(10)
	report.AllRuns = domain.Totals{
		Tokens: domain.TokenUsage{
			Input: domain.Tokens(1_000), CacheRead: domain.Tokens(200), CacheCreation: domain.Tokens(30),
			Output: domain.Tokens(4), Reasoning: domain.Tokens(2),
		},
		Money: domain.CategoryMoney{Total: domain.KnownMoney(usd(777)), Complete: false},
	}

	stats := screens.RunsHeadStatsOf(report)

	if got, ok := stats.Tokens.Value(); !ok || got != 1_234 {
		t.Errorf("tokens = %d (present %v), want 1234: reasoning is part of output", got, ok)
	}
	if stats.TotalCost != report.AllRuns.Money {
		t.Errorf("total cost = %+v, want %+v", stats.TotalCost, report.AllRuns.Money)
	}
}
