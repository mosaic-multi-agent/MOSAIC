package descriptor

import (
	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// customEntryNames writes each custom tool entry in the harness's MCP form, in entry order,
// deduplicated first-seen. An entry whose form the descriptor does not declare keeps its
// neutral "server" / "server/tool" text.
func customEntryNames(d *domain.HarnessDescriptor, entries []domain.MCPEntry) []string {
	names := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		name, ok := mcpname.Format(d.Tools.MCP, mcpname.Name{Server: e.Server, Tool: e.Tool})
		if !ok {
			name = customtool.EntryText(e)
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// customDestinations places the written custom tool names. When the descriptor declares
// custom-tool destinations and no explicit mapping entry exists for the generic, each declared
// destination receives all names; otherwise the names go to one main destination.
func customDestinations(d *domain.HarnessDescriptor, hasMappingEntry bool, names []string) []domain.ToolDestination {
	if len(d.Tools.CustomToolDestinations) > 0 && !hasMappingEntry {
		dests := make([]domain.ToolDestination, len(d.Tools.CustomToolDestinations))
		for i, decl := range d.Tools.CustomToolDestinations {
			dests[i] = domain.ToolDestination{
				Kind:      decl.Kind,
				Field:     decl.Field,
				Format:    decl.Format,
				Separator: decl.Separator,
				Names:     append([]string(nil), names...),
			}
		}
		return dests
	}
	return []domain.ToolDestination{{Kind: domain.DestMain, Names: names}}
}
