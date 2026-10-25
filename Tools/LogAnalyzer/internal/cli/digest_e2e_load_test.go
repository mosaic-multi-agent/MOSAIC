package cli_test

// End-to-end loading of hand-written digests: an older digest with facts
// missing, a different major version, duplicate run ids, and a merge.

import (
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func TestDigestLoad_OlderDigestWithMissingFact_LoadsAndShowsNotProvided(t *testing.T) {
	report := e2eReport(t, e2eFixture("older", "older.digest.json"))

	runs := e2eRunsOf(report)

	run := runs["20261019T080000Z-0a1d"]
	if run == nil {
		t.Fatalf("the older digest's run must load; got %v", runs)
	}
	if v := e2eTokens(run, "input"); v != float64(120) {
		t.Errorf("input = %v, want 120", v)
	}
	for _, missing := range []string{"cache_read", "cache_creation", "reasoning"} {
		if v := e2eTokens(run, missing); v != nil {
			t.Errorf("%s = %v, want null (not provided)", missing, v)
		}
	}
	if n := len(e2eFindingsOfKind(report, domain.FindingInvalidDigest)); n != 0 {
		t.Errorf("an older digest must not be reported invalid; got %d finding(s)", n)
	}
}

func TestDigestLoad_DifferentMajor_ProducesFindingAndOtherSourcesLoad(t *testing.T) {
	report := e2eReport(t, e2eFixture("major"))

	runs := e2eRunsOf(report)

	if n := len(e2eFindingsOfKind(report, domain.FindingUnsupportedDigestVersion)); n != 1 {
		t.Errorf("unsupported_digest_version findings = %d, want 1", n)
	}
	if runs["20261019T100000Z-0b0b"] == nil {
		t.Errorf("the supported digest must still load; got %v", runs)
	}
	if runs["20261019T090000Z-f00d"] != nil {
		t.Error("the different-major digest must contribute no run")
	}
	if !e2eIncomplete(report) {
		t.Error("a skipped digest must mark the report incomplete")
	}
}

func TestDigestLoad_DuplicateRunId_YieldsOneRunAndOneFinding(t *testing.T) {
	report := e2eReport(t, e2eFixture("dups"))

	runs := e2eRunsOf(report)

	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if n := len(e2eFindingsOfKind(report, domain.FindingDuplicateRun)); n != 1 {
		t.Errorf("duplicate_run findings = %d, want 1", n)
	}
	if v := e2eTokens(runs["20261019T110000Z-d0d0"], "input"); v != float64(22) {
		t.Errorf("input = %v, want 22 from the newest digest", v)
	}
}

func TestDigestMerge_ExportFromDirectoryOfDigests_HoldsTheirRuns(t *testing.T) {
	merged := e2eExport(t, e2eFixture("merge"), "")

	report := e2eReport(t, merged)

	runs := e2eRunsOf(report)
	for _, id := range []string{"20261018T080000Z-0001", "20261018T090000Z-0002", "20261018T100000Z-0003"} {
		if runs[id] == nil {
			t.Errorf("merged digest is missing run %s; got %v", id, runs)
		}
	}
	if len(runs) != 3 {
		t.Errorf("runs = %d, want 3", len(runs))
	}
	if report["unattributable"] == nil {
		t.Error("the merged digest must keep the unattributable bucket")
	}
	if n := len(e2eFindingsOfKind(report, domain.FindingDuplicateRun)); n != 0 {
		t.Errorf("distinct runs must not produce duplicate findings; got %d", n)
	}
}

func TestDigestExport_RefusesToOverwriteWithoutForce(t *testing.T) {
	first := e2eExport(t, e2eFixture("merge"), "")
	before := e2eReadFile(t, first)

	code, _, _ := e2eRun(t, "export", "--path", e2eFixture("older", "older.digest.json"), "--out", first)

	if code != cli.ExitUsage {
		t.Errorf("exit = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if after := e2eReadFile(t, first); after != before {
		t.Error("the existing digest must stay untouched without --force")
	}
	code, _, _ = e2eRun(t, "export", "--path", e2eFixture("older", "older.digest.json"), "--out", first, "--force")
	if code != cli.ExitSuccess {
		t.Fatalf("exit with --force = %d, want ExitSuccess", code)
	}
	if after := e2eReadFile(t, first); after == before {
		t.Error("--force must replace the existing digest")
	}
}
