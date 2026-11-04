package evaluate

import "mosaic-agent-test/internal/domain"

// ExclusionOf reports why r would be excluded from the pass-rate denominator,
// or the empty string when it would be counted.
//
// State integrity takes precedence over spawn failure when both are present,
// so one run yields exactly one exclusion reason and the two are never
// double-counted.
//
// NeedsRetry(r) is true exactly when ExclusionOf(r) is non-empty.
func ExclusionOf(r domain.TestResult) domain.ExclusionReason {
	// A model mismatch is a definitive finding that the run did not use the
	// selected model; it outranks every other reason and is never retried.
	if r.ModelValidity == domain.ModelMismatch {
		return domain.ExclusionModelMismatch
	}
	// State integrity takes precedence: a lock reclaim makes all subsequent
	// verdicts suspect, regardless of other disposal markers.
	if hasReason(r.Reasons, domain.ReasonStateIntegrity) {
		return domain.ExclusionStateIntegrity
	}
	// Spawn failure: the harness process exited non-zero, so the subject may
	// never have run. Evidence about the tool's environment, not the subject.
	if r.SubjectResult.Disposition == domain.DispositionSpawnFailed {
		return domain.ExclusionSpawnFailed
	}
	// Missing model evidence: the subject model could not be determined. It
	// outranks echo mismatch and infrastructure but yields to the two retryable
	// reasons above, whose retry may produce evidence. Never retried itself.
	if r.ModelValidity == domain.ModelNoEvidence {
		return domain.ExclusionModelEvidenceMissing
	}
	// Echo mismatch: a stubbed collaborator did not echo its stub faithfully.
	// Evidence about the tool's environment, not the subject.
	if hasReason(r.Reasons, domain.ReasonEchoMismatch) {
		return domain.ExclusionEchoMismatch
	}
	// Infrastructure: a runner error or recovered panic prevented the attempt
	// from starting. Evidence about the infrastructure, not the subject.
	// Lowest precedence in the chain.
	if hasReason(r.Reasons, domain.ReasonInfrastructure) {
		return domain.ExclusionInfrastructure
	}
	return ""
}
