package engine

import (
	"errors"

	"mosaic-run/internal/domain"
)

// routeSuccess routes after the row at pos returned SUCCESS.
//
// A row of a block is routed by its typed On Success entry within its scope
// (see afterScopeRow). A row of the run scope is routed by its typed On Success
// entry:
//
//   - a target dispatches exactly the row it names, above or below;
//   - NEXT dispatches the row below, entering the block that row begins, and
//     completes the run past the last row;
//   - a block entry enters the named block;
//   - COMPLETE completes the run.
func routeSuccess(in NextInput, sc ScheduleContext, pos position, currentRow domain.RoutingRow) domain.EngineDecision {
	workflow := in.Workflow
	r := routerFor(in, sc)

	if _, inBlock := workflow.BlockOf(pos.rowIdx); inBlock {
		adv, err := r.afterScopeRow(pos.rowIdx, pos.iteration)
		var unroutable *domain.UnroutableEntryError
		if errors.As(err, &unroutable) {
			return domain.EngineDecision{Stop: &domain.StopDecision{Reason: unroutable.Error(), Err: unroutable}}
		}
		return decisionFor(in, sc, adv, err, noScheduleOnEntry)
	}

	entry := currentRow.Success
	kind := entry.Kind

	switch kind {
	case domain.SuccessComplete:
		return domain.EngineDecision{Complete: &domain.CompleteDecision{FinalState: in.State.CurrentState}}
	case domain.SuccessNext:
		adv, err := r.enterRow(pos.rowIdx + 1)
		return decisionFor(in, sc, adv, err, noScheduleOnEntry)
	case domain.SuccessTarget:
		target := entry.Target.Index
		if target < 0 || target >= len(workflow.Table.Rows) {
			return unroutableStop(pos.rowIdx, columnOnSuccess, string(kind))
		}
		return decisionFor(in, sc, advance{RowIndex: target}, nil, noScheduleOnEntry)
	case domain.SuccessBlockEntry:
		b, ok := workflow.BlockByName(entry.Block)
		if !ok {
			return unroutableStop(pos.rowIdx, columnOnSuccess, string(kind))
		}
		adv, err := r.enterBlock(b)
		return decisionFor(in, sc, adv, err, noScheduleOnEntry)
	default:
		// A zero kind, or a fork outside Mode 1: never routed.
		return unroutableStop(pos.rowIdx, columnOnSuccess, string(kind))
	}
}

// routerFor is the router Next routes with: blocks are routed by scopes and
// iteration dependencies over the iterations the Execution Log shows completed.
func routerFor(in NextInput, sc ScheduleContext) router {
	return newRouter(in.Workflow, sc.Schedules, true).withScopes(in.State.ExecutionLog)
}

// Column names an UnroutableEntryError carries.
const (
	columnOnSuccess  = "On Success"
	columnOnFindings = "On Findings"
)
