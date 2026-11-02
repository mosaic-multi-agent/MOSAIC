package descriptor

import (
	"strings"

	"mosaic-deploy/internal/domain"
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

// ReverseCustomTool reports whether harnessName is the descriptor's custom tool template
// applied to a server name, and returns that server name. ok is false when the
// descriptor declares no CustomToolTemplate, when the template does not contain exactly
// one "%s", or when harnessName does not start with the text before "%s" and end with
// the text after it with a non-empty server in between.
func ReverseCustomTool(d *domain.HarnessDescriptor, harnessName string) (server string, ok bool) {
	tmpl := d.Tools.CustomToolTemplate
	if strings.Count(tmpl, "%s") != 1 {
		return "", false
	}
	prefix, suffix, _ := strings.Cut(tmpl, "%s")
	if len(harnessName) <= len(prefix)+len(suffix) {
		return "", false
	}
	if !strings.HasPrefix(harnessName, prefix) || !strings.HasSuffix(harnessName, suffix) {
		return "", false
	}
	return harnessName[len(prefix) : len(harnessName)-len(suffix)], true
}
