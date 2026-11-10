package domain

// ModelValidity is the outcome of comparing the selected subject model with the
// model(s) observed in the run's logs. The empty value means "not assessed":
// no result was evaluated for the run.
type ModelValidity string

const (
	// ModelVerified: every observed subject model equals the selection.
	ModelVerified ModelValidity = "verified"
	// ModelMismatch: at least one observed subject model differs from the selection.
	ModelMismatch ModelValidity = "mismatch"
	// ModelNoEvidence: the subject model could not be determined.
	ModelNoEvidence ModelValidity = "no_evidence"
)

// ModelVerdict is the suite-level model verdict derived from the validity of
// every run.
type ModelVerdict string

const (
	// ModelVerdictVerified: at least one run exists and every run is verified.
	ModelVerdictVerified ModelVerdict = "MODEL_VERIFIED"
	// ModelVerdictUnverified: any run is not verified, or no run exists.
	ModelVerdictUnverified ModelVerdict = "MODEL_UNVERIFIED"
)

// ReasonModelUnverified is the failure reason of a run whose subject model is
// not verified (mismatch or no evidence).
const ReasonModelUnverified FailureReason = "MODEL_UNVERIFIED"

// Exclusion reasons for runs whose subject model is not verified. Neither is
// ever retried.
const (
	ExclusionModelMismatch        ExclusionReason = "model_mismatch"
	ExclusionModelEvidenceMissing ExclusionReason = "model_evidence_missing"
)
