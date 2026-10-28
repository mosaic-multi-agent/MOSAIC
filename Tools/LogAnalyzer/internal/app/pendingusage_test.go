package app_test

// Tests for surfacing pending usage in the report and in the per-run total.
//
// Rules under test:
//   - Analyze asks the probe about each named run folder and carries the
//     answer on the run report.
//   - TotalForRun carries the pending count and the unreadable flag on the total.
//   - Without a probe, or when the probe has nothing to report, the figures
//     and the zero-value state are unchanged.

import (
	"context"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func probeWith(state domain.UsageIngestState) *fakeIngestProbe {
	return &fakeIngestProbe{states: map[string]domain.UsageIngestState{ingestTestRunDir: state}}
}

func analyzeIngestRun(t *testing.T, svc *app.Service) domain.RunReport {
	t.Helper()
	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil || result.Outcome != app.OutcomeReport || len(result.Report.Runs) != 1 {
		t.Fatalf("expected a report with one run; err=%v outcome=%v runs=%d", err, result.Outcome, len(result.Report.Runs))
	}
	return result.Report.Runs[0]
}

func TestAnalyze_RunWithPendingUsage_CarriesThePendingCount(t *testing.T) {
	probe := probeWith(domain.UsageIngestState{PendingRecords: 4})
	svc, _ := ingestTestService(t, logsRootSource(), nil, probe)

	run := analyzeIngestRun(t, svc)

	if run.UsageIngest.PendingRecords != 4 || run.UsageIngest.StateUnreadable {
		t.Errorf("UsageIngest = %+v, want 4 pending and readable", run.UsageIngest)
	}
	if len(probe.calls) != 1 || probe.calls[0] != ingestTestRunDir {
		t.Errorf("probe asked about %v, want exactly the run folder %q", probe.calls, ingestTestRunDir)
	}
}

func TestAnalyze_RunWithUnreadableIngestState_CarriesTheUnreadableFlag(t *testing.T) {
	svc, _ := ingestTestService(t, logsRootSource(), nil, probeWith(domain.UsageIngestState{StateUnreadable: true}))

	run := analyzeIngestRun(t, svc)

	if !run.UsageIngest.StateUnreadable || run.UsageIngest.PendingRecords != 0 {
		t.Errorf("UsageIngest = %+v, want unreadable with no pending count", run.UsageIngest)
	}
}

func TestAnalyze_RunWithNothingPending_HasZeroUsageIngestState(t *testing.T) {
	svc, _ := ingestTestService(t, logsRootSource(), nil, probeWith(domain.UsageIngestState{}))

	run := analyzeIngestRun(t, svc)

	if run.UsageIngest != (domain.UsageIngestState{}) {
		t.Errorf("UsageIngest = %+v, want the zero value", run.UsageIngest)
	}
}

func TestAnalyze_PendingUsageDoesNotChangeTheFigures(t *testing.T) {
	probe := probeWith(domain.UsageIngestState{PendingRecords: 4})
	svc, _ := ingestTestService(t, logsRootSource(), nil, probe)

	run := analyzeIngestRun(t, svc)

	if len(probe.calls) == 0 {
		t.Fatal("probe never asked; the figures check below would prove nothing")
	}
	if v, ok := run.Totals.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("run input = (%d, %v), want (1000000, true): pending spans are not counted", v, ok)
	}
}

func TestAnalyze_WithoutProbe_ReportsNoUsageIngestState(t *testing.T) {
	svc, _ := ingestTestService(t, logsRootSource(), nil, nil)

	run := analyzeIngestRun(t, svc)

	if run.UsageIngest != (domain.UsageIngestState{}) {
		t.Errorf("UsageIngest = %+v, want the zero value when the feature is off", run.UsageIngest)
	}
}

func TestTotalForRun_RunWithPendingUsage_CarriesThePendingCount(t *testing.T) {
	svc, _ := ingestTestService(t, logsRootSource(), nil, probeWith(domain.UsageIngestState{PendingRecords: 7}))

	total, outcome, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v outcome=%v", err, outcome)
	}
	if total.PendingUsageRecords != 7 || total.UsageStateUnreadable {
		t.Errorf("total pending = %d unreadable = %v, want 7 and false", total.PendingUsageRecords, total.UsageStateUnreadable)
	}
	if v, ok := total.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("total input = (%d, %v), want (1000000, true): pending spans are not counted", v, ok)
	}
}

func TestTotalForRun_RunWithUnreadableIngestState_CarriesTheFlag(t *testing.T) {
	svc, _ := ingestTestService(t, logsRootSource(), nil, probeWith(domain.UsageIngestState{StateUnreadable: true}))

	total, _, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil {
		t.Fatalf("TotalForRun: %v", err)
	}
	if !total.UsageStateUnreadable || total.PendingUsageRecords != 0 {
		t.Errorf("total pending = %d unreadable = %v, want 0 and true", total.PendingUsageRecords, total.UsageStateUnreadable)
	}
}

func TestTotalForRun_WithoutPendingUsage_HasZeroPendingFigures(t *testing.T) {
	probe := probeWith(domain.UsageIngestState{})
	svc, _ := ingestTestService(t, logsRootSource(), nil, probe)

	total, _, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil {
		t.Fatalf("TotalForRun: %v", err)
	}
	if total.PendingUsageRecords != 0 || total.UsageStateUnreadable {
		t.Errorf("total pending = %d unreadable = %v, want 0 and false", total.PendingUsageRecords, total.UsageStateUnreadable)
	}
}

func TestRunTotalFor_RunReportWithPendingUsage_CarriesItOnTheTotal(t *testing.T) {
	report := domain.Report{Runs: []domain.RunReport{{
		Run:         appTestRunRef,
		UsageIngest: domain.UsageIngestState{PendingRecords: 3},
	}}}

	total, ok := app.RunTotalFor(report, appTestRunID)

	if !ok {
		t.Fatal("RunTotalFor: run not found")
	}
	if total.PendingUsageRecords != 3 || total.UsageStateUnreadable {
		t.Errorf("total pending = %d unreadable = %v, want 3 and false", total.PendingUsageRecords, total.UsageStateUnreadable)
	}
}

func TestRunTotalFor_RunReportWithUnreadableState_CarriesTheFlagOnTheTotal(t *testing.T) {
	report := domain.Report{Runs: []domain.RunReport{{
		Run:         appTestRunRef,
		UsageIngest: domain.UsageIngestState{StateUnreadable: true},
	}}}

	total, _ := app.RunTotalFor(report, appTestRunID)

	if !total.UsageStateUnreadable {
		t.Error("UsageStateUnreadable = false, want true")
	}
}
