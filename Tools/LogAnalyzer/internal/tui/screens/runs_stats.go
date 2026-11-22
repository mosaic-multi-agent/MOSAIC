package screens

import (
	"slices"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// RunsHeadStats are the figures of the runs-list head: how many runs, over what
// period, and what a run typically costs and lasts.
type RunsHeadStats struct {
	Runs       int                  // len(Report.Runs)
	InProgress int                  // runs with Provisional
	First      time.Time            // earliest Timeline.Axis.Start; valid when HasRange
	Last       time.Time            // latest Timeline.Axis.End; valid when HasRange
	HasRange   bool                 // at least one run has a timeline axis
	TotalCost  domain.CategoryMoney // Report.AllRuns.Money
	Tokens     domain.TokenCount    // sum of the billable categories of Report.AllRuns.Tokens
	Cost       RunsCostStats
	Time       RunsTimeStats
}

// RunsCostStats are the statistics of per-run cost.
type RunsCostStats struct {
	Count  int          // runs counted: not Provisional, complete known cost
	Avg    domain.Money // valid when Count > 0
	Median domain.Money // valid when Count > 0
	P90    domain.Money // valid when HasP90
	HasP90 bool         // Count >= 10
}

// RunsTimeStats are the statistics of per-run duration.
type RunsTimeStats struct {
	Count  int           // runs counted: not Provisional, Timeline.HasAxis
	Avg    time.Duration // valid when Count > 0
	Median time.Duration // valid when Count > 0
}

// RunsHeadStatsOf computes the head statistics from r.Runs (named runs only;
// r.Unattributable never counts). Pure.
func RunsHeadStatsOf(r domain.Report) RunsHeadStats {
	stats := RunsHeadStats{
		Runs:      len(r.Runs),
		TotalCost: r.AllRuns.Money,
		Tokens:    tokenSum(r.AllRuns.Tokens),
	}
	var costs []int64
	var durations []int64
	for _, run := range r.Runs {
		if run.Provisional {
			stats.InProgress++
		}
		if run.Timeline.HasAxis {
			axis := run.Timeline.Axis
			if !stats.HasRange || axis.Start.Before(stats.First) {
				stats.First = axis.Start
			}
			if !stats.HasRange || axis.End.After(stats.Last) {
				stats.Last = axis.End
			}
			stats.HasRange = true
		}
		if run.Provisional {
			continue
		}
		if m := run.Totals.Money; m.Complete && m.Total.State == domain.MoneyKnown {
			costs = append(costs, m.Total.Amount.Nanos())
		}
		if run.Timeline.HasAxis {
			durations = append(durations, int64(runDuration(run)))
		}
	}
	slices.Sort(costs)
	slices.Sort(durations)

	stats.Cost.Count = len(costs)
	if len(costs) > 0 {
		stats.Cost.Avg = domain.Money(meanHalfUp(costs))
		stats.Cost.Median = domain.Money(medianHalfUp(costs))
	}
	if len(costs) >= p90MinRuns {
		stats.Cost.HasP90 = true
		stats.Cost.P90 = domain.Money(costs[nearestRank(len(costs), 90)-1])
	}
	stats.Time.Count = len(durations)
	if len(durations) > 0 {
		stats.Time.Avg = time.Duration(meanHalfUp(durations))
		stats.Time.Median = time.Duration(medianHalfUp(durations))
	}
	return stats
}

// p90MinRuns is the fewest counted runs for which the cost p90 is shown.
const p90MinRuns = 10

// meanHalfUp is the mean of non-empty non-negative values, rounded half up.
func meanHalfUp(values []int64) int64 {
	var sum int64
	for _, v := range values {
		sum += v
	}
	n := int64(len(values))
	return (2*sum + n) / (2 * n)
}

// medianHalfUp is the median of sorted non-empty values; an even count gives the
// mean of the two middle values, rounded half up.
func medianHalfUp(sorted []int64) int64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2] + 1) / 2
}

// nearestRank is the 1-based nearest-rank position ceil(percent*n/100), computed in
// integers so it is exact.
func nearestRank(n, percent int) int {
	return (percent*n + 99) / 100
}
