package app_test

// Tests for the ingest-otel use case.
//
// Rules under test:
//   - The run is looked up in the resolved source; its folder and the export
//     path go to the ingester unchanged.
//   - An unknown run, an unresolvable or unusable source and a digest source
//     never reach the ingester and are told apart in the outcome.
//   - The result and the infrastructure errors of the ingester come back unchanged.
//   - The use case does not run the analysis (no event file is read).

import (
	"context"
	"errors"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const exportPath = "/exports/copilot-otel.jsonl"

func ingestRequest(runID string) app.IngestRequest {
	return app.IngestRequest{ExplicitPath: "/test-logs", RunID: runID, ExportPath: exportPath}
}

func TestIngestOTel_KnownRun_IngestsIntoTheRunFolderFromTheExportPath(t *testing.T) {
	ing := &fakeUsageIngester{result: domain.UsageIngestResult{Status: domain.IngestDone, RecordsWritten: 2, Finalized: 1}}
	svc, _ := ingestTestService(t, logsRootSource(), ing, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if len(ing.calls) != 1 {
		t.Fatalf("ingester called %d times, want 1", len(ing.calls))
	}
	if got := ing.calls[0]; got.RunDir != ingestTestRunDir || got.ExportPath != exportPath {
		t.Errorf("ingester request = %+v, want run dir %q and export %q", got, ingestTestRunDir, exportPath)
	}
	if !out.RunFound || out.Outcome != app.OutcomeReport {
		t.Errorf("outcome = %+v, want RunFound with OutcomeReport", out)
	}
}

func TestIngestOTel_ReturnsTheResultOfTheIngesterUnchanged(t *testing.T) {
	want := domain.UsageIngestResult{
		Status: domain.IngestLockHeld, Pending: 3,
		Diagnostics: []domain.UsageIngestDiagnostic{{Kind: "lock_held", Message: "another ingest runs"}},
	}
	svc, _ := ingestTestService(t, logsRootSource(), &fakeUsageIngester{result: want}, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if out.Result.Status != domain.IngestLockHeld || out.Result.Pending != 3 ||
		len(out.Result.Diagnostics) != 1 || out.Result.Diagnostics[0].Kind != "lock_held" {
		t.Errorf("result = %+v, want %+v", out.Result, want)
	}
}

func TestIngestOTel_UnknownRun_DoesNotIngestAndReportsRunNotFound(t *testing.T) {
	ing := &fakeUsageIngester{}
	svc, _ := ingestTestService(t, logsRootSource(), ing, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest("20990101T000000Z-zzzz"))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if out.RunFound {
		t.Error("RunFound = true for a run the source does not hold")
	}
	if out.Outcome != app.OutcomeReport {
		t.Errorf("Outcome = %v, want OutcomeReport: the source itself was fine", out.Outcome)
	}
	if len(ing.calls) != 0 {
		t.Errorf("ingester called %d times for an unknown run", len(ing.calls))
	}
}

func TestIngestOTel_UnresolvableSource_ReportsSourceNotFoundWithoutIngesting(t *testing.T) {
	ing := &fakeUsageIngester{}
	svc, _ := ingestTestService(t, domain.Source{Kind: domain.SourceNotFound, Path: "/test-logs"}, ing, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if out.Outcome != app.OutcomeSourceNotFound || len(ing.calls) != 0 {
		t.Errorf("outcome = %v, ingester calls = %d; want OutcomeSourceNotFound and no call", out.Outcome, len(ing.calls))
	}
}

func TestIngestOTel_UnusableSource_ReportsSourceUnusableWithoutIngesting(t *testing.T) {
	ing := &fakeUsageIngester{}
	src := domain.Source{Kind: domain.SourceUnusable, Path: "/test-logs", Reason: "not a logs tree"}
	svc, _ := ingestTestService(t, src, ing, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if out.Outcome != app.OutcomeSourceUnusable || len(ing.calls) != 0 {
		t.Errorf("outcome = %v, ingester calls = %d; want OutcomeSourceUnusable and no call", out.Outcome, len(ing.calls))
	}
}

func TestIngestOTel_DigestSource_IsNotUsableForIngest(t *testing.T) {
	ing := &fakeUsageIngester{}
	svc, _ := ingestTestService(t, domain.Source{Kind: domain.SourceDigestFile, Path: "/test-logs/a.digest.json"}, ing, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil {
		t.Fatalf("IngestOTel: %v", err)
	}
	if out.Outcome != app.OutcomeSourceUnusable || len(ing.calls) != 0 {
		t.Errorf("outcome = %v, ingester calls = %d; a digest holds no run folder to ingest into", out.Outcome, len(ing.calls))
	}
}

func TestIngestOTel_InfrastructureErrorOfTheIngester_IsReturned(t *testing.T) {
	boom := errors.New("disk failure")
	svc, _ := ingestTestService(t, logsRootSource(),
		&fakeUsageIngester{result: domain.UsageIngestResult{Status: domain.IngestFailed}, err: boom}, nil)

	_, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the error of the ingester", err)
	}
}

func TestIngestOTel_DoesNotReadEventFiles(t *testing.T) {
	svc, reader := ingestTestService(t, logsRootSource(), &fakeUsageIngester{}, nil)

	out, err := svc.IngestOTel(context.Background(), ingestRequest(appTestRunID))

	if err != nil || !out.RunFound {
		t.Fatalf("IngestOTel: err=%v out=%+v; the run must be found for this test to mean anything", err, out)
	}
	if n := reader.readCounts[appTestOrchFile]; n != 0 {
		t.Errorf("orchestrator file read %d times; ingest-otel does not analyse", n)
	}
}
