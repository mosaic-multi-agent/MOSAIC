package session_test

// Helpers for the "ask the orchestrator" tests: a probe that scripts the manual
// dialogue's row question and records whether the option was offered.

import (
	"slices"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// rowProbe records, per opening of the manual dialogue, whether its row
// question offered the ask-orchestrator option.
type rowProbe struct {
	offered []bool
}

// scriptRow makes u answer its row question with answers in order (the last
// repeats) and records every offer in the returned probe.
func scriptRow(u *manualUser, answers ...string) *rowProbe {
	probe := &rowProbe{}
	var fns []func(interaction.ChoiceQuestion) interaction.ChoiceAnswer
	for _, a := range answers {
		a := a
		fns = append(fns, func(q interaction.ChoiceQuestion) interaction.ChoiceAnswer {
			probe.offered = append(probe.offered, slices.ContainsFunc(q.Options, func(o interaction.Option) bool {
				return o.ID == domain.ManualAskOrchestratorOptionID
			}))
			return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: a}
		})
	}
	u.one[domain.QuestionManualRow] = fns
	return probe
}

// onlyRowAsked reports whether the user was asked nothing but row questions.
func onlyRowAsked(u *manualUser) bool {
	for _, id := range u.asked {
		if id != domain.QuestionManualRow {
			return false
		}
	}
	return true
}
