package session_test

// What a review-class trigger dispatch carries, whatever trigger fired it: the
// run-scoped Orchestration.md as its only input artifact, no outputs, no human
// in the loop, and a task description naming the matched trigger. The run
// folder of these rigs holds no Orchestration.md, so the input is sent without
// inspecting the folder.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

const infraReviewAgent = "orchestration-review"

// runProductWithReview runs the fixture with one Product iteration A and one
// scripted dispatch of the review agent. Checkpoints stay disabled: a
// review-only fixture declares no checkpoint-class agent, and review triggers
// are not gated by them. The successful review triggers a routing
// consultation, answered with a stop so the run ends right after the dispatch.
func runProductWithReview(t *testing.T, fixture string) *blockRig {
	t.Helper()
	rig := newProductRig(t, fixture, infraReviewAgent, "A")
	queueProductIteration(rig, "A")
	rig.queueOK(infraReviewAgent)
	rig.consult.queueStop("review done")
	cfg := rig.config("product-block", domain.ExecutionModeAuto)

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	return rig
}

// requireReviewDispatchCarriesOrchestration asserts the single review dispatch
// of the rig: one run-scoped Orchestration.md input, the trigger-dispatch
// shape, the matched trigger named, and the same input in the Inputs cell.
func requireReviewDispatchCarriesOrchestration(t *testing.T, rig *blockRig, trigger string) {
	t.Helper()
	want := []string{dispatchPath("Orchestration.md")}

	req := rig.onlyRequestOf(t, infraReviewAgent)

	requireSequence(t, "review input_artifacts", req.InputArtifacts, want)
	requireTriggerDispatchShape(t, req)
	requireDescriptionNames(t, req, trigger)
	rows := infraRowsOf(rig.store, infraReviewAgent)
	if len(rows) != 1 || rows[0].Inputs != want[0] {
		t.Errorf("want one review row with Inputs %q, got %+v", want[0], rows)
	}
}

func TestSession_Start_ReviewOnIterationEnd_CarriesOrchestrationAsOnlyInput(t *testing.T) {
	rig := runProductWithReview(t, "infra-contents-review-orch.md")

	requireReviewDispatchCarriesOrchestration(t, rig, "ITERATION_END")
}

func TestSession_Start_ReviewOnInvocationInterval_CarriesOrchestrationAsOnlyInput(t *testing.T) {
	rig := runProductWithReview(t, "infra-contents-review-interval-orch.md")

	requireReviewDispatchCarriesOrchestration(t, rig, "INVOCATION_INTERVAL")
}
