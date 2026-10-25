package cli_test

// The command-line displays show the suite-level model verdict: the
// suite-finished progress line carries the observed model label only when the
// suite is verified and the invalid verdict otherwise, and the result the run
// command prints (text or JSON) carries the verdict from the report model.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
	"mosaic-agent-test/internal/report"
)

const (
	displayedModel = "model-observed-x"
	otherModel     = "model-selected-y"
)

func suiteFinished(verdict domain.ModelVerdict, label string) domain.ProgressEvent {
	return domain.ProgressEvent{
		Kind:         domain.ProgressSuiteFinished,
		Counts:       map[domain.Verdict]int{domain.VerdictPass: 3, domain.VerdictFail: 1},
		ModelVerdict: verdict,
		ModelLabel:   label,
	}
}

func TestFormatEvent_SuiteFinished_ModelVerdict(t *testing.T) {
	cases := []struct {
		name string
		ev   domain.ProgressEvent
		want string
	}{
		{"verified suite is labelled by its observed model", suiteFinished(domain.ModelVerdictVerified, displayedModel), "suite finished: FAIL=1 PASS=3 [model " + displayedModel + "]"},
		{"unverified suite carries the verdict only", suiteFinished(domain.ModelVerdictUnverified, ""), "suite finished: FAIL=1 PASS=3 [MODEL UNVERIFIED]"},
		{"unverified suite never prints a label", suiteFinished(domain.ModelVerdictUnverified, displayedModel), "suite finished: FAIL=1 PASS=3 [MODEL UNVERIFIED]"},
		{"event from an older producer prints the counts alone", suiteFinished("", ""), "suite finished: FAIL=1 PASS=3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cli.FormatEvent(tc.ev); got != tc.want {
				t.Errorf("FormatEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLineSink_SuiteFinishedLineCarriesTheVerdict(t *testing.T) {
	var buf bytes.Buffer
	sink := cli.NewLineSink(&buf)

	sink.Emit(suiteFinished(domain.ModelVerdictUnverified, ""))

	if !strings.Contains(buf.String(), "[MODEL UNVERIFIED]") {
		t.Errorf("stream = %q, want the suite-finished line to carry [MODEL UNVERIFIED]", buf.String())
	}
}

// resultWith builds a result through report.Build holding one run.
func resultWith(run report.RunReport) report.Result {
	agg := domain.AggregateResult{
		TestName: "model-test", Verdict: domain.VerdictPass, Counted: 1, Passed: 1, PassRate: 1,
		TotalCost: domain.CostReport{Attribution: domain.AttributionAttributed},
	}
	if run.ModelValidity != domain.ModelVerified {
		agg = domain.AggregateResult{
			TestName: "model-test", Verdict: domain.VerdictFail, Excluded: 1, InfrastructureFailure: true,
			Reasons:   []domain.FailureReason{domain.ReasonInfrastructure},
			TotalCost: domain.CostReport{Attribution: domain.AttributionAttributed},
		}
	}
	return report.Build("model-suite", time.Time{}, time.Time{}, []report.TestReport{{
		TestName: "model-test", Layer: domain.LayerOrchestrator, Aggregate: agg, Runs: []report.RunReport{run},
	}}, "")
}

func verifiedRunReport() report.RunReport {
	return report.RunReport{
		Key:                   domain.RunKey{RunID: "r-1", TestName: "model-test", RunNumber: 1},
		Verdict:               domain.VerdictPass,
		// The selection differs textually from the observed model, so a label
		// taken from the selection cannot satisfy the display assertions.
		SubjectModel:          otherModel,
		ObservedSubjectModels: []string{displayedModel},
		ModelValidity:         domain.ModelVerified,
	}
}

// mixedResult holds a verified test and a mismatching one in the same suite.
func mixedResult() report.Result {
	verified := verifiedRunReport()
	verified.Key.TestName = "verified-test"
	invalid := mismatchRunReport()
	invalid.Key.TestName = "invalid-test"
	passAgg := domain.AggregateResult{
		TestName: "verified-test", Verdict: domain.VerdictPass, Counted: 1, Passed: 1, PassRate: 1,
		TotalCost: domain.CostReport{Attribution: domain.AttributionAttributed},
	}
	failAgg := domain.AggregateResult{
		TestName: "invalid-test", Verdict: domain.VerdictFail, Excluded: 1, InfrastructureFailure: true,
		Reasons:   []domain.FailureReason{domain.ReasonInfrastructure},
		TotalCost: domain.CostReport{Attribution: domain.AttributionAttributed},
	}
	return report.Build("model-suite", time.Time{}, time.Time{}, []report.TestReport{
		{TestName: "verified-test", Layer: domain.LayerOrchestrator, Aggregate: passAgg, Runs: []report.RunReport{verified}},
		{TestName: "invalid-test", Layer: domain.LayerOrchestrator, Aggregate: failAgg, Runs: []report.RunReport{invalid}},
	}, "")
}

func TestRunCommand_Text_MixedSuiteShowsTheVerdictAndNoModelLabel(t *testing.T) {
	_, stdout, _ := runWithResult(t, mixedResult())

	if !strings.Contains(stdout, "MODEL UNVERIFIED") {
		t.Errorf("stdout has no invalid verdict although one run is invalid.\n%s", stdout)
	}
	if strings.Contains(stdout, "Model: ") {
		t.Errorf("stdout carries a model heading although one run is invalid.\n%s", stdout)
	}
}

func TestRunCommand_JSON_MixedSuiteCarriesTheVerdictAndNoLabel(t *testing.T) {
	_, stdout, _ := runWithResult(t, mixedResult(), "--format", "json")

	var doc struct {
		ModelVerdict   string `json:"model_verdict"`
		ModelLabel     string `json:"model_label"`
		UnverifiedRuns int    `json:"unverified_runs"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if doc.ModelVerdict != "MODEL_UNVERIFIED" || doc.ModelLabel != "" || doc.UnverifiedRuns != 1 {
		t.Errorf("verdict/label/unverified = %q/%q/%d, want MODEL_UNVERIFIED//1", doc.ModelVerdict, doc.ModelLabel, doc.UnverifiedRuns)
	}
}

func mismatchRunReport() report.RunReport {
	return report.RunReport{
		Key:                   domain.RunKey{RunID: "r-1", TestName: "model-test", RunNumber: 1},
		Verdict:               domain.VerdictFail,
		SubjectModel:          otherModel,
		ObservedSubjectModels: []string{displayedModel},
		ModelValidity:         domain.ModelMismatch,
	}
}

func runWithResult(t *testing.T, result report.Result, extraArgs ...string) (code int, stdout, stderr string) {
	t.Helper()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	runner := &fakeSuiteRunner{result: result}
	opts := baseOptions(out, errOut, scriptedPreflight(preflight.Plan{}, cleanReport(), nil), runner)
	args := append([]string{"run", "suite.yaml"}, extraArgs...)
	code = cli.Execute(context.Background(), args, opts)
	return code, out.String(), errOut.String()
}

func TestRunCommand_Text_InvalidSuiteShowsTheVerdictAndNoModelLabel(t *testing.T) {
	_, stdout, _ := runWithResult(t, resultWith(mismatchRunReport()))

	if !strings.Contains(stdout, "MODEL UNVERIFIED") {
		t.Errorf("stdout has no invalid verdict.\n%s", stdout)
	}
	if strings.Contains(stdout, "Model: ") {
		t.Errorf("stdout carries a model heading for an invalid suite.\n%s", stdout)
	}
	for _, value := range []string{displayedModel, otherModel} {
		if strings.Contains(stdout, value) {
			t.Errorf("stdout shows the model %q for an invalid run.\n%s", value, stdout)
		}
	}
}

func TestRunCommand_Text_VerifiedSuiteShowsTheObservedModelAndNoInvalidVerdict(t *testing.T) {
	code, stdout, _ := runWithResult(t, resultWith(verifiedRunReport()))

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess", code)
	}
	if !strings.Contains(stdout, "Model: "+displayedModel) {
		t.Errorf("stdout has no model heading for the verified suite.\n%s", stdout)
	}
	if strings.Contains(stdout, "MODEL UNVERIFIED") {
		t.Errorf("a verified suite shows the invalid verdict.\n%s", stdout)
	}
	if strings.Contains(stdout, "Model: "+otherModel) {
		t.Errorf("the verified suite is labelled by the selection %q instead of the observed model.\n%s", otherModel, stdout)
	}
}

func TestRunCommand_JSON_CarriesTheSuiteVerdict(t *testing.T) {
	cases := []struct {
		name        string
		result      report.Result
		wantVerdict string
		wantLabel   string
	}{
		{"invalid suite", resultWith(mismatchRunReport()), "MODEL_UNVERIFIED", ""},
		{"verified suite", resultWith(verifiedRunReport()), "MODEL_VERIFIED", displayedModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stdout, _ := runWithResult(t, tc.result, "--format", "json")

			var doc struct {
				ModelVerdict string `json:"model_verdict"`
				ModelLabel   string `json:"model_label"`
			}
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if doc.ModelVerdict != tc.wantVerdict || doc.ModelLabel != tc.wantLabel {
				t.Errorf("model_verdict/model_label = %q/%q, want %q/%q", doc.ModelVerdict, doc.ModelLabel, tc.wantVerdict, tc.wantLabel)
			}
		})
	}
}

// An invalid run already makes its test aggregate FAIL as an infrastructure
// failure, so the exit code is the existing failure code.
func TestRunCommand_InvalidSuite_ExitsWithTheInfrastructureFailureCode(t *testing.T) {
	code, _, _ := runWithResult(t, resultWith(mismatchRunReport()))

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want ExitFailure (%d) for a suite with an invalid run", code, cli.ExitFailure)
	}
}

// The report file path the frontend derives comes from the suite and the time
// supplied by the composition root; the model selection never reaches it.
func TestRunCommand_ReportPath_NeverCarriesTheSelectedModel(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	capture := &captureWriteFile{}
	runner := &fakeSuiteRunner{result: resultWith(verifiedRunReport())}
	opts := withReportFile(
		baseOptions(out, errOut, scriptedPreflight(preflight.Plan{}, cleanReport(), nil), runner),
		"report-suite-20260822T110541.json",
		capture.write,
	)

	cli.Execute(context.Background(), []string{"run", "suite.yaml", "--subject-model", otherModel}, opts)

	call, ok := capture.lastCall()
	if !ok {
		t.Fatalf("no report was written (stderr=%s)", errOut.String())
	}
	if strings.Contains(call.path, otherModel) {
		t.Errorf("report path %q carries the selected model %q", call.path, otherModel)
	}
	if call.path != "report-suite-20260822T110541.json" {
		t.Errorf("report path = %q, want the path supplied by the composition root unchanged", call.path)
	}
}
