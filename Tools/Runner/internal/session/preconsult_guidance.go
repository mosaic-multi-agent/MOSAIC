package session

import (
	"context"
	"fmt"
	"path/filepath"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
)

const guidanceReviewCancelledMessage = "run stopped: guidance review cancelled; resume to continue"

// establishGuidance is step 8c of the run-start sequence: obtain the run's
// standing guidance (the saved file, or a fresh pre-consultation), let the
// frontend review it, and publish it for the dispatch loop. It applies only to
// auto and auto-review runs with pre-consultation enabled.
func (s *sessionImpl) establishGuidance(ctx context.Context, rs *runStartCtx) (domain.RunOutcome, bool, error) {
	if !rs.config.PreConsultation {
		return domain.RunOutcome{}, false, nil
	}
	if rs.config.Mode != domain.ExecutionModeAuto && rs.config.Mode != domain.ExecutionModeAutoReview {
		return domain.RunOutcome{}, false, nil
	}
	folder := rs.config.RunFolder
	store := s.deps.GuidanceStore

	g, found, err := store.Load(folder)
	if err != nil {
		return s.startFailed("guidance file could not be loaded: "+err.Error(), err), true, nil
	}
	if found {
		logGuidanceChange(s.deps.Debug, domain.GuidanceLoaded, g)
	} else {
		fresh, outcome, done := s.preConsultGuidance(ctx, rs)
		if done {
			return outcome, true, nil
		}
		g = fresh
		if err := store.Save(folder, g); err != nil {
			return s.guidanceSaveFailed(err), true, nil
		}
	}

	g, outcome, done := s.reviewGuidance(ctx, rs, g, found)
	if done {
		return outcome, true, nil
	}
	s.deps.Guidance.activate(g, store, folder, s.deps.Debug)
	return domain.RunOutcome{}, false, nil
}

// preConsultGuidance runs pre-consultation. On failure or a stop instruction it
// returns the finished outcome and done=true; nothing is saved.
func (s *sessionImpl) preConsultGuidance(ctx context.Context, rs *runStartCtx) (domain.Guidance, domain.RunOutcome, bool) {
	if s.deps.PreConsult == nil {
		// Unreachable after the start-time port check; kept so no path can
		// dereference a nil port.
		missing := newPreConsultMissing(rs.config.Mode)
		return domain.Guidance{}, s.startFailed("pre-consultation failed: "+missing.Error(), missing), true
	}
	advice, pcErr := s.preConsultOrchestratorAgent(ctx, domain.ConsultationRequest{
		Context:               domain.ConsultContextPreConsultation,
		OrchestrationArtifact: filepath.Join(rs.config.RunFolder, "Orchestration.md"),
	})
	if pcErr != nil {
		// The run folder, artifact and any setup row are kept; pre-consultation
		// writes nothing to the artifact, and a resume retries it.
		return domain.Guidance{}, s.startFailed("pre-consultation failed: "+pcErr.Error(), pcErr), true
	}
	if advice.Stop != nil {
		// Checked before the advice is applied; the artifact stays untouched and
		// a resume re-runs pre-consultation.
		s.deps.Debug.Log(domain.EventSessionConsultStop, "consultant stop instruction",
			domain.F("reason", advice.Stop.Reason),
			domain.F("context", "pre_consultation"),
		)
		return domain.Guidance{}, domain.RunOutcome{
			Status:              domain.RunStoppedByConsultant,
			Message:             "consultant stop: " + advice.Stop.Reason,
			StopReason:          advice.Stop.Reason,
			PreConsultationStop: true,
		}, true
	}
	return domain.Guidance{TaskDescription: advice.TaskDescription, Constraints: advice.Constraints}.Normalized(), domain.RunOutcome{}, false
}

func (s *sessionImpl) guidanceSaveFailed(err error) domain.RunOutcome {
	return s.startFailed("guidance file could not be saved: "+err.Error(), err)
}

// reviewGuidance offers g for review until the user accepts it. offerRegenerate
// is true for saved guidance on its first review.
func (s *sessionImpl) reviewGuidance(ctx context.Context, rs *runStartCtx, g domain.Guidance, offerRegenerate bool) (domain.Guidance, domain.RunOutcome, bool) {
	folder := rs.config.RunFolder
	for {
		ans, err := s.deps.Interact.SelectOne(ctx, guidanceReviewQuestion(g, offerRegenerate))
		if err != nil || ans.Status == interaction.Unsupported {
			logGuidanceChange(s.deps.Debug, domain.GuidanceAccepted, g)
			return g, domain.RunOutcome{}, false
		}
		if ans.Status == interaction.Cancelled {
			return g, domain.RunOutcome{Status: domain.RunStopped, Message: guidanceReviewCancelledMessage}, true
		}
		switch ans.OptionID {
		case domain.GuidanceOptionEdit:
			edited, ok := s.askGuidanceEdit(ctx, g)
			if !ok {
				continue
			}
			if err := s.deps.GuidanceStore.Save(folder, edited); err != nil {
				return g, s.guidanceSaveFailed(err), true
			}
			logGuidanceChange(s.deps.Debug, domain.GuidanceEdited, edited)
			return edited, domain.RunOutcome{}, false
		case domain.GuidanceOptionRegenerate:
			if !offerRegenerate {
				continue
			}
			fresh, outcome, done := s.preConsultGuidance(ctx, rs)
			if done {
				return g, outcome, true
			}
			if err := s.deps.GuidanceStore.Save(folder, fresh); err != nil {
				return g, s.guidanceSaveFailed(err), true
			}
			logGuidanceChange(s.deps.Debug, domain.GuidanceRegenerated, fresh)
			g, offerRegenerate = fresh, false
		default:
			logGuidanceChange(s.deps.Debug, domain.GuidanceAccepted, g)
			return g, domain.RunOutcome{}, false
		}
	}
}

// askGuidanceEdit asks for both fields. ok is false when the user cancelled,
// which returns to the review with nothing saved.
func (s *sessionImpl) askGuidanceEdit(ctx context.Context, g domain.Guidance) (domain.Guidance, bool) {
	task, ok := s.askGuidanceField(ctx, domain.QuestionGuidanceTask, "Appended to task_description", g.TaskDescription)
	if !ok {
		return g, false
	}
	constraints, ok := s.askGuidanceField(ctx, domain.QuestionGuidanceConstraints, "Appended to constraints", g.Constraints)
	if !ok {
		return g, false
	}
	return domain.Guidance{TaskDescription: task, Constraints: constraints}.Normalized(), true
}

// askGuidanceField asks one multi-line field. An unsupported frontend or an
// error leaves the field unchanged.
func (s *sessionImpl) askGuidanceField(ctx context.Context, id interaction.QuestionID, title, current string) (string, bool) {
	ans, err := s.deps.Interact.AskText(ctx, interaction.TextQuestion{
		Question:  interaction.Question{ID: id, Title: title, Prompt: title},
		Default:   current,
		Multiline: true,
	})
	if err != nil {
		return current, true
	}
	switch ans.Status {
	case interaction.Answered:
		return ans.Text, true
	case interaction.Cancelled:
		return current, false
	default:
		return current, true
	}
}

// guidanceReviewQuestion builds the Accept / Edit [/ Regenerate] question.
func guidanceReviewQuestion(g domain.Guidance, offerRegenerate bool) interaction.ChoiceQuestion {
	options := []interaction.Option{
		{ID: domain.GuidanceOptionAccept, Label: "Accept"},
		{ID: domain.GuidanceOptionEdit, Label: "Edit"},
	}
	if offerRegenerate {
		options = append(options, interaction.Option{ID: domain.GuidanceOptionRegenerate, Label: "Regenerate"})
	}
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              domain.QuestionGuidanceReview,
			Title:           "Standing guidance",
			Prompt:          "Review the guidance appended to every engine-routed dispatch",
			Detail:          guidanceReviewDetail(g),
			DefaultOptionID: domain.GuidanceOptionAccept,
		},
		Options: options,
	}
}

func guidanceReviewDetail(g domain.Guidance) string {
	return fmt.Sprintf("Appended to task_description:\n%s\n\nAppended to constraints:\n%s",
		orEmptyMarker(g.TaskDescription), orEmptyMarker(g.Constraints))
}

func orEmptyMarker(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}
