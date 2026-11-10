package cli_test

// End-to-end round trip: export from the fixture logs, then report on the
// digest, and compare with the report on the logs.

import "testing"

func TestDigestRoundTrip_ReportOnDigestEqualsReportOnLogs(t *testing.T) {
	logs := e2eFixture("logs")
	logReport := e2eReport(t, logs)
	digestPath := e2eExport(t, logs, "")

	digestReport := e2eReport(t, digestPath)

	// The comparison must not be vacuous: the fixture has runs, a bucket and findings.
	if len(e2eRunsOf(logReport)) != 4 {
		t.Fatalf("fixture logs should report 4 named runs, got %d", len(e2eRunsOf(logReport)))
	}
	if logReport["unattributable"] == nil {
		t.Fatal("fixture logs should report an unattributable bucket")
	}
	if len(e2eFindingsOf(logReport)) < 2 {
		t.Fatalf("fixture logs should carry findings, got %d", len(e2eFindingsOf(logReport)))
	}
	src := digestReport["source"].(map[string]any)
	if src["kind"] != "digest_file" {
		t.Fatalf("the digest report must be loaded from the digest file, got source kind %v", src["kind"])
	}

	runless := e2eRunlessKinds(logReport)
	want := e2eNormalised(logReport, runless)
	got := e2eNormalised(digestReport, runless)
	if d := e2eFirstDiff("report", want, got); d != "" {
		t.Errorf("digest report differs from log report at %s", d)
	}
}

func TestDigestRoundTrip_SingleRunExportEqualsThatRunInLogReport(t *testing.T) {
	logs := e2eFixture("logs")
	logRuns := e2eRunsOf(e2eReport(t, logs))
	digestPath := e2eExport(t, logs, e2eRichRun)

	digestRuns := e2eRunsOf(e2eReport(t, digestPath))

	if len(digestRuns) != 1 {
		t.Fatalf("a --run export must hold exactly one run, got %d", len(digestRuns))
	}
	if d := e2eFirstDiff("run", logRuns[e2eRichRun], digestRuns[e2eRichRun]); d != "" {
		t.Errorf("exported run differs from the log run at %s", d)
	}
}

func TestDigestRoundTrip_ExportedFileKeepsLegacyAndPerRecordUsage(t *testing.T) {
	logs := e2eFixture("logs")
	digestRuns := e2eRunsOf(e2eReport(t, e2eExport(t, logs, "")))

	rich, legacy := digestRuns[e2eRichRun], digestRuns[e2eLegacyRun]

	if rich == nil || legacy == nil {
		t.Fatalf("both runs must survive the export; got %v", digestRuns)
	}
	// Per-record run: orchestrator o1+o2 input 110 plus worker 500 plus reviewer 40.
	if v := e2eTokens(rich, "input"); v != float64(650) {
		t.Errorf("rich run input = %v, want 650", v)
	}
	// Legacy run: the orchestrator has a per-record usage_record (70), so its
	// legacy turn usage (300) is ignored; impl keeps its legacy invocation_end 900.
	if v := e2eTokens(legacy, "input"); v != float64(970) {
		t.Errorf("legacy run input = %v, want 970", v)
	}
}
