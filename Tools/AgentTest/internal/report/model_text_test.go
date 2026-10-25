package report_test

// Tests for the models appearing in the human-readable (text) report and for
// both renderings agreeing on their values.
//
// A verified run prints "observed subject model: <v>" (the model its logs
// show) and "selected stub model: <v>" (the stub selection, which is never
// verified). When the stub selection is empty the text value is "unknown", the
// same mapping the JSON wire projection applies, so the two renderings always
// agree on one shared value. A run that is not verified prints neither line;
// that rule is covered with the model verdict tests.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/report"
)

// fixtureVerifiedWithModels is one verified run: the selection, the observed
// model and the stub selection set as given.
func fixtureVerifiedWithModels(selected, observed, stub string) report.Result {
	run := verifiedRun("model-attribution-test", 1)
	run.SubjectModel = selected
	run.ObservedSubjectModels = []string{observed}
	run.StubModel = stub
	return buildModelResult(passTest("model-attribution-test", run))
}

// TestRenderText_ObservedSubjectModelAppearsInOutput asserts that a verified
// run's report names the model its logs show, so a developer can attribute a
// result to the model that ran without opening the machine-readable report.
func TestRenderText_ObservedSubjectModelAppearsInOutput(t *testing.T) {
	const wantModel = "claude-opus-4-5"
	result := fixtureVerifiedWithModels(wantModel, wantModel, "claude-haiku-4-5")

	out, err := renderText(t, result)
	if err != nil {
		t.Fatalf("RenderText returned an error: %v", err)
	}

	wantSubstr := "observed subject model: " + wantModel
	if !strings.Contains(out, wantSubstr) {
		t.Errorf("terminal report has no %q line.\n--- got ---\n%s", wantSubstr, out)
	}
}

// TestRenderText_SelectedStubModelAppearsInOutput asserts that a verified
// run's report names the stub model that was selected.
func TestRenderText_SelectedStubModelAppearsInOutput(t *testing.T) {
	const wantStubModel = "claude-haiku-4-5"
	result := fixtureVerifiedWithModels("claude-opus-4-5", "claude-opus-4-5", wantStubModel)

	out, err := renderText(t, result)
	if err != nil {
		t.Fatalf("RenderText returned an error: %v", err)
	}

	wantSubstr := "selected stub model: " + wantStubModel
	if !strings.Contains(out, wantSubstr) {
		t.Errorf("terminal report has no %q line.\n--- got ---\n%s", wantSubstr, out)
	}
}

// TestRenderText_AbsentStubModelRendersAsUnknown asserts that an empty stub
// selection prints "selected stub model: unknown" rather than an empty or
// missing line. An absent model must be visibly distinct from a real one.
func TestRenderText_AbsentStubModelRendersAsUnknown(t *testing.T) {
	result := fixtureVerifiedWithModels("claude-opus-4-5", "claude-opus-4-5", "")

	out, err := renderText(t, result)
	if err != nil {
		t.Fatalf("RenderText returned an error: %v", err)
	}

	const wantSubstr = "selected stub model: unknown"
	if !strings.Contains(out, wantSubstr) {
		t.Errorf("terminal report does not contain %q for an absent stub selection.\n--- got ---\n%s", wantSubstr, out)
	}
}

// TestReportsAgree_ModelsConsistent asserts that both the terminal and
// machine-readable reports carry the same observed model and stub selection
// for the same run.
func TestReportsAgree_ModelsConsistent(t *testing.T) {
	for _, tc := range []struct {
		name        string
		observed    string
		stubModel   string
		wantStub    string // the agreed value both renderings must show
		wantObserve string
	}{
		{"both models recorded", "claude-opus-4-5", "claude-haiku-4-5", "claude-haiku-4-5", "claude-opus-4-5"},
		{"no stub recorded", "claude-opus-4-5", "", "unknown", "claude-opus-4-5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := fixtureVerifiedWithModels(tc.observed, tc.observed, tc.stubModel)

			textOut, textErr := renderText(t, result)
			jsonOut, jsonErr := renderJSON(t, result)
			if textErr != nil {
				t.Fatalf("RenderText returned an error: %v", textErr)
			}
			if jsonErr != nil {
				t.Fatalf("RenderJSON returned an error: %v", jsonErr)
			}

			if got := decodeJSONRunStubModel(t, jsonOut); got != tc.wantStub {
				t.Errorf("JSON stub_model = %q, want %q", got, tc.wantStub)
			}
			if got := decodeJSONRunObservedModels(t, jsonOut); len(got) != 1 || got[0] != tc.wantObserve {
				t.Errorf("JSON observed_subject_models = %v, want [%s]", got, tc.wantObserve)
			}
			for _, want := range []string{"observed subject model: " + tc.wantObserve, "selected stub model: " + tc.wantStub} {
				if !strings.Contains(textOut, want) {
					t.Errorf("terminal report does not contain %q; both renderings must agree.\n--- got ---\n%s", want, textOut)
				}
			}
		})
	}
}
