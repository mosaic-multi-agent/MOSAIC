package session_test

// Tests that a loop-back dispatch to a creator lists the artifacts the creator
// needs to revise its work, whether or not the run was interrupted: after a
// resume, the re-dispatch of a reviewer's row (auto-review route-back on
// findings, mechanical re-dispatch of a PARTIALLY_DONE step) lists the reviewer
// row's output artifacts, exactly as the live dispatch does.
//
// Coverage:
//
//   - A resumed route-back lists the reviewer row's Output, for a run-scope row
//     pair and for a row pair inside a block (Output resolved for the recorded
//     plan iteration).
//   - A resumed PARTIALLY_DONE re-dispatch lists the row's previous outputs.
//   - The live and the resumed dispatch list the same inputs.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

func TestSession_Resume_RunScopeReviewerFindings_CreatorListsReviewerOutput(t *testing.T) {
	f := resumeHITLReview(t, hitlAfterReviewer(domain.StatusCOMPLETED_NEEDS_ACTION), nil, scriptAllSucceed)

	requireInvocationOrder(t, f, "agent-a", "agent-b", "agent-c")
	requireInputs(t, requireNthRequest(t, f, "agent-a", 0), "review.md")
}

func TestSession_Resume_BlockReviewerFindings_CreatorListsReviewerOutputOfRecordedIteration(t *testing.T) {
	seed := blockSeed("build-review", 4, "2", domain.StatusCOMPLETED_NEEDS_ACTION, nil)

	f := resumeBlockRun(t, seed, scriptBlockSucceed)

	requireInputs(t, requireNthRequest(t, f, "implementer", 0), "Stage-2/tests.md", "Stage-2/build-review.md")
}

func TestSession_Resume_RunScopeReviewerPartiallyDone_RedispatchListsItsPreviousOutput(t *testing.T) {
	f := resumeHITLReview(t, hitlAfterReviewer(domain.StatusPARTIALLY_DONE), nil, scriptAllSucceed)

	first := requireNthRequest(t, f, "agent-b", 0)
	if first.AgentInstanceID != "agent-b#3" {
		t.Fatalf("want the re-dispatch of the reviewer as agent-b#3, got %q", first.AgentInstanceID)
	}
	requireInputs(t, first, "design.md", "review.md")
}

func TestSession_Resume_BlockReviewerPartiallyDone_RedispatchListsItsPreviousOutput(t *testing.T) {
	seed := blockSeed("build-review", 4, "2", domain.StatusPARTIALLY_DONE, nil)

	f := resumeBlockRun(t, seed, scriptBlockSucceed)

	requireInputs(t, requireNthRequest(t, f, "build-review", 0), "Stage-2/impl.md", "Stage-2/build-review.md")
}

// loopBackCase runs one loop-back dispatch twice: live (the reviewer answers in
// this process) and resumed (the run was stopped after the reviewer's answer
// and is resumed), and names the request to compare.
type loopBackCase struct {
	name   string
	agent  string // agent whose dispatch is compared
	live   func(t *testing.T) *harness.MockAdapter
	resume func(t *testing.T) *harness.MockAdapter
	// liveIndex is the position of the compared dispatch among the agent's
	// dispatches in the live run; the resumed run's is always the first.
	liveIndex int
}

func loopBackCases() []loopBackCase {
	return []loopBackCase{
		{
			name: "run-scope route-back on findings", agent: "agent-a", liveIndex: 0,
			live: func(t *testing.T) *harness.MockAdapter {
				return resumeHITLReview(t, hitlAfterCreator(), nil, scriptReviewerFindingsThenApproval)
			},
			resume: func(t *testing.T) *harness.MockAdapter {
				return resumeHITLReview(t, hitlAfterReviewer(domain.StatusCOMPLETED_NEEDS_ACTION), nil, scriptAllSucceed)
			},
		},
		{
			name: "run-scope partially done re-dispatch", agent: "agent-b", liveIndex: 1,
			live: func(t *testing.T) *harness.MockAdapter {
				return resumeHITLReview(t, hitlAfterCreator(), nil, scriptReviewerPartialThenSuccess)
			},
			resume: func(t *testing.T) *harness.MockAdapter {
				return resumeHITLReview(t, hitlAfterReviewer(domain.StatusPARTIALLY_DONE), nil, scriptAllSucceed)
			},
		},
		{
			name: "block route-back on findings", agent: "implementer", liveIndex: 0,
			live: func(t *testing.T) *harness.MockAdapter {
				return resumeBlockRun(t, blockSeed("implementer", 3, "1", domain.StatusSUCCESS, nil), scriptBuildReviewFindings)
			},
			resume: func(t *testing.T) *harness.MockAdapter {
				return resumeBlockRun(t, blockSeed("build-review", 4, "1", domain.StatusCOMPLETED_NEEDS_ACTION, nil), scriptBlockSucceed)
			},
		},
		{
			name: "block partially done re-dispatch", agent: "build-review", liveIndex: 1,
			live: func(t *testing.T) *harness.MockAdapter {
				return resumeBlockRun(t, blockSeed("implementer", 3, "1", domain.StatusSUCCESS, nil), scriptBuildReviewPartial)
			},
			resume: func(t *testing.T) *harness.MockAdapter {
				return resumeBlockRun(t, blockSeed("build-review", 4, "1", domain.StatusPARTIALLY_DONE, nil), scriptBlockSucceed)
			},
		},
	}
}

func TestSession_LoopBack_LiveAndResumedDispatchListTheSameInputs(t *testing.T) {
	for _, tc := range loopBackCases() {
		t.Run(tc.name, func(t *testing.T) {
			live := requireNthRequest(t, tc.live(t), tc.agent, tc.liveIndex)
			resumed := requireNthRequest(t, tc.resume(t), tc.agent, 0)

			got, want := sortedInputs(resumed), sortedInputs(live)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("resumed inputs %v differ from live inputs %v", got, want)
			}
			if len(want) == 0 {
				t.Errorf("want the loop-back to list inputs, live listed none")
			}
		})
	}
}
