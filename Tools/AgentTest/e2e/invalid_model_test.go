package e2e

// A run is valid only when the model the logs show for the subject session is
// the model that was selected. A different model, or no model evidence at all,
// makes the run invalid: excluded from the pass rate, reported as an
// infrastructure failure naming both values, and never retried.

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"mosaic-agent-test/internal/cli"
	"mosaic-agent-test/internal/domain"
)

// countingEvidence answers every query with a fixed observation and counts the
// queries, one per attempt.
type countingEvidence struct {
	answer  domain.ObservedModelEvidence
	err     error
	queries atomic.Int32
}

func (c *countingEvidence) ObservedSubjectModels(_ context.Context, _ domain.ObservedModelQuery) (domain.ObservedModelEvidence, error) {
	c.queries.Add(1)
	return c.answer, c.err
}

func invalidModelScenario(evidence domain.ObservedModelProvider) Scenario {
	return Scenario{
		Dir:       "testdata/e2e/happy-path",
		SuitePath: "happy-path.suite.yaml",
		Script: fakeOptionsFor(
			domain.HarnessCapabilities{SupportsDirectSubstitution: true},
			[]subjectStep{
				invoke("dispatch", "greeter"),
				finish(`{"agent_instance_id":"standin-subject#1","status_code":"SUCCESS","status_message":"Done."}`),
			},
		),
		ObservedModels: evidence,
	}
}

// assertInvalidModelRun checks everything an invalid run must look like on the
// wire, and returns the single exclusion for further checks.
func assertInvalidModelRun(t *testing.T, out Outcome, wantReason string) wireExclusionDoc {
	t.Helper()

	if out.ExitCode != cli.ExitFailure {
		t.Fatalf("exit code = %d, want %d (ExitFailure)\nstdout: %s\nstderr: %s", out.ExitCode, cli.ExitFailure, out.Stdout, out.Stderr)
	}
	if len(out.Result.Tests) != 1 {
		t.Fatalf("Result.Tests = %+v, want exactly one test\nstdout: %s", out.Result.Tests, out.Stdout)
	}
	test := out.Result.Tests[0]
	agg := test.Aggregate
	if agg.Verdict != string(domain.VerdictFail) {
		t.Errorf("aggregate verdict = %q, want FAIL", agg.Verdict)
	}
	if !agg.InfrastructureFailure {
		t.Errorf("aggregate infrastructure_failure = false, want true after one invalid run")
	}
	if agg.Counted != 0 || agg.Excluded != 1 {
		t.Errorf("counted/excluded = %d/%d, want 0/1; the run must stay out of the pass rate", agg.Counted, agg.Excluded)
	}
	if got := out.Result.Counts[string(domain.VerdictPass)]; got != 0 {
		t.Errorf("Counts[PASS] = %d, want 0", got)
	}
	if len(test.Runs) != 1 {
		t.Errorf("report holds %d runs, want exactly 1: an invalid run is not retried", len(test.Runs))
	}
	if len(agg.Exclusions) != 1 {
		t.Fatalf("exclusions = %+v, want exactly one\nstdout: %s", agg.Exclusions, out.Stdout)
	}
	if agg.Exclusions[0].Reason != wantReason {
		t.Errorf("exclusion reason = %q, want %q", agg.Exclusions[0].Reason, wantReason)
	}
	return agg.Exclusions[0]
}

func TestInvalidModel_ObservedModelDiffersFromSelection_RunIsInvalidAndNotRetried(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{"some-other-model"}}}

	out := RunScenario(t, invalidModelScenario(evidence))

	exclusion := assertInvalidModelRun(t, out, string(domain.ExclusionModelMismatch))
	for _, want := range []string{scenarioSubjectModel, "some-other-model"} {
		if !strings.Contains(exclusion.Detail, want) {
			t.Errorf("exclusion detail %q does not contain %q; it must show the selected and the observed model", exclusion.Detail, want)
		}
	}
	if n := evidence.queries.Load(); n != 1 {
		t.Errorf("evidence queried %d times, want 1 (a single attempt)", n)
	}
}

func TestInvalidModel_OneOfSeveralObservedModelsDiffers_RunIsInvalid(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{scenarioSubjectModel, "intruder-model"}}}

	out := RunScenario(t, invalidModelScenario(evidence))

	exclusion := assertInvalidModelRun(t, out, string(domain.ExclusionModelMismatch))
	if !strings.Contains(exclusion.Detail, "intruder-model") {
		t.Errorf("exclusion detail %q does not name the differing model", exclusion.Detail)
	}
}

func TestInvalidModel_AliasOfSelectedModel_IsStillAMismatch(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{strings.ToUpper(scenarioSubjectModel)}}}

	out := RunScenario(t, invalidModelScenario(evidence))

	assertInvalidModelRun(t, out, string(domain.ExclusionModelMismatch))
}

func TestInvalidModel_EvidenceWithoutSubjectModel_RunIsInvalidAndNotRetried(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{
		NoEvidence: domain.NoEvidenceNoSubjectUsage,
		Detail:     "the logs hold no request for the subject session",
	}}

	out := RunScenario(t, invalidModelScenario(evidence))

	exclusion := assertInvalidModelRun(t, out, string(domain.ExclusionModelEvidenceMissing))
	if !strings.Contains(exclusion.Detail, scenarioSubjectModel) {
		t.Errorf("exclusion detail %q does not name the selected model", exclusion.Detail)
	}
	if n := evidence.queries.Load(); n != 1 {
		t.Errorf("evidence queried %d times, want 1 (a single attempt)", n)
	}
}

func TestInvalidModel_EmptyEvidence_RunIsInvalid(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{}}

	out := RunScenario(t, invalidModelScenario(evidence))

	assertInvalidModelRun(t, out, string(domain.ExclusionModelEvidenceMissing))
}

func TestInvalidModel_EvidencePortError_RunIsInvalid(t *testing.T) {
	evidence := &countingEvidence{err: context.DeadlineExceeded}

	out := RunScenario(t, invalidModelScenario(evidence))

	assertInvalidModelRun(t, out, string(domain.ExclusionModelEvidenceMissing))
}

func TestInvalidModel_ObservedModelEqualsSelection_RunIsReportedAsNormalResult(t *testing.T) {
	evidence := &countingEvidence{answer: domain.ObservedModelEvidence{Models: []string{scenarioSubjectModel}}}

	out := RunScenario(t, invalidModelScenario(evidence))

	if out.ExitCode != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", out.ExitCode, cli.ExitSuccess, out.Stdout, out.Stderr)
	}
	agg := out.Result.Tests[0].Aggregate
	if agg.Verdict != string(domain.VerdictPass) || agg.Excluded != 0 || agg.InfrastructureFailure {
		t.Errorf("aggregate = %+v, want a normal PASS with no exclusions", agg)
	}
	if n := evidence.queries.Load(); n != 1 {
		t.Errorf("evidence queried %d times, want 1", n)
	}
}
