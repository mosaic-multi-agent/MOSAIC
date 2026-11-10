package session

import (
	"context"
	"path/filepath"

	"mosaic-run/internal/domain"
)

// beginOutputs records the pre-invocation state of the declared outputs of a
// step. It is called immediately before the step's first attempt; the baseline
// is reused for every HITL re-dispatch of that step, so the gate checks
// everything any attempt wrote.
func (s *sessionImpl) beginOutputs(ctx context.Context, runFolder string, declared []string) {
	q := domain.OutputQuery{Root: filepath.Dir(runFolder), Declared: declared}
	if s.deps.Outputs == nil {
		s.outputBaseline = domain.OutputBaseline{Query: q}
		return
	}
	s.outputBaseline = s.deps.Outputs.Baseline(ctx, q)
}

// writtenOutputs returns the declared outputs the step's attempts created or
// modified since beginOutputs. Without a wired detector it conservatively
// reports every declared concrete path, so the HITL gate stays fail-closed: a
// wildcard Output is replaced by every existing file matching it.
func (s *sessionImpl) writtenOutputs(ctx context.Context) []string {
	if s.deps.Outputs == nil {
		q := s.outputBaseline.Query
		return listOutputWildcards(q.Root, q.Declared)
	}
	return s.deps.Outputs.Written(ctx, s.outputBaseline)
}

// readApprovals reads human_approved for each written output, in order.
func (s *sessionImpl) readApprovals(ctx context.Context, written []string) []domain.ArtifactApproval {
	var approvals []domain.ArtifactApproval
	for _, p := range written {
		approvals = append(approvals, domain.ArtifactApproval{
			Path:     p,
			Approval: s.deps.Approvals.ReadApproval(ctx, p),
		})
	}
	return approvals
}
