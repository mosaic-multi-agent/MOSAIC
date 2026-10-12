package session_test

// Helpers of the infrastructure trigger dispatch content tests: rigs over the
// infra-contents fixtures and readers of the requests and rows an
// infrastructure dispatch produced.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

const infraCommitAgent = "commit-manager-git"

// stagePlanDoc is a Plan.md schedule of one TDD-approach Stage iteration.
func stagePlanDoc(id string) string {
	return "# Plan\n\n## Stages\n\n| Stage | Name | Goal | HITL | Approach |\n|---|---|---|:---:|---|\n" +
		"| " + id + " | Item " + id + " | Goal | FALSE | TDD |\n"
}

// newStageGroupedRig is the rig of the grouped Stage block fixture with the
// iteration id scheduled. The shared ContractsDesign.md exists in the run
// folder before the run.
func newStageGroupedRig(t *testing.T, id string) *blockRig {
	t.Helper()
	rig := newBlockRig(t, "infra-contents-stage-grouped-orch.md", nil,
		"test-writer", "tests-review", "implementer", "impl-review", infraCommitAgent)
	rig.writeRunFile(t, "Plan.md", stagePlanDoc(id))
	rig.writeRunFile(t, "ContractsDesign.md", "# Contracts\n")
	return rig
}

// queueStageRows queues the four row agents of the grouped Stage fixture; the
// agents named in writes write their files while they run.
func queueStageRows(rig *blockRig, writes map[string]string) {
	for _, agent := range []string{"test-writer", "tests-review", "implementer", "impl-review"} {
		if rel, ok := writes[agent]; ok {
			rig.queueOK(agent, runFileWrite(rel, "# "+rel+"\n"))
			continue
		}
		rig.queueOK(agent)
	}
}

// newProductRig is the rig of a Product block fixture with the iteration ids
// scheduled by the planner; every agent definition exists, including the
// infrastructure agent infraAgent.
func newProductRig(t *testing.T, fixture, infraAgent string, iterations ...string) *blockRig {
	t.Helper()
	rig := newBlockRig(t, fixture, nil, "planner", "builder", "checker", infraAgent)
	rig.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids(iterations...))))
	return rig
}

// queueProductIteration queues the builder and checker of one Product
// iteration, each writing its own fill-in file.
func queueProductIteration(rig *blockRig, id string) {
	rig.queueOK("builder", runFileWrite("built-"+id+".md", "# built\n"))
	rig.queueOK("checker", runFileWrite("checked-"+id+".md", "# checked\n"))
}

// requestsOf returns, in dispatch order, the request of every dispatch of agent.
func (r *blockRig) requestsOf(agent string) []domain.ProtocolRequest {
	var out []domain.ProtocolRequest
	for _, inv := range r.adapter.Invocations() {
		if inv.Agent.Identifier == agent {
			out = append(out, inv.Request)
		}
	}
	return out
}

// onlyRequestOf returns the single request of agent and fails the test when
// the agent was dispatched any other number of times.
func (r *blockRig) onlyRequestOf(t *testing.T, agent string) domain.ProtocolRequest {
	t.Helper()
	reqs := r.requestsOf(agent)
	if len(reqs) != 1 {
		t.Fatalf("want exactly 1 dispatch of %s, got %d", agent, len(reqs))
	}
	return reqs[0]
}

// infraRowsOf returns, in order, the applied infrastructure rows of agent that
// record a dispatch (HITL rejection rows are not part of them).
func infraRowsOf(store *memStore, agent string) []domain.CompletedStep {
	var out []domain.CompletedStep
	for _, s := range store.Applied {
		if s.IsInfrastructure && !s.HITLRejected && strings.HasPrefix(s.AgentInstance, agent+"#") {
			out = append(out, s)
		}
	}
	return out
}

// requireTriggerDispatchShape asserts what every trigger dispatch carries
// whatever its class: no output artifacts, no human in the loop.
func requireTriggerDispatchShape(t *testing.T, req domain.ProtocolRequest) {
	t.Helper()
	if len(req.OutputArtifacts) != 0 {
		t.Errorf("want no output_artifacts on a trigger dispatch, got %q", req.OutputArtifacts)
	}
	if req.HumanInTheLoop {
		t.Error("want human_in_the_loop false on a trigger dispatch, got true")
	}
}

// requireDescriptionNames asserts the task description contains every token.
func requireDescriptionNames(t *testing.T, req domain.ProtocolRequest, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		if !strings.Contains(req.TaskDescription, tok) {
			t.Errorf("want task description to name %q, got %q", tok, req.TaskDescription)
		}
	}
}
