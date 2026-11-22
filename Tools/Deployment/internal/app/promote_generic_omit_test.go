package app

// promote_generic_omit_test.go covers the generation core's handling of the three optional
// generic fields: an empty or nil PromoteInput value leaves the key out of the output
// entirely, while a supplied value is written. Tests are in package app to reach the
// unexported buildGenericAgent.

import (
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

func buildOmitInput(tier, rationale string, skills []string) PromoteInput {
	return PromoteInput{
		Source:          sourceWithoutRecoverableFields(),
		NumericID:       "9",
		Key:             "field-recovery-agent",
		Role:            domain.RoleSubagent,
		Version:         "1.0.0",
		RecommendedTier: tier,
		TierRationale:   rationale,
		RequiredSkills:  skills,
	}
}

// TestBuildGenericAgent_EmptyOptionalFields_KeysLeftOut verifies that an empty tier, an empty
// rationale and nil or empty skills each leave their key out of the generated frontmatter.
func TestBuildGenericAgent_EmptyOptionalFields_KeysLeftOut(t *testing.T) {
	cases := map[string]struct {
		in  PromoteInput
		key string
	}{
		"empty tier":         {buildOmitInput("", "r", []string{"s"}), "recommended_tier"},
		"empty rationale":    {buildOmitInput("t", "", []string{"s"}), "tier_rationale"},
		"nil skills":         {buildOmitInput("t", "r", nil), "required_skills"},
		"empty skills slice": {buildOmitInput("t", "r", []string{}), "required_skills"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			out, err := buildGenericAgent(tc.in)
			if err != nil {
				t.Fatalf("buildGenericAgent: %v", err)
			}

			// Assert
			doc, parseErr := docformat.Parse(out)
			if parseErr != nil {
				t.Fatalf("parse output: %v", parseErr)
			}
			if _, ok := doc.Frontmatter().Get("id"); !ok {
				t.Fatal("output has no id; guard against a vacuous pass")
			}
			if v, present := doc.Frontmatter().Get(tc.key); present {
				t.Errorf("%s is present (%+v); an unsupplied field must be left out", tc.key, v)
			}
		})
	}
}

// TestBuildGenericAgent_NoOptionalFieldsSupplied_NoneWritten verifies that with all three left
// empty none of the keys appears, in the parsed frontmatter or as raw text.
func TestBuildGenericAgent_NoOptionalFieldsSupplied_NoneWritten(t *testing.T) {
	// Act
	out, err := buildGenericAgent(buildOmitInput("", "", nil))
	if err != nil {
		t.Fatalf("buildGenericAgent: %v", err)
	}

	// Assert
	doc, parseErr := docformat.Parse(out)
	if parseErr != nil {
		t.Fatalf("parse output: %v", parseErr)
	}
	if _, ok := doc.Frontmatter().Get("id"); !ok {
		t.Fatal("output has no id; guard against a vacuous pass")
	}
	for _, k := range []string{"recommended_tier", "tier_rationale", "required_skills"} {
		if _, present := doc.Frontmatter().Get(k); present {
			t.Errorf("%s is present in the output:\n%s", k, out)
		}
	}
}

// TestBuildGenericAgent_SuppliedOptionalFields_StillWritten verifies the omission rule does not
// swallow values that were supplied.
func TestBuildGenericAgent_SuppliedOptionalFields_StillWritten(t *testing.T) {
	// Act
	out, err := buildGenericAgent(buildOmitInput("HIGH", "deep work", []string{"lean-tdd"}))
	if err != nil {
		t.Fatalf("buildGenericAgent: %v", err)
	}

	// Assert
	if got, _ := parseFrontmatterField(t, out, "recommended_tier"); got != "HIGH" {
		t.Errorf("recommended_tier = %q, want HIGH", got)
	}
	if got, _ := parseFrontmatterField(t, out, "tier_rationale"); got != "deep work" {
		t.Errorf("tier_rationale = %q, want the supplied text", got)
	}
	if got := parseFrontmatterList(t, out, "required_skills"); len(got) != 1 || got[0] != "lean-tdd" {
		t.Errorf("required_skills = %v, want [lean-tdd]", got)
	}
}
