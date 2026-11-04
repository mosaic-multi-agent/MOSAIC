package report_test

// The text report labels a run and a suite by the observed subject model only.
// A suite with any invalid run says so on its first line and in its summary and
// carries no model label at all; the selected and observed values of an invalid
// run appear only in its excluded-run detail line.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

// noRunsLine is the verdict line of a suite in which no run was evaluated.
const noRunsLine = "MODEL UNVERIFIED: no runs were evaluated; this is not a model result"

func unverifiedLine(n int) string {
	return "MODEL UNVERIFIED: " + strconv.Itoa(n) + " run(s) have no verified subject model; this is not a model result"
}

func renderTextOK(t *testing.T, r report.Result) string {
	t.Helper()
	out, err := renderText(t, r)
	if err != nil {
		t.Fatalf("RenderText returned an error: %v", err)
	}
	return out
}

func TestRenderText_VerifiedSuite_HeaderCarriesObservedModelLabel(t *testing.T) {
	result := buildModelResult(passTest("t1", verifiedRun("t1", 1)))

	out := renderTextOK(t, result)

	const want = "Model: " + verifiedLabel
	headerEnd := strings.Index(out, "\n\n")
	if headerEnd < 0 || !strings.Contains(out[:headerEnd], want) {
		t.Errorf("header has no %q line.\n--- got ---\n%s", want, out)
	}
	if strings.Contains(out, "MODEL UNVERIFIED") {
		t.Errorf("a suite of verified runs must not carry the invalid verdict.\n--- got ---\n%s", out)
	}
}

func TestRenderText_VerifiedRun_PrintsObservedModelAndSelectedStub(t *testing.T) {
	result := buildModelResult(passTest("t1", verifiedRun("t1", 1)))

	out := renderTextOK(t, result)

	for _, want := range []string{"observed subject model: " + selectedModel, "selected stub model: " + stubSelection} {
		if !strings.Contains(out, want) {
			t.Errorf("run line %q missing.\n--- got ---\n%s", want, out)
		}
	}
	for _, old := range []string{"subject model:", "stub model:"} {
		if lineStartsWith(out, old) {
			t.Errorf("a line starts with the selection-based label %q; labels come from the observed model.\n--- got ---\n%s", old, out)
		}
	}
}

// The run line shows the observed value even when it differs textually from
// the selection field, so a label taken from the selection cannot satisfy it.
func TestRenderText_VerifiedRun_LabelIsTakenFromObservedList(t *testing.T) {
	run := verifiedRun("t1", 1)
	run.SubjectModel = "selection-text"
	run.ObservedSubjectModels = []string{"observed-text"}

	out := renderTextOK(t, buildModelResult(passTest("t1", run)))

	if !strings.Contains(out, "observed subject model: observed-text") {
		t.Errorf("run line does not show the observed model.\n--- got ---\n%s", out)
	}
	if strings.Contains(out, "selection-text") {
		t.Errorf("the selection %q appears in the report of a verified run.\n--- got ---\n%s", "selection-text", out)
	}
}

func TestRenderText_VerifiedRun_AbsentStubSelectionRendersAsUnknown(t *testing.T) {
	run := verifiedRun("t1", 1)
	run.StubModel = ""

	out := renderTextOK(t, buildModelResult(passTest("t1", run)))

	if !strings.Contains(out, "selected stub model: unknown") {
		t.Errorf("an absent stub selection must render as unknown.\n--- got ---\n%s", out)
	}
}

func TestRenderText_InvalidRun_CarriesNoModelLabel(t *testing.T) {
	cases := []struct {
		name string
		run  report.RunReport
	}{
		{"mismatch", invalidRun("t1", domain.ModelMismatch, observedOther)},
		{"no evidence", invalidRun("t1", domain.ModelNoEvidence)},
		{"not assessed", invalidRun("t1", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildModelResult(invalidTest("t1", tc.run))

			out := renderTextOK(t, result)

			first := strings.SplitN(out, "\n", 2)[0]
			if first != unverifiedLine(1) {
				t.Errorf("first line = %q, want %q", first, unverifiedLine(1))
			}
			if lineStartsWith(out, "Model:") {
				t.Errorf("an invalid suite carries a Model: heading.\n--- got ---\n%s", out)
			}
			for _, prefix := range []string{"observed subject model:", "selected stub model:", "subject model:", "stub model:"} {
				if lineStartsWith(out, prefix) {
					t.Errorf("an invalid run prints a %q line.\n--- got ---\n%s", prefix, out)
				}
			}
			rest := withoutExcludedLines(out)
			for _, value := range []string{selectedModel, observedOther, stubSelection} {
				if strings.Contains(rest, value) {
					t.Errorf("value %q appears outside the excluded-run detail.\n--- got ---\n%s", value, out)
				}
			}
		})
	}
}

func TestRenderText_InvalidRun_SelectedAndObservedAppearInExcludedDetail(t *testing.T) {
	result := buildModelResult(invalidTest("t1", invalidRun("t1", domain.ModelMismatch, observedOther)))

	out := renderTextOK(t, result)

	var detail string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "excluded [") {
			detail = l
		}
	}
	if detail == "" {
		t.Fatalf("no excluded-run detail line.\n--- got ---\n%s", out)
	}
	for _, want := range []string{selectedModel, observedOther} {
		if !strings.Contains(detail, want) {
			t.Errorf("excluded detail %q does not carry %q", detail, want)
		}
	}
}

func TestRenderText_InvalidSuite_SummaryCarriesTheVerdictToo(t *testing.T) {
	result := buildModelResult(
		passTest("t1", verifiedRun("t1", 1)),
		invalidTest("t2", invalidRun("t2", domain.ModelNoEvidence)),
	)

	out := renderTextOK(t, result)

	if n := strings.Count(out, unverifiedLine(1)); n != 2 {
		t.Errorf("verdict line appears %d times, want 2 (first line and summary).\n--- got ---\n%s", n, out)
	}
	summary := out[strings.LastIndex(out, "\n\n"):]
	if !strings.Contains(summary, unverifiedLine(1)) {
		t.Errorf("summary has no verdict line.\n--- summary ---\n%s", summary)
	}
	if lineStartsWith(out, "Model:") {
		t.Errorf("a suite with an invalid run carries a Model: heading even though one run was verified.\n--- got ---\n%s", out)
	}
}

func TestRenderText_InvalidSuite_CountsEveryUnverifiedRun(t *testing.T) {
	result := buildModelResult(
		invalidTest("t1", invalidRun("t1", domain.ModelMismatch, observedOther)),
		invalidTest("t2", invalidRun("t2", domain.ModelNoEvidence)),
	)

	out := renderTextOK(t, result)

	if !strings.Contains(out, unverifiedLine(2)) {
		t.Errorf("verdict line does not count 2 unverified runs.\n--- got ---\n%s", out)
	}
}

func TestRenderText_SuiteWithoutRuns_SaysNoRunsWereEvaluated(t *testing.T) {
	result := buildModelResult()

	out := renderTextOK(t, result)

	const want = noRunsLine
	if first := strings.SplitN(out, "\n", 2)[0]; first != want {
		t.Errorf("first line = %q, want %q", first, want)
	}
	if n := strings.Count(out, want); n != 2 {
		t.Errorf("verdict line appears %d times, want 2.\n--- got ---\n%s", n, out)
	}
}

// The verdict is not a model result: nothing about it names a model.
func TestRenderText_InvalidVerdictLine_NamesNoModel(t *testing.T) {
	result := buildModelResult(invalidTest("t1", invalidRun("t1", domain.ModelMismatch, observedOther)))

	out := renderTextOK(t, result)

	found := false
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, "MODEL UNVERIFIED") {
			continue
		}
		found = true
		for _, value := range []string{selectedModel, observedOther, stubSelection} {
			if strings.Contains(l, value) {
				t.Errorf("verdict line %q names the model %q", l, value)
			}
		}
	}
	if !found {
		t.Errorf("no verdict line.\n--- got ---\n%s", out)
	}
}
