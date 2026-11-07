package session

import (
	"strings"

	"mosaic-run/internal/domain"
)

// rowAtIndex returns the routing table row at the given zero-based index, or
// false if no row with that index exists.
func rowAtIndex(table domain.RoutingTable, idx int) (domain.RoutingRow, bool) {
	for _, row := range table.Rows {
		if row.Index == idx {
			return row, true
		}
	}
	return domain.RoutingRow{}, false
}

// extractAgentIdentifier extracts the agent identifier from an agent instance
// ID (e.g. "agent-a#1" -> "agent-a"). If the input contains no "#", it is
// returned unchanged.
func extractAgentIdentifier(instanceID string) string {
	if idx := strings.LastIndex(instanceID, "#"); idx >= 0 {
		return instanceID[:idx]
	}
	return instanceID
}

// bindRunContext calls BindRunContext on v if v is non-nil and implements
// domain.RunContextBinder. Consultants that do not implement the interface are
// silently skipped, so callers need not check before calling.
func bindRunContext(v any, rc domain.RunContext) {
	if v == nil {
		return
	}
	if binder, ok := v.(domain.RunContextBinder); ok {
		binder.BindRunContext(rc)
	}
}

// rowAgentID returns the bare agent id of a routing row (without any mode
// qualifier). Rows built without an explicit AgentID fall back to the whole
// Subagent cell, which equals the bare id for unqualified rows.
func rowAgentID(row domain.RoutingRow) string {
	if row.AgentID != "" {
		return row.AgentID
	}
	return row.Agent
}

// uniqueAgentIdentifiers returns the unique bare agent identifiers from the
// routing table in first-occurrence order.
func uniqueAgentIdentifiers(table domain.RoutingTable) []string {
	seen := make(map[string]bool, len(table.Rows))
	result := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		id := rowAgentID(row)
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// onInfrastructureAgentTrigger is the named no-op hook that marks the
// infrastructure-agent trigger point in the dispatch loop (FR-40). It is
// called alongside Deps.OnInfrastructureTrigger after each harness invocation,
// making the trigger point discoverable by name. Production code passes nil
// for Deps.OnInfrastructureTrigger; tests inject a counter.
func onInfrastructureAgentTrigger() {
	// no-op: production hook; real infrastructure-agent dispatch goes here.
}

// resolveToRunScoped prepends the run-scoped folder prefix to each artifact
// path that does not already carry it. Paths that already start with prefix
// are passed through unchanged, preventing double-prefixing.
func resolveToRunScoped(paths []string, prefix string) []string {
	if len(paths) == 0 {
		return paths
	}
	resolved := make([]string, len(paths))
	for i, p := range paths {
		if strings.HasPrefix(p, prefix) {
			resolved[i] = p
		} else {
			resolved[i] = prefix + p
		}
	}
	return resolved
}

// formatInputs formats a list of input artifact paths as a comma-separated
// string for the Inputs column of the Execution Log. Returns "" when the
// list is empty (rendered as "-" in the table).
func formatInputs(paths []string) string {
	return strings.Join(paths, ", ")
}
