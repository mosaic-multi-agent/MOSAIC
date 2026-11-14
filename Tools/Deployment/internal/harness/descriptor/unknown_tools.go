package descriptor

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// UnknownToolNames returns the names, in input order and each once, that are neither a
// d.Tools.Universe name, nor a name listed in any d.Tools.Mappings destination, nor parseable as
// an MCP name in d.Tools.MCP. Returns nil when !d.Tools.ReportUnknownNames.
//
// Hierarchical harness tool names such as "edit/editFiles" count as known when listed in the
// universe or a mapping; any other "<server>/<tool>" name is read as an MCP name and is not
// reported, because GHCP does not let Deploy tell the two apart.
func UnknownToolNames(d *domain.HarnessDescriptor, names []string) []string {
	if !d.Tools.ReportUnknownNames {
		return nil
	}
	known := make(map[string]bool, len(d.Tools.Universe))
	for _, u := range d.Tools.Universe {
		known[u.Name] = true
	}
	for _, m := range d.Tools.Mappings {
		for _, dest := range m.Destinations {
			for _, n := range dest.Names {
				known[n] = true
			}
		}
	}
	var unknown []string
	for _, n := range names {
		if known[n] {
			continue
		}
		known[n] = true
		if len(mcpname.Parse(d.Tools.MCP, n)) == 0 {
			unknown = append(unknown, n)
		}
	}
	return unknown
}
