package runner_test

// Tests for SubjectModel and StubModel being placed in RunEvidence from the
// tier-model map inside runner.BuildEvidence (TDD RED phase).
//
// Stage 14 requires each run's report to record the models actually used.
// The values originate in the tier-model map built by buildTierModelMap from
// preflight.ResolvedTest.Models — the same map sent to the Deploy port. After
// the run, BuildEvidence must read the resolved tier-model values back out and
// place them in RunEvidence.SubjectModel and RunEvidence.StubModel, so the
// carry-through through evaluate.Evaluate into TestResult and then into the
// report is possible.
//
// The design contract (ContractsDesign.md § domain.RunEvidence) states:
//   "SubjectModel and StubModel are the models this run actually used, taken
//    from the same tier-model map the deployment port was given — not from a
//    parallel source, and not from the test definition."
//
// These tests drive runner.Run with a capturing evaluator that records the
// RunEvidence it receives, then assert the observable state of the model
// fields. They say nothing about the intermediate storage the implementation
// uses — only the value that reaches the evaluator matters.

import (
	"context"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// captureEvaluator returns a domain.AttemptEvaluator that records the
// RunEvidence it is called with into *captured, and returns a zero TestResult.
// It allows a test to inspect evidence without caring about the verdict.
func captureEvaluator(captured *domain.RunEvidence) domain.AttemptEvaluator {
	return func(ev domain.RunEvidence) domain.TestResult {
		*captured = ev
		return domain.TestResult{}
	}
}

// TestRun_SubjectModelPlacedInEvidenceFromTierModelMap asserts that when
// ResolvedTest.Models.Subject is non-empty, RunEvidence.SubjectModel holds
// the same value when the evaluator is called. The value must come from the
// tier-model map, not from a separate derivation, so it cannot drift from
// what was actually deployed.
func TestRun_SubjectModelPlacedInEvidenceFromTierModelMap(t *testing.T) {
	// Arrange
	h := newHarness(t)
	req := catalogueRequest("model-evidence-subject-model")
	req.Test.Models = domain.ModelSelection{
		Subject: "claude-opus-4-5",
		// Stub deliberately absent — a different test covers the stub tier.
	}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, req, captureEvaluator(&ev))

	// Assert
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if ev.SubjectModel != "claude-opus-4-5" {
		t.Errorf("RunEvidence.SubjectModel = %q, want %q; "+
			"BuildEvidence must read the resolved subject-tier model from the "+
			"tier-model map (Models.Subject = %q) and place it in RunEvidence — "+
			"not from a parallel source, and not from the test definition, "+
			"so the value the report records is always the one that was deployed",
			ev.SubjectModel, "claude-opus-4-5", "claude-opus-4-5")
	}
}

// TestRun_StubModelPlacedInEvidenceFromTierModelMap asserts that when
// ResolvedTest.Models.Stub is non-empty, RunEvidence.StubModel holds the
// same value when the evaluator is called. The value must come from the
// stub tier of the tier-model map — the same entry sent to the Deploy port —
// not from Subject.StubModel or any authoring field.
func TestRun_StubModelPlacedInEvidenceFromTierModelMap(t *testing.T) {
	// Arrange
	h := newHarness(t)
	req := catalogueRequest("model-evidence-stub-model")
	req.Test.Models = domain.ModelSelection{
		Subject: "claude-opus-4-5",
		Stub:    "claude-haiku-3-5",
	}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, req, captureEvaluator(&ev))

	// Assert
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if ev.StubModel != "claude-haiku-3-5" {
		t.Errorf("RunEvidence.StubModel = %q, want %q; "+
			"BuildEvidence must read the resolved stub-tier model from the "+
			"tier-model map (Models.Stub = %q) and place it in RunEvidence — "+
			"not from Subject.StubModel or any authoring field — "+
			"so the value the report records is always the one that was deployed",
			ev.StubModel, "claude-haiku-3-5", "claude-haiku-3-5")
	}
}

// TestRun_StubModelFallsBackToSubjectModelInEvidence asserts the critical
// scenario where the user selects a subject model but no stub model. The
// tier-model map resolves the stub tier to the subject value as a fallback,
// so stubs are deployed on the subject model. RunEvidence.StubModel must
// reflect this resolved value — not the empty string from Models.Stub — so
// the report can correctly attribute the run.
func TestRun_StubModelFallsBackToSubjectModelInEvidence(t *testing.T) {
	// Arrange: subject model set, stub model absent — the common user case.
	h := newHarness(t)
	req := catalogueRequest("model-evidence-stub-fallback")
	req.Test.Models = domain.ModelSelection{
		Subject: "model-A",
		Stub:    "", // no explicit stub model
	}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, req, captureEvaluator(&ev))

	// Assert
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if ev.StubModel != "model-A" {
		t.Errorf("RunEvidence.StubModel = %q, want %q; "+
			"when Models.Stub is empty the tier-model map falls back to the subject "+
			"tier value so stubs run on the subject model — the report must reflect "+
			"the actual model used, not the empty string from ModelSelection.Stub",
			ev.StubModel, "model-A")
	}
}

// TestRun_AbsentSubjectModelAbortsBeforeSpawn asserts that when
// ResolvedTest.Models.Subject is empty — no model was selected — the run
// never falls back to a harness default: the deployed definitions carry no
// selected model, so the run aborts with an error before the harness is
// spawned or launched.
func TestRun_AbsentSubjectModelAbortsBeforeSpawn(t *testing.T) {
	// Arrange — no runtime model selection
	h := newHarness(t)
	req := catalogueRequest("model-evidence-absent-subject-model")
	req.Test.Models = domain.ModelSelection{} // neither model supplied

	// Act
	_, err := runner.Run(context.Background(), h.Deps, req, nil)

	// Assert
	if err == nil {
		t.Fatal("Run returned nil error with no subject model selected; want an abort before the harness starts")
	}
	for _, c := range h.Rec.all() {
		if c == "adapter.SpawnPlan" || c == "launcher.Launch" {
			t.Errorf("%s was called although no subject model was selected; the run must abort before spawn", c)
		}
	}
}
