package report_test

// Fixtures for the model verdict and label tests: runs that are verified,
// mismatching, without evidence or not assessed, assembled into results by
// report.Build so the suite-level verdict is derived the way production does.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

const (
	// Distinct values so a leak of any one of them into a heading is visible.
	selectedModel   = "model-selected-a"
	observedOther   = "model-observed-b"
	stubSelection   = "model-stub-c"
	verifiedLabel   = selectedModel
	invalidDetailFn = "selected subject model %s, observed %s"
)

// verifiedRun is a run whose observed subject model equals its selection.
func verifiedRun(testName string, number int) report.RunReport {
	return report.RunReport{
		Key:                   domain.RunKey{RunID: "run-001", TestName: testName, RunNumber: number},
		Verdict:               domain.VerdictPass,
		Duration:              time.Second,
		SubjectModel:          selectedModel,
		StubModel:             stubSelection,
		ObservedSubjectModels: []string{selectedModel},
		ModelValidity:         domain.ModelVerified,
	}
}

// invalidRun is a run that selected selectedModel and is not verified. observed
// lists what the logs showed (none for a run without evidence).
func invalidRun(testName string, validity domain.ModelValidity, observed ...string) report.RunReport {
	return report.RunReport{
		Key:                   domain.RunKey{RunID: "run-001", TestName: testName, RunNumber: 1},
		Verdict:               domain.VerdictFail,
		Reasons:               []domain.FailureReason{domain.ReasonModelUnverified},
		Duration:              time.Second,
		SubjectModel:          selectedModel,
		StubModel:             stubSelection,
		ObservedSubjectModels: observed,
		ModelValidity:         validity,
	}
}

// passTest is a test whose runs all counted and passed.
func passTest(name string, runs ...report.RunReport) report.TestReport {
	return report.TestReport{
		TestName: name,
		Layer:    domain.LayerSubagent,
		Aggregate: domain.AggregateResult{
			TestName: name, Verdict: domain.VerdictPass,
			Counted: len(runs), Passed: len(runs), PassRate: 1,
			TotalCost: domain.CostReport{Attribution: domain.AttributionAttributed},
		},
		Runs: runs,
	}
}

// invalidTest is a test holding one invalid run, excluded as an infrastructure
// failure. The exclusion detail carries the selected and observed values, the
// only place they may appear in the text report.
func invalidTest(name string, run report.RunReport) report.TestReport {
	reason := domain.ExclusionModelMismatch
	if run.ModelValidity != domain.ModelMismatch {
		reason = domain.ExclusionModelEvidenceMissing
	}
	observed := "none"
	if len(run.ObservedSubjectModels) > 0 {
		observed = strings.Join(run.ObservedSubjectModels, ",")
	}
	return report.TestReport{
		TestName: name,
		Layer:    domain.LayerSubagent,
		Aggregate: domain.AggregateResult{
			TestName: name, Verdict: domain.VerdictFail,
			Reasons:               []domain.FailureReason{domain.ReasonInfrastructure},
			Excluded:              1,
			InfrastructureFailure: true,
			TotalCost:             domain.CostReport{Attribution: domain.AttributionAttributed},
			Exclusions: []domain.ExcludedRun{{
				Key:    run.Key,
				Reason: reason,
				Detail: fmt.Sprintf(invalidDetailFn, run.SubjectModel, observed),
			}},
		},
		Runs: []report.RunReport{run},
	}
}

// buildModelResult derives a Result through report.Build.
func buildModelResult(tests ...report.TestReport) report.Result {
	return report.Build("model-suite", fixtureStarted(), fixtureFinished(), tests, "")
}

// decodeJSONRunObservedModels decodes observed_subject_models from the first
// run of the first test in the JSON output.
func decodeJSONRunObservedModels(t *testing.T, data []byte) []string {
	t.Helper()
	var doc struct {
		Tests []struct {
			Runs []struct {
				Observed []string `json:"observed_subject_models"`
			} `json:"runs"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decoding JSON report: %v", err)
	}
	if len(doc.Tests) == 0 || len(doc.Tests[0].Runs) == 0 {
		t.Fatal("JSON report has no tests or no runs to decode observed_subject_models from")
	}
	return doc.Tests[0].Runs[0].Observed
}

// lineStartsWith reports whether any line of text, trimmed, starts with prefix.
func lineStartsWith(text, prefix string) bool {
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			return true
		}
	}
	return false
}

// withoutExcludedLines drops the "excluded [...]" detail lines, the one place
// selected and observed values may appear for an invalid run.
func withoutExcludedLines(text string) string {
	var keep []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "excluded [") {
			continue
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}
