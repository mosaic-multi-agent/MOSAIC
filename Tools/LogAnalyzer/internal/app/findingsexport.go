package app

import (
	"context"
	"errors"

	"mosaic-log-analyzer/internal/domain"
)

// FindingsExportRequest parameterises a findings export.
type FindingsExportRequest struct {
	Findings  []domain.Finding // the findings to write, in order (the overlay's scope)
	OutPath   string           // must satisfy domain.IsFindingsFileName
	Overwrite bool             // replace an existing OutPath
}

// FindingsExportStatus is the coarse result of a findings export.
type FindingsExportStatus uint8

const (
	FindingsExportWritten     FindingsExportStatus = iota // file written
	FindingsExportFileExists                              // OutPath exists, Overwrite false; nothing written
	FindingsExportInvalidPath                             // OutPath fails IsFindingsFileName; nothing written
)

// FindingsExportResult reports what a findings export did.
type FindingsExportResult struct {
	Status   FindingsExportStatus
	OutPath  string
	Findings int // number of findings written; 0 unless Written
}

// ExportFindings writes req.Findings through Deps.FindingsFiles. An empty
// Findings slice is valid and writes a document with no findings. It returns an
// error only for a nil writer, a cancelled ctx or a write failure other than
// domain.ErrFindingsFileExists.
func (s *Service) ExportFindings(ctx context.Context, req FindingsExportRequest) (FindingsExportResult, error) {
	if err := ctx.Err(); err != nil {
		return FindingsExportResult{}, err
	}
	res := FindingsExportResult{OutPath: req.OutPath}
	if !domain.IsFindingsFileName(req.OutPath) {
		res.Status = FindingsExportInvalidPath
		return res, nil
	}
	if s.deps.FindingsFiles == nil {
		return FindingsExportResult{}, errors.New("export findings: no findings file writer configured")
	}
	err := s.deps.FindingsFiles.WriteFindings(ctx, req.OutPath, req.Findings, req.Overwrite)
	switch {
	case errors.Is(err, domain.ErrFindingsFileExists):
		res.Status = FindingsExportFileExists
		return res, nil
	case err != nil:
		return FindingsExportResult{}, err
	}
	res.Status = FindingsExportWritten
	res.Findings = len(req.Findings)
	return res, nil
}
