package tui

import "mosaic-run/internal/domain"

// consultStartedMsg is the message ProgramRef.ConsultationStarted posts to the
// program: an orchestrator-agent consultation began.
type consultStartedMsg struct {
	activity domain.ConsultationActivity
}

// consultEndedMsg is the message ProgramRef.ConsultationEnded posts to the
// program: the running consultation ended, whatever its outcome.
type consultEndedMsg struct{}
