package app_test

// promote_optional_fields_test.go covers the optional generic fields of single-agent promote:
// recommended_tier, tier_rationale and required_skills are written only when the source already
// carries the key or the user supplied a non-empty value. A field that is skipped, cancelled or
// answered empty is left out of the generic agent: no key, no empty value.

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// optionalFieldKeys are the three generic fields that are left out when not supplied.
var optionalFieldKeys = []string{"recommended_tier", "tier_rationale", "required_skills"}

// promoteOptionalFields promotes src (via the stub) into a fresh MOSAIC root and returns the
// result and the root. The source file path is the subject of the three field questions.
func promoteOptionalFields(t *testing.T, src []byte, build func(subject string) *interactiontest.Stub) (app.PromoteResult, string, string) {
	t.Helper()
	srcPath := writeSourceFile(t, src)
	mosaicRoot := writeMosaicRoot(t)
	deps, _ := newPromoteDeps(t, build(srcPath), mosaicRoot)
	result, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
		FilePath: srcPath, Category: "TestCategory", HarnessID: "stub-harness",
	})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	return result, mosaicRoot, srcPath
}

// parsePromoted reads and parses the promoted file at path.
func parsePromoted(t *testing.T, path string) (*docformat.Document, []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read promoted file: %v", err)
	}
	doc, err := docformat.Parse(b)
	if err != nil {
		t.Fatalf("parse promoted file: %v", err)
	}
	if _, ok := doc.Frontmatter().Get("id"); !ok {
		t.Fatal("promoted file has no id; guard against a vacuous pass on an empty file")
	}
	return doc, b
}

// assertKeysAbsent fails when the promoted frontmatter carries any of keys, whatever its value.
func assertKeysAbsent(t *testing.T, doc *docformat.Document, raw []byte, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, present := doc.Frontmatter().Get(k); present {
			t.Errorf("%s is present in the promoted file; an unsupplied field must be left out\n%s", k, raw)
		}
	}
}

// TestPromote_OptionalFieldsNotSupplied_LeftOutOfGenericAgent verifies that tier, rationale
// and skills that are skipped, cancelled or answered empty produce none of the three keys.
func TestPromote_OptionalFieldsNotSupplied_LeftOutOfGenericAgent(t *testing.T) {
	scenarios := map[string]func(subject string) *interactiontest.Stub{
		"skipped": func(string) *interactiontest.Stub { return interactiontest.NewBuilder().Build() },
		"cancelled": func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().
				AnswerCancelledChoice(domain.QPromoteRecommendedTier, s).
				AnswerCancelledText(domain.QPromoteTierRationale, s).
				AnswerCancelledText(domain.QPromoteRequiredSkills, s).Build()
		},
		"answered empty": func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().
				AnswerSelectOneCustom(domain.QPromoteRecommendedTier, s, "").
				AnswerText(domain.QPromoteTierRationale, s, "").
				AnswerText(domain.QPromoteRequiredSkills, s, "").Build()
		},
		"whitespace and separators only": func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().
				AnswerText(domain.QPromoteTierRationale, s, "  \t ").
				AnswerText(domain.QPromoteRequiredSkills, s, " , ,, ").Build()
		},
	}
	for name, build := range scenarios {
		t.Run(name, func(t *testing.T) {
			// Act
			result, _, _ := promoteOptionalFields(t, eligibleAgentWithoutRecoverableFields(), build)

			// Assert
			doc, raw := parsePromoted(t, result.DestinationPath)
			assertKeysAbsent(t, doc, raw, optionalFieldKeys...)
			if len(result.RecoveredFields) != 0 {
				t.Errorf("RecoveredFields = %v, want none", result.RecoveredFields)
			}
		})
	}
}

// TestPromote_OptionalFieldsSupplied_Written verifies supplied values are written, and that
// supplying one field does not bring the other two back as empty values.
func TestPromote_OptionalFieldsSupplied_Written(t *testing.T) {
	// Arrange
	build := func(s string) *interactiontest.Stub {
		return interactiontest.NewBuilder().
			AnswerSelectOne(domain.QPromoteRecommendedTier, s, "HIGH").
			AnswerText(domain.QPromoteTierRationale, s, "needs deep reasoning").
			AnswerText(domain.QPromoteRequiredSkills, s, "lean-tdd, efficient-file-reading").Build()
	}

	// Act
	result, _, _ := promoteOptionalFields(t, eligibleAgentWithoutRecoverableFields(), build)

	// Assert
	if got, _ := parseFrontmatterFieldSvc(t, result.DestinationPath, "recommended_tier"); got != "HIGH" {
		t.Errorf("recommended_tier = %q, want HIGH", got)
	}
	if got, _ := parseFrontmatterFieldSvc(t, result.DestinationPath, "tier_rationale"); got != "needs deep reasoning" {
		t.Errorf("tier_rationale = %q, want the supplied text", got)
	}
	want := []string{"lean-tdd", "efficient-file-reading"}
	if got := parseFrontmatterListSvc(t, result.DestinationPath, "required_skills"); !reflect.DeepEqual(got, want) {
		t.Errorf("required_skills = %v, want %v", got, want)
	}
}

// TestPromote_OnlyOneOptionalFieldSupplied_OtherTwoLeftOut verifies each field is judged on
// its own: the supplied one is written, the two unsupplied ones are absent.
func TestPromote_OnlyOneOptionalFieldSupplied_OtherTwoLeftOut(t *testing.T) {
	cases := map[string]struct {
		supplied string
		build    func(s string) *interactiontest.Stub
	}{
		"tier": {"recommended_tier", func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().AnswerSelectOne(domain.QPromoteRecommendedTier, s, "LOW").Build()
		}},
		"rationale": {"tier_rationale", func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().AnswerText(domain.QPromoteTierRationale, s, "cheap work").Build()
		}},
		"skills": {"required_skills", func(s string) *interactiontest.Stub {
			return interactiontest.NewBuilder().AnswerText(domain.QPromoteRequiredSkills, s, "lean-tdd").Build()
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			result, _, _ := promoteOptionalFields(t, eligibleAgentWithoutRecoverableFields(), tc.build)

			// Assert
			doc, raw := parsePromoted(t, result.DestinationPath)
			if _, present := doc.Frontmatter().Get(tc.supplied); !present {
				t.Errorf("%s was supplied but is absent from the promoted file", tc.supplied)
			}
			var others []string
			for _, k := range optionalFieldKeys {
				if k != tc.supplied {
					others = append(others, k)
				}
			}
			assertKeysAbsent(t, doc, raw, others...)
		})
	}
}

// TestPromote_SourceCarriesOneOptionalField_KeepsItAndLeavesOthersOut verifies a key the source
// already carries is kept and not asked about, while the two the source lacks are left out.
func TestPromote_SourceCarriesOneOptionalField_KeepsItAndLeavesOthersOut(t *testing.T) {
	// Arrange
	src := bytes.Replace(eligibleAgentWithoutRecoverableFields(),
		[]byte("role: subagent\n"), []byte("role: subagent\ntier_rationale: keep this text\n"), 1)
	var stub *interactiontest.Stub

	// Act
	result, _, srcPath := promoteOptionalFields(t, src, func(string) *interactiontest.Stub {
		stub = interactiontest.NewBuilder().Build()
		return stub
	})

	// Assert
	if got, _ := parseFrontmatterFieldSvc(t, result.DestinationPath, "tier_rationale"); got != "keep this text" {
		t.Errorf("tier_rationale = %q, want the source value", got)
	}
	if stub.WasAsked(domain.QPromoteTierRationale, srcPath) {
		t.Error("rationale was asked although the source carries it")
	}
	doc, raw := parsePromoted(t, result.DestinationPath)
	assertKeysAbsent(t, doc, raw, "recommended_tier", "required_skills")
}

// TestPromote_OptionalFieldsNotSupplied_AgentLoadsInCatalog verifies the generic agent without
// the three fields is a valid catalog agent with no tier and no skills.
func TestPromote_OptionalFieldsNotSupplied_AgentLoadsInCatalog(t *testing.T) {
	// Arrange
	build := func(string) *interactiontest.Stub { return interactiontest.NewBuilder().Build() }

	// Act
	result, mosaicRoot, _ := promoteOptionalFields(t, eligibleAgentWithoutRecoverableFields(), build)

	// Assert
	doc, raw := parsePromoted(t, result.DestinationPath)
	assertKeysAbsent(t, doc, raw, optionalFieldKeys...)
	cat, err := catalog.Load(mosaicRoot, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	agent, found := cat.Agent(result.Key)
	if !found {
		t.Fatalf("agent %q not found in the catalog after promote", result.Key)
	}
	if agent.RecommendedTier != "" || agent.TierRationale != "" || len(agent.RequiredSkills) != 0 {
		t.Errorf("agent carries tier %q, rationale %q, skills %v; want none", agent.RecommendedTier, agent.TierRationale, agent.RequiredSkills)
	}
}
