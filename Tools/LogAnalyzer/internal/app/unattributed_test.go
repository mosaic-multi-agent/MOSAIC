package app_test

// Tests for reading the run-level unattributed stream and counting it in the
// analysis and the per-run total.
//
// Rules under test:
//   - The file named by RunEntry.UnattributedFile is read once, as the run's own
//     unattributed stream (not as orchestrator or agent stream).
//   - Its usage_records are part of the run report and of TotalForRun's total.
//   - Reading findings of that file reach the quality summary.
//   - A run without the file reads and reports exactly what it did before.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	appTestUnattrFile  = "/test-logs/20260101T000000Z-abcd/00_unattributed_events.jsonl"
	appTestUnattrModel = domain.ModelID("model-a")
)

func appTestUsageRecord(recordID string, input int64) domain.Event {
	usage := domain.TokenUsage{Input: domain.Tokens(input)}
	return domain.Event{
		Type: domain.EventUsageRecord,
		UsageRecord: &domain.UsageRecordFields{
			RecordID: recordID, Model: appTestUnattrModel, Usage: usage, HasUsage: true,
		},
	}
}

// unattributedService wires one run with an orchestrator usage_record of 1M
// input tokens and, when withUnattributed, an unattributed stream with one
// record of 500K input tokens. Model-a costs $3.00 per million input tokens.
func unattributedService(t *testing.T, withUnattributed bool) (*app.Service, *fakeEventReader) {
	t.Helper()
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source {
			return domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
		},
		enumerateFunc: func(src domain.Source) (domain.Inventory, []domain.Finding) {
			entry := domain.RunEntry{
				Run: appTestRunRef, Dir: "/test-logs/20260101T000000Z-abcd", OrchestratorFile: appTestOrchFile,
			}
			if withUnattributed {
				entry.UnattributedFile = appTestUnattrFile
			}
			return domain.Inventory{Source: src, Runs: []domain.RunEntry{entry}}, nil
		},
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile, appTestUsageRecord("o-1", 1_000_000), appTestRunEndEvent())
	reader.addEvents(appTestUnattrFile, appTestUsageRecord("u-1", 500_000))
	store := newFakePricingStore()
	store.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing(appTestUnattrModel)})
	svc := app.New(app.Deps{
		Source: logSource, Reader: reader, Pricing: store, Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	})
	return svc, reader
}

func analyzeUnattributedRun(t *testing.T, svc *app.Service) domain.RunReport {
	t.Helper()
	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil || result.Outcome != app.OutcomeReport || len(result.Report.Runs) != 1 {
		t.Fatalf("expected a report with one run; err=%v outcome=%v runs=%d", err, result.Outcome, len(result.Report.Runs))
	}
	return result.Report.Runs[0]
}

func TestAnalyze_RunWithUnattributedFile_ReportsUnattributedUsageSeparately(t *testing.T) {
	svc, _ := unattributedService(t, true)

	run := analyzeUnattributedRun(t, svc)

	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the usage of the unattributed file")
	}
	if v, ok := run.Unattributed.Totals.Tokens.Input.Value(); !ok || v != 500_000 {
		t.Errorf("unattributed input = (%d, %v), want (500000, true)", v, ok)
	}
	if v, ok := run.Orchestrator.Totals.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("orchestrator input = (%d, %v), want (1000000, true): unattributed usage must not be folded in", v, ok)
	}
}

func TestAnalyze_RunWithUnattributedFile_RunTotalIncludesIt(t *testing.T) {
	svc, _ := unattributedService(t, true)

	run := analyzeUnattributedRun(t, svc)

	if v, ok := run.Totals.Tokens.Input.Value(); !ok || v != 1_500_000 {
		t.Errorf("run input = (%d, %v), want (1500000, true)", v, ok)
	}
	if got := run.Totals.Money.Total.Amount.Nanos(); got != 4_500_000_000 {
		t.Errorf("run money = %d nanos, want 4500000000 ($4.50 for 1.5M input tokens)", got)
	}
}

func TestAnalyze_RunWithUnattributedFile_ReadsTheFileOnceWithoutFindings(t *testing.T) {
	svc, reader := unattributedService(t, true)

	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if n := reader.readCounts[appTestUnattrFile]; n != 1 {
		t.Errorf("unattributed file read %d times, want 1", n)
	}
	if !result.Report.Quality.IsClean() {
		t.Errorf("quality findings = %+v, want none: unattributed usage needs no agent_instance_id", result.Report.Quality.Findings)
	}
}

func TestAnalyze_UnattributedFileFindings_ReachTheQualitySummary(t *testing.T) {
	svc, reader := unattributedService(t, true)
	reader.addFindings(appTestUnattrFile, domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Path: appTestUnattrFile, Line: 2,
	})

	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if n := result.Report.Quality.Counts[domain.FindingMalformedLine]; n != 1 {
		t.Errorf("malformed_line findings = %d, want 1 from the unattributed file", n)
	}
}

func TestAnalyze_RunWithoutUnattributedFile_HasNoUnattributedUsage(t *testing.T) {
	svc, reader := unattributedService(t, false)

	run := analyzeUnattributedRun(t, svc)

	if run.Unattributed != nil {
		t.Errorf("run.Unattributed = %+v, want nil", run.Unattributed)
	}
	if n := reader.readCounts[appTestUnattrFile]; n != 0 {
		t.Errorf("a file the inventory did not name was read %d times", n)
	}
	if v, ok := run.Totals.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("run input = (%d, %v), want (1000000, true)", v, ok)
	}
}

func TestTotalForRun_RunWithUnattributedFile_TotalIncludesUnattributedUsage(t *testing.T) {
	svc, _ := unattributedService(t, true)

	total, outcome, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v outcome=%v", err, outcome)
	}
	if v, ok := total.Tokens.Input.Value(); !ok || v != 1_500_000 {
		t.Errorf("total input = (%d, %v), want (1500000, true)", v, ok)
	}
	if total.Money.State != domain.MoneyKnown || total.Money.Amount.Nanos() != 4_500_000_000 {
		t.Errorf("total money = %+v, want known 4500000000 nanos", total.Money)
	}
	if !total.Complete {
		t.Error("total.Complete = false, want true: every model is priced")
	}
}

func TestTotalForRun_RunWithoutUnattributedFile_TotalIsUnchanged(t *testing.T) {
	svc, _ := unattributedService(t, false)

	total, _, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil {
		t.Fatalf("TotalForRun: %v", err)
	}
	if v, ok := total.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("total input = (%d, %v), want (1000000, true)", v, ok)
	}
	if total.UnknownRunMerged != 0 || total.UnknownRunResidual != 0 {
		t.Errorf("merge counters = %d/%d, want 0/0", total.UnknownRunMerged, total.UnknownRunResidual)
	}
}
