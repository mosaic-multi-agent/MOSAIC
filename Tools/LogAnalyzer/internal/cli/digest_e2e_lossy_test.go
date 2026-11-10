package cli_test

// End-to-end lossy-run attribution: a run whose log file lost a line stays
// incomplete after an export, and a clean run exported alone is complete.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestDigestLossyRun_LoadsIncompleteWithRunScopedFinding(t *testing.T) {
	logs := e2eFixture("logs")
	logReport := e2eReport(t, logs)
	if got := e2eFindingsOfKind(logReport, domain.FindingMalformedLine); len(got) != 1 || got[0]["run_id"] != nil {
		t.Fatalf("fixture precondition: the log report must hold one run-less malformed_line finding; got %v", got)
	}

	report := e2eReport(t, e2eExport(t, logs, e2eLossyRun))

	if !e2eIncomplete(report) {
		t.Error("the lossy run's digest must load as incomplete")
	}
	found := e2eFindingsOfKind(report, domain.FindingMalformedLine)
	if len(found) != 1 || found[0]["run_id"] != e2eLossyRun {
		t.Errorf("malformed_line must be scoped to the lossy run; got %v", found)
	}
}

func TestDigestLossyRun_ExportOfCleanRunLoadsComplete(t *testing.T) {
	logs := e2eFixture("logs")

	report := e2eReport(t, e2eExport(t, logs, e2eCleanRun))

	if e2eIncomplete(report) {
		t.Errorf("a clean run's digest must load as complete; findings: %v", e2eFindingsOf(report))
	}
	if n := len(e2eFindingsOfKind(report, domain.FindingMalformedLine)); n != 0 {
		t.Errorf("the lossy run's finding must not leak into another run's digest; got %d", n)
	}
	if runs := e2eRunsOf(report); len(runs) != 1 || runs[e2eCleanRun] == nil {
		t.Errorf("the digest must hold exactly the clean run; got %v", runs)
	}
}

func TestDigestLossyRun_FullExportKeepsGeneralFindingsGeneral(t *testing.T) {
	report := e2eReport(t, e2eExport(t, e2eFixture("logs"), ""))

	folders := e2eFindingsOfKind(report, domain.FindingUnrecognisedFolder)

	if len(folders) != 1 || folders[0]["run_id"] != nil {
		t.Errorf("an unrecognised folder belongs to no run; got %v", folders)
	}
}
