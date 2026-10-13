package domain

// EngineDecision is the engine's answer to "what happens next".
// Exactly one of the fields is non-nil.
type EngineDecision struct {
	Dispatch  *DispatchDecision
	Complete  *CompleteDecision
	Deviation *DeviationDecision
	Consult   *ConsultDecision
	Stop      *StopDecision
}

// ConsultDecision signals that this decision belongs to the routing consultant,
// not to the engine. It is the only decision the engine produces in
// ExecutionModeOrchestrated.
//
// It differs from DeviationDecision in cause, not in handling: a Deviation
// means "the engine tried and could not decide"; a Consult means "the engine
// was never entitled to decide". Both are resolved by the same session routine
// and both produce a ConsultationRequest.
type ConsultDecision struct {
	// Trigger states why the consultant is being asked.
	Trigger ConsultTrigger

	// CurrentRow, CurrentPhase and CurrentStage describe the position the run
	// is at. CurrentRow is -1 on the first decision of a new run.
	CurrentRow   int
	CurrentPhase string
	CurrentStage string

	// ArtifactState is the state at the time of the decision.
	ArtifactState ArtifactState
}

// ConsultTrigger classifies why a consultation is being made.
type ConsultTrigger string

const (
	// ConsultTriggerOrchestratedMode: the run is in ExecutionModeOrchestrated,
	// where every routing decision is the consultant's.
	ConsultTriggerOrchestratedMode ConsultTrigger = "orchestrated-mode"

	// ConsultTriggerDeviation: the engine produced a DeviationDecision.
	ConsultTriggerDeviation ConsultTrigger = "deviation"
)

// DispatchDecision carries the steps to execute.
// The Steps slice contains exactly one element in this version, but carries
// a collection to preserve the concurrency seam so that parallel stage
// dispatch can be added later without rewriting the engine-session contract.
type DispatchDecision struct {
	Steps []DispatchStep
}

// CompleteDecision signals that the run has finished successfully.
type CompleteDecision struct {
	FinalState CurrentState // the current_state to write one last time
}

// DeviationDecision signals that the engine cannot decide deterministically.
type DeviationDecision struct {
	Info DeviationInfo
}

// StopDecision signals that the run should stop (e.g. graceful stop request).
type StopDecision struct {
	Reason string
	// Err is the typed cause when the stop originates from an error (for
	// example *PositionUnresolvedError); nil otherwise. Reason == Err.Error()
	// when set.
	Err error
}

// DeviationKind classifies why the engine could not decide.
type DeviationKind string

const (
	DeviationNonSuccess     DeviationKind = "non-success-status"
	DeviationAmbiguousRoute DeviationKind = "ambiguous-routing"
	DeviationHarnessError   DeviationKind = "harness-error"
	// DeviationReviewLoopLimit: Mode 3 stopped auto-routing a reviewer's findings
	// because its COMPLETED_NEEDS_ACTION count reached review_loop_limit.
	DeviationReviewLoopLimit DeviationKind = "review-loop-limit"
	// DeviationScheduleDefect: a block schedule a producing or in-block writer
	// row produced is still defective after the schedule-defect re-dispatch
	// limit. The session itself enters the consultation; the engine never
	// produces this kind.
	DeviationScheduleDefect DeviationKind = "schedule-defect"
	// DeviationFinishedWork: after an engine-routed SUCCESS, the step the
	// engine would dispatch next (row + iteration value) already has a SUCCESS
	// entry with that recorded WorkflowRow and Iteration, so the engine
	// consults instead of repeating finished work. PlannedContinuation is that
	// step. Internal only: never serialised. Response is the zero
	// ProtocolResponse when Next runs on resume (LastResponse == nil) and must
	// not be relied on by any consumer.
	DeviationFinishedWork DeviationKind = "finished-work"
)

// DeviationInfo carries everything the deviation resolver needs.
type DeviationInfo struct {
	Kind                DeviationKind
	Response            ProtocolResponse // the deviating response
	CurrentRow          int              // row index of the row that deviated
	CurrentPhase        string
	CurrentStage        string
	// PlannedContinuation is where the happy path would have gone; set for
	// DeviationFinishedWork (the already-finished step), nil otherwise.
	PlannedContinuation *DispatchStep
	ArtifactState       ArtifactState // current state at time of deviation
	// ErrorReason is a Runner-constructed error description. When non-empty it
	// becomes the consultation's last_error_reason; otherwise the reason is
	// derived from the deviating response as before.
	ErrorReason string
}

// ResumeInfo is the output of engine resume-point derivation: where to continue
// from after parsing an existing artifact.
type ResumeInfo struct {
	// RowIndex is the routing table row to dispatch next.
	// When RerunLast is true, this is the row of the interrupted invocation.
	// When RerunLast is false, this is the row after the last completed one.
	RowIndex int

	// Phase is the phase of the row identified by RowIndex.
	Phase string

	// Stage is the iteration value for the row. For an interrupted row it is the
	// recorded value of the log entry as read; for an advanced row it is the
	// OAF 3.0 value (domain.FormatIterationValue, e.g. "1.Test"). It is ""
	// outside a block. Consumers should rely on Iteration.
	Stage string

	// Block is the block of the row at RowIndex; "" outside every block, on a
	// fresh start and when the run is complete.
	Block BlockName

	// Iteration is the ID of the iteration the position is in, verbatim as the
	// schedule spells it (interrupted row: as recorded); "" when Block is "".
	Iteration IterationID

	// GroupIndex is the index into AdmittedWorkflow.Groups for the row's
	// execution group (0 or 1), or -1 when the row is outside the staged phase.
	GroupIndex int

	// Seq is the last allocated global sequence number; the next invocation
	// uses Seq + 1. Derived from the artifact's (reconciled) global_sequence field.
	Seq int

	// RerunLast is true when the last logged invocation did not complete
	// (detected by comparing the execution log's last entry against the
	// current_state -- a mismatch indicates an interruption).
	// The session must re-dispatch the same row rather than advancing.
	RerunLast bool
}
