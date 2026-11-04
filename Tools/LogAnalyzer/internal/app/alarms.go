package app

import (
	"context"

	"mosaic-log-analyzer/internal/domain"
)

// alarmFindings turns adapter transcript alarms into one data-loss finding per
// loaded named run, located at the run's first alarm. Alarms of runs that are not
// in the aggregate are ignored, and so is an unwired alarm source.
func (s *Service) alarmFindings(ctx context.Context, src domain.Source, agg domain.Aggregate) []domain.Finding {
	if s.deps.Alarms == nil {
		return nil
	}
	loaded := make(map[domain.RunRef]bool, len(agg.Runs))
	for _, r := range agg.Runs {
		loaded[r.Run] = true
	}
	var findings []domain.Finding
	seen := make(map[domain.RunRef]bool)
	for _, a := range s.deps.Alarms.TranscriptAlarms(ctx, src) {
		if !loaded[a.Run] || seen[a.Run] {
			continue
		}
		seen[a.Run] = true
		findings = append(findings, domain.Finding{
			Kind:     domain.FindingTranscriptFormatChanged,
			Severity: domain.SeverityWarning,
			Run:      a.Run,
			Path:     a.Path,
			Line:     a.Line,
			Detail: "the " + string(a.Harness) + " transcript has assistant turns but no usage records; " +
				"the harness transcript format may have changed, so this run's usage and cost are incomplete",
		})
	}
	return findings
}
