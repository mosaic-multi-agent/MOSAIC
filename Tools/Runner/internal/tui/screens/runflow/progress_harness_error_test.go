package runflow

// Tests of how the progress screen and the done screen show the Runner-recorded
// failure statuses (HARNESS_ERROR, CONFIG_ERROR): as their own status text, in
// a style of their own, different from a subagent-returned BLOCKED and from the
// styles of the other statuses.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

var runnerRecordedStatuses = []string{"HARNESS_ERROR", "CONFIG_ERROR"}

func TestProgressRow_RunnerRecordedStatus_HasItsOwnStyle(t *testing.T) {
	styles := progressStyles()
	for _, status := range runnerRecordedStatuses {
		t.Run(status, func(t *testing.T) {
			label := completedLabel("agent#1", status)
			got := extractFirstRowLine(styles, status)

			if !strings.Contains(got, label) {
				t.Fatalf("row %q lacks the label %q", got, label)
			}
			for name, other := range map[string]string{
				"BLOCKED (Error)":          "  " + styles.Error.Render(label),
				"PARTIALLY_DONE (Warning)": "  " + styles.Warning.Render(label),
				"SUCCESS (Success)":        "  " + styles.Success.Render(label),
			} {
				if got == other {
					t.Errorf("%s row renders like %s: %q", status, name, got)
				}
			}
		})
	}
}

func TestProgressRow_RunnerRecordedStatus_DiffersFromBlockedWhateverTheLabel(t *testing.T) {
	styles := progressStyles()
	blocked := extractFirstRowLine(styles, "BLOCKED")
	for _, status := range runnerRecordedStatuses {
		got := strings.Replace(extractFirstRowLine(styles, status), status, "BLOCKED", 1)
		if got == blocked {
			t.Errorf("%s row has the style of a BLOCKED row: %q", status, got)
		}
	}
}

func TestProgressRow_RunnerRecordedStatusInHistory_RendersLikeACompletedRow(t *testing.T) {
	styles := progressStyles()
	for _, status := range runnerRecordedStatuses {
		t.Run(status, func(t *testing.T) {
			s := NewProgressScreen(80, 24, styles)
			s.SetHistory([]ProgressRow{{AgentInstance: "agent#1", Phase: "run", Status: status}})

			live := extractFirstRowLine(styles, status)

			if got := firstRowLine(s); got != live {
				t.Errorf("history row = %q, want the row of a completed %s step %q", got, status, live)
			}
		})
	}
}

func TestProgressScreen_FailedInfrastructureRow_RunnerRecordedStatus_ShowsStatusAndReason(t *testing.T) {
	for _, status := range runnerRecordedStatuses {
		t.Run(status, func(t *testing.T) {
			view := progressViewOf(120, infraRow(status, "", "timeout: git stalled"))

			want := "commit-manager-git#2  " + status + "  infrastructure failed: timeout: git stalled"
			if !strings.Contains(view, want) {
				t.Errorf("View() lacks %q:\n%s", want, view)
			}
		})
	}
}

func TestProgressScreen_FailedInfrastructureRow_RunnerRecordedStatus_HasItsOwnStyle(t *testing.T) {
	styles := progressStyles()
	for _, status := range runnerRecordedStatuses {
		t.Run(status, func(t *testing.T) {
			row := infraRow(status, "", "timeout: git stalled")
			view := progressViewOf(120, row)
			label := infraFailureLabel(row)

			for name, st := range map[string]string{
				"Error":   styles.Error.Render(label),
				"Warning": styles.Warning.Render(label),
				"Success": styles.Success.Render(label),
			} {
				if strings.Contains(view, st) {
					t.Errorf("%s infrastructure row is rendered in the %s style:\n%s", status, name, view)
				}
			}
		})
	}
}

func TestProgressScreen_FailedInfrastructureRow_BlockedKeepsItsStyle(t *testing.T) {
	styles := progressStyles()
	row := infraRow("BLOCKED", "E101", "nothing to commit against")

	view := progressViewOf(120, row)

	if want := styles.Error.Render(infraFailureLabel(row)); !strings.Contains(view, want) {
		t.Errorf("BLOCKED infrastructure row is not in the Error style:\n%s", view)
	}
}

func TestDoneScreen_View_RunnerRecordedInfrastructureFailures_AreListedWithTheirStatus(t *testing.T) {
	outcome := domain.RunOutcome{Status: domain.RunFailed, Message: "run failed", InfraFailures: []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusHARNESS_ERROR, Summary: "timeout: git stalled"},
		{AgentInstance: "checkpoint-manager-git#3", Status: domain.StatusCONFIG_ERROR, Summary: "agent definition not resolved: missing"},
	}}
	s := NewDoneScreen(outcome, "", 120, 40, progressStyles())

	view := s.View()

	for _, want := range []string{
		"Infrastructure failures in this run (2):",
		"  commit-manager-git#2  HARNESS_ERROR  timeout: git stalled",
		"  checkpoint-manager-git#3  CONFIG_ERROR  agent definition not resolved: missing",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() lacks %q:\n%s", want, view)
		}
	}
}
