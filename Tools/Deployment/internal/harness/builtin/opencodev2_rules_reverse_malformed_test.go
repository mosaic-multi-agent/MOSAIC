package builtin_test

// opencodev2_rules_reverse_malformed_test.go specifies how the descriptor reads back a deployed
// V2 rule list that a user hand-edited into an invalid shape: invalid items are skipped, the
// valid ones still count, and nothing fails.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func malformedRuleItem(pairs ...string) domain.FieldValue {
	var ps []domain.FieldPair
	for i := 0; i+1 < len(pairs); i += 2 {
		ps = append(ps, domain.FieldPair{Key: pairs[i], Value: domain.ScalarValue(pairs[i+1], domain.QuotePlain)})
	}
	return domain.MappingValue(ps)
}

func malformedRules() domain.FieldValue {
	return domain.ListValue([]domain.FieldValue{
		malformedRuleItem("action", "read", "resource", "*", "effect", "allow"),
		domain.ScalarValue("glob", domain.QuotePlain),                           // not a mapping
		malformedRuleItem("action", "grep", "resource", "*", "effect", "maybe"), // unknown effect
		malformedRuleItem("action", "edit", "effect", "allow"),                  // missing resource
		malformedRuleItem("resource", "*", "effect", "allow"),                   // missing action
		malformedRuleItem("action", "shell", "resource", "", "effect", "allow"), // empty resource
		malformedRuleItem("effect", "allow", "resource", "*", "action", "question"),
	}, domain.ListBlock)
}

func TestOpenCodeV2Reverse_InvalidRuleItemsAreSkippedAndValidOnesStillRead(t *testing.T) {
	d := loadV2(t)

	entries := descriptor.ExtractToolEntries(d, malformedRules())
	grants := descriptor.ExtractToolGrants(d, malformedRules())

	if want := []string{"read", "question"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
	wantGrants := []domain.ToolGrant{
		{Name: "read", Disposition: domain.Allow},
		{Name: "question", Disposition: domain.Allow},
	}
	if !reflect.DeepEqual(grants, wantGrants) {
		t.Errorf("grants = %v, want %v", grants, wantGrants)
	}
}
