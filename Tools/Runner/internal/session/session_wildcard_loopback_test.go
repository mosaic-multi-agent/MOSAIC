package session_test

// Tests of wildcard handling in the artifacts a dispatched request lists: an
// Input wildcard that also matches a concrete Input lists the path once, and a
// wildcard Output of the reviewer is injected on the loop-back to the creator
// as the matching run-folder files (no match lists nothing).

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

func newWildcardLoopbackRig(t *testing.T) *blockRig {
	t.Helper()
	rig := newBlockRig(t, "wildcard-loopback-orch.md", nil, "agent-a", "agent-b")
	rig.writeRunFiles(t, "notes/n1.md", "notes/n2.md")
	return rig
}

// scriptReviewerFindingsOnce makes the reviewer report findings once (writing
// writes), then approve; the creator succeeds twice.
func scriptReviewerFindingsOnce(rig *blockRig, writes ...harness.ScriptedWrite) {
	rig.adapter.Queue("agent-b",
		harness.ScriptedEntry{
			Response: &domain.ProtocolResponse{
				StatusCode: domain.StatusCOMPLETED_NEEDS_ACTION, StatusMessage: "findings",
			},
			Writes: writes,
		},
		statusEntry(domain.StatusSUCCESS, "", "approved"))
	rig.queueOKTimes("agent-a", 2)
}

func TestSession_Start_InputWildcardMatchingAConcreteInput_ListsThePathOnce(t *testing.T) {
	rig := newWildcardLoopbackRig(t)
	scriptReviewerFindingsOnce(rig)

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-loopback", domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	first := rig.dispatchedInputs("agent-a")[0]
	requireSequence(t, "first creator dispatch inputs", first, wildcardRunPaths("notes/n1.md", "notes/n2.md"))
}

func TestSession_LoopBack_ReviewerWildcardOutput_IsInjectedAsTheMatchingFiles(t *testing.T) {
	rig := newWildcardLoopbackRig(t)
	scriptReviewerFindingsOnce(rig,
		runFileWrite("reviews/r2.md", "# r2\n"), runFileWrite("reviews/r1.md", "# r1\n"))

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-loopback", domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	inputs := rig.dispatchedInputs("agent-a")
	if len(inputs) != 2 {
		t.Fatalf("want 2 creator dispatches, got %d", len(inputs))
	}
	requireSequence(t, "reviews listed on the loop-back", filterContaining(inputs[1], "/reviews/"),
		wildcardRunPaths("reviews/r1.md", "reviews/r2.md"))
	requireNoWildcardEntry(t, inputs[1])
}

func TestSession_LoopBack_ReviewerWildcardOutputWithoutMatch_ListsNothing(t *testing.T) {
	rig := newWildcardLoopbackRig(t)
	scriptReviewerFindingsOnce(rig)

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-loopback", domain.ExecutionModeAutoReview))

	requireRunStatus(t, got, err, domain.RunCompleted)
	inputs := rig.dispatchedInputs("agent-a")
	if len(inputs) != 2 {
		t.Fatalf("want 2 creator dispatches, got %d", len(inputs))
	}
	requireSequence(t, "reviews listed on the loop-back", filterContaining(inputs[1], "reviews"), nil)
	requireNoWildcardEntry(t, inputs[1])
}

// filterContaining returns the entries of paths that contain sub, in order.
func filterContaining(paths []string, sub string) []string {
	var out []string
	for _, p := range paths {
		if strings.Contains(p, sub) {
			out = append(out, p)
		}
	}
	return out
}

// requireNoWildcardEntry fails when a dispatched input still holds a '*'.
func requireNoWildcardEntry(t *testing.T, paths []string) {
	t.Helper()
	for _, p := range paths {
		if strings.Contains(p, "*") {
			t.Errorf("dispatched inputs hold an unexpanded wildcard entry %q: %v", p, paths)
		}
	}
}
