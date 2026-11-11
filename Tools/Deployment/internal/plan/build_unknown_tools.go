package plan

// build_unknown_tools.go reports tool names written into an agent for a harness that silently
// ignores names it does not know.

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// unknownHarnessToolGaps returns one GapUnknownHarnessTool per tool name in the agent's tools
// field of result that is neither a known tool of the harness nor a valid MCP name, in written
// order. It returns nil when the descriptor does not ask for unknown names to be reported.
func unknownHarnessToolGaps(desc *domain.HarnessDescriptor, agentKey string, result domain.ToolResult) []domain.Gap {
	if !desc.Tools.ReportUnknownNames {
		return nil
	}
	var written []string
	for _, f := range result.Fields {
		if f.Key == desc.Frontmatter.ToolsKey {
			written = append(written, fieldToolNames(f.Value)...)
		}
	}
	var gaps []domain.Gap
	for _, name := range descriptor.UnknownToolNames(desc, written) {
		gaps = append(gaps, domain.Gap{
			Kind:    domain.GapUnknownHarnessTool,
			Subject: name,
			Owner:   agentKey,
			Detail: fmt.Sprintf("tool %q is neither a known %s tool nor an MCP name in the form %s or %s; %s ignores it",
				name, desc.DisplayName, desc.Tools.MCP.ServerForm, desc.Tools.MCP.ToolForm, desc.DisplayName),
		})
	}
	return gaps
}

// fieldToolNames lists the tool names a tools-field value carries.
func fieldToolNames(v domain.FieldValue) []string {
	switch v.Kind {
	case domain.KindList:
		names := make([]string, 0, len(v.Items))
		for _, it := range v.Items {
			names = append(names, it.Scalar)
		}
		return names
	case domain.KindScalar:
		var names []string
		for _, n := range strings.Split(v.Scalar, ",") {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
		return names
	case domain.KindMapping:
		names := make([]string, 0, len(v.Pairs))
		for _, p := range v.Pairs {
			names = append(names, p.Key)
		}
		return names
	}
	return nil
}
