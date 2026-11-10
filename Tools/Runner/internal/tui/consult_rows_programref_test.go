package tui

// ProgramRef is the carrier of consultation announcements from the session to
// the program: starting posts the activity, ending posts the end message, and
// before the program is wired both are no-ops.

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// Compile-time proof that the session can be handed a ProgramRef as its
// consultation observer.
var _ domain.ConsultationObserver = (*ProgramRef)(nil)

// waitForMsg returns the first message accepted by match, or fails after 2s.
func waitForMsg(t *testing.T, got chan tea.Msg, match func(tea.Msg) bool) tea.Msg {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg := <-got:
			if match(msg) {
				return msg
			}
		case <-deadline:
			t.Fatal("the expected message did not reach the program within 2s")
			return nil
		}
	}
}

func TestProgramRef_ConsultationStarted_PostsTheActivityToTheProgram(t *testing.T) {
	model := recordingModel{got: make(chan tea.Msg, 16)}
	p, done := runHeadless(model)
	defer func() { p.Quit(); <-done }()
	ref := NewProgramRef()
	ref.set(p)
	want := domain.ConsultationActivity{Context: domain.ConsultContextRouting, Kind: domain.DeviationScheduleDefect}

	ref.ConsultationStarted(context.Background(), want)

	msg := waitForMsg(t, model.got, func(m tea.Msg) bool { _, ok := m.(consultStartedMsg); return ok })
	if got := msg.(consultStartedMsg).activity; got != want {
		t.Errorf("posted activity = %+v, want %+v", got, want)
	}
}

func TestProgramRef_ConsultationEnded_PostsTheEndToTheProgram(t *testing.T) {
	model := recordingModel{got: make(chan tea.Msg, 16)}
	p, done := runHeadless(model)
	defer func() { p.Quit(); <-done }()
	ref := NewProgramRef()
	ref.set(p)

	ref.ConsultationEnded(context.Background())

	waitForMsg(t, model.got, func(m tea.Msg) bool { _, ok := m.(consultEndedMsg); return ok })
}

func TestProgramRef_Consultation_BeforeTheProgramIsWired_IsANoOp(t *testing.T) {
	ref := NewProgramRef()

	ref.ConsultationStarted(context.Background(), domain.ConsultationActivity{Context: domain.ConsultContextPreConsultation})
	ref.ConsultationEnded(context.Background())
}
