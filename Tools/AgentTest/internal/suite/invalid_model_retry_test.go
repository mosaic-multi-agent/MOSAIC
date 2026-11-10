package suite_test

// Tests that a run whose observed subject model is not the selected one (or
// cannot be determined) gets exactly one attempt, is excluded from the pass
// rate and marks the test as an infrastructure failure, while other exclusion
// reasons keep their single retry.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/suite"
)

func runSingleTest(t *testing.T, outcomes ...scriptedOutcome) (calls int, agg domain.AggregateResult) {
	t.Helper()
	runner := newScriptedRunner()
	runner.scriptFor("test-a", outcomes...)
	s := suite.New(suite.Options{
		Runner:            runner,
		Clock:             newFakeClock(),
		MaxConcurrentRuns: 1,
		RunID:             deterministicRunID,
	})

	result, err := runSuite(t, s, context.Background(), buildPlan(resolvedTest("test-a", 1, 1.0)))
	if err != nil {
		t.Fatalf("Suite.Run returned an error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Fatalf("got %d test reports, want 1", len(result.Tests))
	}
	return len(runner.callsFor("test-a")), result.Tests[0].Aggregate
}

func TestSuiteRun_InvalidModelRun_IsNotRetried(t *testing.T) {
	cases := []struct {
		name     string
		evidence domain.RunEvidence
		reason   domain.ExclusionReason
		detail   []string
	}{
		{"mismatch", withObservedModel(passingEvidence(), "model-b"),
			domain.ExclusionModelMismatch, []string{"model-a", "model-b"}},
		{"no evidence", withoutModelEvidence(passingEvidence()),
			domain.ExclusionModelEvidenceMissing, []string{"model-a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls, agg := runSingleTest(t,
				scriptedOutcome{evidence: tc.evidence},
				// A retry would pass cleanly; it must never be consumed.
				scriptedOutcome{evidence: passingEvidence()},
			)

			if calls != 1 {
				t.Errorf("runner was called %d times, want exactly 1; an invalid-model run is reported immediately, not retried", calls)
			}
			if agg.Excluded != 1 || agg.Counted != 0 {
				t.Errorf("Excluded = %d, Counted = %d, want 1 and 0", agg.Excluded, agg.Counted)
			}
			if !agg.InfrastructureFailure || agg.Verdict != domain.VerdictFail {
				t.Errorf("InfrastructureFailure = %v, Verdict = %q, want true and FAIL after a single invalid run", agg.InfrastructureFailure, agg.Verdict)
			}
			if len(agg.Exclusions) != 1 || agg.Exclusions[0].Reason != tc.reason {
				t.Fatalf("Exclusions = %+v, want one with reason %q", agg.Exclusions, tc.reason)
			}
			for _, want := range tc.detail {
				if !strings.Contains(agg.Exclusions[0].Detail, want) {
					t.Errorf("exclusion detail %q does not contain %q", agg.Exclusions[0].Detail, want)
				}
			}
		})
	}
}

func TestSuiteRun_InvalidModelRun_AfterValidRepetition_StaysOutOfPassRate(t *testing.T) {
	runner := newScriptedRunner()
	runner.scriptFor("test-a",
		// The scripted runner reserves a window of two outcomes per repetition
		// (the original attempt and its possible retry), so each repetition's
		// retry slot is filled explicitly to keep the repetitions aligned.
		scriptedOutcome{evidence: passingEvidence()}, // repetition 1
		scriptedOutcome{evidence: passingEvidence()}, // repetition 1 retry slot, never consumed
		scriptedOutcome{evidence: withObservedModel(passingEvidence(), "model-b")}, // repetition 2
		scriptedOutcome{evidence: passingEvidence()},                               // repetition 2 retry slot, must not be consumed
	)
	s := suite.New(suite.Options{
		Runner:            runner,
		Clock:             newFakeClock(),
		MaxConcurrentRuns: 1,
		RunID:             deterministicRunID,
	})

	result, err := runSuite(t, s, context.Background(), buildPlan(resolvedTest("test-a", 2, 0.5)))
	if err != nil {
		t.Fatalf("Suite.Run returned an error: %v", err)
	}

	if n := len(runner.callsFor("test-a")); n != 2 {
		t.Errorf("runner was called %d times, want 2 (one attempt per repetition)", n)
	}
	agg := result.Tests[0].Aggregate
	if agg.Counted != 1 || agg.Passed != 1 || agg.Excluded != 1 {
		t.Errorf("Counted/Passed/Excluded = %d/%d/%d, want 1/1/1", agg.Counted, agg.Passed, agg.Excluded)
	}
	if !agg.InfrastructureFailure || agg.Verdict != domain.VerdictFail {
		t.Errorf("InfrastructureFailure = %v, Verdict = %q, want true and FAIL; the pass rate alone must not hide an invalid run", agg.InfrastructureFailure, agg.Verdict)
	}
}

func TestSuiteRun_StateIntegrityRunWithoutModelEvidence_IsStillRetried(t *testing.T) {
	calls, agg := runSingleTest(t,
		scriptedOutcome{evidence: withoutModelEvidence(stateIntegrityEvidence())},
		scriptedOutcome{evidence: passingEvidence()},
	)

	if calls != 2 {
		t.Errorf("runner was called %d times, want 2; the state-integrity retry keeps its own reason", calls)
	}
	if agg.Excluded != 1 || agg.Counted != 1 || agg.Verdict != domain.VerdictPass {
		t.Errorf("Excluded/Counted/Verdict = %d/%d/%q, want 1/1/PASS", agg.Excluded, agg.Counted, agg.Verdict)
	}
	if len(agg.Exclusions) == 1 && agg.Exclusions[0].Reason != domain.ExclusionStateIntegrity {
		t.Errorf("exclusion reason = %q, want %q", agg.Exclusions[0].Reason, domain.ExclusionStateIntegrity)
	}
}

func TestSuiteRun_SpawnFailedRunWithoutModelEvidence_IsStillRetried(t *testing.T) {
	calls, agg := runSingleTest(t,
		scriptedOutcome{evidence: withoutModelEvidence(spawnFailedEvidence())},
		scriptedOutcome{evidence: passingEvidence()},
	)

	if calls != 2 {
		t.Errorf("runner was called %d times, want 2; a harness that never started is retried once", calls)
	}
	if agg.Verdict != domain.VerdictPass || agg.Counted != 1 {
		t.Errorf("Verdict/Counted = %q/%d, want PASS/1 from the retry", agg.Verdict, agg.Counted)
	}
	if len(agg.Exclusions) == 1 && agg.Exclusions[0].Reason != domain.ExclusionSpawnFailed {
		t.Errorf("exclusion reason = %q, want %q", agg.Exclusions[0].Reason, domain.ExclusionSpawnFailed)
	}
}

func TestSuiteRun_SpawnFailedRunWithWrongModel_IsNotRetried(t *testing.T) {
	calls, agg := runSingleTest(t,
		scriptedOutcome{evidence: withObservedModel(spawnFailedEvidence(), "model-b")},
		scriptedOutcome{evidence: passingEvidence()},
	)

	if calls != 1 {
		t.Errorf("runner was called %d times, want 1; a run that used a wrong model is never retried", calls)
	}
	if len(agg.Exclusions) != 1 || agg.Exclusions[0].Reason != domain.ExclusionModelMismatch {
		t.Errorf("Exclusions = %+v, want one with reason %q", agg.Exclusions, domain.ExclusionModelMismatch)
	}
}

func TestSuiteRun_ValidModelRun_IsEvaluatedNormally(t *testing.T) {
	calls, agg := runSingleTest(t, scriptedOutcome{evidence: passingEvidence()})

	if calls != 1 || agg.Verdict != domain.VerdictPass || agg.Counted != 1 || agg.InfrastructureFailure {
		t.Errorf("calls/Verdict/Counted/InfrastructureFailure = %d/%q/%d/%v, want 1/PASS/1/false", calls, agg.Verdict, agg.Counted, agg.InfrastructureFailure)
	}
	if len(agg.Runs) != 1 || agg.Runs[0].ModelValidity != domain.ModelVerified {
		t.Errorf("Runs = %+v, want one run with ModelValidity %q", agg.Runs, domain.ModelVerified)
	}
}

func TestSuiteRun_RunnerError_LeavesModelValidityEmptyAndIsAttemptedOnce(t *testing.T) {
	runner := newScriptedRunner()
	runner.scriptFor("test-a",
		scriptedOutcome{err: errors.New("provisioning failed")},
		scriptedOutcome{evidence: passingEvidence()},
	)
	s := newSuite(runner, newFakeClock(), &recordingSink{})

	result, err := runSuite(t, s, context.Background(), buildPlan(resolvedTest("test-a", 1, 1.0)))

	if err != nil {
		t.Fatalf("Suite.Run returned an error: %v", err)
	}
	if got := len(runner.callsFor("test-a")); got != 1 {
		t.Errorf("runner was called %d times, want 1", got)
	}
	runs := result.Tests[0].Aggregate.Runs
	if len(runs) != 1 || runs[0].ModelValidity != "" {
		t.Errorf("Runs = %+v, want one run with empty ModelValidity (no model was assessed)", runs)
	}
}
