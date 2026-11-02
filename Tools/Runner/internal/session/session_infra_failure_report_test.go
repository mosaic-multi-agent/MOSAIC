package session_test

// Tests of how trigger-dispatched infrastructure agents are reported in the
// Runner log: every completed step writes the step-done event, every
// non-SUCCESS outcome also writes one dedicated failure event, and the
// declared on_failure policy is applied exactly as before.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

const firstCheckpointInstance = infraCheckpointAgent + "#2"

func TestSession_InfraDispatch_Success_WritesStepDoneWithClassAndNoFailureEvent(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "checkpoint taken", ""))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "checkpoint taken", ""))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	done := rig.logger.stepDoneOf(firstCheckpointInstance)
	if len(done) != 1 {
		t.Fatalf("want exactly 1 %s event for %s, got %d", domain.EventSessionStepDone, firstCheckpointInstance, len(done))
	}
	requireFields(t, done[0], map[string]string{"status": "SUCCESS", "error_code": "", "class": "checkpoint"})
	if n := len(rig.logger.entriesOf(domain.EventSessionInfraFailed)); n != 0 {
		t.Errorf("want no %s event for successful infrastructure steps, got %d", domain.EventSessionInfraFailed, n)
	}
}

func TestSession_InfraDispatch_Failure_WritesStepDoneAndOneFailureEventNamingEverything(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "status text", "scripted reason"))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "checkpoint taken", ""))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event (the second step succeeded), got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{
		"agent": firstCheckpointInstance, "class": "checkpoint", "status": "BLOCKED",
		"error_code": "E101", "policy": "continue",
	})
	if triggers, _ := failed[0].field("triggers"); !strings.Contains(triggers, "INVOCATION_INTERVAL") {
		t.Errorf("triggers field = %q, want it to name INVOCATION_INTERVAL", triggers)
	}
	if !strings.Contains(failed[0].Message, "scripted reason") {
		t.Errorf("event message = %q, want the response's error reason", failed[0].Message)
	}
	done := rig.logger.stepDoneOf(firstCheckpointInstance)
	if len(done) != 1 {
		t.Fatalf("want the failed step to also write %s once, got %d", domain.EventSessionStepDone, len(done))
	}
	requireFields(t, done[0], map[string]string{"status": "BLOCKED", "error_code": "E101", "class": "checkpoint"})
}

func TestSession_InfraDispatch_FailureWithoutErrorReason_MessageIsTheStatusMessage(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusNEEDS_CLARIFICATION, domain.ErrorNone, "which branch should be used", ""))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "checkpoint taken", ""))

	rig.run(t)

	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event for a non-BLOCKED non-SUCCESS status, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"status": "NEEDS_CLARIFICATION", "error_code": ""})
	if !strings.Contains(failed[0].Message, "which branch should be used") {
		t.Errorf("event message = %q, want the status message when no error reason is given", failed[0].Message)
	}
}

func TestSession_InfraDispatch_HaltPolicyFailure_NamesHaltAndStopsTheRun(t *testing.T) {
	rig := newInfraReportRig(t, infraHaltFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorPERMISSION_DENIED, "denied", "no write access"))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunStopped)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"policy": "halt", "error_code": "E502", "agent": firstCheckpointInstance})
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusBLOCKED, domain.ErrorPERMISSION_DENIED)
}

func TestSession_InfraDispatch_GracefulStopAfterFailedStep_OutcomeStillListsTheFailure(t *testing.T) {
	var rig *infraReportRig
	stopAfterCheckpoint := func() bool {
		return infraFilterInvocationCountFor(rig.adapter.Invocations(), infraCheckpointAgent) >= 1
	}
	rig = newInfraReportRig(t, infraContinueFixture, true, stopAfterCheckpoint)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "status text", "reason"))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunStopped)
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND)
}

// requireListedFailure asserts the outcome lists exactly one failed
// infrastructure step with the given agent, status and error code.
func requireListedFailure(t *testing.T, got domain.RunOutcome, agent string, status domain.StatusCode, code domain.ErrorCode) {
	t.Helper()
	if len(got.InfraFailures) != 1 {
		t.Fatalf("outcome InfraFailures = %+v, want exactly the one failed step", got.InfraFailures)
	}
	f := got.InfraFailures[0]
	if f.AgentInstance != agent || f.Status != status || f.ErrorCode != code {
		t.Errorf("listed failure = %+v, want %s %s %s", f, agent, status, code)
	}
}

func TestSession_InfraDispatch_PolicyIsUnchangedForEveryStatusAndErrorCode(t *testing.T) {
	replies := map[string]harness.ScriptedEntry{
		"BLOCKED E101":      checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "m", "r"),
		"BLOCKED E501":      checkpointReply(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "m", "r"),
		"BLOCKED E502":      checkpointReply(domain.StatusBLOCKED, domain.ErrorPERMISSION_DENIED, "m", "r"),
		"BLOCKED no code":   checkpointReply(domain.StatusBLOCKED, domain.ErrorNone, "m", "r"),
		"NEEDS_CLARIFY":     checkpointReply(domain.StatusNEEDS_CLARIFICATION, domain.ErrorNone, "m", ""),
		"CAPABILITY_EXCEED": checkpointReply(domain.StatusCAPABILITY_EXCEEDED, domain.ErrorNone, "m", ""),
		"PARTIALLY_DONE":    checkpointReply(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "m", ""),
	}
	for name, reply := range replies {
		t.Run("continue/"+name, func(t *testing.T) {
			rig := newInfraReportRig(t, infraContinueFixture, true, nil)
			rig.queueWorkflow()
			rig.queueCheckpoint(reply)
			rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

			got := rig.run(t)

			requireRunStatus(t, got, nil, domain.RunCompleted)
			if n := len(rig.logger.entriesOf(domain.EventSessionInfraFailed)); n != 1 {
				t.Errorf("want 1 %s event, got %d", domain.EventSessionInfraFailed, n)
			}
		})
		t.Run("halt/"+name, func(t *testing.T) {
			rig := newInfraReportRig(t, infraHaltFixture, true, nil)
			rig.queueWorkflow()
			rig.queueCheckpoint(reply)

			got := rig.run(t)

			requireRunStatus(t, got, nil, domain.RunStopped)
			if n := len(rig.logger.entriesOf(domain.EventSessionInfraFailed)); n != 1 {
				t.Errorf("want 1 %s event, got %d", domain.EventSessionInfraFailed, n)
			}
			if len(got.InfraFailures) != 1 {
				t.Errorf("outcome InfraFailures = %+v, want the one failed step", got.InfraFailures)
			}
		})
	}
}

func TestSession_InfraDispatch_HarnessError_WritesFailureEventWithE501AndTheErrorText(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(harness.ScriptedEntry{Err: errors.New("transport broke")})
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event for a harness-level error, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"status": "BLOCKED", "error_code": "E501", "policy": "continue"})
	if !strings.Contains(failed[0].Message, "transport broke") {
		t.Errorf("event message = %q, want the harness error text", failed[0].Message)
	}
	if n := len(rig.logger.stepDoneOf(firstCheckpointInstance)); n != 1 {
		t.Errorf("want the recorded step to write %s once, got %d", domain.EventSessionStepDone, n)
	}
}

func TestSession_InfraDispatch_UnresolvableDefinition_WritesFailureEventAndNeverDispatches(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, false, nil)
	rig.queueWorkflow()

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) == 0 {
		t.Fatalf("want a %s event for the definition that cannot be resolved, got none", domain.EventSessionInfraFailed)
	}
	requireFields(t, failed[0], map[string]string{"status": "BLOCKED", "error_code": "E501", "agent": firstCheckpointInstance})
	if n := infraFilterInvocationCountFor(rig.adapter.Invocations(), infraCheckpointAgent); n != 0 {
		t.Errorf("want no dispatch of an agent whose definition cannot be resolved, got %d", n)
	}
}

func TestSession_InfraDispatch_GracefulStopBeforeDispatch_WritesNeitherEvent(t *testing.T) {
	var rig *infraReportRig
	rig = newInfraReportRig(t, infraContinueFixture, true, func() bool { return len(rig.store.Applied) >= 1 })
	rig.queueWorkflow()

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunStopped)
	if n := len(rig.logger.stepDoneOf(firstCheckpointInstance)); n != 0 {
		t.Errorf("want no %s event for a step that was not dispatched, got %d", domain.EventSessionStepDone, n)
	}
	if n := len(rig.logger.entriesOf(domain.EventSessionInfraFailed)); n != 0 {
		t.Errorf("want no %s event for a step that was not dispatched, got %d", domain.EventSessionInfraFailed, n)
	}
}

func TestSession_InfraDispatch_FailureIsOnTheOutcomeFailureList(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "status text", "reason"))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	if len(got.InfraFailures) != 1 {
		t.Fatalf("outcome InfraFailures = %+v, want exactly the one failed step", got.InfraFailures)
	}
	f := got.InfraFailures[0]
	if f.AgentInstance != firstCheckpointInstance || f.Status != domain.StatusBLOCKED || f.ErrorCode != domain.ErrorINPUT_NOT_FOUND {
		t.Errorf("listed failure = %+v, want %s BLOCKED E101", f, firstCheckpointInstance)
	}
}

func TestSession_InfraDispatch_RunWithoutInfraFailures_OutcomeListsNone(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	if len(got.InfraFailures) != 0 {
		t.Errorf("outcome InfraFailures = %+v, want none", got.InfraFailures)
	}
}
