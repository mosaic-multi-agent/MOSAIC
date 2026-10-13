package session_test

// Helpers of the Input wildcard session tests: run-scoped path lists and the
// readers of what a dispatch received.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
)

// wildcardRunPaths returns the run-scoped form of rels, as the dispatched
// request carries them.
func wildcardRunPaths(rels ...string) []string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, dispatchPath(r))
	}
	return out
}

// dispatchedInputs returns, in dispatch order, the Input artifacts of every
// dispatch of agent.
func (r *blockRig) dispatchedInputs(agent string) [][]string {
	var out [][]string
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent {
			out = append(out, inv.Request.InputArtifacts)
		}
	}
	return out
}

// dispatchedOutputs returns, in dispatch order, the Output artifacts of every
// dispatch of agent.
func (r *blockRig) dispatchedOutputs(agent string) [][]string {
	var out [][]string
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent {
			out = append(out, inv.Request.OutputArtifacts)
		}
	}
	return out
}

// requireInputsOfEveryDispatch asserts that agent was dispatched exactly
// times times and that every dispatch received want, in order.
func (r *blockRig) requireInputsOfEveryDispatch(t *testing.T, agent string, times int, want []string) {
	t.Helper()
	got := r.dispatchedInputs(agent)
	if len(got) != times {
		t.Fatalf("want %d dispatches of %s, got %d", times, agent, len(got))
	}
	for i, inputs := range got {
		requireSequence(t, fmt.Sprintf("%s dispatch %d inputs", agent, i+1), inputs, want)
	}
}

// inputsCellOf returns the Inputs cell recorded for the first workflow step of
// agent.
func (r *blockRig) inputsCellOf(t *testing.T, agent string) string {
	t.Helper()
	for _, s := range workflowSteps(r.store) {
		if strings.HasPrefix(s.AgentInstance, agent+"#") {
			return s.Inputs
		}
	}
	t.Fatalf("no workflow step recorded for %s", agent)
	return ""
}

// wildcardResearchFiles are the run-folder files the Input wildcard tests
// start from: two matches of *-Research.md, a name that does not match, and a
// match one level down that '*' must not reach.
var wildcardResearchFiles = []string{
	"beta-Research.md", "alpha-Research.md", "Research.md", "sub/gamma-Research.md",
}

// wildcardStageFiles are run-folder files of Stage-*/Plan.md: two matches
// (created in non-lexical order), a different file name of a matching folder,
// and a folder that does not match.
var wildcardStageFiles = []string{
	"Stage-b-2/Plan.md", "Stage-A/Plan.md", "Stage-A/Other.md", "Other/Plan.md",
}

// writeRunFiles creates every rel in rels (content is irrelevant) in the run
// folder before the run.
func (r *blockRig) writeRunFiles(t *testing.T, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		r.writeRunFile(t, rel, "# "+rel+"\n")
	}
}

// rejectFirstReadOf is an approval reader that answers ApprovalFalse to the
// first read of a path ending in suffix and ApprovalTrue to every other read.
type rejectFirstReadOf struct {
	mu     sync.Mutex
	suffix string
	done   bool
}

func (r *rejectFirstReadOf) ReadApproval(_ context.Context, path string) domain.HumanApproval {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done && strings.HasSuffix(strings.ReplaceAll(path, `\`, "/"), r.suffix) {
		r.done = true
		return domain.ApprovalFalse
	}
	return domain.ApprovalTrue
}

// requireConsultedDispatchRan asserts that the dispatch the consultant
// instructed for agent actually ran, and states what the consultant was told
// otherwise, so a refused dispatch cannot hide behind a passing run status.
func (r *blockRig) requireConsultedDispatchRan(t *testing.T, agent string) {
	t.Helper()
	if len(r.dispatchedInputs(agent)) > 0 {
		return
	}
	var told []string
	for i, req := range r.consult.Requests {
		msg, reason := "<nil>", "<nil>"
		if req.LastStatusMessage != nil {
			msg = *req.LastStatusMessage
		}
		if req.LastErrorReason != nil {
			reason = *req.LastErrorReason
		}
		told = append(told, fmt.Sprintf("request %d: status message %q, error reason %q", i+1, msg, reason))
	}
	t.Fatalf("the consulted dispatch of %s did not run (refused or dropped); consultant was told: %s",
		agent, strings.Join(told, "; "))
}
