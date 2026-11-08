package session_test

// Tests of infrastructure agents that fail without the subagent returning
// anything: an invocation that ends in a harness error is a HARNESS_ERROR row
// naming the failure kind, an agent definition that cannot be resolved is a
// CONFIG_ERROR row. Neither looks like a subagent-returned BLOCKED, both still
// count as failures for on_failure and the failure report, and a BLOCKED the
// infrastructure agent really returned stays exactly as it was.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// checkpointRowOf returns the single recorded row of the checkpoint agent.
func checkpointRowOf(t *testing.T, rig *infraReportRig) domain.CompletedStep {
	t.Helper()
	rows := infraRowsOf(rig.store, infraCheckpointAgent)
	if len(rows) == 0 {
		t.Fatalf("want a recorded row of %s, got none", infraCheckpointAgent)
	}
	return rows[0]
}

func TestSession_InfraDispatch_InvocationTimeout_RowIsHarnessErrorNamingTheKind(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(harness.ScriptedEntry{Err: fmt.Errorf("no output for 10m: %w", commonharness.ErrTimeout)})
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	rig.run(t)

	row := checkpointRowOf(t, rig)
	if row.Status != domain.StatusHARNESS_ERROR {
		t.Errorf("want row status HARNESS_ERROR, got %q", row.Status)
	}
	if row.ErrorCode != domain.ErrorNone {
		t.Errorf("want no error code on the row, got %q", row.ErrorCode)
	}
	if !row.IsInfrastructure {
		t.Error("want the row recorded as an infrastructure row")
	}
	requireSummaryNamesKind(t, row.Summary, "timeout", "no output for 10m")
}

func TestSession_InfraDispatch_InvocationFailureKinds_AreNamedInTheSummary(t *testing.T) {
	cases := []struct {
		name string
		err  error
		kind string
	}{
		{"process failure", fmt.Errorf("exit status 2: %w", commonharness.ErrNonZeroExit), "process-failure"},
		{"empty response", fmt.Errorf("nothing came back: %w", commonharness.ErrEmptyResponse), "empty-response"},
		{"unparseable response", fmt.Errorf("agent said SECRET-TEXT: %w", commonharness.ErrMalformedJSON), "unparseable-response"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rig := newInfraReportRig(t, infraContinueFixture, true, nil)
			rig.queueWorkflow()
			rig.queueCheckpoint(harness.ScriptedEntry{Err: c.err})
			rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

			rig.run(t)

			row := checkpointRowOf(t, rig)
			if row.Status != domain.StatusHARNESS_ERROR {
				t.Fatalf("want row status HARNESS_ERROR, got %q", row.Status)
			}
			requireSummaryNamesKind(t, row.Summary, c.kind, "")
			if c.kind == "unparseable-response" && strings.Contains(row.Summary, "SECRET-TEXT") {
				t.Errorf("want the agent's output kept out of the Summary, got %q", row.Summary)
			}
		})
	}
}

func TestSession_InfraDispatch_HarnessErrorUnderHalt_StopsTheRunAndListsTheFailure(t *testing.T) {
	rig := newInfraReportRig(t, infraHaltFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(harness.ScriptedEntry{Err: errors.New("transport broke")})

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunStopped)
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusHARNESS_ERROR, domain.ErrorNone)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"policy": "halt", "status": "HARNESS_ERROR", "error_code": ""})
}

func TestSession_InfraDispatch_HarnessErrorUnderContinue_RunProceedsAndStepDoneNamesTheStatus(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(harness.ScriptedEntry{Err: errors.New("transport broke")})
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusHARNESS_ERROR, domain.ErrorNone)
	done := rig.logger.stepDoneOf(firstCheckpointInstance)
	if len(done) != 1 {
		t.Fatalf("want the recorded step to write %s once, got %d", domain.EventSessionStepDone, len(done))
	}
	requireFields(t, done[0], map[string]string{"status": "HARNESS_ERROR", "error_code": ""})
}

func TestSession_InfraDispatch_HarnessError_DoesNotMoveTheWorkflowPosition(t *testing.T) {
	rig := newInfraReportRig(t, infraHaltFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(harness.ScriptedEntry{Err: errors.New("transport broke")})

	rig.run(t)

	if last := rig.store.state.CurrentState.LastAgent; last == firstCheckpointInstance {
		t.Errorf("want the failed infrastructure attempt never to become the workflow position, got last_agent=%q", last)
	}
}

func TestSession_InfraDispatch_UnresolvableDefinition_RowIsConfigErrorNamingTheDefinition(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, false, nil)
	rig.queueWorkflow()

	rig.run(t)

	row := checkpointRowOf(t, rig)
	if row.Status != domain.StatusCONFIG_ERROR {
		t.Errorf("want row status CONFIG_ERROR, got %q", row.Status)
	}
	if row.ErrorCode != domain.ErrorNone {
		t.Errorf("want no error code on the row, got %q", row.ErrorCode)
	}
	if !row.IsInfrastructure {
		t.Error("want the row recorded as an infrastructure row")
	}
	if !strings.Contains(row.Summary, "agent definition not resolved") {
		t.Errorf("want the Summary to name the unresolved definition, got %q", row.Summary)
	}
}

func TestSession_InfraDispatch_UnresolvableDefinitionUnderHalt_StopsTheRunAndListsTheFailure(t *testing.T) {
	rig := newInfraReportRig(t, infraHaltFixture, false, nil)
	rig.queueWorkflow()

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunStopped)
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusCONFIG_ERROR, domain.ErrorNone)
	failed := rig.logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"policy": "halt", "status": "CONFIG_ERROR", "error_code": ""})
}

func TestSession_InfraDispatch_UnresolvableDefinitionUnderContinue_RunProceeds(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, false, nil)
	rig.queueWorkflow()

	got := rig.run(t)

	requireRunStatus(t, got, nil, domain.RunCompleted)
	if len(got.InfraFailures) == 0 || got.InfraFailures[0].Status != domain.StatusCONFIG_ERROR {
		t.Errorf("want the outcome to list the CONFIG_ERROR failure, got %+v", got.InfraFailures)
	}
}

func TestSession_InfraDispatch_SubagentReturnedBlockedE501_StaysBlockedE501(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	rig.queueWorkflow()
	rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "git missing", "no git binary"))
	rig.queueCheckpoint(checkpointReply(domain.StatusSUCCESS, domain.ErrorNone, "ok", ""))

	got := rig.run(t)

	row := checkpointRowOf(t, rig)
	if row.Status != domain.StatusBLOCKED || row.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("want the subagent-returned BLOCKED/E501 recorded as such, got %s/%s", row.Status, row.ErrorCode)
	}
	requireListedFailure(t, got, firstCheckpointInstance, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE)
}
