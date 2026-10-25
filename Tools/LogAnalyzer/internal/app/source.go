package app

import (
	"context"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/domain"
)

// outcomeContinue is a package-internal sentinel returned by resolveSource to
// signal that a usable source was found and the caller should proceed. It is
// never surfaced to callers of the public API.
const outcomeContinue = Outcome(255)

// sourceResolver decides how a path becomes a source. The logs-only resolver
// is used by the total flow; the digest-aware one is used by Analyze.
type sourceResolver struct {
	// classify turns a path into a source plus the digest files found there.
	classify func(path string) (domain.Source, []string)
	// digestsAt lists the digest files beside an already-usable source (the
	// default location). Nil when digests are not considered.
	digestsAt func(src domain.Source) []string
}

// logsOnlyResolver resolves sources from the log structure alone.
func (s *Service) logsOnlyResolver() sourceResolver {
	return sourceResolver{
		classify: func(path string) (domain.Source, []string) {
			return s.deps.Source.Classify(path), nil
		},
	}
}

// resolveSource determines the log source following the binding order:
// explicit path → default OrchestrationLogs/ location → user prompt. It only
// considers log structure; digests are not looked at.
//
// Returns the resolved source and outcomeContinue when a usable source is
// available. Returns an early-exit Outcome (OutcomeSourceNotFound or
// OutcomeSourceUnusable) when no usable source could be found.
func (s *Service) resolveSource(ctx context.Context, req Request) (domain.Source, Outcome, error) {
	src, _, outcome, err := s.resolveWith(ctx, req, s.logsOnlyResolver())
	return src, outcome, err
}

// resolveWith applies the binding resolution order with the given resolver and
// also returns the digest files discovered for the resolved source.
func (s *Service) resolveWith(ctx context.Context, req Request, r sourceResolver) (domain.Source, []string, Outcome, error) {
	if req.ExplicitPath != "" {
		src, files := r.classify(req.ExplicitPath)
		switch {
		case src.IsUsable():
			return src, files, outcomeContinue, nil
		case src.Kind == domain.SourceUnusable:
			return src, nil, OutcomeSourceUnusable, nil
		default: // SourceNotFound
			return src, nil, OutcomeSourceNotFound, nil
		}
	}

	// No explicit path: probe the default location beneath the working directory.
	src := s.deps.Source.Default(s.deps.WorkDir)
	if src.IsUsable() {
		var files []string
		if r.digestsAt != nil {
			files = r.digestsAt(src)
		}
		return src, files, outcomeContinue, nil
	}

	// Neither explicit path nor default is available — ask the user.
	ans, err := s.deps.Interaction.AskText(ctx, interaction.TextQuestion{
		Question: interaction.Question{
			ID:     QuestionLogSourcePath,
			Title:  "Log source",
			Prompt: "Enter the path to your logs root or run folder:",
		},
	})
	if err != nil {
		return domain.Source{}, nil, OutcomeSourceNotFound, err
	}
	if ans.Status != interaction.Answered {
		// Declined, skipped or cancelled — a clean not-found outcome, not an error.
		return domain.Source{Kind: domain.SourceNotFound}, nil, OutcomeSourceNotFound, nil
	}

	// Classify the user-supplied path.
	prompted, files := r.classify(ans.Text)
	if !prompted.IsUsable() {
		return prompted, nil, OutcomeSourceNotFound, nil
	}
	return prompted, files, outcomeContinue, nil
}
