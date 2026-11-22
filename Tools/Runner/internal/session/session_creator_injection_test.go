package session_test

// Tests that a reviewer -> creator loop-back lists the creator's own previously
// produced output artifacts, recorded in the Artifacts registry, in addition to
// the reviewer's output: for the run-scope pair and the in-block pair, with the
// reviewer answering in this process (live) and after a resume.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_LoopBack_RunScopeCreator_ListsItsPreviousOutputLive(t *testing.T) {
	f := resumeHITLReview(t, hitlAfterCreator(), hitlCreatorRegistry(), scriptReviewerFindingsThenApproval)

	requireInputs(t, requireNthRequest(t, f, "agent-a", 0), "design.md", "review.md")
}

func TestSession_LoopBack_RunScopeCreator_ListsItsPreviousOutputAfterResume(t *testing.T) {
	pos := hitlAfterReviewer(domain.StatusCOMPLETED_NEEDS_ACTION)

	f := resumeHITLReview(t, pos, hitlCreatorRegistry(), scriptAllSucceed)

	requireInputs(t, requireNthRequest(t, f, "agent-a", 0), "design.md", "review.md")
}

func TestSession_LoopBack_BlockCreator_ListsItsPreviousOutputLive(t *testing.T) {
	seed := blockSeed("implementer", 3, "1", domain.StatusSUCCESS, blockCreatorRegistry("1"))

	f := resumeBlockRun(t, seed, scriptBuildReviewFindings)

	requireInputs(t, requireNthRequest(t, f, "implementer", 0),
		"Stage-1/tests.md", "Stage-1/build-review.md", "Stage-1/impl.md")
}

func TestSession_LoopBack_BlockCreator_ListsItsPreviousOutputAfterResume(t *testing.T) {
	seed := blockSeed("build-review", 4, "2", domain.StatusCOMPLETED_NEEDS_ACTION, blockCreatorRegistry("2"))

	f := resumeBlockRun(t, seed, scriptBlockSucceed)

	requireInputs(t, requireNthRequest(t, f, "implementer", 0),
		"Stage-2/tests.md", "Stage-2/build-review.md", "Stage-2/impl.md")
}

// An artifact the creator row does not declare as an output is not injected,
// even when the registry records another agent's output.
func TestSession_LoopBack_RegistryEntryOfAnotherRow_IsNotInjected(t *testing.T) {
	registry := append(hitlCreatorRegistry(), domain.ArtifactRegistryEntry{
		Artifact:  domain.RunScopedFolder(testRunID) + "/other.md",
		CreatedIn: "PLANNING",
		CreatedBy: "agent-c#9",
	})

	f := resumeHITLReview(t, hitlAfterCreator(), registry, scriptReviewerFindingsThenApproval)

	requireInputs(t, requireNthRequest(t, f, "agent-a", 0), "design.md", "review.md")
}
