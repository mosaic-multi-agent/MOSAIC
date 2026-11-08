package plan_test

// resolve_kept_test.go verifies how artifact resolution treats workflow ids that the caller
// marks as kept: they are not looked up in the catalog and name no agents, while every other
// unknown id and every catalog workflow keeps failing strictly.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// newKeptTestCatalog returns a catalog holding the workflow "quick-fix" (names "worker-a"),
// the workflow "extra-wf" (names "extra-agent"), and both agents.
func newKeptTestCatalog() *fakeCatalog {
	return &fakeCatalog{
		root:         "/fake/catalog",
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{makeAgent("worker-a", "1.0"), makeAgent("extra-agent", "1.0")},
		workflows: []domain.Workflow{
			makeWorkflow("quick-fix", "worker-a"),
			makeWorkflow("extra-wf", "extra-agent"),
		},
	}
}

func TestResolveArtifactsFrom_KeptIdAbsentFromCatalog_Resolves(t *testing.T) {
	cat := newKeptTestCatalog()

	set, err := plan.ResolveArtifactsFrom(cat, plan.Selection{
		WorkflowIDs:     []string{"quick-fix", "my-custom"},
		KeptWorkflowIDs: []string{"my-custom"},
	})

	if err != nil {
		t.Fatalf("ResolveArtifactsFrom with a kept id absent from the catalog: error = %v, want nil", err)
	}
	if !containsAgent(set.Agents, "worker-a") {
		t.Errorf("artifact set lacks worker-a, the agent of the catalog workflow quick-fix")
	}
}

func TestResolveArtifactsFrom_KeptId_AddsNoAgentOrSkillOfItsOwn(t *testing.T) {
	cat := newKeptTestCatalog()
	cat.workers[1].RequiredSkills = []string{"extra-skill"}
	cat.skills = []domain.Skill{makeSkill("extra-skill", "1.0")}

	set, err := plan.ResolveArtifactsFrom(cat, plan.Selection{
		WorkflowIDs:     []string{"quick-fix", "extra-wf"},
		KeptWorkflowIDs: []string{"extra-wf"},
	})

	if err != nil {
		t.Fatalf("ResolveArtifactsFrom: error = %v, want nil", err)
	}
	if containsAgent(set.Agents, "extra-agent") {
		t.Errorf("artifact set holds extra-agent, which only the kept id extra-wf names; a kept id must resolve no agents")
	}
	if containsSkill(set.Skills, "extra-skill") {
		t.Errorf("artifact set holds extra-skill, which only the kept id extra-wf's agent requires; a kept id must resolve no skills")
	}
	if !containsAgent(set.Agents, "worker-a") {
		t.Errorf("artifact set lacks worker-a, the agent of the catalog workflow quick-fix")
	}
}

func TestResolveArtifactsFrom_KeptIdNamingMissingAgent_DoesNotFail(t *testing.T) {
	cat := newKeptTestCatalog()
	cat.workflows = append(cat.workflows, makeWorkflow("ghost-wf", "no-such-agent"))

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{
		WorkflowIDs:     []string{"quick-fix", "ghost-wf"},
		KeptWorkflowIDs: []string{"ghost-wf"},
	})

	if err != nil {
		t.Errorf("a kept id must not be validated against the catalog's agents: error = %v, want nil", err)
	}
}

func TestResolveArtifactsFrom_UnmarkedUnknownIdBesideKeptId_FailsNamingId(t *testing.T) {
	cat := newKeptTestCatalog()

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{
		WorkflowIDs:     []string{"quick-fix", "mystery-wf", "my-custom"},
		KeptWorkflowIDs: []string{"my-custom"},
	})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "mystery-wf") {
		t.Errorf("error %q does not name the unknown workflow id", err.Error())
	}
}

// A kept id elsewhere in the selection must not loosen validation of catalog workflows.
func TestResolveArtifactsFrom_CatalogWorkflowNamingMissingAgent_FailsNamingAgent(t *testing.T) {
	cat := newKeptTestCatalog()
	cat.workflows = append(cat.workflows, makeWorkflow("ghost-wf", "no-such-agent"))

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{
		WorkflowIDs:     []string{"ghost-wf"},
		KeptWorkflowIDs: []string{"my-custom"},
	})

	if !errors.Is(err, plan.ErrUnknownAgent) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownAgent)", err)
	}
	if !strings.Contains(err.Error(), "no-such-agent") {
		t.Errorf("error %q does not name the missing agent", err.Error())
	}
}

func TestResolveArtifactsFrom_NoKeptIds_UnknownIdStillFails(t *testing.T) {
	cat := newKeptTestCatalog()

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"my-custom"}})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "my-custom") {
		t.Errorf("error %q does not name the unknown workflow id", err.Error())
	}
}

func TestResolveArtifactsFrom_NoKeptIds_ResolvesCatalogWorkflowAgents(t *testing.T) {
	cat := newKeptTestCatalog()

	set, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"quick-fix", "extra-wf"}})

	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	for _, key := range []string{"worker-a", "extra-agent"} {
		if !containsAgent(set.Agents, key) {
			t.Errorf("artifact set lacks %q, an agent of a selected catalog workflow", key)
		}
	}
}
