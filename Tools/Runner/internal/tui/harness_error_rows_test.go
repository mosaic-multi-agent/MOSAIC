package tui

// Tests of how the model shows the Runner-recorded failure statuses
// (HARNESS_ERROR, CONFIG_ERROR) on the progress screen: live rows equal the
// rows rebuilt from the Execution Log, the rows carry the status and the
// failure-kind summary, and the status is drawn differently from BLOCKED.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"mosaic-run/internal/domain"
)

// forceANSIColors makes lipgloss emit color escapes for the test, so that the
// style of a rendered row can be compared.
func forceANSIColors(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func harnessErrorEntry(agent string, status domain.StatusCode, summary string) domain.ExecutionLogEntry {
	e := entry(agent, "EXECUTION", "1.Implementation", status)
	e.Summary = summary
	return e
}

// harnessErrorScript records a harness error, its re-dispatch succeeding, and
// a subagent-returned BLOCKED.
func harnessErrorScript() *liveScript {
	s := &liveScript{}
	s.running("planner#1", "EXECUTION", "1.Implementation").
		record(harnessErrorEntry("planner#1", domain.StatusHARNESS_ERROR, "timeout: planner stalled")).
		running("planner#2", "EXECUTION", "1.Implementation").
		record(harnessErrorEntry("planner#2", domain.StatusSUCCESS, "planned")).
		running("builder#3", "EXECUTION", "2.Test").
		record(harnessErrorEntry("builder#3", domain.StatusBLOCKED, "builder blocked")).
		record(harnessErrorEntry("builder#4", domain.StatusCONFIG_ERROR, "agent definition not resolved: missing"))
	return s
}

func TestRowParity_RunnerRecordedStatuses_LiveRowsEqualRebuiltRows(t *testing.T) {
	s := harnessErrorScript()
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestHistoryRows_RunnerRecordedStatus_RowCarriesStatusSummaryAndNoErrorCode(t *testing.T) {
	rows := historyRows(harnessErrorScript().log)

	if rows[0].Status != "HARNESS_ERROR" || rows[0].Reason != "timeout: planner stalled" || rows[0].ErrorCode != "" {
		t.Errorf("harness-error row = %+v, want status HARNESS_ERROR, the failure-kind summary and no error code", rows[0])
	}
	if rows[3].Status != "CONFIG_ERROR" || rows[3].ErrorCode != "" {
		t.Errorf("config-error row = %+v, want status CONFIG_ERROR and no error code", rows[3])
	}
}

// lineOf returns the line of view that contains substr.
func lineOf(t *testing.T, view, substr string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no line with %q in view:\n%s", substr, view)
	return ""
}

// requireStatusStyleDiffersFromBlocked asserts that the row of agent a
// (HARNESS_ERROR or CONFIG_ERROR) is drawn in another style than the BLOCKED
// row of agent builder#3.
func requireStatusStyleDiffersFromBlocked(t *testing.T, view, agent, status string) {
	t.Helper()
	got := strings.Replace(lineOf(t, view, agent+"  "+status), agent+"  "+status, "builder#3  BLOCKED", 1)
	blocked := lineOf(t, view, "builder#3  BLOCKED")
	if got == blocked {
		t.Errorf("%s row of %s has the style of a BLOCKED row: %q", status, agent, got)
	}
}

func TestProgressView_RunnerRecordedStatus_IsDrawnDifferentlyFromBlockedLiveAndRebuilt(t *testing.T) {
	forceANSIColors(t)
	script := harnessErrorScript()
	m := newParityModel()
	script.feed(m)
	m.height, m.width = 40, 120

	live := m.progressScreen.View()
	requireStatusStyleDiffersFromBlocked(t, live, "planner#1", "HARNESS_ERROR")
	requireStatusStyleDiffersFromBlocked(t, live, "builder#4", "CONFIG_ERROR")

	for name, view := range map[string]string{"live": live, "rebuilt": rebuiltView(t, script)} {
		if !strings.Contains(view, "planner#1  HARNESS_ERROR") {
			t.Errorf("%s view lacks the HARNESS_ERROR status text:\n%s", name, view)
		}
	}
	if rebuilt := rebuiltView(t, script); lineOf(t, rebuilt, "planner#1  HARNESS") != lineOf(t, live, "planner#1  HARNESS") {
		t.Errorf("rebuilt HARNESS_ERROR row = %q, want the live row %q", lineOf(t, rebuilt, "planner#1  HARNESS"), lineOf(t, live, "planner#1  HARNESS"))
	}
}

// rebuiltView is the progress view rebuilt from the script's Execution Log.
func rebuiltView(t *testing.T, s *liveScript) string {
	t.Helper()
	m := newInfraRowsModel(t, s.log)
	m.selections.isNewRun = false
	m.height, m.width = 40, 120
	return m.newProgressScreenWithHistory(stylesFromTheme(m.theme)).View()
}

// infraHarnessErrorScript records a failed commit manager step per
// Runner-recorded status and a workflow step.
func infraHarnessErrorScript() *liveScript {
	s := &liveScript{}
	s.running("commit-manager-git#1", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#1", domain.StatusHARNESS_ERROR, domain.ErrorNone, "timeout: commit stalled")).
		running("commit-manager-git#2", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#2", domain.StatusCONFIG_ERROR, domain.ErrorNone, "agent definition not resolved: missing")).
		running("planner#3", "EXECUTION", "1.Implementation").
		record(infraEntry("planner#3", domain.StatusSUCCESS, domain.ErrorNone, "planned"))
	return s
}

func TestInfraFailureRows_RunnerRecordedStatuses_LiveRowsEqualRebuiltAndCarryStatusAndReason(t *testing.T) {
	script := infraHarnessErrorScript()
	m := newInfraRowsModel(t, script.log)

	script.feed(m)

	live, rebuilt := liveRows(m), m.loadHistory()
	if !reflect.DeepEqual(live, rebuilt) {
		t.Errorf("live rows =\n%+v\nwant the rows rebuilt from the Execution Log\n%+v", live, rebuilt)
	}
	for i, want := range []struct{ status, reason string }{
		{"HARNESS_ERROR", "timeout: commit stalled"},
		{"CONFIG_ERROR", "agent definition not resolved: missing"},
	} {
		row := live[i]
		if !row.Infrastructure || row.Status != want.status || row.Reason != want.reason || row.ErrorCode != "" {
			t.Errorf("row %d = %+v, want an infrastructure row with status %s, reason %q and no error code", i, row, want.status, want.reason)
		}
	}
}

func TestInfraFailureRows_ProgressViewShowsRunnerRecordedStatusLiveAndRebuilt(t *testing.T) {
	script := infraHarnessErrorScript()
	m := newInfraRowsModel(t, script.log)
	script.feed(m)
	// The full row is wider than the 80-column view; only the part that
	// always fits is asserted.
	want := "commit-manager-git#1  HARNESS_ERROR  infrastructure failed: timeout"

	if view := m.progressScreen.View(); !containsStr(view, want) {
		t.Errorf("live progress view lacks %q:\n%s", want, view)
	}
	m.selections.isNewRun = false
	if view := m.newProgressScreenWithHistory(stylesFromTheme(m.theme)).View(); !containsStr(view, want) {
		t.Errorf("progress view rebuilt from history lacks %q:\n%s", want, view)
	}
}

func TestInfraFailureDone_RunnerRecordedStatus_IsListedInTheRunSummary(t *testing.T) {
	outcome := domain.RunOutcome{Status: domain.RunCompleted, Message: "done", InfraFailures: []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#1", Status: domain.StatusHARNESS_ERROR, Summary: "timeout: commit stalled"},
	}}
	m := newFlowModel(outcome)
	m.screen = screenProgress

	m.Update(runDoneMsg{outcome: outcome})

	if m.doneScreen == nil {
		t.Fatal("no done screen after the run completed")
	}
	if view := m.doneScreen.View(); !containsStr(view, "commit-manager-git#1  HARNESS_ERROR  timeout: commit stalled") {
		t.Errorf("done screen lacks the HARNESS_ERROR infrastructure failure:\n%s", view)
	}
}

func TestInfraFailureRows_RunnerRecordedStatus_IsDrawnDifferentlyFromBlockedInfrastructureRow(t *testing.T) {
	forceANSIColors(t)
	script := &liveScript{}
	script.running("commit-manager-git#1", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#1", domain.StatusHARNESS_ERROR, domain.ErrorNone, "timeout: stalled")).
		running("commit-manager-git#2", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#2", domain.StatusBLOCKED, domain.ErrorNone, "timeout: stalled"))
	m := newInfraRowsModel(t, script.log)
	script.feed(m)
	m.progressScreen.Resize(120, 40)

	m.selections.isNewRun = false
	rebuilt := m.newProgressScreenWithHistory(stylesFromTheme(m.theme))
	rebuilt.Resize(120, 40)
	for name, view := range map[string]string{"live": m.progressScreen.View(), "rebuilt": rebuilt.View()} {
		harnessLine := strings.Replace(lineOf(t, view, "commit-manager-git#1  HARNESS_ERROR"), "commit-manager-git#1  HARNESS_ERROR", "commit-manager-git#2  BLOCKED", 1)
		if harnessLine == lineOf(t, view, "commit-manager-git#2  BLOCKED") {
			t.Errorf("%s: HARNESS_ERROR infrastructure row has the style of a BLOCKED one: %q", name, harnessLine)
		}
	}
}
