package domain

import "strings"

// RuleEffect is the effect of one rules-shape permission rule. Wire value verbatim.
type RuleEffect string

const (
	EffectAllow RuleEffect = "allow"
	EffectAsk   RuleEffect = "ask"
	EffectDeny  RuleEffect = "deny"
)

// WholeResource is the resource of every rule Deploy generates.
const WholeResource = "*"

// PermissionRule is one entry of a rules-shape tools value.
type PermissionRule struct {
	Action   string
	Resource string
	Effect   RuleEffect
}

// RuleFrame holds a rules-shape descriptor's fixed rules.
type RuleFrame struct {
	Leading  []PermissionRule // emitted first, before every allow rule
	Trailing []PermissionRule // emitted last
}

// ActionOf returns the action of the universe tool named name (its Action, else its Name).
// A name that is not a universe tool is returned unchanged.
func (s ToolSpec) ActionOf(name string) string {
	for _, t := range s.Universe {
		if t.Name == name {
			if t.Action != "" {
				return t.Action
			}
			return t.Name
		}
	}
	return name
}

// IsFixedRuleAction reports whether action is the action of any rule in s.Rules.Leading
// or s.Rules.Trailing.
func (s ToolSpec) IsFixedRuleAction(action string) bool {
	for _, r := range s.Rules.Leading {
		if r.Action == action {
			return true
		}
	}
	for _, r := range s.Rules.Trailing {
		if r.Action == action {
			return true
		}
	}
	return false
}

// PermissionRulesValue renders rules as a frontmatter value: a block list of
// action/resource/effect mappings in input order. A scalar starting with "*" is double quoted,
// which YAML requires; every other scalar is plain.
func PermissionRulesValue(rules []PermissionRule) FieldValue {
	items := make([]FieldValue, len(rules))
	for i, r := range rules {
		items[i] = MappingValue([]FieldPair{
			{Key: "action", Value: ruleScalar(r.Action)},
			{Key: "resource", Value: ruleScalar(r.Resource)},
			{Key: "effect", Value: ScalarValue(string(r.Effect), QuotePlain)},
		})
	}
	return ListValue(items, ListBlock)
}

func ruleScalar(text string) FieldValue {
	if strings.HasPrefix(text, "*") {
		return ScalarValue(text, QuoteDouble)
	}
	return ScalarValue(text, QuotePlain)
}

// ParsePermissionRules reads a rule list value. Lenient, never fails: a value that is not a list
// gives nil, and an item that is not a mapping with a non-empty action and resource and an effect
// of allow, ask or deny is skipped. Extra keys and any key order are accepted.
func ParsePermissionRules(v FieldValue) []PermissionRule {
	if v.Kind != KindList {
		return nil
	}
	var rules []PermissionRule
	for _, item := range v.Items {
		if r, ok := parsePermissionRule(item); ok {
			rules = append(rules, r)
		}
	}
	return rules
}

func parsePermissionRule(item FieldValue) (PermissionRule, bool) {
	if item.Kind != KindMapping {
		return PermissionRule{}, false
	}
	var r PermissionRule
	for _, p := range item.Pairs {
		if p.Value.Kind != KindScalar {
			continue
		}
		switch p.Key {
		case "action":
			r.Action = p.Value.Scalar
		case "resource":
			r.Resource = p.Value.Scalar
		case "effect":
			r.Effect = RuleEffect(p.Value.Scalar)
		}
	}
	switch r.Effect {
	case EffectAllow, EffectAsk, EffectDeny:
	default:
		return PermissionRule{}, false
	}
	return r, r.Action != "" && r.Resource != ""
}
