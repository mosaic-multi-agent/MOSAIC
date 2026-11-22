package plan_test

// resolve_workflows_test.go covers workflow-driven artifact resolution against a synthetic
// in-memory catalog:
//   - A workflow resolves its referenced agents and their transitive, deduplicated skills
//   - The agent count is referenced agents plus orchestrator and orchestrator-script
//   - Multiple overlapping workflows deduplicate agents and skills
//   - An empty selection yields exactly the orchestrator and orchestrator-script
//   - An unknown workflow ID returns ErrUnknownWorkflow
//   - Agents and skills are sorted by key; the orchestrator appears exactly once

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	wfSmall = "wf-small"
	wfLarge = "wf-large"
)

// newWorkflowCatalog returns a synthetic catalog with two overlapping workflows.
//
//	wf-small references: zeta-planner, beta-reviewer, gamma-builder
//	wf-large references: zeta-planner, beta-reviewer, delta-runner, alpha-designer
//
// Skills are required as follows: skill-files by zeta-planner, beta-reviewer and gamma-builder
// (shared), skill-core by zeta-planner and delta-runner, skill-extra by gamma-builder and
// alpha-designer. Workers and skills are deliberately listed out of key order.
func newWorkflowCatalog() *fakeCatalog {
	planner := makeAgent("zeta-planner", "1.0")
	planner.RequiredSkills = []string{"skill-core", "skill-files"}
	reviewer := makeAgent("beta-reviewer", "1.0")
	reviewer.RequiredSkills = []string{"skill-files"}
	builder := makeAgent("gamma-builder", "1.0")
	builder.RequiredSkills = []string{"skill-files", "skill-extra"}
	runner := makeAgent("delta-runner", "1.0")
	runner.RequiredSkills = []string{"skill-core"}
	designer := makeAgent("alpha-designer", "1.0")
	designer.RequiredSkills = []string{"skill-extra"}

	script := makeAgent("orchestrator-script", "1.0")
	script.Role = domain.RoleOrchestrator

	return &fakeCatalog{
		orchestrator:         makeOrchestrator(),
		orchestratorScript:   script,
		orchestratorScriptOK: true,
		workers:              []domain.Agent{planner, reviewer, builder, runner, designer},
		skills: []domain.Skill{
			makeSkill("skill-files", "1.0"),
			makeSkill("skill-extra", "1.0"),
			makeSkill("skill-core", "1.0"),
		},
		workflows: []domain.Workflow{
			makeWorkflow(wfSmall, "zeta-planner", "beta-reviewer", "gamma-builder"),
			makeWorkflow(wfLarge, "zeta-planner", "beta-reviewer", "delta-runner", "alpha-designer"),
		},
	}
}

// countAgentKey returns how many agents in the slice carry the given key.
func countAgentKey(agents []domain.Agent, key string) int {
	n := 0
	for _, a := range agents {
		if a.Key == key {
			n++
		}
	}
	return n
}

// assertNoDuplicateSkills fails the test if any skill key occurs more than once.
func assertNoDuplicateSkills(t *testing.T, skills []domain.Skill) {
	t.Helper()
	seen := make(map[string]int)
	for _, s := range skills {
		seen[s.Key]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Errorf("skill %q appears %d times in ArtifactSet.Skills; each skill must appear at most once", key, count)
		}
	}
}

func TestResolveArtifacts_SingleWorkflow_ContainsReferencedAgents(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall}, nil, nil, nil)
	must(t, err)

	for _, key := range []string{"zeta-planner", "beta-reviewer", "gamma-builder"} {
		if !containsAgent(set.Agents, key) {
			t.Errorf("ArtifactSet.Agents missing %q; the workflow must include all its referenced agents", key)
		}
	}
	if containsAgent(set.Agents, "delta-runner") {
		t.Error("ArtifactSet.Agents contains delta-runner, which only the other workflow references")
	}
}

func TestResolveArtifacts_SingleWorkflow_AgentCountMatchesWorkflowPlusOrchestrator(t *testing.T) {
	cat := newWorkflowCatalog()
	referenced := len(cat.workflows[0].ReferencedAgents)

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall}, nil, nil, nil)
	must(t, err)

	want := referenced + 1 + 1 // referenced agents + orchestrator + orchestrator-script
	if len(set.Agents) != want {
		t.Errorf("ArtifactSet.Agents: got %d agents, want %d (%d referenced + orchestrator + orchestrator-script)",
			len(set.Agents), want, referenced)
	}
}

func TestResolveArtifacts_SingleWorkflow_IncludesTransitiveSkills(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall}, nil, nil, nil)
	must(t, err)

	for _, key := range []string{"skill-core", "skill-files", "skill-extra"} {
		if !containsSkill(set.Skills, key) {
			t.Errorf("ArtifactSet.Skills missing %q; it is required by an agent in the workflow", key)
		}
	}
}

func TestResolveArtifacts_SingleWorkflow_SkillsAreDeduplicated(t *testing.T) {
	cat := newWorkflowCatalog()

	// skill-files is required by three agents of wf-small.
	set, err := plan.ResolveArtifacts(cat, []string{wfSmall}, nil, nil, nil)
	must(t, err)

	assertNoDuplicateSkills(t, set.Skills)
	if !containsSkill(set.Skills, "skill-files") {
		t.Error("ArtifactSet.Skills missing the shared skill skill-files")
	}
}

func TestResolveArtifacts_TwoWorkflows_DeduplicatesAgents(t *testing.T) {
	cat := newWorkflowCatalog()

	// zeta-planner and beta-reviewer are referenced by both workflows.
	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	for _, a := range set.Agents {
		if n := countAgentKey(set.Agents, a.Key); n > 1 {
			t.Errorf("agent %q appears %d times in ArtifactSet.Agents; each agent must appear at most once", a.Key, n)
		}
	}
}

func TestResolveArtifacts_TwoWorkflows_UnionContainsAllAgents(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	all := []string{"zeta-planner", "beta-reviewer", "gamma-builder", "delta-runner", "alpha-designer"}
	for _, key := range all {
		if !containsAgent(set.Agents, key) {
			t.Errorf("ArtifactSet.Agents missing %q; it is referenced by one of the selected workflows", key)
		}
	}
	if want := len(all) + 2; len(set.Agents) != want {
		t.Errorf("ArtifactSet.Agents: got %d agents, want %d (union of both workflows + orchestrator + orchestrator-script)",
			len(set.Agents), want)
	}
}

func TestResolveArtifacts_TwoWorkflows_SkillsAreDeduplicated(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	assertNoDuplicateSkills(t, set.Skills)
	if len(set.Skills) != 3 {
		t.Errorf("ArtifactSet.Skills: got %d skills, want 3 (skill-core, skill-extra, skill-files)", len(set.Skills))
	}
}

func TestResolveArtifacts_EmptyWorkflowIDs_ResultHoldsOnlyOrchestrators(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, nil, nil, nil, nil)
	must(t, err)

	if len(set.Agents) != 2 {
		t.Errorf("ArtifactSet.Agents: got %d agents, want 2 (orchestrator + orchestrator-script)", len(set.Agents))
	}
	if !containsAgent(set.Agents, "orchestrator") {
		t.Error("ArtifactSet.Agents missing orchestrator; the orchestrator is always included")
	}
	if !containsAgent(set.Agents, "orchestrator-script") {
		t.Error("ArtifactSet.Agents missing orchestrator-script")
	}
}

func TestResolveArtifacts_UnknownWorkflow_ReturnsErrUnknownWorkflow(t *testing.T) {
	cat := newWorkflowCatalog()

	_, err := plan.ResolveArtifacts(cat, []string{"this-workflow-does-not-exist"}, nil, nil, nil)
	if err == nil {
		t.Fatal("ResolveArtifacts with unknown workflow ID: expected error, got nil")
	}
	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Errorf("ResolveArtifacts with unknown workflow: got error %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
}

func TestResolveArtifacts_AgentsAreSortedByKey(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	if len(set.Agents) < 2 {
		t.Fatalf("ArtifactSet.Agents has %d entries; the synthetic catalog must yield at least 2", len(set.Agents))
	}
	for i := 1; i < len(set.Agents); i++ {
		if set.Agents[i].Key < set.Agents[i-1].Key {
			t.Errorf("ArtifactSet.Agents is not sorted by Key at index %d: %q < %q",
				i, set.Agents[i].Key, set.Agents[i-1].Key)
		}
	}
}

func TestResolveArtifacts_SkillsAreSortedByKey(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	if len(set.Skills) < 2 {
		t.Fatalf("ArtifactSet.Skills has %d entries; the synthetic catalog must yield at least 2", len(set.Skills))
	}
	for i := 1; i < len(set.Skills); i++ {
		if set.Skills[i].Key < set.Skills[i-1].Key {
			t.Errorf("ArtifactSet.Skills is not sorted by Key at index %d: %q < %q",
				i, set.Skills[i].Key, set.Skills[i-1].Key)
		}
	}
}

func TestResolveArtifacts_OrchestratorAlwaysIncluded_WithWorkflow(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall}, nil, nil, nil)
	must(t, err)

	if !containsAgent(set.Agents, "orchestrator") {
		t.Error("ArtifactSet.Agents missing orchestrator; it must be present when workflows are selected")
	}
}

func TestResolveArtifacts_OrchestratorAppearsExactlyOnce(t *testing.T) {
	cat := newWorkflowCatalog()

	set, err := plan.ResolveArtifacts(cat, []string{wfSmall, wfLarge}, nil, nil, nil)
	must(t, err)

	if n := countAgentKey(set.Agents, "orchestrator"); n != 1 {
		t.Errorf("orchestrator appears %d times in ArtifactSet.Agents; must appear exactly once", n)
	}
}
