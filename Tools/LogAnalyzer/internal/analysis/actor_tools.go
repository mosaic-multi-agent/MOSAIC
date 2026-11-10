package analysis

import (
	"mosaic-log-analyzer/internal/domain"
)

// buildActorSessions builds the Session screen figures: the orchestrator first,
// then one session per invocation sorted by instance id.
func buildActorSessions(run domain.RunAggregate, rr domain.RunReport) []domain.ActorSession {
	harnesses := resolveActorHarnesses(run)
	out := make([]domain.ActorSession, 0, 1+len(rr.Context.Invocations))

	orchCalls := tagCalls(run.Activity.OrchestratorToolCalls, harnesses.orchestrator)
	out = append(out, newActorSession(rr, rr.Context.Orchestrator, orchCalls))

	toolCalls := make(map[domain.AgentInstanceID][]domain.ToolCall, len(run.Activity.Invocations))
	for _, inv := range run.Activity.Invocations {
		toolCalls[inv.Actor.Instance] = inv.ToolCalls
	}
	for _, ctx := range rr.Context.Invocations {
		calls := tagCalls(toolCalls[ctx.Actor.Instance], harnesses.of(ctx.Actor))
		out = append(out, newActorSession(rr, ctx, calls))
	}
	return out
}

// newActorSession pairs the actor's tool figures, computed with the Tools view's
// rules, with its findings.
func newActorSession(rr domain.RunReport, ctx domain.ActorContext, calls []harnessedCall) domain.ActorSession {
	tools := toolFigures(calls, growthByTool([]domain.ActorContext{ctx}))
	return domain.ActorSession{
		Actor:    ctx.Actor,
		Findings: buildActorFindings(rr, ctx, tools),
		Tools:    tools,
	}
}
