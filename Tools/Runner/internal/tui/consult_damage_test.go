package tui

// The repair path from the session's observer calls to the progress screen:
// ProgramRef forwards the repair verdict to the program, the root model records
// it on the screen, and a repair consultation's activity reaches the screen
// with its attempt number.

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// Compile-time proof that the session finds the repair capability on a ProgramRef.
var _ domain.RepairOutcomeObserver = (*ProgramRef)(nil)

func TestProgramRef_RepairJudged_PostsTheOutcomeToTheProgram(t *testing.T) {
	model := recordingModel{got: make(chan tea.Msg, 16)}
	p, done := runHeadless(model)
	defer func() { p.Quit(); <-done }()
	ref := NewProgramRef()
	ref.set(p)
	want := domain.RepairOutcome{Attempt: 2, Accepted: false, Problems: 4}

	ref.RepairJudged(context.Background(), want)

	msg := waitForMsg(t, model.got, func(m tea.Msg) bool { _, ok := m.(repairJudgedMsg); return ok })
	if got := msg.(repairJudgedMsg).outcome; got != want {
		t.Errorf("posted outcome = %+v, want %+v", got, want)
	}
}

func TestProgramRef_RepairJudged_BeforeTheProgramIsWired_IsANoOp(t *testing.T) {
	ref := NewProgramRef()

	ref.RepairJudged(context.Background(), domain.RepairOutcome{Attempt: 1, Accepted: true})
}

func TestConsultRepair_Start_ShowsTheRowWithItsRepairAttempt(t *testing.T) {
	m := newParityModel()
	repair := domain.ConsultationActivity{Context: domain.ConsultContextRouting, RepairAttempt: 1}

	m.Update(consultStartedMsg{activity: repair})

	requireConsultation(t, m, repair)
}

func TestConsultRepair_Judged_RecordsTheOutcomeOnTheScreenAfterTheRowEnded(t *testing.T) {
	m := newParityModel()
	want := domain.RepairOutcome{Attempt: 1, Accepted: true}
	m.Update(consultStartedMsg{activity: domain.ConsultationActivity{Context: domain.ConsultContextRouting, RepairAttempt: 1}})
	m.Update(consultEndedMsg{})

	m.Update(repairJudgedMsg{outcome: want})

	requireNoConsultation(t, m)
	if got := m.progressScreen.RepairOutcomes(); len(got) != 1 || got[0] != want {
		t.Errorf("RepairOutcomes() = %+v, want [%+v]", got, want)
	}
}

func TestConsultRepair_TwoVerdicts_AreRecordedInOrder(t *testing.T) {
	m := newParityModel()
	first := domain.RepairOutcome{Attempt: 1, Problems: 2}
	second := domain.RepairOutcome{Attempt: 2, Accepted: true}

	m.Update(repairJudgedMsg{outcome: first})
	m.Update(repairJudgedMsg{outcome: second})

	if got := m.progressScreen.RepairOutcomes(); len(got) != 2 || got[0] != first || got[1] != second {
		t.Errorf("RepairOutcomes() = %+v, want [%+v %+v]", got, first, second)
	}
}

func TestConsultRepair_Judged_WithoutProgressScreen_IsHarmless(t *testing.T) {
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = nil

	m.Update(repairJudgedMsg{outcome: domain.RepairOutcome{Attempt: 1, Accepted: true}})

	if m.progressScreen != nil {
		t.Error("a repair verdict created a progress screen")
	}
}
