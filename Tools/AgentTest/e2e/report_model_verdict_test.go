package e2e

// The report of a real run names what ran: the observed subject model beside
// the selection in every run record, and a suite-level model verdict. A suite
// with any invalid run carries the invalid verdict and no model label; a suite
// of verified runs is labelled by the observed model.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
)

type modelReportDoc struct {
	ModelVerdict   string `json:"model_verdict"`
	ModelLabel     string `json:"model_label"`
	UnverifiedRuns int    `json:"unverified_runs"`
	Tests          []struct {
		Runs []struct {
			SubjectModel          string   `json:"subject_model"`
			ObservedSubjectModels []string `json:"observed_subject_models"`
			ModelVerification     string   `json:"model_verification"`
		} `json:"runs"`
	} `json:"tests"`
}

func decodeModelReport(t *testing.T, out Outcome) modelReportDoc {
	t.Helper()
	var doc modelReportDoc
	if err := json.Unmarshal(out.Stdout, &doc); err != nil {
		t.Fatalf("stdout is not the JSON report: %v\n%s", err, out.Stdout)
	}
	if len(doc.Tests) != 1 || len(doc.Tests[0].Runs) != 1 {
		t.Fatalf("report holds %d tests, want one test with one run\n%s", len(doc.Tests), out.Stdout)
	}
	return doc
}

func TestReportModel_VerifiedRun_IsLabelledByTheObservedModel(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{scenarioSubjectModel}}}

	out := RunScenario(t, invalidModelScenario(evidence))

	doc := decodeModelReport(t, out)
	if doc.ModelVerdict != "MODEL_VERIFIED" || doc.ModelLabel != scenarioSubjectModel || doc.UnverifiedRuns != 0 {
		t.Errorf("verdict/label/unverified = %q/%q/%d, want MODEL_VERIFIED/%q/0", doc.ModelVerdict, doc.ModelLabel, doc.UnverifiedRuns, scenarioSubjectModel)
	}
	run := doc.Tests[0].Runs[0]
	if run.ModelVerification != "verified" || len(run.ObservedSubjectModels) != 1 || run.ObservedSubjectModels[0] != scenarioSubjectModel {
		t.Errorf("run record = %+v, want verified with observed [%s]", run, scenarioSubjectModel)
	}
	if run.SubjectModel != scenarioSubjectModel {
		t.Errorf("subject_model = %q, want the selection %q beside the observed model", run.SubjectModel, scenarioSubjectModel)
	}
}

func TestReportModel_InvalidRun_CarriesTheVerdictAndNoLabel(t *testing.T) {
	cases := []struct {
		name         string
		answer       domain.ObservedModelEvidence
		wantRun      string
		wantObserved []string
	}{
		{"mismatch", domain.ObservedModelEvidence{Models: []string{"some-other-model"}}, "mismatch", []string{"some-other-model"}},
		{"no evidence", domain.ObservedModelEvidence{NoEvidence: domain.NoEvidenceNoSubjectUsage, Detail: "no subject usage in the logs"}, "no_evidence", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RunScenario(t, invalidModelScenario(&countingEvidence{answer: tc.answer}))

			doc := decodeModelReport(t, out)
			if doc.ModelVerdict != "MODEL_UNVERIFIED" || doc.ModelLabel != "" || doc.UnverifiedRuns != 1 {
				t.Errorf("verdict/label/unverified = %q/%q/%d, want MODEL_UNVERIFIED//1", doc.ModelVerdict, doc.ModelLabel, doc.UnverifiedRuns)
			}
			run := doc.Tests[0].Runs[0]
			if run.ModelVerification != tc.wantRun {
				t.Errorf("model_verification = %q, want %q", run.ModelVerification, tc.wantRun)
			}
			if strings.Join(run.ObservedSubjectModels, ",") != strings.Join(tc.wantObserved, ",") {
				t.Errorf("observed_subject_models = %v, want %v", run.ObservedSubjectModels, tc.wantObserved)
			}
		})
	}
}

func TestReportModel_InvalidRun_TextReportCarriesNoModelLabel(t *testing.T) {
	sc := invalidModelScenario(&countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{"some-other-model"}}})
	sc.Args = []string{"run", filepath.Join(moduleRoot(t), sc.Dir, sc.SuitePath), "--subject-model", scenarioSubjectModel}

	out := RunScenario(t, sc)

	text := string(out.Stdout)
	// stdout starts with the progress stream; the report follows it. The
	// verdict opens the report, so it must precede the report's Suite header.
	lines := strings.Split(text, "\n")
	verdictAt, suiteAt := -1, -1
	for i, line := range lines {
		if verdictAt < 0 && strings.HasPrefix(line, "MODEL UNVERIFIED") {
			verdictAt = i
		}
		if suiteAt < 0 && strings.HasPrefix(line, "Suite:") {
			suiteAt = i
		}
	}
	if verdictAt < 0 || suiteAt < 0 || verdictAt > suiteAt {
		t.Errorf("verdict line at %d, Suite header at %d: want the invalid verdict to open the report\n%s", verdictAt, suiteAt, text)
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "excluded [") {
			continue
		}
		for _, value := range []string{scenarioSubjectModel, "some-other-model"} {
			if strings.Contains(line, value) {
				t.Errorf("line %q carries the model %q outside the infrastructure-failure detail", line, value)
			}
		}
	}
}
