package app

// deployed_tools_mcp.go converts the MCP tool names of a deployed source agent: a name is read
// with the source harness's forms and written with the target harness's forms, keeping allow
// and deny. An ambiguous single-tool name is resolved by declared servers, then by an answered
// split; one neither decides is reported and written nowhere.

import (
	"slices"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// convertMCP converts the grant when it is an MCP name in the source's forms and reports whether
// it claimed the name.
func (c *toolConverter) convertMCP(grant domain.ToolGrant) bool {
	readings := mcpname.ParseAll(c.in.Source.Tools.MCP, grant.Name)
	switch len(readings) {
	case 0:
		return false
	case 1:
		c.writeMCP(grant, readings[0])
		return true
	}
	if n, ok := c.resolveSplit(grant.Name, readings); ok {
		c.writeMCP(grant, n)
		return true
	}
	c.out.Ambiguous = append(c.out.Ambiguous, AmbiguousName{
		Source: grant.Name, Disposition: grant.Disposition, Splits: readings,
	})
	c.record(ToolConversion{Source: grant.Name, Rule: ToolRuleAmbiguous})
	return true
}

// resolveSplit decides an ambiguous name: the longest declared server whose form leaves a tool
// decides first, otherwise the answered server when it names one of the readings.
func (c *toolConverter) resolveSplit(name string, readings []mcpname.Name) (mcpname.Name, bool) {
	servers := slices.Clone(c.in.DeclaredServers)
	slices.SortStableFunc(servers, func(a, b string) int { return len(b) - len(a) })
	for _, server := range servers {
		if n, ok := mcpname.SplitAt(c.in.Source.Tools.MCP, name, server); ok {
			return n, true
		}
	}
	answer, answered := c.in.SplitAnswers[name]
	if !answered {
		return mcpname.Name{}, false
	}
	for _, n := range readings {
		if n.Server == answer {
			return n, true
		}
	}
	return mcpname.Name{}, false
}

// writeMCP writes the name in the target's forms, or drops it when the target has no form for it.
func (c *toolConverter) writeMCP(grant domain.ToolGrant, n mcpname.Name) {
	cv := ToolConversion{Source: grant.Name, Rule: ToolRuleMCP, Server: n.Server, Tool: n.Tool}
	text, ok := mcpname.Format(c.in.Target.Tools.MCP, n)
	if !ok {
		if grant.Disposition == domain.Allow {
			c.out.Dropped = append(c.out.Dropped, DroppedTool{
				Name:   grant.Name,
				Reason: c.in.Target.ID + " has no MCP name form for this name",
			})
		}
		c.record(cv)
		return
	}
	c.emitTarget(grant.Name, text, grant.Disposition == domain.Deny)
	c.record(cv)
}

// emitTarget writes a target name as an allow, or as a deny where the target has permission
// shape; a deny the target cannot express is reported dropped so it is never granted.
func (c *toolConverter) emitTarget(source, target string, deny bool) {
	switch {
	case !deny:
		if !c.inTarget[target] {
			c.inTarget[target] = true
			c.out.TargetTools = append(c.out.TargetTools, target)
		}
	case c.in.Target.Tools.Shape == domain.ShapePermission:
		c.inDenied[target] = true
		if !slices.Contains(c.out.TargetDenied, target) {
			c.out.TargetDenied = append(c.out.TargetDenied, target)
		}
	default:
		c.inDenied[target] = true
		c.out.Dropped = append(c.out.Dropped, DroppedTool{
			Name:   source,
			Reason: "denied in " + c.in.Source.ID + "; " + c.in.Target.ID + " cannot express deny",
		})
	}
}

// withholdDeniedGrants removes from the allowed target names every name the source also denies
// (through another spelling), so a deny is never turned into a grant.
func (c *toolConverter) withholdDeniedGrants() {
	if len(c.inDenied) == 0 {
		return
	}
	c.out.TargetTools = slices.DeleteFunc(c.out.TargetTools, func(n string) bool { return c.inDenied[n] })
}
