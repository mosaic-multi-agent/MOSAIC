package domain

// Debug-log events of the mid-run and start-time artifact repair cycle.
// Pre-consultation emits none of them.
const (
	// A read found Orchestration.md damaged.
	// Fields: when ("consultation" | "start"), problems (count), first (first problem).
	EventSessionRepairDamageFound = "session.repair.damage_found"

	// The damaged artifact was copied beside the last-good copy.
	// Fields: path.
	EventSessionRepairDamagedCopySaved = "session.repair.damaged_copy_saved"

	// Saving the damaged copy failed; the repair proceeds without it.
	// Fields: error.
	EventSessionRepairDamagedCopyFailed = "session.repair.damaged_copy_failed"

	// A repair consultation is made.
	// Fields: attempt, last_good (path or ""), dispatch_log (path or "").
	EventSessionRepairConsult = "session.repair.consult"

	// A repair consultation's result was verified and accepted.
	// Fields: attempt.
	EventSessionRepairAccepted = "session.repair.accepted"

	// A repair consultation's result was verified and not accepted.
	// Fields: attempt, problems (count), reasons (every problem, joined with "; ").
	EventSessionRepairRejected = "session.repair.rejected"

	// The run ended with the artifact still damaged.
	// Fields: reason (an UnrepairedReason), consultations.
	EventSessionRepairStop = "session.repair.stop"
)
