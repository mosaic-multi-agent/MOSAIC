package domain_test

// permission_rules_test.go specifies the rules tool shape vocabulary: rendering a rule list as a
// frontmatter value, reading one back leniently, and resolving a universe tool to the permission
// action that governs it.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
)

func rule(action, resource string, effect domain.RuleEffect) domain.PermissionRule {
	return domain.PermissionRule{Action: action, Resource: resource, Effect: effect}
}

// ruleItem builds one mapping item with the given pairs, all plain scalars.
func ruleItem(kv ...string) domain.FieldValue {
	var pairs []domain.FieldPair
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, domain.FieldPair{Key: kv[i], Value: domain.ScalarValue(kv[i+1], domain.QuotePlain)})
	}
	return domain.MappingValue(pairs)
}

func ruleList(items ...domain.FieldValue) domain.FieldValue {
	return domain.ListValue(items, domain.ListBlock)
}

func TestPermissionRulesValue_RendersABlockListOfMappingsInInputOrder(t *testing.T) {
	rules := []domain.PermissionRule{
		rule("*", "*", domain.EffectDeny),
		rule("read", "*", domain.EffectAllow),
		rule("external_directory", "*", domain.EffectAsk),
	}

	got := domain.PermissionRulesValue(rules)

	if got.Kind != domain.KindList || got.List != domain.ListBlock {
		t.Fatalf("want a block list, got kind %v list style %v", got.Kind, got.List)
	}
	if len(got.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(got.Items))
	}
	for i, r := range rules {
		item := got.Items[i]
		if item.Kind != domain.KindMapping || len(item.Pairs) != 3 {
			t.Fatalf("item %d: want a 3-pair mapping, got %+v", i, item)
		}
		wantKeys := []string{"action", "resource", "effect"}
		wantVals := []string{r.Action, r.Resource, string(r.Effect)}
		for j, p := range item.Pairs {
			if p.Key != wantKeys[j] || p.Value.Scalar != wantVals[j] {
				t.Errorf("item %d pair %d: want %s=%q, got %s=%q", i, j, wantKeys[j], wantVals[j], p.Key, p.Value.Scalar)
			}
		}
	}
}

func TestPermissionRulesValue_StarLeadingScalarsAreDoubleQuotedOthersPlain(t *testing.T) {
	got := domain.PermissionRulesValue([]domain.PermissionRule{
		rule("*", "*", domain.EffectDeny),
		rule("hitl_*", "*", domain.EffectAllow),
	})

	if len(got.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(got.Items))
	}
	first, second := got.Items[0], got.Items[1]
	if first.Pairs[0].Value.Quote != domain.QuoteDouble || first.Pairs[1].Value.Quote != domain.QuoteDouble {
		t.Errorf("a scalar starting with * must be double quoted, got %v and %v",
			first.Pairs[0].Value.Quote, first.Pairs[1].Value.Quote)
	}
	if first.Pairs[2].Value.Quote != domain.QuotePlain {
		t.Errorf("effect must be plain, got %v", first.Pairs[2].Value.Quote)
	}
	if second.Pairs[0].Value.Quote != domain.QuotePlain {
		t.Errorf("hitl_* does not start with *, want plain, got %v", second.Pairs[0].Value.Quote)
	}
	if second.Pairs[1].Value.Quote != domain.QuoteDouble {
		t.Errorf("resource * must be double quoted, got %v", second.Pairs[1].Value.Quote)
	}
}

func TestPermissionRulesValue_EmptyInputGivesAnEmptyList(t *testing.T) {
	for _, in := range [][]domain.PermissionRule{nil, {}} {
		got := domain.PermissionRulesValue(in)
		if got.Kind != domain.KindList || len(got.Items) != 0 {
			t.Errorf("want an empty list, got %+v", got)
		}
	}
}

func TestParsePermissionRules_ReadsWhatPermissionRulesValueRenders(t *testing.T) {
	rules := []domain.PermissionRule{
		rule("*", "*", domain.EffectDeny),
		rule("edit", "*", domain.EffectAllow),
		rule("external_directory", "*", domain.EffectAsk),
	}

	got := domain.ParsePermissionRules(domain.PermissionRulesValue(rules))

	if !reflect.DeepEqual(got, rules) {
		t.Errorf("round trip: want %v, got %v", rules, got)
	}
}

func TestParsePermissionRules_NonListValueGivesNil(t *testing.T) {
	for name, v := range map[string]domain.FieldValue{
		"scalar":  domain.ScalarValue("read", domain.QuotePlain),
		"mapping": ruleItem("read", "allow"),
		"zero":    {},
	} {
		if got := domain.ParsePermissionRules(v); got != nil {
			t.Errorf("%s: want nil, got %v", name, got)
		}
	}
}

func TestParsePermissionRules_SkipsInvalidItemsAndKeepsTheRestInOrder(t *testing.T) {
	v := ruleList(
		ruleItem("action", "read", "resource", "*", "effect", "allow"),
		domain.ScalarValue("read", domain.QuotePlain),                         // not a mapping
		ruleItem("action", "edit", "resource", "*", "effect", "maybe"),        // bad effect
		ruleItem("action", "", "resource", "*", "effect", "allow"),            // empty action
		ruleItem("action", "grep", "resource", "", "effect", "allow"),         // empty resource
		ruleItem("action", "glob", "effect", "allow"),                         // missing resource
		ruleItem("effect", "deny", "resource", "*", "action", "shell"),        // key order differs: kept
		ruleItem("action", "web", "resource", "*", "effect", "ask", "x", "y"), // extra key: kept
	)

	got := domain.ParsePermissionRules(v)

	want := []domain.PermissionRule{
		rule("read", "*", domain.EffectAllow),
		rule("shell", "*", domain.EffectDeny),
		rule("web", "*", domain.EffectAsk),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func rulesSpec() domain.ToolSpec {
	return domain.ToolSpec{
		Shape: domain.ShapeRules,
		Universe: []domain.HarnessTool{
			{Name: "read"},
			{Name: "write", Action: "edit"},
			{Name: "patch", Action: "edit"},
		},
		Rules: domain.RuleFrame{
			Leading:  []domain.PermissionRule{rule("*", "*", domain.EffectDeny)},
			Trailing: []domain.PermissionRule{rule("external_directory", "*", domain.EffectAsk)},
		},
	}
}

func TestToolSpecActionOf_UniverseToolUsesItsActionElseItsName(t *testing.T) {
	s := rulesSpec()

	if got := s.ActionOf("write"); got != "edit" {
		t.Errorf("write: want action edit, got %q", got)
	}
	if got := s.ActionOf("patch"); got != "edit" {
		t.Errorf("patch: want action edit, got %q", got)
	}
	if got := s.ActionOf("read"); got != "read" {
		t.Errorf("read has no action, want its name, got %q", got)
	}
}

func TestToolSpecActionOf_NonUniverseNameIsReturnedUnchanged(t *testing.T) {
	if got := rulesSpec().ActionOf("hitl_*"); got != "hitl_*" {
		t.Errorf("want hitl_* unchanged, got %q", got)
	}
}

func TestToolSpecIsFixedRuleAction_TrueOnlyForLeadingAndTrailingActions(t *testing.T) {
	s := rulesSpec()

	for _, a := range []string{"*", "external_directory"} {
		if !s.IsFixedRuleAction(a) {
			t.Errorf("%q is the action of a fixed rule", a)
		}
	}
	for _, a := range []string{"read", "edit", "hitl_*", ""} {
		if s.IsFixedRuleAction(a) {
			t.Errorf("%q is not the action of a fixed rule", a)
		}
	}
}
