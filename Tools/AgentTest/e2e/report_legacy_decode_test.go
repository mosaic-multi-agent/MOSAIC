package e2e

// Regression guard: a stored report written before the observed model and the
// suite verdict existed still decodes. The fixture is a copy of such a report
// kept under this tool's own testdata/. This is support coverage, not evidence
// for the new behavior.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyReport_WithoutModelFields_StillDecodes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "testdata", "legacy_report", "pre_model_report.json"))
	if err != nil {
		t.Fatalf("reading the legacy report fixture: %v", err)
	}

	var legacy wireResultDoc
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatalf("legacy report does not decode through the wire reader: %v", err)
	}
	var model modelReportDoc
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("legacy report does not decode through the model-aware reader: %v", err)
	}

	if legacy.SuiteID != "checkout-suite" || len(legacy.Tests) < 2 {
		t.Errorf("legacy fields = suite %q with %d tests, want checkout-suite with its tests intact", legacy.SuiteID, len(legacy.Tests))
	}
	if len(legacy.Tests[0].Runs) != 1 || legacy.Tests[0].Runs[0].Verdict != "PASS" {
		t.Errorf("legacy run records not intact: %+v", legacy.Tests[0].Runs)
	}
	if model.ModelVerdict != "" || model.ModelLabel != "" || model.UnverifiedRuns != 0 {
		t.Errorf("absent suite model fields = %q/%q/%d, want zero values", model.ModelVerdict, model.ModelLabel, model.UnverifiedRuns)
	}
	run := model.Tests[0].Runs[0]
	if run.ModelVerification != "" || len(run.ObservedSubjectModels) != 0 {
		t.Errorf("absent run model fields = %q/%v, want zero values", run.ModelVerification, run.ObservedSubjectModels)
	}
	if run.SubjectModel != "unknown" {
		t.Errorf("subject_model = %q, want the legacy value to stay readable", run.SubjectModel)
	}
}
