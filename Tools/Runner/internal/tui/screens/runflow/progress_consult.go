package runflow

import (
	"fmt"
	"strings"

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

// RecordRepairOutcome keeps o for display; outcomes stay visible for the rest
// of the run, in the order recorded, after the consultation row ends.
func (s *ProgressScreen) RecordRepairOutcome(o domain.RepairOutcome) {
	s.repairOutcomes = append(s.repairOutcomes, o)
}

// RepairOutcomes returns the recorded outcomes in order.
func (s *ProgressScreen) RepairOutcomes() []domain.RepairOutcome {
	return append([]domain.RepairOutcome(nil), s.repairOutcomes...)
}

// repairOutcomeLines renders one line per recorded repair outcome, in order,
// or "" when none is recorded.
func (s *ProgressScreen) repairOutcomeLines() string {
	lines := make([]string, 0, len(s.repairOutcomes))
	for _, o := range s.repairOutcomes {
		prefix := fmt.Sprintf("  artifact repair, attempt %d of 2: ", o.Attempt)
		if o.Accepted {
			lines = append(lines, s.styles.Success.Render(prefix+"accepted"))
			continue
		}
		noun := "problems"
		if o.Problems == 1 {
			noun = "problem"
		}
		lines = append(lines, s.styles.Warning.Render(fmt.Sprintf("%snot accepted (%d %s remaining)", prefix, o.Problems, noun)))
	}
	return strings.Join(lines, "\n")
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
	} else if details := consultationDetails(c); details != "" {
		label += fmt.Sprintf(" (%s)", details)
	}
	return s.styles.Selected.Render("▶ " + label)
}

// consultationDetails lists what the routing row shows in parentheses: the
// deviation kind and a note that the consultation carries a user message.
func consultationDetails(c *domain.ConsultationActivity) string {
	var parts []string
	if c.RepairAttempt > 0 {
		parts = append(parts, fmt.Sprintf("artifact damaged, repair %d", c.RepairAttempt))
	}
	if c.Kind != "" {
		parts = append(parts, string(c.Kind))
	}
	if c.HasUserMessage {
		parts = append(parts, "user message")
	}
	return strings.Join(parts, ", ")
}
