package app_test

// Tests for export run selection, source outcomes, path validation and the
// non-blocking guarantee.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const exOut = "/out/runs.digest.json"

// exportReq builds an export request against the scripted logs root.
func exportReq(runID string) app.ExportRequest {
	return app.ExportRequest{ExplicitPath: dgLogsPath, RunID: runID, OutPath: exOut}
}

// doExport runs Export and fails the test on a returned error.
func doExport(t *testing.T, e *digestEnv, req app.ExportRequest) app.ExportResult {
	t.Helper()
	res, err := e.service().Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export returned unexpected error: %v", err)
	}
	return res
}

// onlyWrite requires exactly one successful digest write and returns it.
func onlyWrite(t *testing.T, e *digestEnv) digestWrite {
	t.Helper()
	if len(e.digests.writes) != 1 {
		t.Fatalf("digest writes = %d, want exactly 1", len(e.digests.writes))
	}
	return e.digests.writes[0]
}

func digestRunIDs(d domain.Digest) []string {
	var ids []string
	for _, r := range d.Aggregate.Runs {
		ids = append(ids, r.Run.ID)
	}
	return ids
}

// logsWithTwoRunsAndBucket scripts a logs root with runs A and B and a bucket.
func logsWithTwoRunsAndBucket() *digestEnv {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.addLogRun(dgRunB, 2_000_000)
	e.addLogBucket(500_000)
	return e
}

func TestExport_NoRunID_WritesAllRunsAndBucket(t *testing.T) {
	e := logsWithTwoRunsAndBucket()

	res := doExport(t, e, exportReq(""))

	if res.Status != app.ExportWritten || res.Outcome != app.OutcomeReport {
		t.Fatalf("Status/Outcome = %v/%v, want ExportWritten/OutcomeReport", res.Status, res.Outcome)
	}
	if res.RunsWritten != 2 || !res.UnattributableWritten {
		t.Errorf("RunsWritten=%d UnattributableWritten=%v, want 2 and true", res.RunsWritten, res.UnattributableWritten)
	}
	w := onlyWrite(t, e)
	if w.path != exOut {
		t.Errorf("written path = %q, want %q", w.path, exOut)
	}
	if got := digestRunIDs(w.d); !reflect.DeepEqual(got, []string{dgRunA, dgRunB}) {
		t.Errorf("digest run ids = %v, want [%s %s]", got, dgRunA, dgRunB)
	}
	if w.d.Aggregate.Unattributable == nil {
		t.Error("digest must include the unattributable bucket")
	}
}

func TestExport_NoRunID_SourceWithoutBucket_ReportsBucketNotWritten(t *testing.T) {
	e := newDigestEnv()
	e.addLogRun(dgRunA, 1_000_000)

	res := doExport(t, e, exportReq(""))

	if res.Status != app.ExportWritten || res.RunsWritten != 1 || res.UnattributableWritten {
		t.Errorf("result = %+v, want written, 1 run, no bucket", res)
	}
	if onlyWrite(t, e).d.Aggregate.Unattributable != nil {
		t.Error("digest must not invent a bucket")
	}
}

func TestExport_BucketOnlySource_WritesBucketWithZeroRuns(t *testing.T) {
	e := newDigestEnv()
	e.addLogBucket(500_000)

	res := doExport(t, e, exportReq(""))

	if res.Status != app.ExportWritten || res.RunsWritten != 0 || !res.UnattributableWritten {
		t.Errorf("result = %+v, want written, 0 runs, bucket written", res)
	}
	if onlyWrite(t, e).d.Aggregate.Unattributable == nil {
		t.Error("digest must hold the bucket")
	}
}

func TestExport_WithRunID_WritesOnlyThatRunWithoutBucket(t *testing.T) {
	e := logsWithTwoRunsAndBucket()

	res := doExport(t, e, exportReq(dgRunB))

	if res.Status != app.ExportWritten || res.RunsWritten != 1 || res.UnattributableWritten {
		t.Errorf("result = %+v, want written, 1 run, no bucket", res)
	}
	w := onlyWrite(t, e)
	if got := digestRunIDs(w.d); !reflect.DeepEqual(got, []string{dgRunB}) {
		t.Errorf("digest run ids = %v, want [%s]", got, dgRunB)
	}
	if w.d.Aggregate.Unattributable != nil {
		t.Error("a run-id export must not include the unattributable bucket")
	}
}

func TestExport_UnknownRunID_WritesNothingAndYieldsNoData(t *testing.T) {
	e := logsWithTwoRunsAndBucket()

	res := doExport(t, e, exportReq("20260909T000000Z-ffff"))

	if res.Status != app.ExportRunNotFound || res.Outcome != app.OutcomeNoData {
		t.Errorf("Status/Outcome = %v/%v, want ExportRunNotFound/OutcomeNoData", res.Status, res.Outcome)
	}
	if res.RunsWritten != 0 || res.UnattributableWritten {
		t.Errorf("counts = %d/%v, want nothing written", res.RunsWritten, res.UnattributableWritten)
	}
	if len(e.digests.writes) != 0 {
		t.Errorf("digest writes = %d, want none", len(e.digests.writes))
	}
}

func TestExport_RunIDOfBucketOnlySource_IsRunNotFound(t *testing.T) {
	e := newDigestEnv()
	e.addLogBucket(500_000)

	res := doExport(t, e, exportReq(dgRunA))

	if res.Status != app.ExportRunNotFound || len(e.digests.writes) != 0 {
		t.Errorf("Status=%v writes=%d, want ExportRunNotFound and no write", res.Status, len(e.digests.writes))
	}
}

func TestExport_SourceOutcomes_MatchAnalyzeAndWriteNothing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *digestEnv)
		want  app.Outcome
	}{
		{"source not found", func(e *digestEnv) {}, app.OutcomeSourceNotFound},
		{"source unusable", func(e *digestEnv) { e.unusable[dgLogsPath] = true }, app.OutcomeSourceUnusable},
		{"no data", func(e *digestEnv) { e.hasLogs = true }, app.OutcomeNoData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newDigestEnv()
			tc.setup(e)

			res := doExport(t, e, exportReq(""))

			if res.Status != app.ExportNotWritten || res.Outcome != tc.want {
				t.Errorf("Status/Outcome = %v/%v, want ExportNotWritten/%v", res.Status, res.Outcome, tc.want)
			}
			if len(e.digests.writes) != 0 {
				t.Errorf("digest writes = %d, want none", len(e.digests.writes))
			}
		})
	}
}

func TestExport_UnknownRunIDInNoDataSource_IsNotWrittenNoData(t *testing.T) {
	e := newDigestEnv()
	e.hasLogs = true

	res := doExport(t, e, exportReq(dgRunA))

	if res.Status != app.ExportNotWritten || res.Outcome != app.OutcomeNoData {
		t.Errorf("Status/Outcome = %v/%v, want ExportNotWritten/OutcomeNoData", res.Status, res.Outcome)
	}
}

func TestExport_DigestArchives_AreMergedIntoOneDigest(t *testing.T) {
	e := newDigestEnv()
	e.digestDir(dgDirPath,
		digestItem{name: "a.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))},
		digestItem{name: "b.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 1_000_000))},
	)

	res := doExport(t, e, app.ExportRequest{ExplicitPath: dgDirPath, OutPath: exOut})

	if res.Status != app.ExportWritten || res.RunsWritten != 2 {
		t.Fatalf("result = %+v, want written with 2 runs", res)
	}
	if res.Source.Kind != domain.SourceDigestDir {
		t.Errorf("Source.Kind = %v, want SourceDigestDir", res.Source.Kind)
	}
	if got := digestRunIDs(onlyWrite(t, e).d); !reflect.DeepEqual(got, []string{dgRunA, dgRunB}) {
		t.Errorf("digest run ids = %v, want both runs sorted", got)
	}
}

func TestExport_InvalidOutPath_ResolvesNothingAndWritesNothing(t *testing.T) {
	for _, out := range []string{"", "runs.json", "/out/runs.digest.json.bak"} {
		t.Run(out, func(t *testing.T) {
			e := logsWithTwoRunsAndBucket()
			req := exportReq("")
			req.OutPath = out

			res := doExport(t, e, req)

			if res.Status != app.ExportInvalidPath {
				t.Errorf("Status = %v, want ExportInvalidPath", res.Status)
			}
			if len(e.logSource.classifyCalls) != 0 || len(e.logSource.defaultCalls) != 0 {
				t.Error("no source may be resolved for an invalid output path")
			}
			if len(e.digests.writes) != 0 {
				t.Errorf("digest writes = %d, want none", len(e.digests.writes))
			}
		})
	}
}

func TestExport_NilDigestStore_ReturnsError(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	svc := app.New(app.Deps{
		Source: e.logSource, Reader: e.reader, Pricing: e.pricing,
		Clock: e.clock, Interaction: e.interaction, WorkDir: "/workdir",
	})

	_, err := svc.Export(context.Background(), exportReq(""))

	if err == nil {
		t.Error("Export without a digest store must return an error")
	}
}

func TestExport_CancelledContext_ReturnsErrorAndWritesNothing(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.service().Export(ctx, exportReq(""))

	if err == nil {
		t.Error("Export with a cancelled context must return an error")
	}
	if len(e.digests.writes) != 0 {
		t.Errorf("digest writes = %d, want none", len(e.digests.writes))
	}
}

func TestExport_NeverAsksQuestionsWhenSourceIsGiven(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	ia := &scriptedInteraction{}
	e.interaction = ia
	e.pricing.table = domain.EmptyPricingTable() // every model unpriced

	res := doExport(t, e, exportReq(""))

	if res.Status != app.ExportWritten {
		t.Fatalf("Status = %v, want ExportWritten", res.Status)
	}
	if len(ia.selectOneCalls)+len(ia.askTextCalls) != 0 {
		t.Errorf("export asked questions: select=%d text=%d", len(ia.selectOneCalls), len(ia.askTextCalls))
	}
}

func TestExport_MalformedPricingFile_DoesNotFailExport(t *testing.T) {
	e := logsWithTwoRunsAndBucket()
	e.pricing.loadErr = context.DeadlineExceeded // stands in for a malformed pricing document

	res := doExport(t, e, exportReq(""))

	if res.Status != app.ExportWritten {
		t.Errorf("Status = %v, want ExportWritten regardless of the pricing file", res.Status)
	}
}

func TestExport_OnlyUnreadableDigests_IsNotWrittenNoData(t *testing.T) {
	e := newDigestEnv()
	e.digestFile(dgFilePath, domain.Digest{}, loadFinding(domain.FindingInvalidDigest, dgFilePath))

	res := doExport(t, e, app.ExportRequest{ExplicitPath: dgFilePath, OutPath: exOut})

	if res.Status != app.ExportNotWritten || res.Outcome != app.OutcomeNoData {
		t.Errorf("Status/Outcome = %v/%v, want ExportNotWritten/OutcomeNoData", res.Status, res.Outcome)
	}
	if len(e.digests.writes) != 0 {
		t.Errorf("digest writes = %d, want none", len(e.digests.writes))
	}
}
