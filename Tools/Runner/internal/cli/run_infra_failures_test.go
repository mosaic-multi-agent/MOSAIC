package cli_test

// Tests of the end-of-run report of infrastructure-agent failures: every
// failure of the run is listed on stderr after the outcome, whatever the
// outcome, and nothing is added for a run without such failures.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

func twoInfraFailures() []domain.InfraFailure {
	return []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusBLOCKED, ErrorCode: domain.ErrorINPUT_NOT_FOUND, Summary: "nothing to commit against"},
		{AgentInstance: "checkpoint-manager-git#5", Status: domain.StatusPARTIALLY_DONE, ErrorCode: domain.ErrorNone, Summary: "half a checkpoint"},
	}
}

// requireFailureReport asserts that errOut carries the header and both
// failure lines, in order, and that stdout carries none of it.
func requireFailureReport(t *testing.T, out, errOut string) {
	t.Helper()
	const header = "Infrastructure failures in this run (2):"
	lines := []string{
		header,
		"  commit-manager-git#2  BLOCKED E101  nothing to commit against",
		"  checkpoint-manager-git#5  PARTIALLY_DONE  half a checkpoint",
	}
	pos := 0
	for _, want := range lines {
		i := strings.Index(errOut[pos:], want)
		if i < 0 {
			t.Errorf("stderr lacks %q (in order) in:\n%s", want, errOut)
			return
		}
		pos += i + len(want)
	}
	if strings.Contains(out, "Infrastructure failures") {
		t.Errorf("stdout %q must not carry the failure report", out)
	}
}

func TestRun_EndOfRun_ListsEveryInfrastructureFailureForEachOutcomeStatus(t *testing.T) {
	statuses := []domain.RunStatus{
		domain.RunCompleted, domain.RunStopped, domain.RunDeviationUnresolved, domain.RunRefused,
		domain.RunFailed, domain.RunStoppedByConsultant, domain.RunStartFailed,
	}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			sess := &scriptedSession{outcome: domain.RunOutcome{
				Status: status, Message: "outcome message", StopReason: "consultant stop reason",
				InfraFailures: twoInfraFailures(),
			}}

			_, out, errOut := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

			requireFailureReport(t, out, errOut)
		})
	}
}

func TestRun_EndOfRun_ListsInfrastructureFailuresOfAnOutcomeReturnedWithAnError(t *testing.T) {
	sess := &scriptedSession{
		outcome: domain.RunOutcome{Status: domain.RunFailed, InfraFailures: twoInfraFailures()},
		err:     errors.New("artifact write failed"),
	}

	code, out, errOut := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want ExitFailure (%d)", code, cli.ExitFailure)
	}
	if !strings.Contains(errOut, "artifact write failed") {
		t.Errorf("stderr %q lacks the session error", errOut)
	}
	requireFailureReport(t, out, errOut)
}

func TestRun_EndOfRun_FailureListDoesNotChangeTheExitCode(t *testing.T) {
	sess := &scriptedSession{outcome: domain.RunOutcome{
		Status: domain.RunCompleted, InfraFailures: twoInfraFailures(),
	}}

	code, _, _ := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d): infrastructure failures do not change the exit code", code, cli.ExitSuccess)
	}
}

func TestRun_EndOfRun_NoInfrastructureFailures_PrintsNoFailureReport(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunCompleted, domain.RunStopped, domain.RunFailed} {
		t.Run(string(status), func(t *testing.T) {
			sess := &scriptedSession{outcome: domain.RunOutcome{Status: status, Message: "outcome message"}}

			_, out, errOut := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

			if strings.Contains(out+errOut, "Infrastructure failures") {
				t.Errorf("output mentions infrastructure failures for a run without any:\nstdout: %q\nstderr: %q", out, errOut)
			}
		})
	}
}

func TestRun_EndOfRun_SingleFailureHeaderCountsOne(t *testing.T) {
	sess := &scriptedSession{outcome: domain.RunOutcome{
		Status:        domain.RunCompleted,
		InfraFailures: twoInfraFailures()[:1],
	}}

	_, _, errOut := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

	if !strings.Contains(errOut, "Infrastructure failures in this run (1):") {
		t.Errorf("stderr %q lacks the header with the count 1", errOut)
	}
	if strings.Contains(errOut, "checkpoint-manager-git#5") {
		t.Errorf("stderr %q lists a failure that is not in the run", errOut)
	}
}
