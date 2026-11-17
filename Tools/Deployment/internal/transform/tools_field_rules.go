package transform

import "mosaic-deploy/internal/domain"

// This file handles the rules tool shape (an ordered list of action/resource/effect rules) for
// the tools-field merge and the appended tools. A rule belongs to one of three groups: fixed
// (the descriptor's leading and trailing rules, always generated, never carried), built-in (the
// action of a universe tool, follows the catalog) and custom (any other action, for example an
// MCP server, kept as deployed).

// ruleKind classifies the action of a rule for the merge.
type ruleKind int

const (
	ruleCustom ruleKind = iota
	ruleBuiltIn
	ruleFixed
)

func classifyRule(d *domain.HarnessDescriptor, builtInActions map[string]bool, r domain.PermissionRule) ruleKind {
	switch {
	case d.Tools.IsFixedRuleAction(r.Action):
		return ruleFixed
	case builtInActions[r.Action]:
		return ruleBuiltIn
	default:
		return ruleCustom
	}
}

// builtInActionSet returns the permission actions of the universe tools.
func builtInActionSet(d *domain.HarnessDescriptor) map[string]bool {
	set := make(map[string]bool, len(d.Tools.Universe))
	for _, t := range d.Tools.Universe {
		set[d.Tools.ActionOf(t.Name)] = true
	}
	return set
}

// grantedActions returns the built-in actions whose last whole-tool rule allows, in order of
// first appearance.
func grantedActions(d *domain.HarnessDescriptor, builtIn map[string]bool, rules []domain.PermissionRule) []string {
	var order []string
	effect := map[string]domain.RuleEffect{}
	for _, r := range rules {
		if r.Resource != domain.WholeResource || classifyRule(d, builtIn, r) != ruleBuiltIn {
			continue
		}
		if _, seen := effect[r.Action]; !seen {
			order = append(order, r.Action)
		}
		effect[r.Action] = r.Effect
	}
	var granted []string
	for _, action := range order {
		if effect[action] == domain.EffectAllow {
			granted = append(granted, action)
		}
	}
	return granted
}

// mergeRulesToolsField is MergeToolsField for the rules shape. The result is the leading rules,
// every built-in rule of generated, every custom rule of generated whose action has no rule in
// deployed, every custom rule of deployed exactly as deployed, then the trailing rules.
// Deployed built-in and fixed rules are not carried. A deployed value that is not a rule list
// counts as no rules. AddedBuiltIn and RemovedBuiltIn name built-in actions.
func mergeRulesToolsField(d *domain.HarnessDescriptor, generated, deployed domain.FieldValue) ToolsFieldMerge {
	builtIn := builtInActionSet(d)
	genRules := domain.ParsePermissionRules(generated)
	depRules := domain.ParsePermissionRules(deployed)

	depCustom := map[string]bool{}
	for _, r := range depRules {
		if classifyRule(d, builtIn, r) == ruleCustom {
			depCustom[r.Action] = true
		}
	}

	result := append([]domain.PermissionRule{}, d.Tools.Rules.Leading...)
	for _, r := range genRules {
		if classifyRule(d, builtIn, r) == ruleBuiltIn {
			result = append(result, r)
		}
	}
	for _, r := range genRules {
		if classifyRule(d, builtIn, r) == ruleCustom && !depCustom[r.Action] {
			result = append(result, r)
		}
	}
	for _, r := range depRules {
		if classifyRule(d, builtIn, r) == ruleCustom {
			result = append(result, r)
		}
	}
	result = append(result, d.Tools.Rules.Trailing...)

	resGranted := grantedActions(d, builtIn, result)
	depGranted := grantedActions(d, builtIn, depRules)
	var out ToolsFieldMerge
	out.Value = domain.PermissionRulesValue(result)
	out.AddedBuiltIn = missingFrom(resGranted, depGranted)
	out.RemovedBuiltIn = missingFrom(depGranted, resGranted)
	return out
}

// missingFrom returns the names of from that are not in other, in from's order.
func missingFrom(from, other []string) []string {
	have := make(map[string]bool, len(other))
	for _, n := range other {
		have[n] = true
	}
	var out []string
	for _, n := range from {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

// appendRulesTools adds one whole-tool allow rule per permission action of names (a universe
// tool name maps to its action, any other name stands for itself) just before the trailing
// rules. An action whose last whole-tool rule already allows adds nothing. It returns the new
// value and whether it changed.
func appendRulesTools(current domain.FieldValue, names []string, d *domain.HarnessDescriptor) (domain.FieldValue, bool) {
	rules := domain.ParsePermissionRules(current)
	var added []domain.PermissionRule
	seen := map[string]bool{}
	for _, name := range names {
		action := d.Tools.ActionOf(name)
		if name == "" || seen[action] {
			continue
		}
		seen[action] = true
		if !lastWholeToolRuleAllows(rules, action) {
			added = append(added, domain.PermissionRule{Action: action, Resource: domain.WholeResource, Effect: domain.EffectAllow})
		}
	}
	if len(added) == 0 {
		return current, false
	}
	at := len(rules)
	for at > 0 && isTrailingRule(d, rules[at-1]) {
		at--
	}
	merged := append(append(append([]domain.PermissionRule{}, rules[:at]...), added...), rules[at:]...)
	return domain.PermissionRulesValue(merged), true
}

func lastWholeToolRuleAllows(rules []domain.PermissionRule, action string) bool {
	allowed := false
	for _, r := range rules {
		if r.Action == action && r.Resource == domain.WholeResource {
			allowed = r.Effect == domain.EffectAllow
		}
	}
	return allowed
}

func isTrailingRule(d *domain.HarnessDescriptor, r domain.PermissionRule) bool {
	for _, t := range d.Tools.Rules.Trailing {
		if t.Action == r.Action {
			return true
		}
	}
	return false
}
