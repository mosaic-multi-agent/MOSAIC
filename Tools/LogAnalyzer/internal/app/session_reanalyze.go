package app

import "mosaic-log-analyzer/internal/domain"

// Reanalyze replaces the session's settings and returns the report recomputed
// from the cached aggregate, without I/O.
func (s *Session) Reanalyze(settings domain.AnalysisSettings) domain.Report {
	s.settings = settings
	return s.Reprice()
}
