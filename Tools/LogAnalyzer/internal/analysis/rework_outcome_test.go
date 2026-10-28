package analysis_test

// Tests for the outcome split: the run's dispatch cost divided into dispatches
// ending SUCCESS, dispatches ending with any other status, and open or
// no-status dispatches.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestOutcomeSplit_GroupsDispatchesByHowTheyEnded(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "SUCCESS", 0, 10, 1000),
		rwClosed("tester#1", "tester", "BLOCKED", 0, 10, 2000),
		rwClosed("docs#1", "docs", "", 0, 10, 4000),
		rwOpen("review#1", "review", 0, 8000),
	)

	split := run.Overview.Outcomes

	rwAssertGroup(t, "success", split.Success, 1, 1000*3000)
	rwAssertGroup(t, "other status", split.OtherStatus, 1, 2000*3000)
	rwAssertGroup(t, "open or no status", split.OpenOrNone, 2, (4000+8000)*3000)
}

func TestOutcomeSplit_GroupCostsSumToTheSubagentCost(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "SUCCESS", 0, 10, 1000),
		rwClosed("tester#1", "tester", "FAILED", 0, 10, 2000),
		rwOpen("review#1", "review", 0, 8000),
	)

	split := run.Overview.Outcomes
	sum, _ := domain.SumMoney(
		split.Success.Totals.Money.Total,
		split.OtherStatus.Totals.Money.Total,
		split.OpenOrNone.Totals.Money.Total,
	)

	assertMoneyKnown(t, "sum of the three groups", sum, 11000*3000)
	assertMoneyKnown(t, "subagent total", run.Overview.Subagents.Totals.Money.Total, 11000*3000)
}

func TestOutcomeSplit_ExcludesTheOrchestrator(t *testing.T) {
	agg := rwAggregate(rwRun{
		id:         ctxRunID,
		orch:       rwInputUsage("orchestrator", 5000),
		dispatches: []rwDispatch{rwClosed("coder#1", "coder", "SUCCESS", 0, 10, 1000)},
	})

	run := rwPrice(t, agg, ctxRunID)

	rwAssertGroup(t, "success", run.Overview.Outcomes.Success, 1, 1000*3000)
	rwAssertEmptyGroup(t, "other status", run.Overview.Outcomes.OtherStatus)
	rwAssertEmptyGroup(t, "open or no status", run.Overview.Outcomes.OpenOrNone)
}

func TestOutcomeSplit_StatusIsComparedExactlyAsLogged(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "success", 0, 10, 1000),
		rwClosed("coder#2", "coder", "SUCCESS ", 20, 30, 2000),
	)

	split := run.Overview.Outcomes

	rwAssertEmptyGroup(t, "success", split.Success)
	rwAssertGroup(t, "other status", split.OtherStatus, 2, 3000*3000)
}

func TestOutcomeSplit_UnpricedMemberMakesTheGroupPartialNeverZero(t *testing.T) {
	failed := rwClosed("coder#1", "coder", "FAILED", 0, 10, 1000)
	failedUnpriced := rwClosed("coder#2", "coder", "FAILED", 20, 30, 0)
	failedUnpriced.usage = rwUnpricedUsage("coder#2", 500)
	onlyUnpriced := rwClosed("tester#1", "tester", "", 0, 10, 0)
	onlyUnpriced.usage = rwUnpricedUsage("tester#1", 700)

	run := rwPriceRun(t, failed, failedUnpriced, onlyUnpriced)

	other := run.Overview.Outcomes.OtherStatus.Totals.Money
	if other.Complete {
		t.Error("other-status money is complete, want partial with an unpriced member")
	}
	assertMoneyKnown(t, "other-status partial total", other.Total, 1000*3000)

	none := run.Overview.Outcomes.OpenOrNone.Totals.Money
	assertMoneyState(t, "no-status total with only unpriced members", none.Total, domain.MoneyUnpriced)
	if none.Complete {
		t.Error("no-status money is complete, want partial")
	}
}
