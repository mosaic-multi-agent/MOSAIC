package analysis

import (
	"fmt"
	"regexp"
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// instanceIDForm is the {type}#{N} form of an agent instance id.
var instanceIDForm = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_.-]*)#[0-9]+$`)

// RunModelIntegrityFindings checks every agent instance of the run, taken from
// both run.Agents and run.Activity.Invocations (one finding per instance id): an
// id of the {type}#{N} form whose known agent type differs from {type} gives one
// instance_type_mismatch finding; an id lacking that form gives one
// unresolved_invocation finding stating its usage record count. Findings are
// warning severity, scoped to run.Run and the instance's actor, ordered by
// instance id.
func RunModelIntegrityFindings(run domain.RunAggregate) []domain.Finding {
	type instance struct {
		actor domain.ActorRef
		usage int
	}
	seen := make(map[domain.AgentInstanceID]instance)
	add := func(actor domain.ActorRef, usage int) {
		if actor.Kind != domain.ActorAgentInstance {
			return
		}
		if _, ok := seen[actor.Instance]; !ok {
			seen[actor.Instance] = instance{actor: actor, usage: usage}
		}
	}
	for _, inv := range run.Activity.Invocations {
		add(inv.Actor, len(inv.Usage))
	}
	for _, a := range run.Agents {
		add(a.Actor, len(a.Invocations))
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	var findings []domain.Finding
	for _, id := range ids {
		in := seen[domain.AgentInstanceID(id)]
		m := instanceIDForm.FindStringSubmatch(id)
		switch {
		case m == nil:
			findings = append(findings, domain.Finding{
				Kind: domain.FindingUnresolvedInvocation, Severity: domain.SeverityWarning,
				Run: run.Run, Actor: in.actor,
				Detail: fmt.Sprintf("instance id %q is not of the {type}#{N} form; %d usage records", id, in.usage),
			})
		case in.actor.Type != "" && in.actor.Type != m[1]:
			findings = append(findings, domain.Finding{
				Kind: domain.FindingInstanceTypeMismatch, Severity: domain.SeverityWarning,
				Run: run.Run, Actor: in.actor,
				Detail: fmt.Sprintf("instance id %q holds agent type %q, not %q", id, in.actor.Type, m[1]),
			})
		}
	}
	return findings
}
