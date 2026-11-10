package app_test

// Regression tests: TotalForRun stays logs-only. A digest file, a digest-only
// directory and a logs root holding digest files give the outcomes and totals
// they give today, and the digest store is never consulted.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
)

func totalFor(t *testing.T, e *digestEnv, path, runID string) (app.RunTotal, app.Outcome) {
	t.Helper()
	total, outcome, err := e.service().TotalForRun(context.Background(), app.Request{ExplicitPath: path}, runID)
	if err != nil {
		t.Fatalf("TotalForRun returned unexpected error: %v", err)
	}
	return total, outcome
}

func requireDigestsUntouched(t *testing.T, e *digestEnv) {
	t.Helper()
	if len(e.digests.discoverCalls) != 0 || len(e.digests.readCalls) != 0 {
		t.Errorf("TotalForRun must not discover or read digests; discover=%v read=%v",
			e.digests.discoverCalls, e.digests.readCalls)
	}
}

func TestTotalForRun_DigestFile_RemainsSourceUnusable(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	total, outcome := totalFor(t, e, dgFilePath, dgRunA)

	if outcome != app.OutcomeSourceUnusable || !reflect.DeepEqual(total, app.RunTotal{}) {
		t.Errorf("TotalForRun = (%+v, %v), want zero total and OutcomeSourceUnusable", total, outcome)
	}
	requireDigestsUntouched(t, e)
}

func TestTotalForRun_DigestOnlyDirectory_RemainsSourceUnusable(t *testing.T) {
	e := newDigestEnv()
	e.digestDir(dgDirPath, digestItem{name: "a.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))})

	total, outcome := totalFor(t, e, dgDirPath, dgRunA)

	if outcome != app.OutcomeSourceUnusable || !reflect.DeepEqual(total, app.RunTotal{}) {
		t.Errorf("TotalForRun = (%+v, %v), want zero total and OutcomeSourceUnusable", total, outcome)
	}
	requireDigestsUntouched(t, e)
}

func TestTotalForRun_LogsRootHoldingDigests_TotalIsFromLogsOnly(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 9_000_000), dgRun(dgRunB, 2_000_000))})

	total, outcome := totalFor(t, e, e.logsPath, dgRunA)

	if outcome != app.OutcomeReport {
		t.Fatalf("outcome = %v, want OutcomeReport", outcome)
	}
	if v, ok := total.Tokens.Input.Value(); !ok || v != 1_000_000 {
		t.Errorf("input = (%d, %v), want the log run's 1000000", v, ok)
	}
	requireDigestsUntouched(t, e)
}

func TestTotalForRun_RunOnlyInDigestInsideLogsRoot_IsNotFoundAsToday(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT2, dgRun(dgRunB, 2_000_000))})

	total, outcome := totalFor(t, e, e.logsPath, dgRunB)

	if outcome != app.OutcomeReport || !reflect.DeepEqual(total, app.RunTotal{}) {
		t.Errorf("TotalForRun = (%+v, %v), want zero total with OutcomeReport (run not among log runs)", total, outcome)
	}
}

func TestTotalForRun_WithDigestStore_EqualsResultWithoutDigestStore(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 9_000_000))})
	withStore, outcomeWith := totalFor(t, e, e.logsPath, dgRunA)

	plain := app.New(app.Deps{
		Source: e.logSource, Reader: e.reader, Pricing: e.pricing,
		Clock: newFakeClock(), Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	})
	without, outcomeWithout, err := plain.TotalForRun(context.Background(), app.Request{ExplicitPath: e.logsPath}, dgRunA)

	if err != nil {
		t.Fatalf("TotalForRun returned unexpected error: %v", err)
	}
	if outcomeWith != outcomeWithout || !reflect.DeepEqual(withStore, without) {
		t.Errorf("results differ with a digest store:\nwith:    %+v (%v)\nwithout: %+v (%v)", withStore, outcomeWith, without, outcomeWithout)
	}
}

func TestTotalForRun_MissingPathWithDigestStore_RemainsSourceNotFound(t *testing.T) {
	e := newDigestEnv()

	_, outcome := totalFor(t, e, "/missing", dgRunA)

	if outcome != app.OutcomeSourceNotFound {
		t.Errorf("outcome = %v, want OutcomeSourceNotFound", outcome)
	}
}
