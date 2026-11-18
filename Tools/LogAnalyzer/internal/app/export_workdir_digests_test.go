package app_test

// Tests that Export without a path resolves the default start like Analyze, so
// the written digest holds the runs from the logs and from the digests lying
// directly in the working directory.

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func exportDefault(t *testing.T, e *digestEnv) app.ExportResult {
	t.Helper()
	res, err := e.service().Export(context.Background(), app.ExportRequest{OutPath: exOut})
	if err != nil {
		t.Fatalf("Export returned unexpected error: %v", err)
	}
	return res
}

func sortedDigestRunIDs(d domain.Digest) []string {
	ids := digestRunIDs(d)
	sort.Strings(ids)
	return ids
}

func TestExport_NoPath_IncludesLogRunsAndWorkdirDigestRuns(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := exportDefault(t, e)

	if res.Status != app.ExportWritten {
		t.Fatalf("Status = %v, want ExportWritten", res.Status)
	}
	if res.RunsWritten != 2 {
		t.Errorf("RunsWritten = %d, want 2", res.RunsWritten)
	}
	want := []string{dgRunA, dgRunB}
	if got := sortedDigestRunIDs(onlyWrite(t, e).d); !reflect.DeepEqual(got, want) {
		t.Errorf("exported run ids = %v, want %v", got, want)
	}
}

func TestExport_NoPath_WorkdirDigestsOnly_WritesTheirRuns(t *testing.T) {
	e := newWorkdirEnv()
	e.workdirDigests(digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunB, 2_000_000))})

	res := exportDefault(t, e)

	if res.Status != app.ExportWritten || res.RunsWritten != 1 {
		t.Fatalf("Status/RunsWritten = %v/%d, want ExportWritten/1", res.Status, res.RunsWritten)
	}
	if got := digestRunIDs(onlyWrite(t, e).d); !reflect.DeepEqual(got, []string{dgRunB}) {
		t.Errorf("exported run ids = %v, want [%s]", got, dgRunB)
	}
}

func TestExport_NoPath_ContainsSameRunsAsDefaultAnalysis(t *testing.T) {
	e := newWorkdirEnv()
	e.addLogRun(dgRunA, 1_000_000)
	e.workdirDigests(
		digestItem{name: "x.digest.json", d: dgDigest(dgT1, dgRun(dgRunA, 9_000_000), dgRun(dgRunB, 2_000_000))},
	)

	analyzed := sortedRunIDs(analyzeDefault(t, e).Report)
	if want := []string{dgRunA, dgRunB}; !reflect.DeepEqual(analyzed, want) {
		t.Fatalf("default analysis run ids = %v, want %v", analyzed, want)
	}
	exportDefault(t, e)

	if got := sortedDigestRunIDs(onlyWrite(t, e).d); !reflect.DeepEqual(got, analyzed) {
		t.Errorf("exported run ids = %v, want the default analysis runs %v", got, analyzed)
	}
}
