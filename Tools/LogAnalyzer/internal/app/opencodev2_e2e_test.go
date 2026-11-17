package app_test

// End-to-end test of opencode-v2 logs with the real scanner, reader and
// transcript adapter on the synthetic runs in testdata/transcript_integrity_v2.
//
// Run 20261110T090000Z-f301 (native mode) holds an orchestrator whose
// transcript is a V2 export, three subagent dispatches (coder#8 and planner#9
// have a folder, reviewer#5 has none), a re-call by session id, and two agent
// folders with V2 export transcripts; planner#9's first prompt names coder#8.
// Run 20261110T100000Z-f302 (Runner mode) has no transcript files at all.
//
// Usage of f301: orchestrator 1,000,000 input / 200,000 output / 500,000 cache
// read, coder#8 400,000 / 100,000, planner#9 100,000 / 50,000, all priced at
// 3.00 input, 0.30 cache read and 15.00 output per million tokens.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/transcript"
)

const (
	v2Logs      = "../../testdata/transcript_integrity_v2/runs"
	v2NativeRun = "20261110T090000Z-f301"
	v2RunnerRun = "20261110T100000Z-f302"
)

func analyzeOpenCodeV2Runs(t *testing.T) domain.Report {
	t.Helper()
	pricing := flatPricing("gpt-6-luna")
	pricing.Provider = "openai"
	store := newFakePricingStore()
	store.table = domain.NewPricingTable([]domain.ModelPricing{pricing})
	svc := app.New(app.Deps{
		Source: logscan.NewScanner(), Reader: logread.NewReader(), Transcripts: transcript.NewReader(),
		Pricing: store, Clock: newFakeClock(), Interaction: &alwaysSkippingInteraction{}, WorkDir: t.TempDir(),
	})

	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: v2Logs})

	if err != nil || res.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze: err=%v outcome=%v", err, res.Outcome)
	}
	return res.Report
}

func assertMoney(t *testing.T, name string, got domain.MoneyValue, wantNanos int64) {
	t.Helper()
	if got.State != domain.MoneyKnown || got.Amount.Nanos() != wantNanos {
		t.Errorf("%s = %+v, want known %d nanodollars", name, got, wantNanos)
	}
}

func TestAnalyze_OpenCodeV2Runs_RaiseNoUnrecognisedHarnessFinding(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	if got := dgFindings(report, domain.FindingUnrecognisedHarness); len(got) != 0 {
		t.Errorf("unrecognised harness findings = %+v, want none for opencode-v2", got)
	}
}

func TestAnalyze_OpenCodeV2NativeRun_UsageIsCountedAndPricedFromTokens(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	run, ok := report.FindRun(v2NativeRun)
	if !ok {
		t.Fatalf("run %s missing from the report", v2NativeRun)
	}
	totals := run.Totals
	if v, _ := totals.Tokens.Input.Value(); v != 1_500_000 {
		t.Errorf("input tokens = %d, want 1500000", v)
	}
	if v, _ := totals.Tokens.Output.Value(); v != 350_000 {
		t.Errorf("output tokens = %d, want 350000", v)
	}
	if v, _ := totals.Tokens.CacheRead.Value(); v != 500_000 {
		t.Errorf("cache read tokens = %d, want 500000", v)
	}
	assertMoney(t, "Money.Input", totals.Money.Input, 4_500_000_000)
	assertMoney(t, "Money.CacheRead", totals.Money.CacheRead, 150_000_000)
	assertMoney(t, "Money.Output", totals.Money.Output, 5_250_000_000)
	assertMoney(t, "Money.Total", totals.Money.Total, 9_900_000_000)
	if !totals.Money.Complete {
		t.Error("every model is priced, so the money must be complete")
	}
	if report.HasUnpricedModels() {
		t.Errorf("UnpricedModels = %v, want none", report.UnpricedModels)
	}
}

func TestAnalyze_OpenCodeV2NativeRun_ReportsMismatchForPlanner9Only(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	got := dgFindings(report, domain.FindingInstancePromptMismatch)

	if len(got) != 1 || got[0].Actor != domain.AgentInstance("planner#9") || got[0].Run != domain.NamedRun(v2NativeRun) {
		t.Errorf("instance prompt mismatch findings = %+v, want exactly one for planner#9 of %s", got, v2NativeRun)
	}
}

func TestAnalyze_OpenCodeV2Runs_ReportNoCaptureGapFinding(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	if got := dgFindings(report, domain.FindingTranscriptCaptureGap); len(got) != 0 {
		t.Errorf("capture gap findings = %+v, want none: the check is claude-code only", got)
	}
}

func TestAnalyze_OpenCodeV2NativeRun_SubagentDispatchWithoutFolderIsReportedOnce(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	got := dgFindings(report, domain.FindingDispatchWithoutInvocation)

	if len(got) != 1 {
		t.Fatalf("dispatch_without_invocation findings = %+v, want exactly one (reviewer#5)", got)
	}
	f := got[0]
	if f.Run != domain.NamedRun(v2NativeRun) || f.Actor != domain.Orchestrator() || !strings.Contains(f.Detail, "reviewer#5") {
		t.Errorf("finding = %+v, want an orchestrator warning for reviewer#5 of %s", f, v2NativeRun)
	}
}

func TestAnalyze_OpenCodeV2Runs_RaiseNoMissingTokenUsageFinding(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	if got := dgFindings(report, domain.FindingMissingTokenUsage); len(got) != 0 {
		t.Errorf("missing_token_usage findings = %+v, want none: opencode-v2 usage comes from usage_record", got)
	}
}

func TestAnalyze_OpenCodeV2RunnerRunWithoutTranscripts_RaisesNoFindings(t *testing.T) {
	report := analyzeOpenCodeV2Runs(t)

	for _, f := range report.Quality.Findings {
		if f.Run == domain.NamedRun(v2RunnerRun) {
			t.Errorf("finding %+v for the Runner-mode run: a missing transcript must raise no finding", f)
		}
	}
	run, ok := report.FindRun(v2RunnerRun)
	if !ok {
		t.Fatalf("run %s missing from the report", v2RunnerRun)
	}
	if v, _ := run.Totals.Tokens.Input.Value(); v != 3_000 {
		t.Errorf("Runner-mode input tokens = %d, want 3000 (orchestrator 1000 + reviewer#2 2000)", v)
	}
}
