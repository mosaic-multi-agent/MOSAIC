package screens_test

// Loads the sample run from its tool-local digest (testdata/digest/sample) through
// the real service, so the table views are tested on a run of 65 dispatches of ten
// agent types with mixed returned status codes. Two of the run's three models are
// priced; the third has no price, so some costs are unpriced.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/pricing"
)

const sampleRunDir = "../../../testdata/digest/sample"

const samplePricingYAML = `
providers:
  github-copilot:
    models:
      claude-sonnet-5.5:
        input: "3.00"
        cache_read: "0.30"
        cache_write_5m: "3.75"
        cache_write_1h: "6.00"
        output: "15.00"
      claude-haiku-5.5:
        input: "1.00"
        cache_read: "0.10"
        cache_write_5m: "1.25"
        cache_write_1h: "2.00"
        output: "5.00"
`

// quietInteraction answers nothing: it never blocks and never asks.
type quietInteraction struct{ domain.Interaction }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC) }

var (
	sampleOnce   sync.Once
	sampleReport domain.Report
	sampleErr    error
)

func loadSampleReport() {
	root, err := os.MkdirTemp("", "sample-run-")
	if err != nil {
		sampleErr = err
		return
	}
	cfg := filepath.Join(root, filepath.FromSlash(pricing.ConfigRelPath))
	if sampleErr = os.MkdirAll(filepath.Dir(cfg), 0o755); sampleErr != nil {
		return
	}
	if sampleErr = os.WriteFile(cfg, []byte(samplePricingYAML), 0o644); sampleErr != nil {
		return
	}
	svc := app.New(app.Deps{
		Source:      logscan.NewScanner(),
		Reader:      logread.NewReader(),
		Pricing:     pricing.NewStore("", root),
		Digests:     digest.NewStore(),
		Clock:       fixedClock{},
		Interaction: quietInteraction{},
		WorkDir:     root,
		ToolVersion: "test",
	})
	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: sampleRunDir})
	if err != nil {
		sampleErr = err
		return
	}
	sampleReport = res.Report
}

// sampleReportOf is the report of the sample run.
func sampleReportOf(t *testing.T) domain.Report {
	t.Helper()
	sampleOnce.Do(loadSampleReport)
	if sampleErr != nil {
		t.Fatalf("load the sample run: %v", sampleErr)
	}
	if len(sampleReport.Runs) != 1 {
		t.Fatalf("sample report has %d runs, want 1", len(sampleReport.Runs))
	}
	return sampleReport
}

// sampleRun is the run report of the sample run.
func sampleRun(t *testing.T) domain.RunReport {
	t.Helper()
	return sampleReportOf(t).Runs[0]
}
