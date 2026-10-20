package runflow

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// StartConsultation shows the single consultation row for c, replacing any
// consultation row already shown.
func (s *ProgressScreen) StartConsultation(c domain.ConsultationActivity) {
	s.consultation = &c
}

// EndConsultation removes the consultation row; no-op when none is shown.
func (s *ProgressScreen) EndConsultation() {
	s.consultation = nil
}

// Consultation returns the consultation row's activity and true while one is
// shown; zero value and false otherwise.
func (s *ProgressScreen) Consultation() (domain.ConsultationActivity, bool) {
	if s.consultation == nil {
		return domain.ConsultationActivity{}, false
	}
	return *s.consultation, true
}

// consultationLine renders the running consultation row, or "" when none is
// shown. Its label differs from a subagent row's so the two cannot be confused.
func (s *ProgressScreen) consultationLine() string {
	c := s.consultation
	if c == nil {
		return ""
	}
	label := "consultation: routing"
	if c.Context == domain.ConsultContextPreConsultation {
		label = "consultation: pre-consultation"
	} else if c.Kind != "" {
		label += fmt.Sprintf(" (%s)", c.Kind)
	}
	return s.styles.Selected.Render("▶ " + label)
}
