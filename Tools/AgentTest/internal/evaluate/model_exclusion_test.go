package evaluate_test

// Tests for the exclusion, retry and aggregate consequences of an invalid
// subject model: excluded from the pass rate, an infrastructure failure on its
// own, and never retried.

import (
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/evaluate"
)

func resultWithModel(validity domain.ModelValidity, reasons ...domain.FailureReason) domain.TestResult {
	verdict := domain.VerdictPass
	if len(reasons) > 0 {
		verdict = domain.VerdictFail
	}
	return domain.TestResult{Key: baseKey(), Verdict: verdict, Reasons: reasons, ModelValidity: validity}
}

func TestExclusionOf_ModelValidity(t *testing.T) {
	spawnFailed := func(r domain.TestResult) domain.TestResult {
		r.SubjectResult.Disposition = domain.DispositionSpawnFailed
		return r
	}
	tests := []struct {
		name      string
		result    domain.TestResult
		wantExcl  domain.ExclusionReason
		wantRetry bool
	}{
		{"mismatch", resultWithModel(domain.ModelMismatch, domain.ReasonModelUnverified),
			domain.ExclusionModelMismatch, false},
		{"no evidence", resultWithModel(domain.ModelNoEvidence, domain.ReasonModelUnverified),
			domain.ExclusionModelEvidenceMissing, false},
		{"verified clean run", resultWithModel(domain.ModelVerified), "", false},
		{"mismatch with assertion failure", resultWithModel(domain.ModelMismatch, domain.ReasonAssertion, domain.ReasonModelUnverified),
			domain.ExclusionModelMismatch, false},
		{"mismatch beats state integrity", resultWithModel(domain.ModelMismatch, domain.ReasonStateIntegrity, domain.ReasonModelUnverified),
			domain.ExclusionModelMismatch, false},
		{"mismatch beats spawn failed", spawnFailed(resultWithModel(domain.ModelMismatch, domain.ReasonInfrastructure, domain.ReasonModelUnverified)),
			domain.ExclusionModelMismatch, false},
		{"mismatch beats echo mismatch", resultWithModel(domain.ModelMismatch, domain.ReasonEchoMismatch, domain.ReasonModelUnverified),
			domain.ExclusionModelMismatch, false},
		{"mismatch beats infrastructure", resultWithModel(domain.ModelMismatch, domain.ReasonInfrastructure, domain.ReasonModelUnverified),
			domain.ExclusionModelMismatch, false},
		{"state integrity beats missing evidence", resultWithModel(domain.ModelNoEvidence, domain.ReasonStateIntegrity, domain.ReasonModelUnverified),
			domain.ExclusionStateIntegrity, true},
		{"spawn failed beats missing evidence", spawnFailed(resultWithModel(domain.ModelNoEvidence, domain.ReasonInfrastructure, domain.ReasonModelUnverified)),
			domain.ExclusionSpawnFailed, true},
		{"missing evidence beats echo mismatch", resultWithModel(domain.ModelNoEvidence, domain.ReasonEchoMismatch, domain.ReasonModelUnverified),
			domain.ExclusionModelEvidenceMissing, false},
		{"missing evidence beats infrastructure", resultWithModel(domain.ModelNoEvidence, domain.ReasonInfrastructure, domain.ReasonModelUnverified),
			domain.ExclusionModelEvidenceMissing, false},
		{"not assessed keeps infrastructure chain", resultWithModel("", domain.ReasonInfrastructure),
			domain.ExclusionInfrastructure, true},
		{"not assessed keeps state integrity chain", resultWithModel("", domain.ReasonStateIntegrity),
			domain.ExclusionStateIntegrity, true},
		{"verified keeps echo mismatch retry", resultWithModel(domain.ModelVerified, domain.ReasonEchoMismatch),
			domain.ExclusionEchoMismatch, true},
		{"verified keeps spawn failed retry", spawnFailed(resultWithModel(domain.ModelVerified, domain.ReasonInfrastructure)),
			domain.ExclusionSpawnFailed, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate.ExclusionOf(tc.result); got != tc.wantExcl {
				t.Errorf("ExclusionOf = %q, want %q", got, tc.wantExcl)
			}
			if got := evaluate.NeedsRetry(tc.result); got != tc.wantRetry {
				t.Errorf("NeedsRetry = %v, want %v", got, tc.wantRetry)
			}
		})
	}
}

func TestAggregate_SingleInvalidModelRun_IsInfrastructureFailure(t *testing.T) {
	cases := []struct {
		name   string
		result domain.TestResult
		reason domain.ExclusionReason
	}{
		{"mismatch", func() domain.TestResult {
			r := resultWithModel(domain.ModelMismatch, domain.ReasonModelUnverified)
			r.ModelValidityDetail = "selected model-a, observed model-b"
			return r
		}(), domain.ExclusionModelMismatch},
		{"no evidence", func() domain.TestResult {
			r := resultWithModel(domain.ModelNoEvidence, domain.ReasonModelUnverified)
			r.ModelValidityDetail = "selected model-a, observed none (tool_failed)"
			return r
		}(), domain.ExclusionModelEvidenceMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agg := evaluate.Aggregate([]domain.TestResult{tc.result}, domain.RepetitionPolicy{PassRate: 1.0})

			if !agg.InfrastructureFailure {
				t.Errorf("InfrastructureFailure = false, want true for one invalid-model run")
			}
			if agg.Verdict != domain.VerdictFail {
				t.Errorf("Verdict = %q, want FAIL", agg.Verdict)
			}
			if agg.Excluded != 1 || agg.Counted != 0 {
				t.Errorf("Excluded = %d, Counted = %d, want 1 and 0; the run must stay out of the pass rate", agg.Excluded, agg.Counted)
			}
			if len(agg.Exclusions) != 1 || agg.Exclusions[0].Reason != tc.reason {
				t.Fatalf("Exclusions = %+v, want one with reason %q", agg.Exclusions, tc.reason)
			}
			if agg.Exclusions[0].Detail != tc.result.ModelValidityDetail {
				t.Errorf("Exclusion detail = %q, want the run's ModelValidityDetail %q", agg.Exclusions[0].Detail, tc.result.ModelValidityDetail)
			}
		})
	}
}

func TestAggregate_InvalidModelRunDoesNotCountTowardPassRate(t *testing.T) {
	invalid := resultWithModel(domain.ModelMismatch, domain.ReasonModelUnverified)
	invalid.ModelValidityDetail = "selected model-a, observed model-b"
	valid := resultWithModel(domain.ModelVerified)

	agg := evaluate.Aggregate([]domain.TestResult{valid, invalid}, domain.RepetitionPolicy{PassRate: 1.0})

	if agg.Counted != 1 || agg.Passed != 1 || agg.PassRate != 1.0 {
		t.Errorf("Counted/Passed/PassRate = %d/%d/%v, want 1/1/1.0", agg.Counted, agg.Passed, agg.PassRate)
	}
	if !agg.InfrastructureFailure || agg.Verdict != domain.VerdictFail {
		t.Errorf("InfrastructureFailure = %v, Verdict = %q, want true and FAIL despite the passing run", agg.InfrastructureFailure, agg.Verdict)
	}
}
