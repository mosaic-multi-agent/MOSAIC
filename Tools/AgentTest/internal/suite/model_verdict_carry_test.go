package suite_test

// The suite carries each run's observed subject model(s) and model validity
// from the evaluated result into the report, and the suite-level model verdict
// derived by the report into the suite-finished progress event.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

// runOneTest runs a single one-repetition test with the scripted outcome and
// returns the report and every progress event.
func runOneTest(t *testing.T, outcome scriptedOutcome) (report.Result, []domain.ProgressEvent) {
	t.Helper()
	runner := newScriptedRunner()
	runner.scriptFor("model-test", outcome)
	sink := &recordingSink{}
	s := newSuite(runner, newFakeClock(), sink)

	result, err := runSuite(t, s, context.Background(), buildPlan(resolvedTest("model-test", 1, 1.0)))
	if err != nil {
		t.Fatalf("Suite.Run returned an error: %v", err)
	}
	if len(result.Tests) != 1 || len(result.Tests[0].Runs) != 1 {
		t.Fatalf("result holds %d tests, want one test with one run", len(result.Tests))
	}
	return result, sink.all()
}

func suiteFinishedEvent(t *testing.T, events []domain.ProgressEvent) domain.ProgressEvent {
	t.Helper()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == domain.ProgressSuiteFinished {
			return events[i]
		}
	}
	t.Fatal("no ProgressSuiteFinished event was emitted")
	return domain.ProgressEvent{}
}

func TestSuite_ObservedModelAndValidityCarriedToRunReport(t *testing.T) {
	cases := []struct {
		name         string
		evidence     domain.RunEvidence
		wantValidity domain.ModelValidity
		wantObserved []string
	}{
		{"verified", passingEvidence(), domain.ModelVerified, []string{scriptedModel}},
		{"mismatch", withObservedModel(passingEvidence(), "model-b"), domain.ModelMismatch, []string{"model-b"}},
		{"no evidence", withoutModelEvidence(passingEvidence()), domain.ModelNoEvidence, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := runOneTest(t, scriptedOutcome{evidence: tc.evidence})

			run := result.Tests[0].Runs[0]
			if run.ModelValidity != tc.wantValidity {
				t.Errorf("RunReport.ModelValidity = %q, want %q", run.ModelValidity, tc.wantValidity)
			}
			if len(run.ObservedSubjectModels) != len(tc.wantObserved) ||
				(len(tc.wantObserved) > 0 && !reflect.DeepEqual(run.ObservedSubjectModels, tc.wantObserved)) {
				t.Errorf("RunReport.ObservedSubjectModels = %v, want %v", run.ObservedSubjectModels, tc.wantObserved)
			}
			if run.SubjectModel != scriptedModel {
				t.Errorf("RunReport.SubjectModel = %q, want the selection %q to stay beside the observed models", run.SubjectModel, scriptedModel)
			}
		})
	}
}

func TestSuite_RunnerErrorRunIsNotAssessed(t *testing.T) {
	result, _ := runOneTest(t, scriptedOutcome{err: errors.New("provisioning failed")})

	run := result.Tests[0].Runs[0]
	if run.ModelValidity != "" {
		t.Errorf("RunReport.ModelValidity = %q, want empty (not assessed) for a repetition whose runner returned an error", run.ModelValidity)
	}
	if result.ModelVerdict != domain.ModelVerdictUnverified || result.UnverifiedRuns != 1 {
		t.Errorf("ModelVerdict = %q, UnverifiedRuns = %d, want MODEL_UNVERIFIED and 1: a run that was not assessed is not verified", result.ModelVerdict, result.UnverifiedRuns)
	}
}

func TestSuite_ModelVerdict_DerivedForTheSuite(t *testing.T) {
	cases := []struct {
		name        string
		evidence    domain.RunEvidence
		wantVerdict domain.ModelVerdict
		wantLabel   string
	}{
		{"verified run", passingEvidence(), domain.ModelVerdictVerified, scriptedModel},
		{"mismatching run", withObservedModel(passingEvidence(), "model-b"), domain.ModelVerdictUnverified, ""},
		{"run without evidence", withoutModelEvidence(passingEvidence()), domain.ModelVerdictUnverified, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, events := runOneTest(t, scriptedOutcome{evidence: tc.evidence})

			if result.ModelVerdict != tc.wantVerdict || result.ModelLabel != tc.wantLabel {
				t.Errorf("Result verdict/label = %q/%q, want %q/%q", result.ModelVerdict, result.ModelLabel, tc.wantVerdict, tc.wantLabel)
			}
			finished := suiteFinishedEvent(t, events)
			if finished.ModelVerdict != result.ModelVerdict || finished.ModelLabel != result.ModelLabel {
				t.Errorf("ProgressSuiteFinished verdict/label = %q/%q, want the result's %q/%q",
					finished.ModelVerdict, finished.ModelLabel, result.ModelVerdict, result.ModelLabel)
			}
		})
	}
}
