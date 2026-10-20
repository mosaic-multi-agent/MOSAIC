package tui

// The progress screen's live rows, built from the session's running notices
// and step-record announcements, equal the rows rebuilt from the Execution Log
// at every point where no step is running.

import (
	"context"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

func TestRowParity_AcceptedSteps_LiveRowsEqualRebuiltRows(t *testing.T) {
	s := (&liveScript{}).
		running("planner#1", "PLANNING", "").
		record(entry("planner#1", "PLANNING", "", domain.StatusSUCCESS)).
		completion("planner#1", "PLANNING", "", "SUCCESS").
		running("builder#2", "EXECUTION", "1.Implementation").
		record(entry("builder#2", "EXECUTION", "1.Implementation", domain.StatusBLOCKED)).
		completion("builder#2", "EXECUTION", "1.Implementation", "BLOCKED")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_RowsWithoutRunningNotice_AreAppendedAsFinishedRows(t *testing.T) {
	s := (&liveScript{}).
		record(entry("commit-manager-git#1", "", "", domain.StatusSUCCESS)).
		running("agent-a#2", "PLANNING", "").
		record(entry("agent-a#2", "PLANNING", "", domain.StatusSUCCESS)).
		record(entry("checkpoint-manager-git#3", "PLANNING", "", domain.StatusSUCCESS))
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_HITLRejectedAttemptAndRedispatch_LiveRowsEqualRebuiltRows(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		running("agent-a#2", "PLANNING", "").
		record(entry("agent-a#2", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#2", "PLANNING", "", "SUCCESS")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_FailedHarnessAttempt_LeavesNoRunningRow(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusBLOCKED))
	m := newParityModel()

	s.feed(m)

	rows := liveRows(m)
	if len(rows) != 1 || rows[0].Status != "BLOCKED" {
		t.Fatalf("rows after a failed attempt = %+v, want one BLOCKED row and none running", rows)
	}
	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_FailedAttemptThenRetry_LiveRowsEqualRebuiltRows(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "EXECUTION", "2.Test").
		record(entry("agent-a#1", "EXECUTION", "2.Test", domain.StatusBLOCKED)).
		running("agent-a#2", "EXECUTION", "2.Test").
		record(entry("agent-a#2", "EXECUTION", "2.Test", domain.StatusSUCCESS)).
		completion("agent-a#2", "EXECUTION", "2.Test", "SUCCESS")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_FailedBypassAttempt_LiveRowsEqualRebuiltRows(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusBLOCKED)).
		record(entry("agent-a#2", "PLANNING", "", domain.StatusBLOCKED)).
		running("agent-a#3", "PLANNING", "").
		record(entry("agent-a#3", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#3", "PLANNING", "", "SUCCESS")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

// A row recorded under another agent instance than the one announced as running
// (a re-dispatch that discharged the HITL gate, a bypass) shows the recorded one.
func TestRowParity_RecordedInstanceDiffersFromAnnounced_RowShowsRecordedInstance(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#2", "PLANNING", "", domain.StatusSUCCESS)).
		completion("agent-a#2", "PLANNING", "", "SUCCESS")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

// The running notice shows the formatted iteration value while the recorded
// entry holds the recorded stage (possibly another notation, including retired
// ones); the finished row shows the recorded stage, as the rebuilt row does,
// with or without a completion notice after it.
func TestRowParity_IterationRendersLikeTheRebuiltRow(t *testing.T) {
	cases := []struct{ name, noticeIteration, recordedStage string }{
		{"same", "1.Test", "1.Test"},
		{"empty notice, recorded stage", "", "A"},
		{"formatted notice, bare recorded stage", "1.Test", "1"},
		{"formatted notice, retired notation", "3.Implementation", "3-Implementation"},
		{"bare notice, qualified recorded stage", "3", "3.Implementation"},
	}
	for _, tc := range cases {
		for _, withCompletion := range []bool{false, true} {
			name := tc.name
			if withCompletion {
				name += " with completion notice"
			}
			t.Run(name, func(t *testing.T) {
				s := (&liveScript{}).
					running("agent-a#1", "EXECUTION", tc.noticeIteration).
					record(entry("agent-a#1", "EXECUTION", tc.recordedStage, domain.StatusSUCCESS))
				if withCompletion {
					s.completion("agent-a#1", "EXECUTION", tc.noticeIteration, "SUCCESS")
				}
				m := newParityModel()

				s.feed(m)

				requireRowsEqualRebuilt(t, m, s)
				if rows := liveRows(m); len(rows) != 1 || rows[0].Stage != tc.recordedStage {
					t.Errorf("rows = %+v, want one row showing the recorded stage %q", rows, tc.recordedStage)
				}
			})
		}
	}
}

// A completion notice that follows the announcement of a different final status
// cannot rewrite the recorded row or an earlier one.
func TestRowParity_CompletionNoticeAfterRecord_DoesNotRewriteRecordedRow(t *testing.T) {
	s := (&liveScript{}).
		running("agent-a#1", "PLANNING", "").
		record(entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)).
		running("agent-b#2", "PLANNING", "").
		record(entry("agent-b#2", "PLANNING", "", domain.StatusBLOCKED)).
		completion("agent-b#2", "PLANNING", "", "SUCCESS")
	m := newParityModel()

	s.feed(m)

	requireRowsEqualRebuilt(t, m, s)
}

func TestRowParity_AfterRestart_HistoryThenLiveRowsEqualRebuiltRows(t *testing.T) {
	fx := newRestartFixture(t, storeWithLog(sampleLog()...))
	driveDoneContinue(t, fx.m)
	s := &liveScript{log: sampleLog()}
	s.running("reviewer#4", "EXECUTION", "1.Implementation").
		record(entry("reviewer#4", "EXECUTION", "1.Implementation", domain.StatusSUCCESS)).
		running("agent-a#5", "EXECUTION", "1.Implementation").
		record(entry("agent-a#5", "EXECUTION", "1.Implementation", domain.StatusBLOCKED))

	s.feed(fx.m)

	if got, want := liveRows(fx.m), historyRows(s.log); !reflect.DeepEqual(got, want) {
		t.Errorf("rows after restart and live steps =\n%+v\nwant the rows rebuilt from the extended log\n%+v", got, want)
	}
}

func TestRowParity_RecordedEntryWithNoProgressScreen_IsIgnored(t *testing.T) {
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = nil

	m.Update(stepRecordedMsg{entry: entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS)})

	if m.progressScreen != nil {
		t.Error("a step-record message created a progress screen, want it ignored")
	}
}

// recordingModel is a Bubble Tea model that keeps the messages it receives.
type recordingModel struct{ got chan tea.Msg }

func (recordingModel) Init() tea.Cmd { return nil }
func (r recordingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	select {
	case r.got <- msg:
	default:
	}
	return r, nil
}
func (recordingModel) View() string { return "" }

func TestProgramRef_StepRecorded_PostsTheEntryToTheProgram(t *testing.T) {
	model := recordingModel{got: make(chan tea.Msg, 16)}
	p, done := runHeadless(model)
	defer func() { p.Quit(); <-done }()
	ref := NewProgramRef()
	ref.set(p)
	want := entry("agent-a#1", "PLANNING", "1.Test", domain.StatusSUCCESS)

	ref.StepRecorded(context.Background(), want)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg := <-model.got:
			if rec, ok := msg.(stepRecordedMsg); ok {
				if !reflect.DeepEqual(rec.entry, want) {
					t.Errorf("posted entry = %+v, want %+v", rec.entry, want)
				}
				return
			}
		case <-deadline:
			t.Fatal("no step-record message reached the program within 2s")
		}
	}
}

func TestProgramRef_StepRecorded_BeforeTheProgramIsWired_IsANoOp(t *testing.T) {
	ref := NewProgramRef()

	ref.StepRecorded(context.Background(), entry("agent-a#1", "PLANNING", "", domain.StatusSUCCESS))
}
