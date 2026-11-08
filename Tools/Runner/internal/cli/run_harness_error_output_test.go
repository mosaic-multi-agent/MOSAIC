package cli_test

// Tests of how the CLI frontend prints the Runner-recorded failure statuses
// (HARNESS_ERROR, CONFIG_ERROR): as their own status in progress notices and in
// the end-of-run list of infrastructure failures, never as BLOCKED.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

func TestInteraction_Notify_HarnessErrorStepNotice_PrintsHarnessErrorStatus(t *testing.T) {
	var out bytes.Buffer
	in := cli.NewInteraction(&out)

	in.Notify(context.Background(), interaction.Notice{
		Level: interaction.NoticeInfo, Title: "planner#1",
		Message: `phase=PLANNING iteration="" status=HARNESS_ERROR`,
	})

	got := out.String()
	if !strings.Contains(got, "planner#1") || !strings.Contains(got, "status=HARNESS_ERROR") {
		t.Errorf("notice line %q lacks the agent or status=HARNESS_ERROR", got)
	}
	if strings.Contains(got, "BLOCKED") {
		t.Errorf("notice line %q shows the harness error as BLOCKED", got)
	}
}

func runnerRecordedInfraFailures() []domain.InfraFailure {
	return []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusHARNESS_ERROR, Summary: "timeout: git stalled"},
		{AgentInstance: "checkpoint-manager-git#5", Status: domain.StatusCONFIG_ERROR, Summary: "agent definition not resolved: missing"},
	}
}

func TestRun_EndOfRun_ListsRunnerRecordedInfrastructureFailuresWithTheirStatus(t *testing.T) {
	for _, status := range []domain.RunStatus{domain.RunCompleted, domain.RunStopped, domain.RunFailed, domain.RunStartFailed} {
		t.Run(string(status), func(t *testing.T) {
			sess := &scriptedSession{outcome: domain.RunOutcome{
				Status: status, Message: "outcome message", InfraFailures: runnerRecordedInfraFailures(),
			}}

			_, out, errOut := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

			pos := 0
			for _, want := range []string{
				"Infrastructure failures in this run (2):",
				"  commit-manager-git#2  HARNESS_ERROR  timeout: git stalled",
				"  checkpoint-manager-git#5  CONFIG_ERROR  agent definition not resolved: missing",
			} {
				i := strings.Index(errOut[pos:], want)
				if i < 0 {
					t.Errorf("stderr lacks %q (in order) in:\n%s", want, errOut)
					return
				}
				pos += i + len(want)
			}
			if strings.Contains(out+errOut, "BLOCKED") {
				t.Errorf("output shows a Runner-recorded failure as BLOCKED:\nstdout: %q\nstderr: %q", out, errOut)
			}
		})
	}
}

func TestRun_EndOfRun_RunnerRecordedInfrastructureFailure_DoesNotChangeTheExitCode(t *testing.T) {
	sess := &scriptedSession{outcome: domain.RunOutcome{
		Status: domain.RunCompleted, InfraFailures: runnerRecordedInfraFailures(),
	}}

	code, _, _ := runCLIWithStore(t, newStage7BaseArgs(), &spyStore{}, sess)

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)", code, cli.ExitSuccess)
	}
}
