package resultstore_test

// Tests that persisting a report is refused when any run lacks an observed
// subject model or has one that differs from the selected model, and that
// accepted reports are filed and labelled by the observed model.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-agent-test/internal/resultstore"
)

const refusalRoot = "/OrchestrationTestResults"

func storeOne(t *testing.T, data []byte) (resultstore.StoreResult, *fakeFS) {
	t.Helper()
	fs := newFakeFS()
	seedFile(fs, "/reports/r.json", data)
	result, err := resultstore.Store(fs, resultstore.StoreRequest{
		TestResultsRoot: refusalRoot,
		ReportFiles:     []string{"/reports/r.json"},
	})
	if err != nil {
		t.Fatalf("Store returned unexpected error: %v", err)
	}
	return result, fs
}

func assertRefused(t *testing.T, result resultstore.StoreResult, fs *fakeFS) {
	t.Helper()
	if result.Stored != 0 {
		t.Errorf("Stored = %d, want 0", result.Stored)
	}
	if result.SkippedUnverifiedModel != 1 {
		t.Errorf("SkippedUnverifiedModel = %d, want 1", result.SkippedUnverifiedModel)
	}
	if len(result.Reports) != 1 {
		t.Fatalf("got %d report entries, want 1", len(result.Reports))
	}
	rep := result.Reports[0]
	if !rep.Skipped || rep.SkipReason != resultstore.SkipUnverifiedModel {
		t.Errorf("entry Skipped=%v SkipReason=%q, want refused with %q", rep.Skipped, rep.SkipReason, resultstore.SkipUnverifiedModel)
	}
	if rep.TargetPath != "" {
		t.Errorf("TargetPath = %q, want empty for a refused report", rep.TargetPath)
	}
	if strings.TrimSpace(rep.Message) == "" {
		t.Error("refused entry has no message explaining the reason")
	}
	for path := range fs.files {
		if strings.HasPrefix(path, refusalRoot) {
			t.Errorf("refused report was written to %q", path)
		}
	}
	for dir := range fs.dirs {
		if strings.HasPrefix(dir, refusalRoot) {
			t.Errorf("refused report created directory %q", dir)
		}
	}
}

// Support coverage (guards against over-refusal); passes before the refusal
// exists and is not evidence for the refusal behavior itself.
func TestStore_EveryRunObservedMatchesSelected_IsStored(t *testing.T) {
	data := buildReport(t,
		[]runSpec{verifiedRun(), verifiedRun()},
		[]runSpec{verifiedRun()})

	result, _ := storeOne(t, data)

	if result.Stored != 1 || result.SkippedUnverifiedModel != 0 {
		t.Errorf("Stored=%d SkippedUnverifiedModel=%d, want 1 and 0", result.Stored, result.SkippedUnverifiedModel)
	}
}

// Support coverage: a repeated observed entry equal to the selected model is
// not a mismatch.
func TestStore_ObservedListAllEqualToSelected_IsStored(t *testing.T) {
	data := buildReport(t, []runSpec{{selected: selectedModel, observed: []string{selectedModel, selectedModel}}})

	result, _ := storeOne(t, data)

	if result.Stored != 1 || result.SkippedUnverifiedModel != 0 {
		t.Errorf("Stored=%d SkippedUnverifiedModel=%d, want 1 and 0", result.Stored, result.SkippedUnverifiedModel)
	}
}

func TestStore_RepetitionsWithDifferingSelectedModel_IsRefused(t *testing.T) {
	data := buildReport(t, []runSpec{verifiedRun(), {selected: otherModel, observed: []string{selectedModel}}})

	result, fs := storeOne(t, data)

	assertRefused(t, result, fs)
}

func TestStore_UnverifiedRun_IsRefused(t *testing.T) {
	cases := []struct {
		name  string
		tests [][]runSpec
	}{
		{"first run observed empty", [][]runSpec{{{selected: selectedModel, observed: []string{}}}}},
		{"first run observed field absent", [][]runSpec{{{selected: selectedModel, omitObserved: true}}}},
		{"first run mismatch", [][]runSpec{{{selected: selectedModel, observed: []string{otherModel}}}}},
		{"later repetition missing", [][]runSpec{{verifiedRun(), {selected: selectedModel, observed: []string{}}}}},
		{"later repetition mismatch", [][]runSpec{{verifiedRun(), verifiedRun(), {selected: selectedModel, observed: []string{otherModel}}}}},
		{"later test mismatch", [][]runSpec{{verifiedRun()}, {{selected: selectedModel, observed: []string{otherModel}}}}},
		{"later test field absent", [][]runSpec{{verifiedRun()}, {verifiedRun(), {selected: selectedModel, omitObserved: true}}}},
		{"observed entry blank", [][]runSpec{{{selected: selectedModel, observed: []string{""}}}}},
		{"observed lists matching and different model", [][]runSpec{{{selected: selectedModel, observed: []string{selectedModel, otherModel}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, fs := storeOne(t, buildReport(t, tc.tests...))

			assertRefused(t, result, fs)
		})
	}
}

func TestStore_LegacyReportWithoutObservedField_IsRefusedWhenPersisted(t *testing.T) {
	result, fs := storeOne(t, loadFixture(t, "legacy_report.json"))

	assertRefused(t, result, fs)
}

func TestStore_RefusedReport_DoesNotBlockOtherReports(t *testing.T) {
	fs := newFakeFS()
	seedFile(fs, "/reports/bad.json", buildReport(t, []runSpec{{selected: selectedModel, observed: []string{otherModel}}}))
	seedFile(fs, "/reports/good.json", loadFixture(t, "valid_report.json"))

	result, err := resultstore.Store(fs, resultstore.StoreRequest{
		TestResultsRoot: refusalRoot,
		ReportFiles:     []string{"/reports/bad.json", "/reports/good.json"},
	})

	if err != nil {
		t.Fatalf("Store returned unexpected error: %v", err)
	}
	if result.Stored != 1 || result.SkippedUnverifiedModel != 1 || len(result.Reports) != 2 {
		t.Fatalf("Stored=%d refused=%d entries=%d, want 1, 1, 2", result.Stored, result.SkippedUnverifiedModel, len(result.Reports))
	}
	if result.Reports[0].SkipReason != resultstore.SkipUnverifiedModel || result.Reports[1].Skipped {
		t.Errorf("entries not in input order: %+v", result.Reports)
	}
}

func TestStoreFromPaths_DirScan_RefusesUnverifiedReport(t *testing.T) {
	fs := newFakeFS()
	seedFile(fs, "/scan/bad.json", buildReport(t, []runSpec{{selected: selectedModel, observed: []string{}}}))

	result, err := resultstore.StoreFromPaths(fs, resultstore.StoreFromPathsRequest{
		TestResultsRoot: refusalRoot,
		Dir:             "/scan",
	})

	if err != nil {
		t.Fatalf("StoreFromPaths returned unexpected error: %v", err)
	}
	assertRefused(t, result, fs)
}

func TestStoreRequests_HaveNoOverrideOption(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(resultstore.StoreRequest{}),
		reflect.TypeOf(resultstore.StoreFromPathsRequest{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).Type.Kind() == reflect.Bool {
				t.Errorf("%s has boolean option %s; the refusal must not be overridable", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}

func TestSummaryLine_CountsRefusedReports(t *testing.T) {
	r := resultstore.StoreResult{Stored: 2, SkippedUnverifiedModel: 1}

	line := r.SummaryLine()

	if !strings.Contains(line, "Stored 2 reports") || !strings.Contains(line, "1 refused") {
		t.Errorf("SummaryLine = %q, want stored count and \"1 refused\"", line)
	}
	if strings.Contains(line, "$") {
		t.Errorf("SummaryLine = %q shows cost", line)
	}
}

func TestSummaryLine_RefusedAlongsideOtherSkips(t *testing.T) {
	r := resultstore.StoreResult{Stored: 1, SkippedDuplicate: 2, SkippedUnverifiedModel: 3}

	line := r.SummaryLine()

	for _, want := range []string{"2 duplicate", "3 refused", "5 skipped"} {
		if !strings.Contains(line, want) {
			t.Errorf("SummaryLine = %q, missing %q", line, want)
		}
	}
}

// ---- observed model drives filing metadata ----

func TestParseAndValidate_NewReport_LabelsByObservedModel(t *testing.T) {
	data := buildReport(t, []runSpec{{selected: selectedModel, observed: []string{"anthropic/observed-model-9"}}})

	parsed, err := resultstore.ParseAndValidate(data)

	if err != nil {
		t.Fatalf("ParseAndValidate must still read the report: %v", err)
	}
	if parsed.ModelShort != "observed-model-9" {
		t.Errorf("ModelShort = %q, want %q (derived from the observed model, not the selection)", parsed.ModelShort, "observed-model-9")
	}
}

func TestParseAndValidate_LegacyReport_StillLabelsBySelectedModel(t *testing.T) {
	parsed, err := resultstore.ParseAndValidate(loadFixture(t, "legacy_report.json"))

	if err != nil {
		t.Fatalf("legacy report must still parse: %v", err)
	}
	if parsed.ModelShort != "claude-sonnet-4.6" || parsed.SubjectVersion != "7.4.1" || parsed.HarnessID != "claude-code" {
		t.Errorf("legacy metadata = %q/%q/%q, want claude-sonnet-4.6/7.4.1/claude-code",
			parsed.ModelShort, parsed.SubjectVersion, parsed.HarnessID)
	}
}

func TestParseAndValidate_NewReportWithEmptyObservedModel_StillParses(t *testing.T) {
	data := buildReport(t, []runSpec{{selected: selectedModel, observed: []string{}}})

	if _, err := resultstore.ParseAndValidate(data); err != nil {
		t.Fatalf("reading must not apply the persist-time refusal: %v", err)
	}
}

// Regression coverage only: an accepted report always has observed == selected,
// so this file name is the same whichever model the implementation derives it
// from. Filing by the observed model is specified by
// TestParseAndValidate_NewReport_LabelsByObservedModel.
func TestStore_AcceptedReport_KeepsHappyPathFileName(t *testing.T) {
	result, fs := storeOne(t, loadFixture(t, "valid_report.json"))

	want := refusalRoot + "/Orchestrator/v1.2.3/happy-path--claude-code--claude-sonnet-4.6--20260823T191126.json"
	if result.Stored != 1 || result.Reports[0].TargetPath != want {
		t.Fatalf("TargetPath = %q, want %q", result.Reports[0].TargetPath, want)
	}
	if _, ok := fs.files[want]; !ok {
		t.Errorf("no file written at %q", want)
	}
}
