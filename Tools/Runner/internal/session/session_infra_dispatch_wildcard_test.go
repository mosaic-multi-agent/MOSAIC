package session_test

// How a fill-in wildcard entry resolves in a commit-class ITERATION_END
// dispatch: de-duplicated against concrete entries that reach the same file,
// kept at the entry's table position, and contributing nothing without a match.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_Start_CommitOnIterationEnd_WildcardOverlappingConcreteEntry_SentOnceAtFirstPosition(t *testing.T) {
	rig := newBlockRig(t, "infra-contents-wildcard-overlap-orch.md", nil,
		"planner", "researcher", "synthesizer", infraCommitAgent)
	rig.queueCommitSetup()
	rig.queueOK("planner", runFileWrite("TopicPlan.md", scheduleDoc("Topic", "Topics", ids("Pricing", "Support"))))
	rig.writeRunFiles(t, "A-Pricing-Findings.md", "B-Pricing-Findings.md")
	rig.queueOK("researcher", runFileWrite("notes-Pricing.md", "# notes\n"))
	rig.queueOK("synthesizer", runFileWrite("summary-Pricing.md", "# summary\n"))
	rig.queueOK(infraCommitAgent)
	rig.queueOK("researcher", runFileWrite("notes-Support.md", "# notes\n"))
	rig.queueOK("synthesizer", runFileWrite("summary-Support.md", "# summary\n"))
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("wildcard-overlap", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	reqs := rig.requestsOf(infraCommitAgent)
	if len(reqs) != 2 {
		t.Fatalf("want 2 commit dispatches (one per topic), got %d", len(reqs))
	}
	// A-Pricing-Findings.md is reached by the concrete entry and by the
	// wildcard: it appears once, at the concrete entry's position, the
	// wildcard's remaining match follows it, and the summary written by the
	// row after the wildcard entry comes last.
	requireSequence(t, "commit of Pricing", reqs[0].InputArtifacts, wildcardRunPaths(
		"notes-Pricing.md", "A-Pricing-Findings.md", "B-Pricing-Findings.md", "summary-Pricing.md"))
	// No findings file exists for Support: the wildcard contributes nothing and
	// the iteration's concrete files are still sent in table order.
	requireSequence(t, "commit of Support", reqs[1].InputArtifacts, wildcardRunPaths(
		"notes-Support.md", "summary-Support.md"))
}
