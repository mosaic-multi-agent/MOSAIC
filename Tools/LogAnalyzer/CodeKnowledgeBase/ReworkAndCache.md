# Rework, cache and comparison figures

Implemented in `internal/analysis/rework.go`, `cache.go` and `compare.go`; figure types in `internal/domain/views_rework.go`, `views_cache.go` and `views_compare.go`.

## Likely-rework rule

Dispatches of a run are ordered by start time ascending, then instance id; dispatches with no start time come last. A dispatch is a **repeat** when its agent type (the unknown type "" included) has an earlier dispatch in that order. Repeats include planned repeats.

A repeat with a start time is **likely rework** when either holds:

- the previous dispatch of the same type did not end `SUCCESS` (open, closed without a status, or any other status code, compared exactly as logged);
- the most recent other dispatch of any type whose end time is set and at or before the repeat's start ended `COMPLETED_NEEDS_ACTION` (when several share that end time, any one qualifies).

First dispatches of a type and repeats without a start time are never likely rework. The result is a heuristic: logs do not record the workflow step of a dispatch (`domain.ReworkLimitNote`), and every place that shows it must say so.

## Outcome split

Subagent dispatches only (orchestrator excluded): `Success` = closed with `SUCCESS`; `OtherStatus` = closed with any other non-empty status; `OpenOrNone` = open or closed without a status. The three costs sum to the subagent cost.

## Cache figures

Per dispatch, agent type and run (`CacheFigures`). Hit ratio, write tokens and write cost come from the scope's totals. Net benefit and unread writes come from per-call records (those with a record id):

- net benefit per call = (cache read at the input rate - cache read cost) - (cache write cost - cache write tokens at the input rate), each call priced at its own model and tier (`cost.RatesFor`); unpriced calls are left out and mark the figure incomplete; no contributing call gives NoData, only unpriced calls give Unpriced; negative values are valid;
- unread writes = cache-write tokens and cost of each invocation's last call; the run scope adds only the orchestrator's last call.

## Run comparison

`analysis.CompareRuns` (reached by the TUI through `app.CompareRuns`) reads only totals, per-type groups and their dispatches, so digest-loaded runs compare. A figure a side lacks is absent. Percent needs both sides present, a non-zero base and (for cost) complete totals; direction is unknown unless both sides are present (and complete for cost). Row order: largest absolute cost change first, then rows without a known cost delta by the present side's cost, ties by type name.
