package domain

// Integrity finding kinds: logs that are internally consistent but wrong. All are
// warning severity and none is a data-loss kind (they never make a report
// incomplete).
const (
	// FindingInstanceTypeMismatch reports an agent instance whose id has the
	// {type}#{N} form but whose agent type differs from {type}.
	FindingInstanceTypeMismatch FindingKind = "instance_type_mismatch"

	// FindingUnresolvedInvocation reports an agent instance whose id lacks the
	// {type}#{N} form.
	FindingUnresolvedInvocation FindingKind = "unresolved_invocation"

	// FindingOverlappingInvocationStart reports an invocation_start that arrived
	// in a stream while the stream's previous start had no invocation_end yet.
	FindingOverlappingInvocationStart FindingKind = "overlapping_invocation_start"

	// FindingDispatchWithoutInvocation reports an orchestrator subagent dispatch
	// whose agent instance id has no invocation in the run.
	FindingDispatchWithoutInvocation FindingKind = "dispatch_without_invocation"

	// FindingTranscriptCaptureGap reports a difference between an invocation's
	// raw transcript and its logged usage records.
	FindingTranscriptCaptureGap FindingKind = "transcript_capture_gap"

	// FindingInstancePromptMismatch reports an agent whose own first prompt
	// names a different agent instance id than its folder.
	FindingInstancePromptMismatch FindingKind = "instance_prompt_mismatch"
)
