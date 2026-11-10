package domain

import "strconv"

// ArtifactDamageReport is the content of the routing consultation request's
// artifact_damage member: what a read found wrong with Orchestration.md and the
// evidence the orchestrator may use to repair it. It carries no path of the
// saved damaged copy, which exists for human diagnosis only.
type ArtifactDamageReport struct {
	Problems     []ArtifactDamageProblem // never empty
	LastGoodCopy string                  // absolute path; "" = none (wire null)
	DispatchLog  string                  // absolute path of a usable run dispatch log; "" = none (wire null)
}

// UnrepairedReason says why a run with a damaged artifact ended without an
// accepted repair.
type UnrepairedReason string

const (
	UnrepairedRejected           UnrepairedReason = "rejected"            // MaxConsultations repairs not accepted
	UnrepairedConsultationFailed UnrepairedReason = "consultation-failed" // a repair consultation failed
	UnrepairedStopped            UnrepairedReason = "stopped"             // user stop, force stop or context cancellation
	UnrepairedNoConsultant       UnrepairedReason = "no-consultant"       // no routing consultant to ask
)

// UnrepairedDamageError ends a run whose damaged artifact was not repaired.
type UnrepairedDamageError struct {
	Reason        UnrepairedReason
	Artifact      string                  // Orchestration.md path
	Problems      []ArtifactDamageProblem // latest known problems; never empty
	DamagedCopy   string                  // "" when no copy was saved
	LastGoodCopy  string                  // "" when none
	Consultations int                     // repair consultations made, 0..2
	Cause         error                   // consultation error for UnrepairedConsultationFailed; nil otherwise
}

// Error names the reason, every problem and every non-empty path.
func (e *UnrepairedDamageError) Error() string {
	msg := "artifact damage not repaired (" + string(e.Reason) + ", " + strconv.Itoa(e.Consultations) + " repair consultations)"
	for _, p := range e.Problems {
		msg += "; " + p.String()
	}
	for _, path := range []struct{ label, value string }{
		{"artifact", e.Artifact}, {"damaged copy", e.DamagedCopy}, {"last-good copy", e.LastGoodCopy},
	} {
		if path.value != "" {
			msg += "; " + path.label + ": " + path.value
		}
	}
	return msg
}

// Unwrap returns the consultation error, if any.
func (e *UnrepairedDamageError) Unwrap() error { return e.Cause }
