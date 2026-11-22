package app

import (
	"context"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// transcriptFindings compares the transcripts of the log runs with their streams.
// An unwired transcript port gives no findings.
func (s *Service) transcriptFindings(ctx context.Context, inv domain.Inventory, streams []analysis.Stream) []domain.Finding {
	if s.deps.Transcripts == nil {
		return nil
	}
	facts := s.deps.Transcripts.Transcripts(ctx, inv)
	findings := analysis.TranscriptCaptureGapFindings(streams, facts)
	return append(findings, analysis.InstancePromptMismatchFindings(facts)...)
}
