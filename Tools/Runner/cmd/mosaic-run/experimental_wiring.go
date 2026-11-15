package main

import (
	"context"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// experimentalNotice warns once per Start when the harness is marked
// experimental in the Runner.
type experimentalNotice struct {
	inner  session.Session
	reason string
	in     domain.Interaction
}

// withExperimentalNotice wraps inner so that every Start on a harness marked
// experimental first emits one warning notice carrying the harness label and
// the reason. An unmarked harness gets inner back unchanged.
func withExperimentalNotice(inner session.Session, harnessID string, in domain.Interaction) session.Session {
	reason, ok := harness.ExperimentalReason(harnessID)
	if !ok {
		return inner
	}
	return &experimentalNotice{inner: inner, reason: reason, in: in}
}

func (e *experimentalNotice) Start(ctx context.Context, rc domain.RunConfig) (domain.RunOutcome, error) {
	e.in.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeWarning,
		Title:   "Experimental harness",
		Message: harness.ExperimentalLabel + " harness: " + e.reason,
	})
	return e.inner.Start(ctx, rc)
}
