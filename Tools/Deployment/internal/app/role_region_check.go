package app

import (
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// docformatRole maps a domain role to the role string docformat's role-aware rules expect.
// Roles without role rules (utility, standalone, unknown) map to "", which skips them.
func docformatRole(role domain.AgentRole) string {
	switch role {
	case domain.RoleSubagent:
		return "subagent"
	case domain.RoleOrchestrator:
		return "orchestrator"
	default:
		return ""
	}
}

// roleRegionGaps runs the role-aware docformat check on doc and returns a gap for the
// orchestrator harness region carried by a non-orchestrator agent. Every other role rule
// (missing or forbidden conduct regions) is deliberately dropped: only the orchestrator-region
// case is surfaced to the user. An unparseable document yields no gaps; structural problems
// are reported elsewhere.
func roleRegionGaps(doc []byte, role domain.AgentRole, agentKey string) []domain.Gap {
	libRole := docformatRole(role)
	if libRole == "" || len(doc) == 0 {
		return nil
	}
	parsed, err := ParseGenericSource(doc)
	if err != nil {
		return nil
	}
	var gaps []domain.Gap
	for _, issue := range docformat.Validate(parsed, docformat.ValidateOptions{Role: libRole}) {
		if issue.Code != "role-forbidden-region" || issue.Node != docformat.OrchestratorHarnessRegionName {
			continue
		}
		gaps = append(gaps, domain.Gap{
			Kind:    domain.GapRoleForbiddenRegion,
			Subject: docformat.OrchestratorHarnessRegionName,
			Owner:   agentKey,
			Detail:  "agent " + agentKey + " (" + libRole + ") [" + issue.Code + "]: " + issue.Message,
		})
	}
	return gaps
}
