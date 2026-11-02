package app

import "mosaic-deploy/internal/domain"

const genericSkill = "skill"

// addSkillAccess gives a non-orchestrator agent explicit skill access when the source
// grants it without writing a name (declared mapping with no names) and the target writes it.
func (c *toolConverter) addSkillAccess() {
	if c.in.Role == domain.RoleOrchestrator || c.inGeneric[genericSkill] {
		return
	}
	// A catalog entry that lists its tools but not skill decides: a fresh deploy gives no access.
	if c.in.CatalogTools != nil && !toolNameSet(c.in.CatalogTools)[genericSkill] {
		return
	}
	if !skillNamesEmpty(c.in.Source) || !skillWritesNames(c.in.Target) {
		return
	}
	c.addGeneric(genericSkill)
}

// skillNamesEmpty reports whether the descriptor declares a skill mapping that writes no name.
func skillNamesEmpty(d *domain.HarnessDescriptor) bool {
	for i := range d.Tools.Mappings {
		m := &d.Tools.Mappings[i]
		if m.Generic == genericSkill {
			return !mappingHasNames(m)
		}
	}
	return false
}

// skillWritesNames reports whether the descriptor maps skill to at least one name.
func skillWritesNames(d *domain.HarnessDescriptor) bool {
	for i := range d.Tools.Mappings {
		m := &d.Tools.Mappings[i]
		if m.Generic == genericSkill {
			return mappingHasNames(m)
		}
	}
	return false
}

func mappingHasNames(m *domain.ToolMapping) bool {
	for _, dest := range m.Destinations {
		if len(dest.Names) > 0 {
			return true
		}
	}
	return false
}

// hasMappingEntry reports whether the descriptor declares a mapping for the generic tool,
// even one with no destinations.
func hasMappingEntry(d *domain.HarnessDescriptor, generic string) bool {
	for i := range d.Tools.Mappings {
		if d.Tools.Mappings[i].Generic == generic {
			return true
		}
	}
	return false
}

// targetValidNames collects the universe names and mapping destination names of a harness.
func targetValidNames(d *domain.HarnessDescriptor) map[string]bool {
	set := toolNameSet(universeNames(d, true))
	for i := range d.Tools.Mappings {
		for _, dest := range d.Tools.Mappings[i].Destinations {
			for _, n := range dest.Names {
				set[n] = true
			}
		}
	}
	return set
}

// universeNames lists the universe names of a harness, optionally including by-convention ones.
func universeNames(d *domain.HarnessDescriptor, withByConvention bool) []string {
	var names []string
	for _, tool := range d.Tools.Universe {
		if withByConvention || !tool.ByConvention {
			names = append(names, tool.Name)
		}
	}
	return names
}

func toolNameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// completeExpansion returns the source's placeholder expansion (the universe without
// by-convention names when none is declared) when every name of it is among tools.
func completeExpansion(src *domain.HarnessDescriptor, tools []string) ([]string, bool) {
	expansion := src.Tools.PlaceholderExpansion
	if len(expansion) == 0 {
		expansion = universeNames(src, false)
	}
	if len(expansion) == 0 {
		return nil, false
	}
	have := toolNameSet(tools)
	// By-convention names are never read back from a deployed file, so their absence says
	// nothing about the expansion.
	byConvention := map[string]bool{}
	for _, tool := range src.Tools.Universe {
		if tool.ByConvention {
			byConvention[tool.Name] = true
		}
	}
	for _, n := range expansion {
		if !have[n] && !byConvention[n] {
			return nil, false
		}
	}
	return expansion, true
}
