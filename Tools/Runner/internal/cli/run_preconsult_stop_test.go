package cli_test

// Tests for how the CLI presents a run the orchestrator stopped at
// pre-consultation: the same exit code and stderr text as a routing stop, and
// not the start-failed resume hint.

import (
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

// TestRunPreConsultationStop_ExitsWithStoppedByConsultantCodeAndPrintsReason
// verifies the exit code and that stderr carries the reason, not the
// start-failed hint.
func TestRunPreConsultationStop_ExitsWithStoppedByConsultantCodeAndPrintsReason(t *testing.T) {
	const reason = "Uncommitted changes on the run branch; resolve them first"
	sess := &scriptedSession{
		outcome: domain.RunOutcome{
			Status:              domain.RunStoppedByConsultant,
			Message:             "consultant stop: " + reason,
			StopReason:          reason,
			PreConsultationStop: true,
		},
	}

	code, out, errOut := runCLI(t, newStage7BaseArgs(), sess)

	if code != cli.ExitStoppedByConsultant {
		t.Errorf("exit code = %d, want ExitStoppedByConsultant (%d)", code, cli.ExitStoppedByConsultant)
	}
	if code == cli.ExitStartFailed {
		t.Errorf("exit code must not be ExitStartFailed (%d)", cli.ExitStartFailed)
	}
	if !strings.Contains(errOut, reason) {
		t.Errorf("stderr %q does not contain the stop reason %q", errOut, reason)
	}
	if strings.Contains(strings.ToLower(errOut), "retries the failed step") {
		t.Errorf("stderr %q carries the start-failed resume hint, want only the stop reason", errOut)
	}
	if strings.Contains(out, reason) {
		t.Errorf("stdout %q must stay machine-readable and not carry the stop reason", out)
	}
}

// TestRunPreConsultationStop_StderrIdenticalToRoutingStop verifies that the
// marker has no influence on what the CLI prints or returns.
func TestRunPreConsultationStop_StderrIdenticalToRoutingStop(t *testing.T) {
	const reason = "stop here"
	routing := &scriptedSession{outcome: domain.RunOutcome{
		Status: domain.RunStoppedByConsultant, Message: "consultant stop: " + reason, StopReason: reason,
	}}
	preConsult := &scriptedSession{outcome: domain.RunOutcome{
		Status: domain.RunStoppedByConsultant, Message: "consultant stop: " + reason, StopReason: reason,
		PreConsultationStop: true,
	}}

	routingCode, _, routingErr := runCLI(t, newStage7BaseArgs(), routing)
	preCode, _, preErr := runCLI(t, newStage7BaseArgs(), preConsult)

	if routingCode != preCode {
		t.Errorf("exit codes differ: routing %d, pre-consultation %d", routingCode, preCode)
	}
	if routingErr != preErr {
		t.Errorf("stderr differs: routing %q, pre-consultation %q", routingErr, preErr)
	}
}

// TestRunPreConsultationStop_DoesNotWriteCompletedMarker verifies that the
// stopped run stays resumable.
func TestRunPreConsultationStop_DoesNotWriteCompletedMarker(t *testing.T) {
	spy := &spyStore{}
	sess := &scriptedSession{outcome: domain.RunOutcome{
		Status: domain.RunStoppedByConsultant, StopReason: "stop", PreConsultationStop: true,
	}}

	runCLIWithStore(t, newStage7BaseArgs(), spy, sess)

	if len(spy.setCalls) != 0 {
		t.Errorf("store.SetPhase called %d time(s), want 0 for a pre-consultation stop", len(spy.setCalls))
	}
}
