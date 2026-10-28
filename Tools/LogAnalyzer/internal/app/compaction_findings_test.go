package app_test

// Tests that Analyze carries the compaction mismatch findings returned by
// pricing into the report's quality summary.

import (
	"context"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// compactionUsageEvent is a per-call usage record at a fixed time whose
// context equals input. Cache read and cache write are present (zero) so the
// call is not partial and can take part in a detected compaction.
func compactionUsageEvent(recordID string, sec int, input int64) domain.Event {
	ev := mergeTestUsageRecordEvent(recordID, "model-a", domain.TokenUsage{
		Input:         domain.Tokens(input),
		CacheRead:     domain.Tokens(0),
		CacheCreation: domain.Tokens(0),
	})
	ev.Timestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(sec) * time.Second)
	return ev
}

// newCompactionTestService builds a service over a one-run source whose
// orchestrator drops from 200000 to 80000 context with no logged compaction.
func newCompactionTestService() *app.Service {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			inv := oneRunTwoAgentInventory(s)
			inv.Runs[0].Agents = nil
			return inv, nil
		},
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile,
		compactionUsageEvent("r1", 0, 200000),
		compactionUsageEvent("r2", 60, 80000),
		appTestRunEndEvent())
	pricing := newFakePricingStore()
	pricing.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing("model-a")})

	return app.New(app.Deps{
		Source:      logSource,
		Reader:      reader,
		Pricing:     pricing,
		Clock:       newFakeClock(),
		Interaction: &alwaysSkippingInteraction{},
		WorkDir:     "/workdir",
	})
}

// TotalForRun does not surface pricing findings: a detected compaction with no
// logged one must leave the outcome, the error and the run total untouched.
func TestTotalForRun_ContextDropWithNoLoggedCompaction_TotalOutcomeAndErrorAreUnaffected(t *testing.T) {
	svc := newCompactionTestService()

	total, outcome, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun: err=%v outcome=%v", err, outcome)
	}
	if v, ok := total.Tokens.Input.Value(); !ok || v != 280000 {
		t.Errorf("total input = (%d, %v), want (280000, true)", v, ok)
	}
	if !total.Complete {
		t.Error("total.Complete = false, want true: the mismatch finding must not change the total")
	}
}

func TestAnalyze_ContextDropWithNoLoggedCompaction_CarriesTheMismatchFindingIntoTheQualitySummary(t *testing.T) {
	svc := newCompactionTestService()

	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})

	if err != nil || result.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze: err=%v outcome=%v", err, result.Outcome)
	}
	if !hasFindingKindApp(result.Report.Quality.Findings, domain.FindingCompactionMismatch) {
		t.Errorf("quality findings = %+v, want a compaction mismatch finding", result.Report.Quality.Findings)
	}
}
