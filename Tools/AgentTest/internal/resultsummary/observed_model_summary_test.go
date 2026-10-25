package resultsummary_test

// Tests that generated summaries group and label new reports by the observed
// subject model, and that reports stored before observed models were recorded
// are still read and summarised by their selected model, without cost, and
// are never modified.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/resultsummary"
)

// generatedSummaryText generates summaries for fs and returns the concatenated
// content of every summary file written.
func generatedSummaryText(t *testing.T, fs *fakeFS, wantFiles ...string) string {
	t.Helper()
	if _, err := resultsummary.Generate(fs, resultsummary.SummaryRequest{TestResultsRoot: "/TestResults"}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	var all strings.Builder
	for _, path := range wantFiles {
		content := readWrittenFile(t, fs, path)
		all.WriteString(content)
		assertNoCost(t, path, content)
	}
	return all.String()
}

// observed_model_report.json is deliberately synthetic: its observed model
// differs from the selected one (which the store would refuse) so that the
// summary label can only come from the observed model.
func TestGenerate_NewReport_GroupsAndLabelsByObservedModel(t *testing.T) {
	fs := newFakeFS()
	seedReport(fs, "/TestResults", "v3.0.0", "observed-suite", "claude-code",
		"observed-model-9", "20261001T100000", loadFixture(t, "observed_model_report.json"))

	text := generatedSummaryText(t, fs,
		"/TestResults/Orchestrator/v3.0.0/user-summary.md",
		"/TestResults/Orchestrator/v3.0.0/internal-summary.md")

	if !strings.Contains(text, "observed-model-9") {
		t.Errorf("summaries do not label the report by its observed model.\ngot:\n%s", text)
	}
	if strings.Contains(text, "selected-model-1") {
		t.Errorf("summaries label the report by the selected model instead of the observed one.\ngot:\n%s", text)
	}
}

func TestGenerate_NewAndLegacyReports_EachLabelledByTheirOwnModel(t *testing.T) {
	fs := newFakeFS()
	seedReport(fs, "/TestResults", "v3.0.0", "observed-suite", "claude-code",
		"observed-model-9", "20261001T100000", loadFixture(t, "observed_model_report.json"))
	seedReport(fs, "/TestResults", "7.4.1", "pre-observed-suite", "claude-code",
		"legacy-model-3", "20260502T083000", loadFixture(t, "pre_observed_report.json"))

	text := generatedSummaryText(t, fs,
		"/TestResults/Orchestrator/v3.0.0/user-summary.md",
		"/TestResults/Orchestrator/7.4.1/user-summary.md",
		"/TestResults/Orchestrator/summary.md")

	for _, want := range []string{"observed-model-9", "legacy-model-3"} {
		if !strings.Contains(text, want) {
			t.Errorf("summaries missing model label %q.\ngot:\n%s", want, text)
		}
	}
	if strings.Contains(text, "selected-model-1") {
		t.Errorf("a new report is labelled by its selection.\ngot:\n%s", text)
	}
}

func TestGenerate_PreObservedStoredReport_SummarisedBySelectedModelWithoutCost(t *testing.T) {
	fs := newFakeFS()
	seedReport(fs, "/TestResults", "7.4.1", "pre-observed-suite", "claude-code",
		"legacy-model-3", "20260502T083000", loadFixture(t, "pre_observed_report.json"))
	before := string(fs.files["/TestResults/Orchestrator/7.4.1/pre-observed-suite--claude-code--legacy-model-3--20260502T083000.json"])

	text := generatedSummaryText(t, fs,
		"/TestResults/Orchestrator/7.4.1/user-summary.md",
		"/TestResults/Orchestrator/7.4.1/internal-summary.md")

	if !strings.Contains(text, "legacy-model-3") || !strings.Contains(text, "pre-observed-suite") {
		t.Errorf("legacy report not summarised by its selected model.\ngot:\n%s", text)
	}
	after := string(fs.files["/TestResults/Orchestrator/7.4.1/pre-observed-suite--claude-code--legacy-model-3--20260502T083000.json"])
	if after != before {
		t.Error("summarising modified the stored legacy report")
	}
}
