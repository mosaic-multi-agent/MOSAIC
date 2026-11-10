package app_test

// Tests for findings carried inside digests when the copy they are scoped to is
// discarded (logs win, or an older digest loses), for a digest inside a
// single-run folder, and for editing prices of a session holding digest runs.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func TestAnalyze_FindingCarriedByDigestCopyLosingToLogs_IsDropped(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{
		name:     "x.digest.json",
		d:        dgDigest(dgT2, dgRun(dgRunA, 9_000_000)),
		findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA))},
	})

	res := analyzeDGReport(t, e, e.logsPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), e.logsPath+"/x.digest.json")
	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 0 {
		t.Errorf("finding scoped to the digest copy that lost to logs must not be reported: %+v", got)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("report must not be incomplete because of a discarded digest copy")
	}
}

func TestAnalyze_FindingCarriedByBucketLosingToNewerDigest_IsDropped(t *testing.T) {
	older := dgDigest(dgT1, dgRun(dgRunA, 100))
	older.Aggregate.Unattributable = dgBucket(1_000_000)
	newer := dgDigest(dgT2, dgRun(dgRunB, 100))
	newer.Aggregate.Unattributable = dgBucket(4_000_000)
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: older,
			findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.UnattributableRun())}},
		digestItem{name: "2.digest.json", d: newer},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	if got := bucketInput(t, res.Report); got != 4_000_000 {
		t.Errorf("bucket input = %d, want 4000000 (newest bucket)", got)
	}
	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 0 {
		t.Errorf("finding scoped to the discarded bucket copy must not be reported: %+v", got)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("report must not be incomplete because of a discarded bucket copy")
	}
}

func TestAnalyze_FindingCarriedByBucketLosingToLogBucket_IsDropped(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 100)
	e.addLogBucket(1_000_000)
	d := dgDigest(dgT2, dgRun(dgRunB, 100))
	d.Aggregate.Unattributable = dgBucket(4_000_000)
	e.digestDir(e.logsPath, digestItem{
		name:     "x.digest.json",
		d:        d,
		findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.UnattributableRun())},
	})

	res := analyzeDGReport(t, e, e.logsPath)

	if got := bucketInput(t, res.Report); got != 1_000_000 {
		t.Errorf("bucket input = %d, want 1000000 (log bucket wins)", got)
	}
	requireOneDuplicate(t, res.Report, domain.UnattributableRun(), e.logsPath+"/x.digest.json")
	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 0 {
		t.Errorf("finding scoped to the digest bucket that lost to logs must not be reported: %+v", got)
	}
}

func TestAnalyze_FindingCarriedByWinningBucket_IsKept(t *testing.T) {
	older := dgDigest(dgT1, dgRun(dgRunA, 100))
	older.Aggregate.Unattributable = dgBucket(1_000_000)
	newer := dgDigest(dgT2, dgRun(dgRunB, 100))
	newer.Aggregate.Unattributable = dgBucket(4_000_000)
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: older},
		digestItem{name: "2.digest.json", d: newer,
			findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.UnattributableRun())}},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 1 {
		t.Errorf("finding scoped to the winning bucket count = %d, want 1", len(got))
	}
	if !res.Report.Quality.Incomplete() {
		t.Error("a kept data-loss finding must make the report incomplete")
	}
}

func TestAnalyze_SingleRunFolderHoldingDigest_KeepsSingleRunKindAndLoadsDigestRun(t *testing.T) {
	e := newDigestEnv()
	runDir := e.logsPath + "/" + dgRunA
	orch := runDir + "/00_orchestrator_events.jsonl"
	e.reader.addEvents(orch, appTestTurnEvent(dgModel, domain.TokenUsage{Input: domain.Tokens(1_000_000)}), appTestRunEndEvent())
	e.logSource.classifyFunc = func(p string) domain.Source {
		if p == runDir {
			return domain.Source{Kind: domain.SourceSingleRun, Path: p}
		}
		return domain.Source{Kind: domain.SourceNotFound, Path: p}
	}
	e.logSource.enumerateFunc = func(src domain.Source) (domain.Inventory, []domain.Finding) {
		return domain.Inventory{
			Source: src,
			Runs:   []domain.RunEntry{{Run: domain.NamedRun(dgRunA), Dir: runDir, OrchestratorFile: orch}},
		}, nil
	}
	e.digestDir(runDir, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := analyzeDGReport(t, e, runDir)

	if res.Source.Kind != domain.SourceSingleRun {
		t.Errorf("Source.Kind = %v, want SourceSingleRun (existing kind unchanged)", res.Source.Kind)
	}
	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
}

func TestEditModelPricing_DigestRun_EnteredPricesPriceTheRun(t *testing.T) {
	e := unpricedDigestEnv()
	res := analyzeDGReport(t, e, dgFilePath)
	svc := e.withInteraction(allRateAnsweringIA("3.00")).service()

	result, err := svc.EditModelPricing(context.Background(), res.Session, res.Report, domain.PricingKey{Model: dgModel})

	if err != nil {
		t.Fatalf("EditModelPricing returned unexpected error: %v", err)
	}
	if result.Outcome != app.PricingEditSaved {
		t.Fatalf("Outcome = %v, want PricingEditSaved", result.Outcome)
	}
	run, ok := result.Report.FindRun(dgRunA)
	if !ok {
		t.Fatal("digest run missing from the re-priced report")
	}
	want := domain.KnownMoney(domain.Money(6_000_000_000))
	if !run.Totals.Money.Complete || !reflect.DeepEqual(run.Totals.Money.Total, want) {
		t.Errorf("money = %+v complete=%v, want %+v complete", run.Totals.Money.Total, run.Totals.Money.Complete, want)
	}
}
