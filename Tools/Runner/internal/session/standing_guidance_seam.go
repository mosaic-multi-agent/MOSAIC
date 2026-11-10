package session

import "mosaic-run/internal/domain"

// NewActiveStandingGuidance returns a holder that already reports g as the
// guidance in effect, saving edits through store into folder and logging them
// to log, as a running session's holder does. It lets a frontend be exercised
// without starting a session.
func NewActiveStandingGuidance(g domain.Guidance, store domain.GuidanceStore, folder string, log domain.DebugLogger) *StandingGuidance {
	s := NewStandingGuidance()
	s.activate(g.Normalized(), store, folder, log)
	return s
}

// ActivateStandingGuidance makes s report g as the guidance in effect, as a
// session start does once the run is already on screen.
func ActivateStandingGuidance(s *StandingGuidance, g domain.Guidance, store domain.GuidanceStore, folder string, log domain.DebugLogger) {
	s.activate(g.Normalized(), store, folder, log)
}

// DeactivateStandingGuidance ends s's guidance, as the end of a session start does.
func DeactivateStandingGuidance(s *StandingGuidance) {
	s.deactivate()
}
