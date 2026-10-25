package report_test

// The suite-level model verdict is derived once, by report.Build, from the
// validity of every run. It is MODEL_VERIFIED only when at least one run
// exists and every run is verified; the label is taken from the observed model
// of the verified runs and is empty otherwise.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/report"
)

func TestBuild_ModelVerdict(t *testing.T) {
	cases := []struct {
		name           string
		tests          []report.TestReport
		wantVerdict    domain.ModelVerdict
		wantLabel      string
		wantUnverified int
	}{
		{
			name:        "every run verified",
			tests:       []report.TestReport{passTest("t1", verifiedRun("t1", 1), verifiedRun("t1", 2)), passTest("t2", verifiedRun("t2", 1))},
			wantVerdict: domain.ModelVerdictVerified,
			wantLabel:   verifiedLabel,
		},
		{
			name: "one mismatching run among verified runs",
			tests: []report.TestReport{
				passTest("t1", verifiedRun("t1", 1)),
				invalidTest("t2", invalidRun("t2", domain.ModelMismatch, observedOther)),
			},
			wantVerdict:    domain.ModelVerdictUnverified,
			wantUnverified: 1,
		},
		{
			name: "one run without evidence in a later test",
			tests: []report.TestReport{
				passTest("t1", verifiedRun("t1", 1), verifiedRun("t1", 2)),
				passTest("t2", verifiedRun("t2", 1)),
				invalidTest("t3", invalidRun("t3", domain.ModelNoEvidence)),
			},
			wantVerdict:    domain.ModelVerdictUnverified,
			wantUnverified: 1,
		},
		{
			name: "a run that was not assessed counts as unverified",
			tests: []report.TestReport{
				passTest("t1", verifiedRun("t1", 1), invalidRun("t1", "")),
			},
			wantVerdict:    domain.ModelVerdictUnverified,
			wantUnverified: 1,
		},
		{
			name: "every run invalid counts every run",
			tests: []report.TestReport{
				invalidTest("t1", invalidRun("t1", domain.ModelMismatch, observedOther)),
				invalidTest("t2", invalidRun("t2", domain.ModelNoEvidence)),
				invalidTest("t3", invalidRun("t3", "")),
			},
			wantVerdict:    domain.ModelVerdictUnverified,
			wantUnverified: 3,
		},
		{
			name:        "no tests at all",
			tests:       nil,
			wantVerdict: domain.ModelVerdictUnverified,
		},
		{
			name:        "tests without runs",
			tests:       []report.TestReport{{TestName: "t1"}},
			wantVerdict: domain.ModelVerdictUnverified,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildModelResult(tc.tests...)

			if got.ModelVerdict != tc.wantVerdict {
				t.Errorf("ModelVerdict = %q, want %q", got.ModelVerdict, tc.wantVerdict)
			}
			if got.ModelLabel != tc.wantLabel {
				t.Errorf("ModelLabel = %q, want %q", got.ModelLabel, tc.wantLabel)
			}
			if got.UnverifiedRuns != tc.wantUnverified {
				t.Errorf("UnverifiedRuns = %d, want %d", got.UnverifiedRuns, tc.wantUnverified)
			}
		})
	}
}

// The label is the observed model, not the selection.
func TestBuild_ModelLabelComesFromObservedModel(t *testing.T) {
	run := verifiedRun("t1", 1)
	run.SubjectModel = "selection-text"
	run.ObservedSubjectModels = []string{"observed-text"}

	got := buildModelResult(passTest("t1", run))

	if got.ModelLabel != "observed-text" {
		t.Errorf("ModelLabel = %q, want the observed model %q, never the selection", got.ModelLabel, "observed-text")
	}
}

// Verified runs that observed different models have no single model to label
// the suite by. The label must not silently take the first run's model; the
// verdict is still derived (verified or unverified, never left empty).
func TestBuild_VerifiedRunsObservingDifferentModels_HaveNoSingleLabel(t *testing.T) {
	runA := verifiedRun("t1", 1)
	runB := verifiedRun("t2", 1)
	runB.ObservedSubjectModels = []string{observedOther}

	got := buildModelResult(passTest("t1", runA), passTest("t2", runB))

	if got.ModelVerdict != domain.ModelVerdictVerified && got.ModelVerdict != domain.ModelVerdictUnverified {
		t.Errorf("ModelVerdict = %q, want a derived verdict", got.ModelVerdict)
	}
	if got.ModelLabel != "" {
		t.Errorf("ModelLabel = %q, want no label when the runs observed %q and %q", got.ModelLabel, selectedModel, observedOther)
	}
}

// A verified run that observed more than one model is labelled by all of them.
func TestRunReport_ModelLabel_MultipleObservedModelsNameEachOne(t *testing.T) {
	run := verifiedRun("t", 1)
	run.SubjectModel = "selection-text"
	run.ObservedSubjectModels = []string{"observed-one", "observed-two"}

	got := run.ModelLabel()

	for _, want := range []string{"observed-one", "observed-two"} {
		if !strings.Contains(got, want) {
			t.Errorf("ModelLabel() = %q, want it to name every observed model, missing %q", got, want)
		}
	}
	if strings.Contains(got, "selection-text") {
		t.Errorf("ModelLabel() = %q carries the selection", got)
	}
}

func TestRunReport_ModelLabel(t *testing.T) {
	cases := []struct {
		name string
		run  report.RunReport
		want string
	}{
		{"verified run labels by its observed model", verifiedRun("t", 1), selectedModel},
		{"observed model wins over the selection", func() report.RunReport {
			r := verifiedRun("t", 1)
			r.SubjectModel = "selection-text"
			r.ObservedSubjectModels = []string{"observed-text"}
			return r
		}(), "observed-text"},
		{"mismatch has no label", invalidRun("t", domain.ModelMismatch, observedOther), ""},
		{"no evidence has no label", invalidRun("t", domain.ModelNoEvidence), ""},
		{"not assessed has no label even with a selection", invalidRun("t", ""), ""},
		{"unverified run never labels by what was observed", func() report.RunReport {
			r := invalidRun("t", domain.ModelNoEvidence, observedOther)
			return r
		}(), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.run.ModelLabel(); got != tc.want {
				t.Errorf("ModelLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}
