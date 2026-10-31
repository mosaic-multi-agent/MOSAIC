package app

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// ConvertDeployedTools converts the tool names of one deployed source-harness agent into
// the inputs deploy's forward mapping needs for the target harness.
//
// Each distinct source name is converted by the first rule that claims it: mapped through
// its generic tool, custom tool by the source's template, kept when valid in the target,
// dropped when built into the source with no target equivalent, otherwise a custom tool.
func ConvertDeployedTools(in DeployedToolsInput) ConvertedTools {
	conv := &toolConverter{
		in:        in,
		out:       ConvertedTools{},
		valid:     targetValidNames(in.Target),
		universe:  toolNameSet(universeNames(in.Source, true)),
		seen:      map[string]bool{},
		inGeneric: map[string]bool{},
		inTarget:  map[string]bool{},
		inUnmap:   map[string]bool{},
	}

	covered := map[string]bool{}
	if in.PlaceholderAgent && in.Role == domain.RoleOrchestrator {
		if names, ok := completeExpansion(in.Source, in.Tools); ok {
			conv.out.UsePlaceholder = true
			covered = toolNameSet(names)
		}
	}

	for _, name := range in.Tools {
		if conv.seen[name] {
			continue
		}
		conv.seen[name] = true
		if covered[name] {
			conv.out.Conversions = append(conv.out.Conversions, ToolConversion{Source: name, Rule: ToolRulePlaceholder})
			continue
		}
		conv.convert(name)
	}

	conv.addSkillAccess()
	return conv.out
}

// toolConverter accumulates the result of one conversion.
type toolConverter struct {
	in        DeployedToolsInput
	out       ConvertedTools
	valid     map[string]bool // names valid in the target (universe, mapping destinations)
	universe  map[string]bool // source universe names
	seen      map[string]bool
	inGeneric map[string]bool
	inTarget  map[string]bool
	inUnmap   map[string]bool
}

// convert applies the ordered rules to one source tool name.
func (c *toolConverter) convert(name string) {
	if generics := c.resolveGenerics(name); len(generics) > 0 {
		for _, g := range generics {
			c.addGeneric(g)
			if !hasMappingEntry(c.in.Target, g) && !c.inUnmap[g] {
				c.inUnmap[g] = true
				c.out.Unmapped = append(c.out.Unmapped, g)
			}
		}
		c.record(ToolConversion{Source: name, Rule: ToolRuleMapped, Generic: generics})
		return
	}
	if server, ok := descriptor.ReverseCustomTool(c.in.Source, name); ok {
		c.addCustom(server)
		c.record(ToolConversion{Source: name, Rule: ToolRuleCustomTemplate, Server: server})
		return
	}
	if c.validInTarget(name) {
		if !c.inTarget[name] {
			c.inTarget[name] = true
			c.out.TargetTools = append(c.out.TargetTools, name)
		}
		c.record(ToolConversion{Source: name, Rule: ToolRuleKept})
		return
	}
	if c.universe[name] {
		c.out.Dropped = append(c.out.Dropped, DroppedTool{
			Name:   name,
			Reason: "built into " + c.in.Source.ID + " with no " + c.in.Target.ID + " equivalent",
		})
		c.record(ToolConversion{Source: name, Rule: ToolRuleDropped})
		return
	}
	c.addCustom(name)
	c.record(ToolConversion{Source: name, Rule: ToolRuleCustom, Server: name})
}

// resolveGenerics returns the generic tools of a source name: every candidate the catalog
// lists when the name is ambiguous and a catalog entry exists, otherwise the first-declared one.
func (c *toolConverter) resolveGenerics(name string) []string {
	candidates := descriptor.ReverseMapToolCandidates(c.in.Source, name)
	if len(candidates) <= 1 {
		return candidates
	}
	if c.in.CatalogTools == nil {
		return candidates[:1]
	}
	listed := toolNameSet(c.in.CatalogTools)
	var picked []string
	for _, cand := range candidates {
		if listed[cand] {
			picked = append(picked, cand)
		}
	}
	if len(picked) == 0 {
		return candidates[:1]
	}
	return picked
}

// validInTarget reports whether the name is already usable in the target harness.
func (c *toolConverter) validInTarget(name string) bool {
	if c.valid[name] {
		return true
	}
	return startsWithCustomPrefix(c.in.Target, name)
}

// startsWithCustomPrefix reports whether the name already begins with the text of the
// harness's custom tool template before its single %s, i.e. it is in the harness's own custom form.
func startsWithCustomPrefix(d *domain.HarnessDescriptor, name string) bool {
	tmpl := d.Tools.CustomToolTemplate
	if strings.Count(tmpl, "%s") != 1 {
		return false
	}
	prefix, _, _ := strings.Cut(tmpl, "%s")
	return prefix != "" && strings.HasPrefix(name, prefix)
}

func (c *toolConverter) record(cv ToolConversion) {
	c.out.Conversions = append(c.out.Conversions, cv)
}

func (c *toolConverter) addGeneric(g string) {
	if !c.inGeneric[g] {
		c.inGeneric[g] = true
		c.out.Generic = append(c.out.Generic, g)
	}
}

func (c *toolConverter) addCustom(server string) {
	if c.out.CustomTools == nil {
		c.out.CustomTools = map[string]string{}
	}
	c.out.CustomTools[server] = server
	c.addGeneric(server)
}
