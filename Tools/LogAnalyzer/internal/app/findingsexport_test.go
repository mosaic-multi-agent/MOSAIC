package app_test

// Tests for Service.ExportFindings: the findings of the overlay's scope go to the
// findings file port in order, a path that is not a findings file name or an
// existing file without overwrite writes nothing, and failures are errors.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// MockFindingsFileWriter implements domain.FindingsFileWriter in memory: it keeps
// the findings written per path and refuses to replace a path without overwrite.
type MockFindingsFileWriter struct {
	existing map[string]bool
	files    map[string][]domain.Finding
	calls    []mockFindingsWrite
	failWith error
}

type mockFindingsWrite struct {
	path      string
	overwrite bool
}

func (m *MockFindingsFileWriter) WriteFindings(_ context.Context, path string, findings []domain.Finding, overwrite bool) error {
	m.calls = append(m.calls, mockFindingsWrite{path: path, overwrite: overwrite})
	if m.failWith != nil {
		return m.failWith
	}
	if (m.existing[path] || m.files[path] != nil) && !overwrite {
		return fmt.Errorf("write %s: %w", path, domain.ErrFindingsFileExists)
	}
	if m.files == nil {
		m.files = map[string][]domain.Finding{}
	}
	m.files[path] = append([]domain.Finding{}, findings...)
	return nil
}

var _ domain.FindingsFileWriter = (*MockFindingsFileWriter)(nil)

const (
	ffRunA = "20261031T073445Z-0ada"
	ffRunB = "20261101T080000Z-beef"
)

func ffFindings() []domain.Finding {
	a, b := domain.NamedRun(ffRunA), domain.NamedRun(ffRunB)
	w := domain.SeverityWarning
	return []domain.Finding{
		{Kind: domain.FindingMalformedLine, Severity: w, Run: a, Path: "x.jsonl", Line: 3, Detail: "d"},
		{Kind: domain.FindingCrossStreamRecord, Severity: w, Run: b},
		{Kind: domain.FindingMissingModel, Severity: domain.SeverityInfo, Run: a},
	}
}

func ffService(w domain.FindingsFileWriter) *app.Service {
	return app.New(app.Deps{FindingsFiles: w})
}

func TestExportFindings_WritesEveryFindingInOrderToThePath(t *testing.T) {
	w := &MockFindingsFileWriter{}
	in := ffFindings()

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: in, OutPath: "out/findings.json"})

	if err != nil {
		t.Fatalf("ExportFindings: %v", err)
	}
	if res.Status != app.FindingsExportWritten || res.OutPath != "out/findings.json" || res.Findings != len(in) {
		t.Errorf("result = %+v, want written to out/findings.json with %d findings", res, len(in))
	}
	got := w.files["out/findings.json"]
	if len(got) != len(in) {
		t.Fatalf("writer holds %d findings, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("finding %d = %+v, want %+v (same order, nothing dropped)", i, got[i], in[i])
		}
	}
}

func TestExportFindings_RunScopeWritesOnlyTheFindingsItWasGiven(t *testing.T) {
	w := &MockFindingsFileWriter{}
	var runB []domain.Finding
	for _, f := range ffFindings() {
		if f.Run == domain.NamedRun(ffRunB) {
			runB = append(runB, f)
		}
	}

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: runB, OutPath: "findings-" + ffRunB + ".json"})

	if err != nil || res.Findings != 1 || len(w.files["findings-"+ffRunB+".json"]) != 1 {
		t.Errorf("res = %+v err = %v files = %v, want exactly the one finding of the run", res, err, w.files)
	}
}

func TestExportFindings_NoFindingsIsValidAndWritesAnEmptyDocument(t *testing.T) {
	w := &MockFindingsFileWriter{}

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{OutPath: "findings.json"})

	if err != nil || res.Status != app.FindingsExportWritten || res.Findings != 0 {
		t.Errorf("res = %+v err = %v, want written with 0 findings", res, err)
	}
	if len(w.calls) != 1 {
		t.Errorf("%d writer call(s), want 1 (an empty document is still written)", len(w.calls))
	}
}

func TestExportFindings_PathThatIsNotAFindingsFileNameWritesNothing(t *testing.T) {
	for _, path := range []string{"", "findings.txt", "findings", "runs.digest.json", ".json"} {
		t.Run(path, func(t *testing.T) {
			w := &MockFindingsFileWriter{}

			res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: ffFindings(), OutPath: path})

			if err != nil {
				t.Fatalf("err = %v, an invalid path is a status, not an error", err)
			}
			if res.Status != app.FindingsExportInvalidPath || res.Findings != 0 {
				t.Errorf("result = %+v, want InvalidPath with 0 findings", res)
			}
			if len(w.calls) != 0 {
				t.Errorf("%d writer call(s) for an invalid path, want none", len(w.calls))
			}
		})
	}
}

func TestExportFindings_ExistingFileWithoutOverwriteReportsFileExistsAndKeepsIt(t *testing.T) {
	w := &MockFindingsFileWriter{existing: map[string]bool{"findings.json": true}}

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: ffFindings(), OutPath: "findings.json"})

	if err != nil {
		t.Fatalf("err = %v, an existing file is a status, not an error", err)
	}
	if res.Status != app.FindingsExportFileExists || res.Findings != 0 || res.OutPath != "findings.json" {
		t.Errorf("result = %+v, want FileExists for findings.json with 0 findings", res)
	}
	if len(w.files) != 0 {
		t.Errorf("the refused export stored %v", w.files)
	}
}

func TestExportFindings_ExistingFileWithOverwriteIsReplaced(t *testing.T) {
	w := &MockFindingsFileWriter{existing: map[string]bool{"findings.json": true}}

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: ffFindings(), OutPath: "findings.json", Overwrite: true})

	if err != nil || res.Status != app.FindingsExportWritten || res.Findings != len(ffFindings()) {
		t.Errorf("res = %+v err = %v, want written", res, err)
	}
	if len(w.calls) != 1 || !w.calls[0].overwrite {
		t.Errorf("writer calls = %+v, want one call with overwrite", w.calls)
	}
}

func TestExportFindings_NoWriterConfiguredIsAnError(t *testing.T) {
	_, err := ffService(nil).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: ffFindings(), OutPath: "findings.json"})

	if err == nil {
		t.Error("want an error when no findings file writer is configured")
	}
}

func TestExportFindings_WriteFailureIsAnErrorNotAStatus(t *testing.T) {
	w := &MockFindingsFileWriter{failWith: errors.New("disk full")}

	res, err := ffService(w).ExportFindings(context.Background(), app.FindingsExportRequest{Findings: ffFindings(), OutPath: "findings.json"})

	if err == nil {
		t.Fatalf("want the write failure returned as an error, got result %+v", res)
	}
	if errors.Is(err, domain.ErrFindingsFileExists) {
		t.Errorf("err = %v must not be classified as exists", err)
	}
}

func TestExportFindings_CancelledContextIsAnErrorAndWritesNothing(t *testing.T) {
	w := &MockFindingsFileWriter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ffService(w).ExportFindings(ctx, app.FindingsExportRequest{Findings: ffFindings(), OutPath: "findings.json"})

	if err == nil {
		t.Error("want an error for a cancelled context")
	}
	if len(w.files) != 0 {
		t.Errorf("a cancelled export stored %v", w.files)
	}
}
