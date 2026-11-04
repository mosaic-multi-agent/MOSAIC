package tui

// The results screens show the suite-level model verdict, prominently and read
// from the report model, and a model label only for a verified suite or run:
// the observed model. An invalid run or suite shows no model label.

import (
	"strings"
	"testing"
	"time"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

const (
	tuiObservedModel = "model-observed-x"
	tuiSelectedModel = "model-selected-y"
)

// modelRun is one run of the named test. Every run selected tuiSelectedModel; a
// verified or mismatching run observed tuiObservedModel, a run without
// evidence observed nothing. The two values differ so a label taken from the
// selection cannot satisfy a display assertion.
func modelRun(testName string, validity domain.ModelValidity) report.RunReport {
	run := report.RunReport{
		Key:           domain.RunKey{TestName: testName, RunNumber: 1},
		Verdict:       domain.VerdictPass,
		Duration:      time.Second,
		ModelValidity: validity,
	}
	switch validity {
	case domain.ModelVerified:
		run.SubjectModel = tuiSelectedModel
		run.ObservedSubjectModels = []string{tuiObservedModel}
	case domain.ModelMismatch:
		run.Verdict = domain.VerdictFail
		run.SubjectModel = tuiSelectedModel
		run.ObservedSubjectModels = []string{tuiObservedModel}
	default:
		run.Verdict = domain.VerdictFail
		run.SubjectModel = tuiSelectedModel
	}
	return run
}

func modelResult(suiteID, testName string, validity domain.ModelValidity) report.Result {
	agg := domain.AggregateResult{TestName: testName, Verdict: domain.VerdictPass, Counted: 1, Passed: 1, PassRate: 1}
	if validity != domain.ModelVerified {
		agg = domain.AggregateResult{
			TestName: testName, Verdict: domain.VerdictFail, Excluded: 1, InfrastructureFailure: true,
			Reasons: []domain.FailureReason{domain.ReasonInfrastructure},
		}
	}
	return report.Build(suiteID, time.Now(), time.Now(), []report.TestReport{{
		TestName: testName, Aggregate: agg, Runs: []report.RunReport{modelRun(testName, validity)},
	}}, "")
}

func resultsViewFor(t *testing.T, result report.Result) Model {
	t.Helper()
	runner := newFakeSuiteRunner().withResult(result)
	m := NewModel(newFixtureOptions([]string{"suite-a.yaml"}, runner))
	m = runSuiteToCompletion(t, m, runner)
	if m.Screen() != ScreenResults {
		t.Fatalf("Screen() = %q, want %q", m.Screen(), ScreenResults)
	}
	return m
}

func assertNoModelValues(t *testing.T, view string) {
	t.Helper()
	for _, value := range []string{tuiObservedModel, tuiSelectedModel} {
		if strings.Contains(view, value) {
			t.Errorf("view shows the model %q for an invalid run.\nview:\n%s", value, view)
		}
	}
}

func TestResults_InvalidSuite_ShowsTheVerdictAndNoModelLabel(t *testing.T) {
	for _, validity := range []domain.ModelValidity{domain.ModelMismatch, domain.ModelNoEvidence, ""} {
		t.Run(string(validity), func(t *testing.T) {
			m := resultsViewFor(t, modelResult("suite-a", "model-test", validity))

			view := safeView(t, m)

			if !strings.Contains(view, "MODEL UNVERIFIED") {
				t.Errorf("results screen has no invalid verdict.\nview:\n%s", view)
			}
			assertNoModelValues(t, view)
		})
	}
}

func TestResults_VerifiedSuite_ShowsTheObservedModelAndNoInvalidVerdict(t *testing.T) {
	m := resultsViewFor(t, modelResult("suite-a", "model-test", domain.ModelVerified))

	view := safeView(t, m)

	if !strings.Contains(view, tuiObservedModel) {
		t.Errorf("results screen does not show the observed model for a verified suite.\nview:\n%s", view)
	}
	if strings.Contains(view, tuiSelectedModel) {
		t.Errorf("results screen shows the selection %q of a verified suite instead of the observed model.\nview:\n%s", tuiSelectedModel, view)
	}
	if strings.Contains(view, "MODEL UNVERIFIED") {
		t.Errorf("a verified suite shows the invalid verdict.\nview:\n%s", view)
	}
}

func TestResults_SessionWithOneInvalidSuite_ShowsTheVerdict(t *testing.T) {
	runner := newPerCallRunner(
		modelResult("suite-a", "test-a", domain.ModelVerified),
		modelResult("suite-b", "test-b", domain.ModelNoEvidence),
	)
	m := driveTwoSuitesToCompletion(t, []string{"suite-a.yaml", "suite-b.yaml"}, runner)
	if m.Screen() != ScreenResults {
		t.Fatalf("Screen() = %q, want %q", m.Screen(), ScreenResults)
	}

	view := safeView(t, m)

	if !strings.Contains(view, "MODEL UNVERIFIED") {
		t.Errorf("session results do not show the invalid verdict although one suite had an invalid run.\nview:\n%s", view)
	}
	assertNoModelValues(t, view)
}

func TestDetail_VerifiedRun_ShowsTheObservedModel(t *testing.T) {
	m := modelWithFinishedResult(t, modelRun("test-under-test", domain.ModelVerified))

	view := safeView(t, m)

	if !strings.Contains(view, tuiObservedModel) {
		t.Errorf("run detail does not show the observed model of a verified run.\nview:\n%s", view)
	}
	if strings.Contains(view, tuiSelectedModel) {
		t.Errorf("run detail shows the selection %q of a verified run instead of the observed model.\nview:\n%s", tuiSelectedModel, view)
	}
}

func TestDetail_InvalidRun_ShowsNoModelLabel(t *testing.T) {
	for _, validity := range []domain.ModelValidity{domain.ModelMismatch, domain.ModelNoEvidence, ""} {
		t.Run(string(validity), func(t *testing.T) {
			m := modelWithFinishedResult(t, modelRun("test-under-test", validity))

			view := safeView(t, m)

			if m.Screen() != ScreenDetail {
				t.Fatalf("Screen() = %q, want %q", m.Screen(), ScreenDetail)
			}
			assertNoModelValues(t, view)
		})
	}
}
