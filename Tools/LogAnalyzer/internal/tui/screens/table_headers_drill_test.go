package screens_test

// A Session screen whose per-call table holds many calls with small figures,
// so the multi-word titles "cache read" and "cache write" wrap onto a second
// header line and the table has to scroll under its cursor.

import "mosaic-log-analyzer/internal/domain"

const manyCallsID = "scroller#1"

// manyCallsRun is the standard Context run plus an actor with 40 small calls.
func manyCallsRun() domain.RunReport {
	run := contextRun()
	calls := make([]domain.ContextCall, 40)
	for i := range calls {
		calls[i] = ctxCallOf(10*(i+1), 4_000, 5, int64(i+1), "model-s")
	}
	run.Context.Invocations = append(run.Context.Invocations, domain.ActorContext{
		Actor: ctxAgent(manyCallsID, "scroller"), Calls: calls,
		First: ctxFig(4_000, false), Peak: ctxFig(4_000, false), Final: ctxFig(4_000, false),
	})
	return run
}
