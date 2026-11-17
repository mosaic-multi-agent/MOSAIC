package descriptor

import "mosaic-deploy/internal/domain"

// This file renders and reads the rules tool shape: an ordered list of
// {action, resource, effect} permission rules in which the last matching rule wins. A tool
// is governed by its permission action (domain.ToolSpec.ActionOf), so several universe tools
// can share one rule.

// expandRulesPlaceholder expands the {tool-permissions} placeholder of a rules-shape descriptor
// into a complete rule set: the leading rules, one allow rule per distinct action of the
// placeholder expansion (the whole universe when none is declared), then the trailing rules.
func expandRulesPlaceholder(d *domain.HarnessDescriptor) domain.ToolResult {
	if d.Frontmatter.ToolsKey == "" {
		return domain.ToolResult{Resolutions: make([]domain.ToolResolution, 0)}
	}
	expansion := d.Tools.PlaceholderExpansion
	if len(expansion) == 0 {
		expansion = make([]string, len(d.Tools.Universe))
		for i, t := range d.Tools.Universe {
			expansion[i] = t.Name
		}
	}
	return domain.ToolResult{
		Fields: []domain.FrontmatterField{{
			Key:   d.Frontmatter.ToolsKey,
			Value: domain.PermissionRulesValue(frameRules(d, actionsOfNames(d, expansion))),
		}},
		Resolutions: make([]domain.ToolResolution, 0),
	}
}

// actionsOfNames maps tool names to their permission actions, first-seen order, without
// duplicates.
func actionsOfNames(d *domain.HarnessDescriptor, names []string) []string {
	var actions []string
	seen := map[string]bool{}
	for _, name := range names {
		action := d.Tools.ActionOf(name)
		if !seen[action] {
			seen[action] = true
			actions = append(actions, action)
		}
	}
	return actions
}

// frameRules builds the leading rules, one whole-tool allow rule per action, then the trailing
// rules.
func frameRules(d *domain.HarnessDescriptor, allowedActions []string) []domain.PermissionRule {
	rules := make([]domain.PermissionRule, 0, len(d.Tools.Rules.Leading)+len(allowedActions)+len(d.Tools.Rules.Trailing))
	rules = append(rules, d.Tools.Rules.Leading...)
	for _, action := range allowedActions {
		rules = append(rules, domain.PermissionRule{Action: action, Resource: domain.WholeResource, Effect: domain.EffectAllow})
	}
	return append(rules, d.Tools.Rules.Trailing...)
}

// buildRuleToolFields assembles the rule list field for the rules shape. The allowed actions
// are those of the universe tools the resolutions (and by-convention tools) name, in universe
// order, followed by every other main-destination name (custom or mapped outside the universe)
// in resolution order. DestField destinations are emitted as separate fields, as for the other
// shapes.
func buildRuleToolFields(d *domain.HarnessDescriptor, resolutions []domain.ToolResolution) []domain.FrontmatterField {
	resolved := make(map[string]bool, len(d.Tools.Universe))
	for _, t := range d.Tools.Universe {
		if t.ByConvention {
			resolved[t.Name] = true
		}
	}
	inUniverse := make(map[string]bool, len(d.Tools.Universe))
	for _, t := range d.Tools.Universe {
		inUniverse[t.Name] = true
	}

	fieldBuckets := make(map[string]*destFieldBucket)
	var fieldOrder []string
	var outsideUniverse []string // main-destination names the universe does not know, resolution order

	for _, res := range resolutions {
		if res.Outcome != domain.ToolMapped && res.Outcome != domain.ToolCustom {
			continue
		}
		for _, dest := range res.Destinations {
			switch dest.Kind {
			case domain.DestMain:
				for _, name := range dest.Names {
					resolved[name] = true
					if !inUniverse[name] {
						outsideUniverse = append(outsideUniverse, name)
					}
				}
			case domain.DestField:
				for _, name := range dest.Names {
					if _, exists := fieldBuckets[dest.Field]; !exists {
						fieldBuckets[dest.Field] = &destFieldBucket{
							format: dest.Format, separator: dest.Separator, names: make(map[string]bool),
						}
						fieldOrder = append(fieldOrder, dest.Field)
					}
					fieldBuckets[dest.Field].names[name] = true
				}
			}
		}
	}

	var names []string
	for _, t := range d.Tools.Universe {
		if resolved[t.Name] {
			names = append(names, t.Name)
		}
	}
	names = append(names, outsideUniverse...)

	fields := []domain.FrontmatterField{{
		Key:   d.Frontmatter.ToolsKey,
		Value: domain.PermissionRulesValue(frameRules(d, actionsOfNames(d, names))),
	}}
	return append(fields, emitFieldBuckets(fieldBuckets, fieldOrder)...)
}

// extractRuleGrants reads a deployed rule list back as tool grants. Only whole-tool rules
// (resource "*") count, and the last such rule of an action decides: allow grants, deny denies,
// ask is neither. Actions of the descriptor's fixed rules are never tools. An action is
// expanded to every universe tool it governs, in universe order (an action no universe tool
// carries stands for itself, as a custom MCP name). Grants follow the order in which the
// actions first appear in the list; by-convention universe tools are excluded; the first
// grant of a name wins.
func extractRuleGrants(d *domain.HarnessDescriptor, v domain.FieldValue) []domain.ToolGrant {
	if d.Frontmatter.ToolsKey == "" {
		return nil
	}
	var order []string
	effect := map[string]domain.RuleEffect{}
	for _, r := range domain.ParsePermissionRules(v) {
		if r.Resource != domain.WholeResource || d.Tools.IsFixedRuleAction(r.Action) {
			continue
		}
		if _, seen := effect[r.Action]; !seen {
			order = append(order, r.Action)
		}
		effect[r.Action] = r.Effect
	}

	byConvention := map[string]bool{}
	for _, t := range d.Tools.Universe {
		if t.ByConvention {
			byConvention[t.Name] = true
		}
	}
	var grants []domain.ToolGrant
	seen := map[string]bool{}
	for _, action := range order {
		var disposition domain.Disposition
		switch effect[action] {
		case domain.EffectAllow:
			disposition = domain.Allow
		case domain.EffectDeny:
			disposition = domain.Deny
		default:
			continue
		}
		for _, name := range namesOfAction(d, action) {
			if byConvention[name] || seen[name] {
				continue
			}
			seen[name] = true
			grants = append(grants, domain.ToolGrant{Name: name, Disposition: disposition})
		}
	}
	return grants
}

// extractRuleEntries returns the names a deployed rule list grants: the allowed grants of
// extractRuleGrants.
func extractRuleEntries(d *domain.HarnessDescriptor, v domain.FieldValue) []string {
	var names []string
	for _, g := range extractRuleGrants(d, v) {
		if g.Disposition == domain.Allow {
			names = append(names, g.Name)
		}
	}
	return names
}

// namesOfAction lists the universe tools governed by action in universe order, or the action
// itself when no universe tool carries it.
func namesOfAction(d *domain.HarnessDescriptor, action string) []string {
	var names []string
	for _, t := range d.Tools.Universe {
		if d.Tools.ActionOf(t.Name) == action {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return []string{action}
	}
	return names
}
