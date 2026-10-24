package app_test

// Tests for digest load problems (other major version, invalid file) and for the
// separation of load-time findings from findings carried inside digests.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const dgBadPath = "/digests/dir/bad.digest.json"

func loadFinding(kind domain.FindingKind, path string) domain.Finding {
	return domain.Finding{Kind: kind, Severity: domain.SeverityWarning, Path: path}
}

// dirWithBadAndGood scripts a digest directory with one failing file (the store
// returns a zero Digest plus the given finding) and one good digest.
func dirWithBadAndGood(bad domain.Finding) *digestEnv {
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "bad.digest.json", findings: []domain.Finding{bad}},
		digestItem{name: "good.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))},
	)
	return e
}

func TestAnalyze_DigestOfOtherMajorVersion_FindingRaisedAndOtherDigestsLoad(t *testing.T) {
	e := dirWithBadAndGood(loadFinding(domain.FindingUnsupportedDigestVersion, dgBadPath))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	got := dgFindings(res.Report, domain.FindingUnsupportedDigestVersion)
	if len(got) != 1 || got[0].Path != dgBadPath {
		t.Errorf("unsupported_digest_version findings = %+v, want one for %q", got, dgBadPath)
	}
	if !res.Report.Quality.Incomplete() {
		t.Error("a skipped digest is data loss: the report must be incomplete")
	}
}

func TestAnalyze_InvalidDigest_FindingRaisedAndOtherDigestsLoad(t *testing.T) {
	e := dirWithBadAndGood(loadFinding(domain.FindingInvalidDigest, dgBadPath))

	res := analyzeDGReport(t, e, dgDirPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	if got := dgFindings(res.Report, domain.FindingInvalidDigest); len(got) != 1 {
		t.Errorf("invalid_digest findings = %d, want 1", len(got))
	}
	if !res.Report.Quality.Incomplete() {
		t.Error("an unreadable digest is data loss: the report must be incomplete")
	}
}

func TestAnalyze_OtherMajorDigestAlongsideLogs_LogRunsStillLoad(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.digestDir(e.logsPath, digestItem{name: "bad.digest.json",
		findings: []domain.Finding{loadFinding(domain.FindingUnsupportedDigestVersion, e.logsPath+"/bad.digest.json")}})

	res := analyzeDGReport(t, e, e.logsPath)

	requireInput(t, res.Report, dgRunA, 1_000_000)
	if got := dgFindings(res.Report, domain.FindingUnsupportedDigestVersion); len(got) != 1 {
		t.Errorf("unsupported_digest_version findings = %d, want 1", len(got))
	}
}

func TestAnalyze_OnlyUnreadableDigests_ReportsTheFindingsWithoutRuns(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, domain.Digest{}, loadFinding(domain.FindingInvalidDigest, dgFilePath))

	res := analyzeDG(t, e, dgFilePath)

	if res.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport so the load findings are shown", res.Outcome)
	}
	if len(res.Report.Runs) != 0 {
		t.Errorf("runs = %v, want none", dgRunIDs(res.Report))
	}
	if got := dgFindings(res.Report, domain.FindingInvalidDigest); len(got) != 1 {
		t.Errorf("invalid_digest findings = %d, want 1", len(got))
	}
}

func TestAnalyze_LoadTimeAndCarriedFindings_RemainDistinguishableByKind(t *testing.T) {
	carried := dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA))
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "1.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))},
		digestItem{name: "2.digest.json", d: dgDigest(dgT2, dgRun(dgRunA, 2_000_000)), findings: []domain.Finding{carried}},
		digestItem{name: "3.digest.json", findings: []domain.Finding{loadFinding(domain.FindingInvalidDigest, dgDirPath+"/3.digest.json")}},
	)

	res := analyzeDGReport(t, e, dgDirPath)

	// The carried finding sits on the winning copy and shows beside both load-time findings.
	counts := res.Report.Quality.Counts
	if counts[domain.FindingMalformedLine] != 1 || counts[domain.FindingDuplicateRun] != 1 || counts[domain.FindingInvalidDigest] != 1 {
		t.Errorf("counts = malformed_line %d, duplicate_run %d, invalid_digest %d; want 1, 1 and 1",
			counts[domain.FindingMalformedLine], counts[domain.FindingDuplicateRun], counts[domain.FindingInvalidDigest])
	}
	for _, kind := range []domain.FindingKind{domain.FindingMalformedLine, domain.FindingDuplicateRun, domain.FindingInvalidDigest} {
		if len(dgFindings(res.Report, kind)) != 1 {
			t.Errorf("findings of kind %s = %d, want 1 in the report", kind, len(dgFindings(res.Report, kind)))
		}
	}
}

func TestAnalyze_CarriedFindingKeepsKindSeverityRunAndActor(t *testing.T) {
	carried := domain.Finding{
		Kind: domain.FindingMissingTokenUsage, Severity: domain.SeverityInfo,
		Run: domain.NamedRun(dgRunA), Actor: domain.AgentInstance("Agent#1"),
	}
	e := newDigestEnv()
	e.digestFile(dgFilePath, dgDigest(dgT1, dgRun(dgRunA, 1_000_000)), carried)

	res := analyzeDGReport(t, e, dgFilePath)

	got := dgFindings(res.Report, domain.FindingMissingTokenUsage)
	if len(got) != 1 || !reflect.DeepEqual(got[0], carried) {
		t.Errorf("carried finding = %+v, want it unchanged: %+v", got, carried)
	}
}

func TestAnalyze_LogFindingWithNoRun_KeepsNoRunWhenDigestsAreLoaded(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	orch := e.logsPath + "/" + dgRunA + "/00_orchestrator_events.jsonl"
	e.reader.addFindings(orch, domain.Finding{Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Path: orch, Line: 3})
	e.digestDir(e.logsPath, digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := analyzeDGReport(t, e, e.logsPath)

	got := dgFindings(res.Report, domain.FindingMalformedLine)
	if len(got) != 1 || got[0].Run != (domain.RunRef{}) {
		t.Errorf("log finding = %+v, want it reported with no run, exactly as raised", got)
	}
}
