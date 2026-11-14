package descriptor

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// ReverseMapToolCandidates returns every generic tool whose mapping lists harnessName in
// any destination's Names, in d.Tools.Mappings declaration order, without duplicates.
// Empty (non-nil) when no mapping names the tool. Its first element always equals
// ReverseMapTool(d, harnessName).
func ReverseMapToolCandidates(d *domain.HarnessDescriptor, harnessName string) []string {
	out := []string{}
	for i := range d.Tools.Mappings {
		mapping := &d.Tools.Mappings[i]
		if mappingNamesTool(mapping, harnessName) {
			out = append(out, mapping.Generic)
		}
	}
	return out
}

// mappingNamesTool reports whether any destination of the mapping lists the name.
func mappingNamesTool(mapping *domain.ToolMapping, name string) bool {
	for _, dest := range mapping.Destinations {
		for _, n := range dest.Names {
			if n == name {
				return true
			}
		}
	}
	return false
}

// ReverseCustomTool reports whether harnessName is a whole-server MCP name in the descriptor's
// own syntax, and returns that server name. ok is false when the descriptor declares no
// whole-server form or harnessName is not a whole-server name in it.
func ReverseCustomTool(d *domain.HarnessDescriptor, harnessName string) (server string, ok bool) {
	names := mcpname.Parse(d.Tools.MCP, harnessName)
	if len(names) != 1 || !names[0].IsWholeServer() {
		return "", false
	}
	return names[0].Server, true
}
