package report_test

// The JSON report carries the observed subject model(s) and the validity of
// every run, and a suite-level model verdict, additively: every key that
// existed before keeps its name and meaning.

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

func decodeReportDoc(t *testing.T, r report.Result) map[string]any {
	t.Helper()
	out, err := renderJSON(t, r)
	if err != nil {
		t.Fatalf("RenderJSON returned an error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("report is not valid JSON: %v\n%s", err, out)
	}
	return doc
}

// runDocs returns the decoded run objects of the document, test by test.
func runDocs(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	var runs []map[string]any
	tests, _ := doc["tests"].([]any)
	for _, tst := range tests {
		for _, r := range tst.(map[string]any)["runs"].([]any) {
			runs = append(runs, r.(map[string]any))
		}
	}
	return runs
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestRenderJSON_VerifiedSuite_CarriesVerdictLabelAndObservedModels(t *testing.T) {
	doc := decodeReportDoc(t, buildModelResult(passTest("t1", verifiedRun("t1", 1))))

	if got := doc["model_verdict"]; got != string(domain.ModelVerdictVerified) {
		t.Errorf("model_verdict = %v, want %q", got, domain.ModelVerdictVerified)
	}
	if got := doc["model_label"]; got != verifiedLabel {
		t.Errorf("model_label = %v, want %q", got, verifiedLabel)
	}
	if got := doc["unverified_runs"]; got != float64(0) {
		t.Errorf("unverified_runs = %v, want 0", got)
	}
	run := runDocs(t, doc)[0]
	if got := run["observed_subject_models"]; !reflect.DeepEqual(got, []any{selectedModel}) {
		t.Errorf("observed_subject_models = %v, want [%s]", got, selectedModel)
	}
	if got := run["model_verification"]; got != "verified" {
		t.Errorf("model_verification = %v, want verified", got)
	}
	if got := run["subject_model"]; got != selectedModel {
		t.Errorf("subject_model = %v, want the selection %q to stay beside the observed models", got, selectedModel)
	}
}

func TestRenderJSON_InvalidSuite_CarriesTheVerdictAndNoLabel(t *testing.T) {
	result := buildModelResult(
		passTest("t1", verifiedRun("t1", 1)),
		invalidTest("t2", invalidRun("t2", domain.ModelMismatch, observedOther)),
		invalidTest("t3", invalidRun("t3", domain.ModelNoEvidence)),
	)

	doc := decodeReportDoc(t, result)

	if got := doc["model_verdict"]; got != string(domain.ModelVerdictUnverified) {
		t.Errorf("model_verdict = %v, want %q", got, domain.ModelVerdictUnverified)
	}
	if got := doc["model_label"]; got != "" {
		t.Errorf("model_label = %v, want empty: an invalid suite carries no model label", got)
	}
	if got := doc["unverified_runs"]; got != float64(2) {
		t.Errorf("unverified_runs = %v, want 2", got)
	}
}

func TestRenderJSON_SuiteWithoutRuns_IsUnverifiedWithZeroUnverifiedRuns(t *testing.T) {
	doc := decodeReportDoc(t, buildModelResult())

	if got := doc["model_verdict"]; got != string(domain.ModelVerdictUnverified) {
		t.Errorf("model_verdict = %v, want %q", got, domain.ModelVerdictUnverified)
	}
	if got := doc["unverified_runs"]; got != float64(0) {
		t.Errorf("unverified_runs = %v, want 0", got)
	}
}

func TestRenderJSON_RunModelVerification(t *testing.T) {
	cases := []struct {
		name         string
		run          report.RunReport
		want         string
		wantObserved []any
	}{
		{"verified", verifiedRun("t", 1), "verified", []any{selectedModel}},
		{"mismatch", invalidRun("t", domain.ModelMismatch, observedOther), "mismatch", []any{observedOther}},
		{"no evidence", invalidRun("t", domain.ModelNoEvidence), "no_evidence", []any{}},
		{"empty validity is not assessed", invalidRun("t", ""), "not_assessed", []any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeReportDoc(t, buildModelResult(passTest("t", tc.run)))

			run := runDocs(t, doc)[0]
			if got := run["model_verification"]; got != tc.want {
				t.Errorf("model_verification = %v, want %q", got, tc.want)
			}
			got, isList := run["observed_subject_models"].([]any)
			if !isList {
				t.Fatalf("observed_subject_models = %#v, want an array (never null)", run["observed_subject_models"])
			}
			if !reflect.DeepEqual(got, tc.wantObserved) {
				t.Errorf("observed_subject_models = %v, want %v", got, tc.wantObserved)
			}
		})
	}
}

// Additive: the keys that existed before are all still present, and the only
// new keys are the documented ones.
func TestRenderJSON_ModelFields_AreTheOnlyAdditions(t *testing.T) {
	legacyTop := []string{
		"catalog_folder", "counts", "errors", "finished_at", "infrastructure_failures",
		"schema_version", "started_at", "suite_id", "tests", "total_cost",
	}
	legacyRun := []string{
		"assertions", "conditions", "cost", "duration_ms", "harness_id", "negative_applied",
		"reasons", "run", "stub_model", "subject_model", "subject_version",
		"termination_reason", "test_id", "test_version", "verdict",
	}
	doc := decodeReportDoc(t, buildModelResult(passTest("t1", verifiedRun("t1", 1))))

	wantTop := append(append([]string{}, legacyTop...), "model_label", "model_verdict", "unverified_runs")
	sort.Strings(wantTop)
	if got := sortedKeys(doc); !reflect.DeepEqual(got, wantTop) {
		t.Errorf("top-level keys = %v, want %v", got, wantTop)
	}
	wantRun := append(append([]string{}, legacyRun...), "model_verification", "observed_subject_models")
	sort.Strings(wantRun)
	if got := sortedKeys(runDocs(t, doc)[0]); !reflect.DeepEqual(got, wantRun) {
		t.Errorf("run keys = %v, want %v", got, wantRun)
	}
	if doc["schema_version"] != report.SchemaVersion || report.SchemaVersion != "1" {
		t.Errorf("schema_version = %v (const %q), want it to stay \"1\" for an additive change", doc["schema_version"], report.SchemaVersion)
	}
}

// The selection keeps its meaning: it is the chosen model, shown beside the
// observed list, and an unknown selection keeps the unknown mapping.
func TestRenderJSON_SelectionFieldsKeepTheirMeaning(t *testing.T) {
	run := invalidRun("t", domain.ModelMismatch, observedOther)
	run.StubModel = ""

	doc := decodeReportDoc(t, buildModelResult(invalidTest("t", run)))

	got := runDocs(t, doc)[0]
	if got["subject_model"] != selectedModel {
		t.Errorf("subject_model = %v, want the selection %q", got["subject_model"], selectedModel)
	}
	if got["stub_model"] != "unknown" {
		t.Errorf("stub_model = %v, want unknown for an absent selection", got["stub_model"])
	}
}

// The invalid run's selected and observed values reach the report through the
// infrastructure-failure detail as well, not through any label field.
func TestRenderJSON_InvalidRun_ValuesAreInTheExclusionDetail(t *testing.T) {
	doc := decodeReportDoc(t, buildModelResult(invalidTest("t", invalidRun("t", domain.ModelMismatch, observedOther))))

	agg := doc["tests"].([]any)[0].(map[string]any)["aggregate"].(map[string]any)
	exclusion := agg["exclusions"].([]any)[0].(map[string]any)
	detail, _ := exclusion["detail"].(string)
	for _, want := range []string{selectedModel, observedOther} {
		if !strings.Contains(detail, want) {
			t.Errorf("exclusion detail %q does not carry %q", detail, want)
		}
	}
	if doc["model_label"] != "" {
		t.Errorf("model_label = %v, want empty for an invalid suite", doc["model_label"])
	}
}

// A verified run that observed more than one model keeps every one of them, in
// order, in the run record.
func TestRenderJSON_VerifiedRun_KeepsEveryObservedModel(t *testing.T) {
	run := verifiedRun("t1", 1)
	run.ObservedSubjectModels = []string{selectedModel, observedOther}

	doc := decodeReportDoc(t, buildModelResult(passTest("t1", run)))

	got := runDocs(t, doc)[0]["observed_subject_models"]
	if !reflect.DeepEqual(got, []any{selectedModel, observedOther}) {
		t.Errorf("observed_subject_models = %v, want [%s %s]", got, selectedModel, observedOther)
	}
}
