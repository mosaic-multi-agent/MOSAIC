package transform_test

// tools_field_rules_test.go specifies transform.MergeToolsField for the rules tool shape: the
// rule list written for a deployed agent follows the catalog for built-in rules, keeps custom
// rules (MCP and other non-universe actions) as deployed, adds custom rules the run produces,
// and always ends with the descriptor's trailing rules.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// rl is one rule as plain strings: action, resource, effect.
type rl struct{ action, resource, effect string }

var (
	rlDenyAll = rl{"*", "*", "deny"}
	rlOutside = rl{"external_directory", "*", "ask"}
)

func rlAllow(action string) rl { return rl{action, "*", "allow"} }

// rulesDescriptor returns a rules-shape descriptor: universe read, write and patch (both
// governed by edit) and shell, the leading deny-all rule and the trailing outside-workspace ask.
func rulesDescriptor() *domain.HarnessDescriptor {
	return &domain.HarnessDescriptor{
		ID: "rules-harness",
		Tools: domain.ToolSpec{
			Shape: domain.ShapeRules,
			Universe: []domain.HarnessTool{
				{Name: "read"},
				{Name: "write", Action: "edit"},
				{Name: "patch", Action: "edit"},
				{Name: "shell"},
			},
			Rules: domain.RuleFrame{
				Leading:  []domain.PermissionRule{{Action: "*", Resource: "*", Effect: domain.EffectDeny}},
				Trailing: []domain.PermissionRule{{Action: "external_directory", Resource: "*", Effect: domain.EffectAsk}},
			},
		},
		Frontmatter: domain.FrontmatterSpec{ToolsKey: "permissions"},
	}
}

// rlValue builds a rule-list value without the domain's own builder.
func rlValue(rules ...rl) domain.FieldValue {
	items := make([]domain.FieldValue, len(rules))
	for i, r := range rules {
		items[i] = domain.MappingValue([]domain.FieldPair{
			{Key: "action", Value: domain.ScalarValue(r.action, domain.QuotePlain)},
			{Key: "resource", Value: domain.ScalarValue(r.resource, domain.QuotePlain)},
			{Key: "effect", Value: domain.ScalarValue(r.effect, domain.QuotePlain)},
		})
	}
	return domain.ListValue(items, domain.ListBlock)
}

// rlRead reads a rule-list value back as plain rules, nil when it is not a list.
func rlRead(v domain.FieldValue) []rl {
	if v.Kind != domain.KindList {
		return nil
	}
	var out []rl
	for _, it := range v.Items {
		if it.Kind != domain.KindMapping {
			continue
		}
		var r rl
		for _, p := range it.Pairs {
			switch p.Key {
			case "action":
				r.action = p.Value.Scalar
			case "resource":
				r.resource = p.Value.Scalar
			case "effect":
				r.effect = p.Value.Scalar
			}
		}
		out = append(out, r)
	}
	return out
}

func assertRules(t *testing.T, label string, got []rl, want []rl) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestMergeToolsField_Rules_NotDeployedReturnsGeneratedUnchanged(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, domain.FieldValue{}, false)

	if !reflect.DeepEqual(got.Value, generated) {
		t.Errorf("Value = %+v, want the generated value unchanged", got.Value)
	}
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, nil)
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_Rules_BuiltInRulesFollowTheCatalogAndCustomRulesAreKept(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlAllow("edit"), rlOutside)
	deployed := rlValue(rlDenyAll, rlAllow("shell"), rlAllow("hitl_*"), rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("edit"), rlAllow("hitl_*"), rlOutside})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"read", "edit"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"shell"})
}

func TestMergeToolsField_Rules_EqualCatalogAndDeployedReportNoChange(t *testing.T) {
	rules := rlValue(rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), rules, rules, true)

	assertRules(t, "Value", rlRead(got.Value), []rl{rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), rlOutside})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, nil)
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_Rules_GeneratedCustomRuleComesBeforeDeployedCustomRules(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlAllow("new_*"), rlOutside)
	deployed := rlValue(rlDenyAll, rlAllow("read"), rlAllow("old_*"), rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("new_*"), rlAllow("old_*"), rlOutside})
}

func TestMergeToolsField_Rules_DeployedRuleOfTheSameActionWinsOverAGeneratedCustomRule(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("hitl_*"), rlOutside)
	deployed := rlValue(rlDenyAll, rl{"hitl_*", "*", "deny"}, rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value), []rl{rlDenyAll, rl{"hitl_*", "*", "deny"}, rlOutside})
}

func TestMergeToolsField_Rules_DeployedCustomRulesKeepTheirOrderAndExactForm(t *testing.T) {
	generated := rlValue(rlDenyAll, rlOutside)
	deployed := rlValue(
		rlDenyAll,
		rl{"zeta_*", "*", "allow"},
		rl{"alpha_tool", "src/*", "ask"},
		rlOutside,
	)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value),
		[]rl{rlDenyAll, rl{"zeta_*", "*", "allow"}, rl{"alpha_tool", "src/*", "ask"}, rlOutside})
}

func TestMergeToolsField_Rules_DeployedBuiltInRulesAreNotCarried(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlOutside)
	deployed := rlValue(rlDenyAll, rlAllow("read"), rl{"read", "docs/*", "deny"}, rlAllow("edit"), rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value), []rl{rlDenyAll, rlAllow("read"), rlOutside})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, []string{"edit"})
}

func TestMergeToolsField_Rules_TrailingRuleIsGeneratedNotDeployedAndWrittenOnce(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlOutside)
	deployed := rlValue(rlDenyAll, rl{"external_directory", "*", "allow"}, rlAllow("hitl_*"))

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value),
		[]rl{rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), rlOutside})
}

func TestMergeToolsField_Rules_LastWholeToolRuleDecidesWhetherABuiltInIsGranted(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("edit"), rlOutside)
	deployed := rlValue(rlDenyAll, rlAllow("edit"), rl{"edit", "*", "deny"}, rlOutside)

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"edit"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_Rules_DeployedValueThatIsNotARuleListCountsAsNoRules(t *testing.T) {
	generated := rlValue(rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), rlOutside)
	deployed := domain.MappingValue([]domain.FieldPair{{Key: "read", Value: domain.ScalarValue("allow", domain.QuotePlain)}})

	got := transform.MergeToolsField(rulesDescriptor(), generated, deployed, true)

	assertRules(t, "Value", rlRead(got.Value), []rl{rlDenyAll, rlAllow("read"), rlAllow("hitl_*"), rlOutside})
	assertNames(t, "AddedBuiltIn", got.AddedBuiltIn, []string{"read"})
	assertNames(t, "RemovedBuiltIn", got.RemovedBuiltIn, nil)
}

func TestMergeToolsField_Rules_ResultIsARuleListNotAPermissionMapping(t *testing.T) {
	got := transform.MergeToolsField(rulesDescriptor(),
		rlValue(rlDenyAll, rlAllow("read"), rlOutside), rlValue(rlDenyAll, rlOutside), true)

	if got.Value.Kind != domain.KindList {
		t.Fatalf("Value kind = %q, want a list of rules", got.Value.Kind)
	}
	for i, it := range got.Value.Items {
		if it.Kind != domain.KindMapping {
			t.Errorf("item %d kind = %q, want a mapping", i, it.Kind)
		}
	}
}
