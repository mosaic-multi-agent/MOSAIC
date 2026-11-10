package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// statusEnum is the documented status_code enum, in display order.
var statusEnum = []string{
	"SUCCESS", "COMPLETED_NEEDS_ACTION", "PARTIALLY_DONE",
	"NEEDS_CLARIFICATION", "CAPABILITY_EXCEEDED", "BLOCKED",
}

func enumRank(code string) int {
	for i, c := range statusEnum {
		if c == code {
			return i
		}
	}
	return len(statusEnum)
}

// statusKey identifies one status outcome.
type statusKey struct {
	kind domain.StatusKind
	code string
}

func dispatchStatus(d domain.Dispatch) statusKey {
	switch {
	case d.Invocation.State == domain.InvocationOpen:
		return statusKey{kind: domain.StatusOpen}
	case d.Invocation.StatusCode == "":
		return statusKey{kind: domain.StatusNone}
	default:
		return statusKey{kind: domain.StatusReported, code: d.Invocation.StatusCode}
	}
}

// statusLess orders enum codes first, then other codes verbatim, then
// "ended without a status code", then "open".
func statusLess(a, b statusKey) bool {
	if a.kind != b.kind {
		return a.kind < b.kind
	}
	if ra, rb := enumRank(a.code), enumRank(b.code); ra != rb {
		return ra < rb
	}
	return a.code < b.code
}

// buildStatusGroups groups dispatches by status outcome; empty groups are omitted.
func buildStatusGroups(dispatches []domain.Dispatch) []domain.StatusGroup {
	members := make(map[statusKey][]domain.Dispatch)
	for _, d := range dispatches {
		k := dispatchStatus(d)
		members[k] = append(members[k], d)
	}
	keys := make([]statusKey, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return statusLess(keys[i], keys[j]) })

	out := make([]domain.StatusGroup, len(keys))
	for i, k := range keys {
		out[i] = domain.StatusGroup{Kind: k.kind, Code: k.code, Figures: groupFigures(members[k])}
	}
	return out
}

// buildStatusBreakdown builds the by-status view for the run and per agent type.
func buildStatusBreakdown(dispatches []domain.Dispatch, groups []typeGroup) domain.StatusBreakdown {
	b := domain.StatusBreakdown{Run: buildStatusGroups(dispatches)}
	for _, g := range groups {
		b.ByAgentType = append(b.ByAgentType, domain.AgentTypeStatus{
			AgentType: g.agentType,
			Groups:    buildStatusGroups(g.dispatches),
		})
	}
	return b
}
