package app_test

// Tests for loading digest sources through Analyze: a digest file, a directory
// of digests, a mixed directory, the default location, outcomes, ordering and
// report parity with the same run read from logs.

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const (
	dgFilePath = "/digests/run.digest.json"
	dgDirPath  = "/digests/dir"
)

func analyzeDG(t *testing.T, e *digestEnv, explicit string) app.Result {
	t.Helper()
	res, err := e.service().Analyze(context.Background(), app.Request{ExplicitPath: explicit})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}
	return res
}

// analyzeDGReport analyses and requires a report with a live Session.
func analyzeDGReport(t *testing.T, e *digestEnv, explicit string) app.Result {
	t.Helper()
	res := analyzeDG(t, e, explicit)
	if res.Outcome != app.OutcomeReport || res.Session == nil {
		t.Fatalf("Analyze(%q) = outcome %v, session nil=%v; want OutcomeReport with a Session",
			explicit, res.Outcome, res.Session == nil)
	}
	return res
}

func TestAnalyze_DigestFile_ReportsItsRunsWithDigestFileSource(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDG(t, e, dgFilePath)

	if res.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", res.Outcome)
	}
	if res.Source.Kind != domain.SourceDigestFile || res.Source.Path != dgFilePath {
		t.Errorf("Source = %+v, want digest file at %q", res.Source, dgFilePath)
	}
	if got, ok := dgInput(res.Report, dgRunA); !ok || got != 1_000_000 {
		t.Errorf("run %s input = (%d, %v), want (1000000, true)", dgRunA, got, ok)
	}
	if res.Session == nil {
		t.Error("Session must be set so digest runs can be re-priced")
	}
}

func TestAnalyze_DigestOnlyDirectory_MergesAllDigestsSortedByRunID(t *testing.T) {
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "a.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))},
		digestItem{name: "b.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000), dgRun(dgRunC, 3_000_000))},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	if res.Source.Kind != domain.SourceDigestDir {
		t.Errorf("Source.Kind = %v, want SourceDigestDir", res.Source.Kind)
	}
	want := []string{dgRunC, dgRunA, dgRunB}
	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, want) {
		t.Errorf("report run ids = %v, want %v (sorted by id)", got, want)
	}
	if ids := aggRunIDs(res.Session.Aggregate()); !reflect.DeepEqual(ids, want) {
		t.Errorf("session aggregate run ids = %v, want %v", ids, want)
	}
}

func aggRunIDs(a domain.Aggregate) []string {
	var ids []string
	for _, r := range a.Runs {
		ids = append(ids, r.Run.ID)
	}
	return ids
}

func TestAnalyze_MixedDirectory_LoadsLogRunsAndDigestRunsKeepingLogsRootKind(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := analyzeDG(t, e, e.logsPath)

	if res.Source.Kind != domain.SourceLogsRoot {
		t.Errorf("Source.Kind = %v, want SourceLogsRoot (existing kind unchanged for a mixed source)", res.Source.Kind)
	}
	want := []string{dgRunA, dgRunB}
	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, want) {
		t.Errorf("report run ids = %v, want %v", got, want)
	}
}

func TestAnalyze_MixedDirectory_DigestRunSortsAheadOfLogRunWhenIDIsSmaller(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunB, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunC, 2_000_000))})

	res := analyzeDGReport(t, e, e.logsPath)

	want := []string{dgRunC, dgRunB}
	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, want) {
		t.Errorf("report run ids = %v, want %v", got, want)
	}
	if ids := aggRunIDs(res.Session.Aggregate()); !reflect.DeepEqual(ids, want) {
		t.Errorf("session aggregate run ids = %v, want %v", ids, want)
	}
}

func TestAnalyze_DefaultLocation_PicksUpDigestsAtItsTopLevel(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.logSource.defaultFunc = func(_ string) domain.Source {
		return domain.Source{Kind: domain.SourceLogsRoot, Path: e.logsPath}
	}
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res, err := e.service().Analyze(context.Background(), app.Request{})
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}

	if _, ok := res.Report.FindRun(dgRunB); !ok {
		t.Errorf("digest run %s from the default location missing; runs = %v", dgRunB, dgRunIDs(res.Report))
	}
}

func TestAnalyze_UnusableSourceWithoutDigests_StillSourceUnusable(t *testing.T) {
	e := newDigestEnv()
	e.unusable["/plain"] = true

	res := analyzeDG(t, e, "/plain")

	if res.Outcome != app.OutcomeSourceUnusable {
		t.Errorf("Outcome = %v, want OutcomeSourceUnusable", res.Outcome)
	}
}

func TestAnalyze_MissingSourceWithoutDigests_StillSourceNotFound(t *testing.T) {
	e := newDigestEnv()

	res := analyzeDG(t, e, "/missing")

	if res.Outcome != app.OutcomeSourceNotFound {
		t.Errorf("Outcome = %v, want OutcomeSourceNotFound", res.Outcome)
	}
}

func TestAnalyze_EmptyLogsRootWithoutDigests_StillNoData(t *testing.T) {
	e := newDigestEnv()
	e.hasLogs = true

	res := analyzeDG(t, e, e.logsPath)

	if res.Outcome != app.OutcomeNoData {
		t.Errorf("Outcome = %v, want OutcomeNoData", res.Outcome)
	}
}

func TestAnalyze_EmptyLogsRootWithDigest_ReportsDigestRuns(t *testing.T) {
	e := newDigestEnv()
	e.hasLogs = true
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))})

	res := analyzeDG(t, e, e.logsPath)

	if res.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", res.Outcome)
	}
	if _, ok := res.Report.FindRun(dgRunA); !ok {
		t.Errorf("digest run missing; runs = %v", dgRunIDs(res.Report))
	}
}

func TestAnalyze_NilDigestStore_LogsOnlyAsToday(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	svc := app.New(app.Deps{
		Source: e.logSource, Reader: e.reader, Pricing: e.pricing,
		Clock: newFakeClock(), Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	})

	res, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: e.logsPath})

	if err != nil || res.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze = (%v, %v), want OutcomeReport and nil error", res.Outcome, err)
	}
	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, []string{dgRunA}) {
		t.Errorf("run ids = %v, want [%s]", got, dgRunA)
	}
}

func TestAnalyze_DigestRun_PricedFromCurrentPricingAtLoadTime(t *testing.T) {
	e := newDigestEnv() // model-a priced at 3.00 per million input tokens
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 2_000_000)))

	res := analyzeDG(t, e, dgFilePath)

	run, ok := res.Report.FindRun(dgRunA)
	if !ok {
		t.Fatal("digest run missing from report")
	}
	want := domain.KnownMoney(domain.Money(6_000_000_000)) // $6.00
	if !run.Totals.Money.Complete || !reflect.DeepEqual(run.Totals.Money.Total, want) {
		t.Errorf("money = %+v (complete=%v), want %+v complete", run.Totals.Money.Total, run.Totals.Money.Complete, want)
	}
}

func TestAnalyze_DigestRun_PricingFindingsFromPricingFileAreReported(t *testing.T) {
	e := newDigestEnv()
	e.pricing.findings = []domain.Finding{{Kind: domain.FindingInvalidPricingEntry, Severity: domain.SeverityWarning}}
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDG(t, e, dgFilePath)

	if got := dgFindings(res.Report, domain.FindingInvalidPricingEntry); len(got) != 1 {
		t.Errorf("invalid_pricing_entry findings = %d, want 1 (produced at load time)", len(got))
	}
}

func TestAnalyze_DigestRun_UnpricedModelIsListedLikeALogRun(t *testing.T) {
	e := newDigestEnv()
	e.pricing.table = domain.EmptyPricingTable()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDG(t, e, dgFilePath)

	if !res.Report.HasUnpricedModels() {
		t.Error("a digest run with an unpriced model must surface it in Report.UnpricedModels")
	}
}

func TestAnalyze_DigestRunReport_EqualsReportOfSameRunReadFromLogs(t *testing.T) {
	// Arrange: analyse the run from logs, then feed its aggregate back as a digest.
	logEnv := newDigestEnv()
	logEnv.addLogRun(dgRunA, 1_500_000)
	logEnv.addLogRun(dgRunB, 700_000)
	fromLogs := analyzeDGReport(t, logEnv, logEnv.logsPath)

	digEnv := newDigestEnv()
	digEnv.digestFile(dgFilePath, domain.Digest{
		Header:    domain.DigestHeader{Version: "1.0.0", WrittenAt: dgT1},
		Aggregate: fromLogs.Session.Aggregate(),
	})

	// Act
	fromDigest := analyzeDG(t, digEnv, dgFilePath)

	// Assert: same tokens, money and views for the same runs.
	if !reflect.DeepEqual(fromLogs.Report.Runs, fromDigest.Report.Runs) {
		t.Errorf("Runs differ:\nlogs:   %+v\ndigest: %+v", fromLogs.Report.Runs, fromDigest.Report.Runs)
	}
	if !reflect.DeepEqual(fromLogs.Report.AllRuns, fromDigest.Report.AllRuns) {
		t.Errorf("AllRuns differ:\nlogs:   %+v\ndigest: %+v", fromLogs.Report.AllRuns, fromDigest.Report.AllRuns)
	}
	if !reflect.DeepEqual(fromLogs.Report.AcrossRuns, fromDigest.Report.AcrossRuns) {
		t.Errorf("AcrossRuns differ:\nlogs:   %+v\ndigest: %+v", fromLogs.Report.AcrossRuns, fromDigest.Report.AcrossRuns)
	}
	if !reflect.DeepEqual(fromLogs.Report.UnpricedModels, fromDigest.Report.UnpricedModels) {
		t.Errorf("UnpricedModels differ: %v vs %v", fromLogs.Report.UnpricedModels, fromDigest.Report.UnpricedModels)
	}
	if a, b := findingKeys(fromLogs.Report), findingKeys(fromDigest.Report); !reflect.DeepEqual(a, b) {
		t.Errorf("findings differ (ignoring path and detail):\nlogs:   %v\ndigest: %v", a, b)
	}
	if fromLogs.Report.Quality.Incomplete() != fromDigest.Report.Quality.Incomplete() {
		t.Error("incomplete state differs between the log report and the digest report")
	}
}

// findingKeys returns the report's findings as sorted kind/severity/run/actor keys.
func findingKeys(r domain.Report) []string {
	var keys []string
	for _, f := range r.Quality.Findings {
		keys = append(keys, fmt.Sprintf("%s|%v|%+v|%+v", f.Kind, f.Severity, f.Run, f.Actor))
	}
	sort.Strings(keys)
	return keys
}
