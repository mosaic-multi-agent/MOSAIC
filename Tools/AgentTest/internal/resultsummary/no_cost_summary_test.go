package resultsummary_test

// Tests that no generated summary rendering shows cost, including when the
// summarised reports are stored-format reports that carry cost fields.
// Cost stays in the report files; the summaries simply do not show it.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/resultsummary"
)

// assertNoCost fails when the rendered Markdown contains a cost column, a
// cost marker or a currency amount.
func assertNoCost(t *testing.T, name, out string) {
	t.Helper()
	lower := strings.ToLower(out)
	if strings.Contains(lower, "| cost") || strings.Contains(lower, "[cost?]") {
		t.Errorf("%s shows cost (a Cost column or marker); summaries must not.\ngot:\n%s", name, out)
	}
	if strings.Contains(out, "$") {
		t.Errorf("%s shows a currency amount; summaries must not.\ngot:\n%s", name, out)
	}
	if strings.Contains(out, "/100t") {
		t.Errorf("%s shows a per-100-tests value; summaries must not.\ngot:\n%s", name, out)
	}
}

// seedStoredCostReports seeds two versions, each with one stored-format
// report whose cost fields carry a distinctive non-zero amount.
func seedStoredCostReports(t *testing.T, fs *fakeFS) {
	t.Helper()
	seedReport(fs, "/TestResults", "v1.0.0", "stored-suite", "claude-code",
		"claude-sonnet-4.6", "20260820T100000", loadFixture(t, "stored_cost_report_a.json"))
	seedReport(fs, "/TestResults", "v2.0.0", "stored-suite", "claude-code",
		"claude-sonnet-4.6", "20260825T100000", loadFixture(t, "stored_cost_report_b.json"))
}

func TestRenderUserSummary_ShowsNoCost(t *testing.T) {
	v := stage1VersionSummary()

	out := resultsummary.RenderUserSummary(v)

	assertNoCost(t, "user summary", out)
}

func TestRenderUserSummary_CostWarningStats_ShowNoCost(t *testing.T) {
	v := multiModelVersionSummary()
	for model, byHarness := range v.ByModel {
		for harness, s := range byHarness {
			s.CostWarning = true
			v.ByModel[model][harness] = s
		}
	}

	out := resultsummary.RenderUserSummary(v)

	assertNoCost(t, "user summary with cost warnings", out)
}

func TestRenderInternalSummary_ShowsNoCost(t *testing.T) {
	v := multiModelVersionSummary()

	out := resultsummary.RenderInternalSummary(v)

	assertNoCost(t, "internal summary", out)
}

func TestRenderCrossVersionSummary_ShowsNoCost(t *testing.T) {
	cv := minimalCrossVersionSummary()

	out := resultsummary.RenderCrossVersionSummary(cv)

	assertNoCost(t, "cross-version summary", out)
}

// TestGenerate_StoredReportsWithCostFields_SummariesShowNoCost summarises
// stored-format reports that carry cost fields: they still parse and
// summarise, every generated summary shows no cost, and the stored files are
// left untouched.
func TestGenerate_StoredReportsWithCostFields_SummariesShowNoCost(t *testing.T) {
	fs := newFakeFS()
	seedStoredCostReports(t, fs)
	before := map[string]string{}
	for path, data := range fs.files {
		before[path] = string(data)
	}

	_, err := resultsummary.Generate(fs, resultsummary.SummaryRequest{TestResultsRoot: "/TestResults"})

	if err != nil {
		t.Fatalf("Generate failed on stored-format reports with cost fields: %v", err)
	}
	for _, path := range []string{
		"/TestResults/Orchestrator/v1.0.0/user-summary.md",
		"/TestResults/Orchestrator/v1.0.0/internal-summary.md",
		"/TestResults/Orchestrator/v2.0.0/user-summary.md",
		"/TestResults/Orchestrator/summary.md",
	} {
		content := readWrittenFile(t, fs, path)
		if !strings.Contains(content, "stored-suite") && !strings.Contains(content, "claude-code") {
			t.Errorf("%s does not summarise the stored reports.\ngot:\n%s", path, content)
		}
		assertNoCost(t, path, content)
	}
	for path, want := range before {
		if got := string(fs.files[path]); got != want {
			t.Errorf("stored file %s was modified by summarising", path)
		}
	}
}
