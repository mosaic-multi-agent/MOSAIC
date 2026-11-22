package analysis

import "mosaic-log-analyzer/internal/domain"

// DispatchWithoutInvocationFindings checks every tool_call_start with a non-empty
// DispatchedInstance on orchestrator streams of named runs. The dispatch has an
// invocation when, in the same run, some agent-instance stream has
// Ref.InstanceHint == id, or agg has an invocation or agent whose
// Actor.Instance == id. Otherwise it gives one dispatch_without_invocation
// finding per dispatch event (warning, Run = Ref.Run, Actor = orchestrator,
// Path = Ref.Path, Detail names the id). Dispatches in the unattributable bucket
// are never checked.
func DispatchWithoutInvocationFindings(streams []Stream, agg domain.Aggregate) []domain.Finding {
	known := map[domain.RunRef]map[domain.AgentInstanceID]bool{}
	mark := func(run domain.RunRef, id domain.AgentInstanceID) {
		if known[run] == nil {
			known[run] = map[domain.AgentInstanceID]bool{}
		}
		known[run][id] = true
	}
	for _, s := range streams {
		if s.Ref.Kind == domain.StreamAgentInstance && s.Ref.InstanceHint != "" {
			mark(s.Ref.Run, s.Ref.InstanceHint)
		}
	}
	for _, r := range agg.Runs {
		for _, inv := range r.Activity.Invocations {
			mark(r.Run, inv.Actor.Instance)
		}
		for _, a := range r.Agents {
			mark(r.Run, a.Actor.Instance)
		}
	}

	var findings []domain.Finding
	for _, s := range streams {
		if s.Ref.Kind != domain.StreamOrchestrator || s.Ref.Run.IsUnattributable() {
			continue
		}
		for _, ev := range s.Events {
			if ev.Type != domain.EventToolCallStart || ev.ToolCallStart == nil {
				continue
			}
			id := ev.ToolCallStart.DispatchedInstance
			if id == "" || known[s.Ref.Run][id] {
				continue
			}
			findings = append(findings, domain.Finding{
				Kind: domain.FindingDispatchWithoutInvocation, Severity: domain.SeverityWarning,
				Path: s.Ref.Path, Run: s.Ref.Run, Actor: domain.Orchestrator(),
				Detail: "subagent dispatch names " + string(id) + ", which has no invocation in the run",
			})
		}
	}
	return findings
}
