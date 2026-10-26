package session_test

// Test of the triggers field of the infrastructure failure event when one
// dispatch matches several declared triggers.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_InfraDispatch_Failure_MultipleMatchedTriggersAreListedInDeclarationOrder(t *testing.T) {
	rig := newInfraReportRig(t, "continue-infra-two-triggers-orch.md", true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "checkpoint taken", ""))
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "status text", "scripted reason"))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	// PHASE_END is declared before INVOCATION_INTERVAL; alphabetical order would differ.
	requireFields(t, failed[0], map[string]string{"triggers": "PHASE_END,INVOCATION_INTERVAL"})
}
