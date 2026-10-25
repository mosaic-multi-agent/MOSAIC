package runner_test

// Regression tests for the user's model selection reaching the tier-model map.
//
// The selection (ResolvedTest.Models) supplies the subject and stub models
// sent on the Deploy call. Coverage here: a selected subject model reaches the
// subject tier and a selected stub model reaches the stub tier, each on its
// own tier. The authoring-layer fields on SubjectUnderTest hold different
// values and are irrelevant to the result.

import (
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// catalogueRequestWithModels builds a catalogue-path request with explicit
// authored model fields and runtime model selection, so each test controls
// both sources independently.
func catalogueRequestWithModels(testID, authoredSubjectModel, authoredStubModel string, sel domain.ModelSelection) runner.Request {
	req := catalogueRequest(testID)
	req.Test.Definition.Subject.Model = authoredSubjectModel
	req.Test.Definition.Subject.StubModel = authoredStubModel
	req.Test.Models = sel
	return req
}

// TestSetup_SelectedModelsReachTheirOwnTiers asserts that a selected subject
// model reaches the subject tier and a selected stub model reaches the stub
// tier, and neither is applied to the wrong tier.
func TestSetup_SelectedModelsReachTheirOwnTiers(t *testing.T) {
	req := catalogueRequestWithModels(
		"selected-models-reach-tiers",
		"authored-subject-model",
		"authored-stub-model",
		domain.ModelSelection{
			Subject: "selected-subject-model",
			Stub:    "selected-stub-model",
		},
	)

	tm := setupDeployTierMap(t, req)

	if got := tm[domain.TierTestSubject]; got != "selected-subject-model" {
		t.Errorf("Deploy TierModels[%q] = %q, want %q", domain.TierTestSubject, got, "selected-subject-model")
	}
	if got := tm[domain.TierTestStub]; got != "selected-stub-model" {
		t.Errorf("Deploy TierModels[%q] = %q, want %q", domain.TierTestStub, got, "selected-stub-model")
	}
}
