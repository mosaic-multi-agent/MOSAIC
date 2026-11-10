package app_test

// Tests for duplicate run ids and duplicate unattributable buckets across logs
// and digests: logs win over digests, the newest digest wins among digests, a
// missing written-at ranks oldest, ties go to the first file in Discover order,
// and every discarded copy yields exactly one duplicate_run finding.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const (
	dgFile1 = "/digests/dir/1.digest.json"
	dgFile2 = "/digests/dir/2.digest.json"
	dgFile3 = "/digests/dir/3.digest.json"
)

// twoDigests scripts a digest directory holding two digests that both contain run A.
func twoDigests(first, second domain.Digest) *digestEnv {
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: first},
		digestItem{name: "2.digest.json", d: second},
	)
	return e
}

func requireOneDuplicate(t *testing.T, r domain.Report, run domain.RunRef, discardedPath string) {
	t.Helper()
	dups := dgFindings(r, domain.FindingDuplicateRun)
	if len(dups) != 1 {
		t.Fatalf("duplicate_run findings = %d, want exactly 1: %+v", len(dups), dups)
	}
	if dups[0].Run != run || dups[0].Path != discardedPath || dups[0].Severity != domain.SeverityInfo {
		t.Errorf("duplicate finding = %+v, want Info, Run %+v, Path %q", dups[0], run, discardedPath)
	}
}

func requireInput(t *testing.T, r domain.Report, id string, want int64) {
	t.Helper()
	if got, ok := dgInput(r, id); !ok || got != want {
		t.Errorf("run %s input = (%d, %v), want (%d, true)", id, got, ok, want)
	}
}

func TestAnalyze_RunInLogsAndDigest_LogDataWinsWithOneFinding(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 9_000_000))})

	res := analyzeDGReport(t, e, e.logsPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	if got := dgRunIDs(res.Report); !reflect.DeepEqual(got, []string{dgRunA}) {
		t.Errorf("run ids = %v, want a single run", got)
	}
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), e.logsPath+"/x.digest.json")
	if res.Report.Quality.Incomplete() {
		t.Error("a duplicate is informational and must not make the report incomplete")
	}
}

func TestAnalyze_RunInLogsAndTwoDigests_OneFindingPerDiscardedDigest(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath,
		digestItem{name: "1.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 2_000_000))},
		digestItem{name: "2.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 3_000_000))},
	)

	res := analyzeDGReport(t, e, e.logsPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	paths := map[string]bool{}
	for _, f := range dgFindings(res.Report, domain.FindingDuplicateRun) {
		paths[f.Path] = true
	}
	d1, d2 := e.logsPath+"/1.digest.json", e.logsPath+"/2.digest.json"
	if len(paths) != 2 || !paths[d1] || !paths[d2] {
		t.Errorf("duplicate findings name paths %v, want the two digests %q and %q", paths, d1, d2)
	}
}

func TestAnalyze_RunInTwoDigests_NewestWrittenAtWins(t *testing.T) {
	e := twoDigests(dgDigest(dgT1, dgRun(dgRunA, 1_000_000)), dgDigest(dgT2, dgRun(dgRunA, 7_000_000)))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 7_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), dgFile1)
}

func TestAnalyze_RunInTwoDigests_NewestWinsRegardlessOfDiscoverOrder(t *testing.T) {
	e := twoDigests(dgDigest(dgT2, dgRun(dgRunA, 7_000_000)), dgDigest(dgT1, dgRun(dgRunA, 1_000_000)))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 7_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), dgFile2)
}

func TestAnalyze_RunInTwoDigestsWithEqualWrittenAt_FirstInDiscoverOrderWins(t *testing.T) {
	e := twoDigests(dgDigest(dgT1, dgRun(dgRunA, 1_000_000)), dgDigest(dgT1, dgRun(dgRunA, 7_000_000)))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), dgFile2)
}

func TestAnalyze_RunInTwoDigests_MissingWrittenAtRanksOldest(t *testing.T) {
	// The undated digest comes first in Discover order, so only the oldest-rank
	// rule (not the tie rule) lets the dated digest win.
	e := twoDigests(dgDigest(time.Time{}, dgRun(dgRunA, 1_000_000)), dgDigest(dgT1, dgRun(dgRunA, 7_000_000)))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 7_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), dgFile1)
}

func TestAnalyze_RunInTwoDigestsBothUndated_FirstInDiscoverOrderWins(t *testing.T) {
	e := twoDigests(dgDigest(time.Time{}, dgRun(dgRunA, 1_000_000)), dgDigest(time.Time{}, dgRun(dgRunA, 7_000_000)))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	requireOneDuplicate(t, res.Report, domain.NamedRun(dgRunA), dgFile2)
}

func TestAnalyze_RunInThreeDigests_NewestWinsAndTwoFindingsRaised(t *testing.T) {
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))},
		digestItem{name: "2.digest.json", d: dgDigest(dgT2.Add(time.Hour), dgRun(dgRunA, 5_000_000))},
		digestItem{name: "3.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 3_000_000))},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 5_000_000)
	paths := map[string]bool{}
	for _, f := range dgFindings(res.Report, domain.FindingDuplicateRun) {
		paths[f.Path] = true
	}
	if len(paths) != 2 || !paths[dgFile1] || !paths[dgFile3] {
		t.Errorf("duplicate findings name paths %v, want the two discarded digests %q and %q", paths, dgFile1, dgFile3)
	}
}

func TestAnalyze_DuplicateOnlyAffectsTheDuplicatedRun(t *testing.T) {
	e := twoDigests(
		dgDigest(dgT1, dgRun(dgRunA, 1_000_000), dgRun(dgRunB, 2_000_000)),
		dgDigest(dgT2, dgRun(dgRunA, 7_000_000)),
	)

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 7_000_000)
	requireInput(t, res.Report, dgRunB, 2_000_000)
}

func TestAnalyze_FindingCarriedByDiscardedRunCopy_IsDropped(t *testing.T) {
	loserFinding := dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA))
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000)), findings: []domain.Finding{loserFinding}},
		digestItem{name: "2.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 7_000_000))},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 0 {
		t.Errorf("finding scoped to the discarded copy must not be reported: %+v", got)
	}
	if res.Report.Quality.Incomplete() {
		t.Error("report must not be incomplete because of a discarded copy")
	}
}

func TestAnalyze_FindingCarriedByWinningCopyAndGeneralFindingOfLoser_AreKept(t *testing.T) {
	general := domain.Finding{Kind: domain.FindingUnrecognisedFolder, Severity: domain.SeverityWarning}
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000)), findings: []domain.Finding{general}},
		digestItem{name: "2.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 7_000_000)),
			findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA))}},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	if got := dgFindings(res.Report, domain.FindingMalformedLine); len(got) != 1 {
		t.Errorf("winner's run-scoped finding count = %d, want 1", len(got))
	}
	if got := dgFindings(res.Report, domain.FindingUnrecognisedFolder); len(got) != 1 {
		t.Errorf("general finding of the discarded digest count = %d, want 1 (not scoped to a discarded copy)", len(got))
	}
}

// ---------------------------------------------------------------------------
// Unattributable buckets follow the same rule and are never combined.
// ---------------------------------------------------------------------------

func bucketInput(t *testing.T, r domain.Report) int64 {
	t.Helper()
	if r.Unattributable == nil {
		t.Fatal("report has no unattributable bucket")
	}
	v, _ := r.Unattributable.Totals.Tokens.Input.Value()
	return v
}

func TestAnalyze_TwoDigestsWithBuckets_NewestBucketWinsOthersNotCombined(t *testing.T) {
	older := dgDigest(dgT1, dgRun(dgRunA, 100))
	older.Aggregate.Unattributable = dgBucket(1_000_000)
	newer := dgDigest(dgT2, dgRun(dgRunB, 100))
	newer.Aggregate.Unattributable = dgBucket(4_000_000)
	e := twoDigests(older, newer)

	res := analyzeDGReport(t, e, dgDirPath)

	if got := bucketInput(t, res.Report); got != 4_000_000 {
		t.Errorf("bucket input = %d, want 4000000 (newest bucket, not combined)", got)
	}
	requireOneDuplicate(t, res.Report, domain.UnattributableRun(), dgFile1)
	if got := dgRunIDs(res.Report); len(got) != 2 {
		t.Errorf("run ids = %v, want the bucket kept out of the named runs", got)
	}
}

func TestAnalyze_BucketInLogsAndDigest_LogBucketWins(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 100)
	e.addLogBucket(1_000_000)
	d := dgDigest(dgT2, dgRun(dgRunB, 100))
	d.Aggregate.Unattributable = dgBucket(4_000_000)
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: d})

	res := analyzeDGReport(t, e, e.logsPath)

	if got := bucketInput(t, res.Report); got != 1_000_000 {
		t.Errorf("bucket input = %d, want 1000000 (log bucket wins)", got)
	}
	requireOneDuplicate(t, res.Report, domain.UnattributableRun(), e.logsPath+"/x.digest.json")
}

func TestAnalyze_BucketInOnlyOneDigest_NoDuplicateFinding(t *testing.T) {
	withBucket := dgDigest(dgT1, dgRun(dgRunA, 100))
	withBucket.Aggregate.Unattributable = dgBucket(2_000_000)
	e := twoDigests(withBucket, dgDigest(dgT2, dgRun(dgRunB, 100)))

	res := analyzeDGReport(t, e, dgDirPath)

	if got := bucketInput(t, res.Report); got != 2_000_000 {
		t.Errorf("bucket input = %d, want 2000000", got)
	}
	if got := dgFindings(res.Report, domain.FindingDuplicateRun); len(got) != 0 {
		t.Errorf("unexpected duplicate findings: %+v", got)
	}
}
