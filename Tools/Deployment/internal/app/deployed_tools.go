package app

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
	"mosaic-deploy/internal/harness/mcpname"
)

// ConvertDeployedTools converts the tool names of one deployed source-harness agent into
// the inputs deploy's forward mapping needs for the target harness.
//
// Each distinct source name is converted by the first rule that claims it: mapped through
// its generic tool, kept when valid in the target, dropped when built into the source with no
// target equivalent, MCP name read with the source's forms and written with the target's, and
// otherwise unknown and dropped. A name denied in the source is never granted in the target.
func ConvertDeployedTools(in DeployedToolsInput) ConvertedTools {
	conv := &toolConverter{
		in:        in,
		out:       ConvertedTools{},
		valid:     targetValidNames(in.Target),
		universe:  toolNameSet(universeNames(in.Source, true)),
		seen:      map[string]bool{},
		inGeneric: map[string]bool{},
		inTarget:  map[string]bool{},
		inDenied:  map[string]bool{},
		inUnmap:   map[string]bool{},
	}

	covered := map[string]bool{}
	if in.PlaceholderAgent && in.Role == domain.RoleOrchestrator {
		if names, ok := completeExpansion(in.Source, allowedNames(in.Tools)); ok {
			conv.out.UsePlaceholder = true
			covered = toolNameSet(names)
		}
	}

	for _, grant := range in.Tools {
		if conv.seen[grant.Name] {
			continue
		}
		conv.seen[grant.Name] = true
		if grant.Disposition == domain.Allow && covered[grant.Name] {
			conv.out.Conversions = append(conv.out.Conversions, ToolConversion{Source: grant.Name, Rule: ToolRulePlaceholder})
			continue
		}
		conv.convert(grant)
	}

	conv.withholdDeniedGrants()
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
	inDenied  map[string]bool // target names denied in the source, whether or not the target writes the deny
	inUnmap   map[string]bool
}

// convert applies the ordered rules to one source tool grant.
func (c *toolConverter) convert(grant domain.ToolGrant) {
	name := grant.Name
	deny := grant.Disposition == domain.Deny
	if generics := c.resolveGenerics(name); len(generics) > 0 {
		if !deny {
			c.mapGenerics(generics)
		}
		c.record(ToolConversion{Source: name, Rule: ToolRuleMapped, Generic: generics})
		return
	}
	if c.validInTarget(name) {
		c.emitTarget(name, name, deny)
		c.record(ToolConversion{Source: name, Rule: ToolRuleKept})
		return
	}
	if c.universe[name] {
		// A denied built-in tool is listing, not intent: nothing is reported for it.
		if !deny {
			c.out.Dropped = append(c.out.Dropped, DroppedTool{
				Name:   name,
				Reason: "built into " + c.in.Source.ID + " with no " + c.in.Target.ID + " equivalent",
			})
		}
		c.record(ToolConversion{Source: name, Rule: ToolRuleDropped})
		return
	}
	if c.convertMCP(grant) {
		return
	}
	if !deny {
		c.out.Dropped = append(c.out.Dropped, DroppedTool{
			Name:   name,
			Reason: "not a known " + c.in.Source.ID + " tool or MCP name",
		})
	}
	c.record(ToolConversion{Source: name, Rule: ToolRuleUnknown})
}

// mapGenerics adds the generic tools of an allowed mapped name and lists those the target has
// no mapping for.
func (c *toolConverter) mapGenerics(generics []string) {
	for _, g := range generics {
		c.addGeneric(g)
		if !hasMappingEntry(c.in.Target, g) && !c.inUnmap[g] {
			c.inUnmap[g] = true
			c.out.Unmapped = append(c.out.Unmapped, g)
		}
	}
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

// startsWithCustomPrefix reports whether the name already is an MCP name in the harness's own
// syntax. Only harnesses whose MCP names carry a distinguishing literal prefix (Claude Code's
// "mcp__") are recognised this way: a bare "<server>_<tool>" or "<server>/<tool>" shape also
// fits ordinary names, so for those the name is not taken as already in the target's form.
func startsWithCustomPrefix(d *domain.HarnessDescriptor, name string) bool {
	prefix, _, _ := strings.Cut(d.Tools.MCP.ToolForm, "{server}")
	return prefix != "" && len(mcpname.Parse(d.Tools.MCP, name)) > 0
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
