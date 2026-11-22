package descriptor_test

// minimal_grant_rules_test.go specifies the emptiness test of RenderMinimalToolGrant for a
// rules-shaped tools value: a rule list grants something only when one of its rules allows.
// A list of deny or ask rules grants nothing and is as dangerous as an empty list.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func ruleMapping(action, effect string) domain.FieldValue {
	return domain.MappingValue([]domain.FieldPair{
		{Key: "action", Value: domain.ScalarValue(action, domain.QuotePlain)},
		{Key: "resource", Value: domain.ScalarValue("*", domain.QuoteDouble)},
		{Key: "effect", Value: domain.ScalarValue(effect, domain.QuotePlain)},
	})
}

// renderMinimalWith runs RenderMinimalToolGrant on a stub whose tools field is value.
func renderMinimalWith(value domain.FieldValue) error {
	stub := newStub(toolsKeyDescriptor("permissions"), func(req domain.ToolRequest) (domain.ToolResult, error) {
		return domain.ToolResult{
			Fields: []domain.FrontmatterField{{Key: "permissions", Value: value}},
		}, nil
	})
	_, err := descriptor.RenderMinimalToolGrant(stub, "agent")
	return err
}

func TestRenderMinimalToolGrant_RuleListWithAnAllowRuleIsAccepted(t *testing.T) {
	value := domain.ListValue([]domain.FieldValue{
		ruleMapping("*", "deny"), ruleMapping("read", "allow"), ruleMapping("external_directory", "ask"),
	}, domain.ListBlock)

	if err := renderMinimalWith(value); err != nil {
		t.Errorf("a rule list with an allow rule grants something, got %v", err)
	}
}

func TestRenderMinimalToolGrant_RuleListWithoutAnAllowRuleIsRefused(t *testing.T) {
	cases := map[string][]domain.FieldValue{
		"deny only": {ruleMapping("*", "deny"), ruleMapping("read", "deny")},
		"ask only":  {ruleMapping("external_directory", "ask")},
		"deny and ask": {
			ruleMapping("*", "deny"), ruleMapping("external_directory", "ask"),
		},
	}
	for name, items := range cases {
		t.Run(name, func(t *testing.T) {
			err := renderMinimalWith(domain.ListValue(items, domain.ListBlock))

			if !errors.Is(err, descriptor.ErrMinimalGrantValueEmpty) {
				t.Errorf("got %v, want ErrMinimalGrantValueEmpty: nothing in the list grants a tool", err)
			}
		})
	}
}

func TestRenderMinimalToolGrant_RuleWithoutAnEffectDoesNotGrant(t *testing.T) {
	noEffect := domain.MappingValue([]domain.FieldPair{
		{Key: "action", Value: domain.ScalarValue("read", domain.QuotePlain)},
		{Key: "resource", Value: domain.ScalarValue("*", domain.QuoteDouble)},
	})

	err := renderMinimalWith(domain.ListValue([]domain.FieldValue{noEffect}, domain.ListBlock))

	if !errors.Is(err, descriptor.ErrMinimalGrantValueEmpty) {
		t.Errorf("got %v, want ErrMinimalGrantValueEmpty", err)
	}
}

func TestRenderMinimalToolGrant_ListOfScalarsKeepsItsResult(t *testing.T) {
	scalars := domain.ListValue([]domain.FieldValue{domain.ScalarValue("read", domain.QuotePlain)}, domain.ListFlow)

	if err := renderMinimalWith(scalars); err != nil {
		t.Errorf("a non-empty list of scalars is accepted as before, got %v", err)
	}
	if err := renderMinimalWith(domain.ListValue(nil, domain.ListFlow)); !errors.Is(err, descriptor.ErrMinimalGrantValueEmpty) {
		t.Errorf("an empty list is still refused, got %v", err)
	}
}
