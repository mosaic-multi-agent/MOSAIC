package plan_test

// resolve_test.go covers artifact set resolution for error cases, utility agents and
// infrastructure agents. Workflow-driven resolution is in resolve_workflows_test.go.
//
// Existing behavior (T16.1, T16.2):
//
// T16.1 — ResolveArtifacts derives the correct, deduplicated set:
//   - Single workflow resolves its referenced agents and their transitive skills
//   - Multiple overlapping workflows deduplicate agents and skills
//   - An unknown workflow ID returns ErrUnknownWorkflow
//   - A workflow referencing a non-existent agent returns ErrUnknownAgent (not a smaller set)
//
// T16.2 — Orchestrator and utility agents:
//   - The orchestrator is always included, even with no selected workflows
//   - A utility agent is included only when explicitly named in utilityAgentIDs
//   - A utility agent not named in utilityAgentIDs is absent from the result
//
// Infrastructure agent extension (T4.1):
//
//   - An infrastructure agent is included only when its key appears in infrastructureAgentIDs
//   - An infrastructure agent absent from infrastructureAgentIDs is not included
//   - An unknown infrastructure agent key returns ErrUnknownAgent
//   - Multiple infrastructure agents of the same class can all be selected without error
//   - Infrastructure agents are deduplicated with workflow-referenced agents
//   - Passing nil for infrastructureAgentIDs leaves existing behavior unchanged
//   - Selected infrastructure agents' required skills are transitively included

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// ---------------------------------------------------------------------------
// T16.1 — Error cases
// ---------------------------------------------------------------------------

// TestResolveArtifacts_WorkflowReferencingMissingAgent_ReturnsErrUnknownAgent verifies that a
// workflow whose referenced_agents includes a key not found in the catalog returns
// ErrUnknownAgent. The error must be explicit; silent omission would produce a broken
// deployment where expected agents are missing.
func TestResolveArtifacts_WorkflowReferencingMissingAgent_ReturnsErrUnknownAgent(t *testing.T) {
	// Use a fake catalog so we can inject a workflow with a nonexistent agent reference.
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workflows: []domain.Workflow{
			makeWorkflow("broken-wf", "this-agent-does-not-exist"),
		},
		// No workers — the referenced agent is intentionally absent.
	}

	_, err := plan.ResolveArtifacts(cat, []string{"broken-wf"}, nil, nil, nil)
	if err == nil {
		t.Fatal("ResolveArtifacts with workflow referencing missing agent: expected error, got nil")
	}
	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Errorf("ResolveArtifacts with missing agent reference: got error %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
}

// TestResolveArtifacts_WorkflowReferencingMissingAgent_DoesNotReturnSmallerSet verifies that
// the missing-agent error is not silently swallowed, producing an artifact set that omits
// the missing reference. The caller must observe the error.
func TestResolveArtifacts_WorkflowReferencingMissingAgent_DoesNotReturnSmallerSet(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workflows: []domain.Workflow{
			makeWorkflow("broken-wf", "this-agent-does-not-exist"),
		},
	}

	set, err := plan.ResolveArtifacts(cat, []string{"broken-wf"}, nil, nil, nil)
	if err == nil {
		// If no error was returned but the missing agent is absent, that's the silent-drop bug.
		if !containsAgent(set.Agents, "this-agent-does-not-exist") {
			t.Error("ResolveArtifacts silently omitted a missing agent reference without returning an error; " +
				"the error must be surfaced")
		}
		t.Fatal("expected ErrUnknownAgent, got nil error")
	}
}

// ---------------------------------------------------------------------------
// T16.2 — Utility agents included only when explicitly selected
// ---------------------------------------------------------------------------

// TestResolveArtifacts_UtilityAgent_IncludedWhenSelected verifies that a utility agent
// appears in the result when its key is passed in utilityAgentIDs.
// The caller is responsible for allow-list filtering before passing the IDs here.
func TestResolveArtifacts_UtilityAgent_IncludedWhenSelected(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator:  makeOrchestrator(),
		utilityAgents: []domain.Agent{makeUtilityAgent("my-utility")},
	}

	set, err := plan.ResolveArtifacts(cat, nil, []string{"my-utility"}, nil, nil)
	must(t, err)

	if !containsAgent(set.Agents, "my-utility") {
		t.Error("ArtifactSet.Agents missing utility agent that was explicitly selected via utilityAgentIDs")
	}
}

// TestResolveArtifacts_UtilityAgent_NotIncludedWhenNotSelected verifies that a utility agent
// known to the catalog does NOT appear in the result when its key is absent from utilityAgentIDs.
// This is the allow-listing boundary: utility agents are opt-in, not opt-out.
func TestResolveArtifacts_UtilityAgent_NotIncludedWhenNotSelected(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator:  makeOrchestrator(),
		utilityAgents: []domain.Agent{makeUtilityAgent("my-utility")},
	}

	// utilityAgentIDs is nil — no utility agents selected.
	set, err := plan.ResolveArtifacts(cat, nil, nil, nil, nil)
	must(t, err)

	if containsAgent(set.Agents, "my-utility") {
		t.Error("ArtifactSet.Agents contains utility agent that was NOT selected via utilityAgentIDs; " +
			"utility agents must be opt-in")
	}
}

// TestResolveArtifacts_UtilityAgent_IncludesTransitiveSkills verifies that a selected
// utility agent's required skills are transitively included in ArtifactSet.Skills.
func TestResolveArtifacts_UtilityAgent_IncludesTransitiveSkills(t *testing.T) {
	skill := makeSkill("utility-skill", "1.0")
	utility := makeUtilityAgent("my-utility")
	utility.RequiredSkills = []string{"utility-skill"}

	cat := &fakeCatalog{
		orchestrator:  makeOrchestrator(),
		utilityAgents: []domain.Agent{utility},
		skills:        []domain.Skill{skill},
	}

	set, err := plan.ResolveArtifacts(cat, nil, []string{"my-utility"}, nil, nil)
	must(t, err)

	if !containsSkill(set.Skills, "utility-skill") {
		t.Error("ArtifactSet.Skills missing skill required by selected utility agent")
	}
}

// ---------------------------------------------------------------------------
// T4.1 — Infrastructure agents included only when explicitly selected
// ---------------------------------------------------------------------------

// TestResolveArtifacts_InfrastructureAgent_IncludedWhenSelected verifies that an
// infrastructure agent appears in the result when its key is passed in infrastructureAgentIDs.
func TestResolveArtifacts_InfrastructureAgent_IncludedWhenSelected(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents:  []domain.Agent{makeInfrastructureAgent("checkpoint-manager-git", "checkpoint")},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, []string{"checkpoint-manager-git"}, nil)
	must(t, err)

	if !containsAgent(set.Agents, "checkpoint-manager-git") {
		t.Error("ArtifactSet.Agents missing infrastructure agent that was explicitly selected via infrastructureAgentIDs")
	}
}

// TestResolveArtifacts_InfrastructureAgent_NotIncludedWhenNotSelected verifies that an
// infrastructure agent known to the catalog does NOT appear when absent from
// infrastructureAgentIDs. Infrastructure agents are opt-in, like utility agents.
func TestResolveArtifacts_InfrastructureAgent_NotIncludedWhenNotSelected(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents:  []domain.Agent{makeInfrastructureAgent("checkpoint-manager-git", "checkpoint")},
	}

	// infrastructureAgentIDs is nil — no infrastructure agents selected.
	set, err := plan.ResolveArtifacts(cat, nil, nil, nil, nil)
	must(t, err)

	if containsAgent(set.Agents, "checkpoint-manager-git") {
		t.Error("ArtifactSet.Agents contains infrastructure agent that was NOT selected via infrastructureAgentIDs; " +
			"infrastructure agents must be opt-in")
	}
}

// TestResolveArtifacts_InfrastructureAgent_UnknownID_ReturnsErrUnknownAgent verifies that an
// unknown key in infrastructureAgentIDs returns ErrUnknownAgent rather than silently producing
// a smaller artifact set. An unknown reference must always be surfaced.
func TestResolveArtifacts_InfrastructureAgent_UnknownID_ReturnsErrUnknownAgent(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		// No infrastructure agents registered — the key below is intentionally unknown.
	}

	_, err := plan.ResolveArtifacts(cat, nil, nil, []string{"this-infra-agent-does-not-exist"}, nil)
	if err == nil {
		t.Fatal("ResolveArtifacts with unknown infrastructure agent ID: expected error, got nil")
	}
	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Errorf("ResolveArtifacts with unknown infrastructure agent: got error %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
}

// TestResolveArtifacts_InfrastructureAgents_SameClass_NoError verifies that selecting multiple
// infrastructure agents of the same class does not produce an error. The "at most one active
// per class" constraint is enforced at run start, not deploy time (AC4.2).
func TestResolveArtifacts_InfrastructureAgents_SameClass_NoError(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents: []domain.Agent{
			makeInfrastructureAgent("checkpoint-manager-git", "checkpoint"),
			makeInfrastructureAgent("checkpoint-manager-s3", "checkpoint"), // same class, different agent
		},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, []string{"checkpoint-manager-git", "checkpoint-manager-s3"}, nil)
	if err != nil {
		t.Errorf("ResolveArtifacts returned unexpected error for multiple same-class infrastructure agents: %v; "+
			"multiple same-class agents are allowed at deploy time (AC4.2)", err)
	}
	if !containsAgent(set.Agents, "checkpoint-manager-git") {
		t.Error("ArtifactSet.Agents missing checkpoint-manager-git")
	}
	if !containsAgent(set.Agents, "checkpoint-manager-s3") {
		t.Error("ArtifactSet.Agents missing checkpoint-manager-s3")
	}
}

// TestResolveArtifacts_InfrastructureAgents_MultipleClasses_AllIncluded verifies that when
// multiple infrastructure agents of different classes are selected, all appear in the set.
func TestResolveArtifacts_InfrastructureAgents_MultipleClasses_AllIncluded(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents: []domain.Agent{
			makeInfrastructureAgent("checkpoint-manager-git", "checkpoint"),
			makeInfrastructureAgent("orchestration-review", "review"),
			makeInfrastructureAgent("commit-manager-git", "commit"),
		},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, []string{
		"checkpoint-manager-git", "orchestration-review", "commit-manager-git",
	}, nil)
	must(t, err)

	for _, key := range []string{"checkpoint-manager-git", "orchestration-review", "commit-manager-git"} {
		if !containsAgent(set.Agents, key) {
			t.Errorf("ArtifactSet.Agents missing selected infrastructure agent %q", key)
		}
	}
}

// TestResolveArtifacts_InfrastructureAgent_DeduplicatesWithWorkflowAgents verifies that an
// agent referenced by both a workflow and infrastructureAgentIDs appears exactly once in
// the result.
func TestResolveArtifacts_InfrastructureAgent_DeduplicatesWithWorkflowAgents(t *testing.T) {
	// Construct an agent that exists in both workflow references and infraAgents.
	agent := makeInfrastructureAgent("double-agent", "checkpoint")
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{agent},
		workflows: []domain.Workflow{
			makeWorkflow("my-wf", "double-agent"),
		},
		infraAgents: []domain.Agent{agent},
	}

	set, err := plan.ResolveArtifacts(cat, []string{"my-wf"}, nil, []string{"double-agent"}, nil)
	must(t, err)

	count := 0
	for _, a := range set.Agents {
		if a.Key == "double-agent" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("agent double-agent appears %d times in ArtifactSet.Agents; must appear exactly once "+
			"even when selected via both workflow reference and infrastructureAgentIDs", count)
	}
}

// TestResolveArtifacts_NilInfrastructureAgentIDs_ExistingBehaviorUnchanged verifies that
// passing nil for infrastructureAgentIDs leaves the result identical to the pre-Stage-4
// behavior: no infrastructure agents are added beyond those already in workflows.
func TestResolveArtifacts_NilInfrastructureAgentIDs_ExistingBehaviorUnchanged(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents:  []domain.Agent{makeInfrastructureAgent("checkpoint-manager-git", "checkpoint")},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, nil, nil)
	must(t, err)

	// Only the orchestrator should be present.
	if len(set.Agents) != 1 {
		t.Errorf("ArtifactSet.Agents has %d agents; want 1 (orchestrator only) when no agents are selected "+
			"and infrastructureAgentIDs is nil", len(set.Agents))
	}
	if !containsAgent(set.Agents, "orchestrator") {
		t.Error("ArtifactSet.Agents missing orchestrator")
	}
}

// TestResolveArtifacts_InfrastructureAgent_IncludesTransitiveSkills verifies that a selected
// infrastructure agent's required skills are transitively included in ArtifactSet.Skills.
func TestResolveArtifacts_InfrastructureAgent_IncludesTransitiveSkills(t *testing.T) {
	skill := makeSkill("infra-skill", "1.0")
	agent := makeInfrastructureAgent("my-infra-agent", "checkpoint")
	agent.RequiredSkills = []string{"infra-skill"}

	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents:  []domain.Agent{agent},
		skills:       []domain.Skill{skill},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, []string{"my-infra-agent"}, nil)
	must(t, err)

	if !containsSkill(set.Skills, "infra-skill") {
		t.Error("ArtifactSet.Skills missing skill required by selected infrastructure agent")
	}
}

// TestResolveArtifacts_InfrastructureAgent_AreSortedByKey verifies that infrastructure agents
// added via infrastructureAgentIDs are included in the sorted-by-key output alongside all
// other agents.
func TestResolveArtifacts_InfrastructureAgent_AreSortedByKey(t *testing.T) {
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		infraAgents: []domain.Agent{
			makeInfrastructureAgent("z-infra", "checkpoint"),
			makeInfrastructureAgent("a-infra", "review"),
		},
	}

	set, err := plan.ResolveArtifacts(cat, nil, nil, []string{"z-infra", "a-infra"}, nil)
	must(t, err)

	for i := 1; i < len(set.Agents); i++ {
		if set.Agents[i].Key < set.Agents[i-1].Key {
			t.Errorf("ArtifactSet.Agents is not sorted by Key at index %d: %q < %q "+
				"(infrastructure agents must be sorted alongside all other agents)",
				i, set.Agents[i].Key, set.Agents[i-1].Key)
		}
	}
}
