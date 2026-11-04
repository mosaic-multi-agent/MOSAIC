package app_test

// Tests for the digest header, the overwrite rules and equivalence of exporting
// from a source path and from an already-loaded analysis.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func TestExport_Header_RecordsToolVersionAndClockTime(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	e.toolVersion = "9.8.7"
	e.clock.fixed = time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)

	doExport(t, e, exportReq(""))

	h := onlyWrite(t, e).d.Header
	if h.ToolVersion != "9.8.7" {
		t.Errorf("Header.ToolVersion = %q, want 9.8.7", h.ToolVersion)
	}
	if !h.WrittenAt.Equal(e.clock.fixed) {
		t.Errorf("Header.WrittenAt = %v, want the injected clock time %v", h.WrittenAt, e.clock.fixed)
	}
}

func TestExport_ExistingOutput_IsRefusedWithoutOverwrite(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	e.digests.existing[exOut] = true

	res, err := e.service().Export(context.Background(), exportReq(""))

	if err != nil {
		t.Fatalf("Export returned unexpected error: %v (an existing file is a status, not an error)", err)
	}
	if res.Status != app.ExportFileExists || res.Outcome != app.OutcomeReport {
		t.Errorf("Status/Outcome = %v/%v, want ExportFileExists/OutcomeReport", res.Status, res.Outcome)
	}
	if res.OutPath != exOut {
		t.Errorf("OutPath = %q, want %q", res.OutPath, exOut)
	}
	if len(e.digests.writes) != 0 {
		t.Errorf("digest writes = %d, want none (existing file untouched)", len(e.digests.writes))
	}
}

func TestExport_ExistingOutput_IsReplacedWithOverwrite(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	e.digests.existing[exOut] = true
	req := exportReq("")
	req.Overwrite = true

	res := doExport(t, e, req)

	if res.Status != app.ExportWritten || res.RunsWritten != 2 {
		t.Errorf("result = %+v, want written with 2 runs", res)
	}
	if w := onlyWrite(t, e); !w.overwrite || w.path != exOut {
		t.Errorf("write = path %q overwrite %v, want %q with overwrite", w.path, w.overwrite, exOut)
	}
}

func TestExport_NewOutput_DoesNotRequestOverwrite(t *testing.T) {
	e := logsWithTwoRunsAndBucket()

	doExport(t, e, exportReq(""))

	if onlyWrite(t, e).overwrite {
		t.Error("overwrite must only be requested when the caller asks for it")
	}
}

// sessionPair exports req from the source path and, on a second env with the
// same data, from the loaded session; it returns both digest writes.
func sessionPair(t *testing.T, build func() *digestEnv, source string, req app.ExportRequest) (fromPath, fromSession digestWrite, res app.ExportResult) {
	t.Helper()
	a := build()
	req.ExplicitPath = source
	doExport(t, a, req)
	fromPath = onlyWrite(t, a)

	b := build()
	svc := b.service()
	loaded, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: source})
	if err != nil || loaded.Session == nil {
		t.Fatalf("Analyze: err=%v session nil=%v", err, loaded.Session == nil)
	}
	req.ExplicitPath = "/ignored/by/session"
	res, err = svc.ExportSession(context.Background(), loaded.Session, req)
	if err != nil {
		t.Fatalf("ExportSession returned unexpected error: %v", err)
	}
	fromSession = onlyWrite(t, b)
	return fromPath, fromSession, res
}

func lossyLogsEnv() *digestEnv {
	e := logsWithTwoRunsAndBucket()
	e.toolVersion = "1.2.3"
	e.enumFindings = []domain.Finding{
		runless(domain.FindingMissingEventFile, exRunDirA+"/TestAgent#1"),
		runless(domain.FindingUnrecognisedFolder, dgLogsPath+"/junk"),
	}
	orchB := exRunDirB + "/00_orchestrator_events.jsonl"
	e.reader.addFindings(orchB, domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Path: orchB, Run: domain.NamedRun(dgRunB),
	})
	return e
}

func TestExportSession_AllRuns_MatchesExportFromSource(t *testing.T) {
	p, s, res := sessionPair(t, lossyLogsEnv, dgLogsPath, app.ExportRequest{OutPath: exOut})

	if !reflect.DeepEqual(p, s) {
		t.Errorf("session export differs from source export:\n source:  %+v\n session: %+v", p, s)
	}
	if res.Status != app.ExportWritten || res.RunsWritten != 2 || !res.UnattributableWritten {
		t.Errorf("result = %+v, want written, 2 runs, bucket", res)
	}
}

func TestExportSession_OneRun_MatchesExportFromSource(t *testing.T) {
	p, s, res := sessionPair(t, lossyLogsEnv, dgLogsPath, app.ExportRequest{OutPath: exOut, RunID: dgRunA})

	if !reflect.DeepEqual(p, s) {
		t.Errorf("session export differs from source export:\n source:  %+v\n session: %+v", p, s)
	}
	if res.RunsWritten != 1 || res.UnattributableWritten {
		t.Errorf("result = %+v, want 1 run and no bucket", res)
	}
}

func TestExportSession_DigestSourcedSession_MatchesExportFromSource(t *testing.T) {
	build := func() *digestEnv {
		e := newDigestEnv()
		e.toolVersion = "1.2.3"
		e.digestDir(dgDirPath,
			digestItem{name: "a.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1)),
				findings: []domain.Finding{dgFinding(domain.FindingMalformedLine, domain.NamedRun(dgRunA))}},
			digestItem{name: "b.digest.json", d: dgDigest(dgT2, dgRun(dgRunB, 2))},
		)
		return e
	}

	p, s, _ := sessionPair(t, build, dgDirPath, app.ExportRequest{OutPath: exOut})

	if !reflect.DeepEqual(p, s) {
		t.Errorf("session export differs from source export:\n source:  %+v\n session: %+v", p, s)
	}
}

func TestExportSession_UnknownRunID_WritesNothingAndYieldsNoData(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	svc := e.service()
	loaded, _ := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgLogsPath})
	if loaded.Session == nil {
		t.Fatal("Analyze returned no session")
	}

	res, err := svc.ExportSession(context.Background(), loaded.Session,
		app.ExportRequest{OutPath: exOut, RunID: "20260909T000000Z-ffff"})

	if err != nil {
		t.Fatalf("ExportSession returned unexpected error: %v", err)
	}
	if res.Status != app.ExportRunNotFound || res.Outcome != app.OutcomeNoData || len(e.digests.writes) != 0 {
		t.Errorf("result = %+v writes=%d, want ExportRunNotFound/OutcomeNoData and no write", res, len(e.digests.writes))
	}
}

func TestExportSession_ExistingOutput_IsRefusedWithoutOverwriteAndReplacedWith(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	e.digests.existing[exOut] = true
	svc := e.service()
	loaded, _ := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgLogsPath})
	if loaded.Session == nil {
		t.Fatal("Analyze returned no session")
	}

	refused, err := svc.ExportSession(context.Background(), loaded.Session, app.ExportRequest{OutPath: exOut})
	if err != nil || refused.Status != app.ExportFileExists || len(e.digests.writes) != 0 {
		t.Fatalf("without overwrite: err=%v status=%v writes=%d, want ExportFileExists and no write",
			err, refused.Status, len(e.digests.writes))
	}

	replaced, err := svc.ExportSession(context.Background(), loaded.Session, app.ExportRequest{OutPath: exOut, Overwrite: true})
	if err != nil || replaced.Status != app.ExportWritten || len(e.digests.writes) != 1 {
		t.Errorf("with overwrite: err=%v status=%v writes=%d, want ExportWritten and one write",
			err, replaced.Status, len(e.digests.writes))
	}
}

func TestExportSession_InvalidPathAndNilSession(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	svc := e.service()
	loaded, _ := svc.Analyze(context.Background(), app.Request{ExplicitPath: dgLogsPath})
	if loaded.Session == nil {
		t.Fatal("Analyze returned no session")
	}

	res, err := svc.ExportSession(context.Background(), loaded.Session, app.ExportRequest{OutPath: "runs.json"})
	if err != nil || res.Status != app.ExportInvalidPath || len(e.digests.writes) != 0 {
		t.Errorf("invalid path: err=%v status=%v writes=%d, want ExportInvalidPath and no write",
			err, res.Status, len(e.digests.writes))
	}

	if _, err := svc.ExportSession(context.Background(), nil, app.ExportRequest{OutPath: exOut}); err == nil {
		t.Error("ExportSession with a nil session must return an error")
	}
}
