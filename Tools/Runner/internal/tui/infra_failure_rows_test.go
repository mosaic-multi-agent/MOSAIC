package tui

// Tests of infrastructure failures in the TUI model: a failed infrastructure
// step is shown the same way live and when rebuilt from the Execution Log
// (row parity), and the done screen lists the failures of a run that ended
// with an error. The model learns the declared infrastructure agents from the
// orchestrator file of the run.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
)

const infraRowsOrchestrator = `<Workflow type="core" name="wf" version="1.0">
## Test Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | COMPLETE | - | - | Plan.md |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | continue | Git commit manager |
</InfrastructureAgent>
</InfrastructureAgents>
`

// writeInfraRowsOrchestrator writes the orchestrator file declaring one
// infrastructure agent, commit-manager-git, and returns its path.
func writeInfraRowsOrchestrator(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orchestrator-script.md")
	if err := os.WriteFile(path, []byte(infraRowsOrchestrator), 0o600); err != nil {
		t.Fatalf("write orchestrator fixture: %v", err)
	}
	return path
}

func infraEntry(agent string, status domain.StatusCode, code domain.ErrorCode, summary string) domain.ExecutionLogEntry {
	e := entry(agent, "EXECUTION", "1.Implementation", status)
	e.ErrorCode = code
	e.Summary = summary
	return e
}

// newInfraRowsModel is the model of a resumed run whose artifact serves log and
// whose orchestrator file declares commit-manager-git.
func newInfraRowsModel(t *testing.T, log []domain.ExecutionLogEntry) *rootModel {
	t.Helper()
	fx := newRestartFixture(t, storeWithLog(log...))
	fx.m.selections.orchestratorFile = writeInfraRowsOrchestrator(t)
	return fx.m
}

// infraRowsScript records a workflow step, a failed and a successful
// infrastructure step, and a failed workflow step.
func infraRowsScript() *liveScript {
	s := &liveScript{}
	s.running("planner#1", "EXECUTION", "1.Implementation").
		record(infraEntry("planner#1", domain.StatusSUCCESS, domain.ErrorNone, "planned"))
	s.running("commit-manager-git#2", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#2", domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "nothing to commit against"))
	s.running("commit-manager-git#3", "EXECUTION", "1.Implementation").
		record(infraEntry("commit-manager-git#3", domain.StatusSUCCESS, domain.ErrorNone, "committed"))
	s.running("planner#4", "EXECUTION", "1.Implementation").
		record(infraEntry("planner#4", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "planner blocked"))
	return s
}

func TestInfraFailureRows_LiveRowsEqualRowsRebuiltFromTheExecutionLog(t *testing.T) {
	script := infraRowsScript()
	m := newInfraRowsModel(t, script.log)

	script.feed(m)

	live, rebuilt := liveRows(m), m.loadHistory()
	if !reflect.DeepEqual(live, rebuilt) {
		t.Errorf("live rows =\n%+v\nwant the rows rebuilt from the Execution Log\n%+v", live, rebuilt)
	}
}

func TestInfraFailureRows_FailedInfrastructureStepCarriesFlagCodeAndReasonLiveAndRebuilt(t *testing.T) {
	script := infraRowsScript()
	m := newInfraRowsModel(t, script.log)
	script.feed(m)

	for name, rows := range map[string][]progressRow{"live": liveRows(m), "rebuilt": m.loadHistory()} {
		t.Run(name, func(t *testing.T) {
			if len(rows) != 4 {
				t.Fatalf("len(rows) = %d, want 4: %+v", len(rows), rows)
			}
			failed, ok := rows[1], rows[2]
			if !failed.Infrastructure || failed.ErrorCode != "E101" || failed.Reason != "nothing to commit against" {
				t.Errorf("failed infrastructure row = %+v, want Infrastructure, ErrorCode E101 and the recorded reason", failed)
			}
			if !ok.Infrastructure || ok.Status != "SUCCESS" {
				t.Errorf("successful infrastructure row = %+v, want Infrastructure with status SUCCESS", ok)
			}
			if rows[0].Infrastructure || rows[3].Infrastructure {
				t.Errorf("workflow rows are flagged infrastructure: %+v, %+v", rows[0], rows[3])
			}
		})
	}
}

func TestInfraFailureRows_ProgressViewShowsFailedInfrastructureStepAsFailedLiveAndRebuilt(t *testing.T) {
	script := infraRowsScript()
	m := newInfraRowsModel(t, script.log)
	script.feed(m)
	// The full row is wider than the 80-column view and is truncated to one
	// screen line, so only the part that always fits is asserted.
	want := "commit-manager-git#2  BLOCKED E101  infrastructure failed: nothing"

	if view := m.progressScreen.View(); !containsStr(view, want) {
		t.Errorf("live progress view lacks %q:\n%s", want, view)
	}

	m.selections.isNewRun = false
	rebuilt := m.newProgressScreenWithHistory(stylesFromTheme(m.theme))
	if view := rebuilt.View(); !containsStr(view, want) {
		t.Errorf("progress view rebuilt from history lacks %q:\n%s", want, view)
	}
}

func TestInfraFailureDone_RunEndedWithAnError_ListsTheReturnedOutcomesFailures(t *testing.T) {
	outcome := domain.RunOutcome{Status: domain.RunFailed, InfraFailures: []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusBLOCKED, ErrorCode: domain.ErrorINPUT_NOT_FOUND, Summary: "nothing to commit against"},
	}}
	sess := &stubFlowSession{outcome: outcome, err: os.ErrPermission}
	m := newRootModel(context.Background(), sess, Options{Theme: tuicommon.DefaultTheme()})
	m.screen = screenProgress

	m.Update(runCmdNoPanic(t, m.startSession()))

	if m.doneScreen == nil {
		t.Fatal("no done screen after the session returned an error")
	}
	view := m.doneScreen.View()
	for _, want := range []string{"Infrastructure failures in this run (1):", "commit-manager-git#2  BLOCKED E101  nothing to commit against"} {
		if !containsStr(view, want) {
			t.Errorf("done screen after a Start error lacks %q:\n%s", want, view)
		}
	}
}

func TestInfraFailureDone_RunEndedNormally_ListsTheOutcomesFailures(t *testing.T) {
	outcome := domain.RunOutcome{Status: domain.RunCompleted, Message: "done", InfraFailures: []domain.InfraFailure{
		{AgentInstance: "commit-manager-git#2", Status: domain.StatusBLOCKED, ErrorCode: domain.ErrorINPUT_NOT_FOUND, Summary: "nothing to commit against"},
	}}
	m := newFlowModel(outcome)
	m.screen = screenProgress

	m.Update(runDoneMsg{outcome: outcome})

	if m.doneScreen == nil {
		t.Fatal("no done screen after the run completed")
	}
	if view := m.doneScreen.View(); !containsStr(view, "commit-manager-git#2  BLOCKED E101  nothing to commit against") {
		t.Errorf("done screen lacks the infrastructure failure:\n%s", view)
	}
}
