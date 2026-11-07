package tui

// resume_hitl_timeout_test.go verifies that a resumed run's wizard asks the HITL
// invocation timeout and that both timeout answers reach the launched session.

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResumeFlow_LaunchedSessionCarriesBothTimeoutAnswers(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()

	f.key(tea.KeyEnter) // confirm the settings summary

	launched := f.launched()
	if len(launched) != 1 {
		t.Fatalf("sessions launched = %d, want 1", len(launched))
	}
	if got := launched[0].cfg.Timeout; got != 45*time.Minute {
		t.Errorf("Timeout = %v, want the answered 45m", got)
	}
	if got := launched[0].cfg.HITLTimeout; got != 3*time.Hour {
		t.Errorf("HITLTimeout = %v, want the answered 3h", got)
	}
}

func TestResumeFlow_BackThenForward_KeepsTheHITLTimeoutAnswer(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()
	f.key(tea.KeyEsc)
	f.key(tea.KeyEnter) // accept the HITL timeout as answered
	f.key(tea.KeyEnter) // confirm the settings summary

	launched := f.launched()
	if len(launched) != 1 {
		t.Fatalf("sessions launched = %d, want 1", len(launched))
	}
	if got := launched[0].cfg.HITLTimeout; got != 3*time.Hour {
		t.Errorf("HITLTimeout = %v, want the earlier answer 3h", got)
	}
}
