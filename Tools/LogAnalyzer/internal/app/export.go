package app

import (
	"context"
	"errors"

	"mosaic-log-analyzer/internal/domain"
)

// ExportRequest parameterises a digest export.
type ExportRequest struct {
	ExplicitPath string // as Request.ExplicitPath; ignored by ExportSession
	RunID        string // "" = all named runs plus the unattributable bucket
	OutPath      string // must satisfy domain.IsDigestFileName
	Overwrite    bool   // replace an existing OutPath
}

// ExportStatus is the coarse result of an export.
type ExportStatus uint8

const (
	ExportWritten     ExportStatus = iota // file written
	ExportFileExists                      // OutPath exists and Overwrite was false; nothing written
	ExportRunNotFound                     // RunID not in the source; nothing written
	ExportNotWritten                      // source yielded NoData / SourceNotFound / SourceUnusable
	ExportInvalidPath                     // OutPath empty or not a digest file name; nothing resolved
)

// ExportResult reports what an export did.
type ExportResult struct {
	Status                ExportStatus
	Outcome               Outcome       // zero value for ExportInvalidPath
	Source                domain.Source // resolved source (session source for ExportSession)
	OutPath               string
	RunsWritten           int  // named runs in the written digest
	UnattributableWritten bool // bucket included in the written digest
}

// Export loads the source like Analyze (logs, digests or both) and writes a digest.
func (s *Service) Export(ctx context.Context, req ExportRequest) (ExportResult, error) {
	if !domain.IsDigestFileName(req.OutPath) {
		return ExportResult{Status: ExportInvalidPath, OutPath: req.OutPath}, nil
	}
	src, digestFiles, outcome, err := s.resolveWith(ctx, Request{ExplicitPath: req.ExplicitPath}, s.digestResolver())
	if err != nil {
		return ExportResult{}, err
	}
	notWritten := ExportResult{Status: ExportNotWritten, Outcome: outcome, Source: src, OutPath: req.OutPath}
	if outcome != outcomeContinue {
		return notWritten, nil
	}
	loaded, err := s.readSource(ctx, src, digestFiles)
	if err != nil {
		return ExportResult{}, err
	}
	if loaded.noData {
		notWritten.Outcome = OutcomeNoData
		return notWritten, nil
	}
	sess := &Session{aggregate: loaded.aggregate, source: src, exportFindings: loaded.exportFindings, settings: domain.DefaultAnalysisSettings()}
	return s.ExportSession(ctx, sess, req)
}

// ExportSession writes a digest from an analysis already loaded by Analyze.
func (s *Service) ExportSession(ctx context.Context, sess *Session, req ExportRequest) (ExportResult, error) {
	if sess == nil {
		return ExportResult{}, errors.New("export: no loaded analysis")
	}
	res := ExportResult{Outcome: OutcomeReport, Source: sess.source, OutPath: req.OutPath}
	if !domain.IsDigestFileName(req.OutPath) {
		res.Status = ExportInvalidPath
		res.Outcome = 0
		return res, nil
	}
	if s.deps.Digests == nil {
		return ExportResult{}, errors.New("export: no digest store configured")
	}

	agg, findings, ok := selectForExport(sess, req.RunID)
	if !ok {
		res.Status = ExportRunNotFound
		res.Outcome = OutcomeNoData
		return res, nil
	}
	if len(agg.Runs) == 0 && agg.Unattributable == nil {
		res.Status = ExportNotWritten
		res.Outcome = OutcomeNoData
		return res, nil
	}

	d := domain.Digest{
		Header: domain.DigestHeader{
			ToolVersion: s.deps.ToolVersion,
			WrittenAt:   s.deps.Clock.Now(),
		},
		Aggregate: agg,
		Findings:  findings,
	}
	if err := s.deps.Digests.Write(ctx, req.OutPath, d, req.Overwrite); err != nil {
		if errors.Is(err, domain.ErrDigestExists) {
			res.Status = ExportFileExists
			return res, nil
		}
		return ExportResult{}, err
	}
	res.Status = ExportWritten
	res.RunsWritten = len(agg.Runs)
	res.UnattributableWritten = agg.Unattributable != nil
	return res, nil
}

// selectForExport picks the runs and findings an export carries. With no run id
// that is everything including the unattributable bucket and general findings;
// with a run id only that run and its own findings. ok is false when the run id
// is not in the session.
func selectForExport(sess *Session, runID string) (domain.Aggregate, []domain.Finding, bool) {
	if runID == "" {
		return sess.aggregate, sess.exportFindings, true
	}
	var agg domain.Aggregate
	for _, r := range sess.aggregate.Runs {
		if r.Run.ID == runID {
			agg.Runs = []domain.RunAggregate{r}
		}
	}
	if len(agg.Runs) == 0 {
		return agg, nil, false
	}
	var findings []domain.Finding
	for _, f := range sess.exportFindings {
		if f.Run == domain.NamedRun(runID) {
			findings = append(findings, f)
		}
	}
	return agg, findings, true
}
