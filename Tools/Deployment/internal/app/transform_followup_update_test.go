package app_test

// transform_followup_update_test.go specifies the hand-over from a transform to a following
// update for the target harness: nothing the transform carried (agent, skill or hook) is
// reported as locally modified, and after an update that skips conflicts every target path
// still has exactly one manifest entry.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// trExecSpy runs the real executor and records the requests it received.
type trExecSpy struct {
	inner deploy.Executor
	reqs  []deploy.ExecRequest
}

func (e *trExecSpy) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.reqs = append(e.reqs, req)
	return e.inner.Execute(ctx, req)
}

// trFollowRun wires a Claude Code to OpenCode transform over a source deployment holding the
// agent fixtures, one skill and one hook bundle, with the real planner so that a following
// update classifies every carried item.
func trFollowRun(t *testing.T) (*trRun, *trExecSpy) {
	t.Helper()
	cat := trCarryCatalog(t)
	cat.agents = []domain.Agent{{Key: "subagent", NumericID: "55", SourcePath: "subagent.md", Version: "2.3.0", Role: domain.RoleSubagent, RequiredSkills: []string{trSkillKey}}}
	cat.skills = []domain.Skill{{
		Key: trSkillKey, Version: trSkillVersion, SourceDir: trAbs(t, "transform/skills/review-skill"),
		EntryFile: "SKILL.md", Files: []string{"SKILL.md", "refs/checklist.md"},
	}}
	spy := &trExecSpy{}
	r := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) {
		d.Catalog = cat
		d.Planner = plan.New()
		spy.inner = d.Executor
		d.Executor = spy
	})
	r.seed(trAgentFixtures...)
	r.seedSkill()
	r.seedHookBundle(trHookKey)
	return r, spy
}

// trTransformThenUpdate transforms, then updates the target harness with the given conflict
// decision, and returns the executor request of the update.
func trTransformThenUpdate(t *testing.T, r *trRun, spy *trExecSpy, decision domain.ConflictDecision) deploy.ExecRequest {
	t.Helper()
	req := r.request()
	req.TargetModel = "pre/answered"
	r.mustRun(req)
	before := len(spy.reqs)

	_, err := r.svc.Update(context.Background(), app.UpdateRequest{
		HarnessID: r.tgt, WorkspacePath: r.ws, ConflictDefault: decision, AutoConfirmPlan: true,
		SkipAll: map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(spy.reqs) != before+1 {
		t.Fatalf("update executor calls = %d, want 1", len(spy.reqs)-before)
	}
	return spy.reqs[len(spy.reqs)-1]
}

// trPlanItem returns the plan item of a ref, failing the test when the update did not plan it.
func trPlanItem(t *testing.T, items []domain.PlanItem, kind domain.ArtifactKind, key string) domain.PlanItem {
	t.Helper()
	for _, it := range items {
		if it.Ref.Kind == kind && it.Ref.Key == key {
			return it
		}
	}
	t.Fatalf("the update plan has no %s item %q", kind, key)
	return domain.PlanItem{}
}

func TestTransformThenUpdate_NothingCarriedIsReportedAsLocallyModified(t *testing.T) {
	r, spy := trFollowRun(t)

	got := trTransformThenUpdate(t, r, spy, domain.DecisionOverwrite)

	// The carried skill and hook must be part of the update plan; without them the conflict
	// check below would pass vacuously.
	trPlanItem(t, got.Plan.Items, domain.ArtifactSkill, trSkillKey)
	trPlanItem(t, got.Plan.Items, domain.ArtifactHook, trHookKey)
	for _, it := range got.Plan.Items {
		switch it.Ref.Kind {
		case domain.ArtifactAgent, domain.ArtifactSkill, domain.ArtifactHook:
			if it.Action == domain.ActionConflict {
				t.Errorf("%s %q at %s is reported as a conflict (%s)", it.Ref.Kind, it.Ref.Key, it.TargetPath, it.Reason)
			}
		}
	}
}

func TestTransformThenUpdate_CarriedSkillAndHookAreRecognisedAsAlreadyDeployed(t *testing.T) {
	r, spy := trFollowRun(t)

	got := trTransformThenUpdate(t, r, spy, domain.DecisionOverwrite)

	for _, ref := range []struct {
		kind domain.ArtifactKind
		key  string
	}{{domain.ArtifactSkill, trSkillKey}, {domain.ArtifactHook, trHookKey}} {
		it := trPlanItem(t, got.Plan.Items, ref.kind, ref.key)
		if it.Action != domain.ActionUnchanged && it.Action != domain.ActionUpdate {
			t.Errorf("%s %q: action = %q (%s), want unchanged or update of a recorded deployment", ref.kind, ref.key, it.Action, it.Reason)
		}
	}
}

func TestTransformThenUpdate_CarriedAgentIsNotAConflict(t *testing.T) {
	r, spy := trFollowRun(t)

	got := trTransformThenUpdate(t, r, spy, domain.DecisionOverwrite)

	it := trPlanItem(t, got.Plan.Items, domain.ArtifactAgent, "subagent")
	if it.Action == domain.ActionConflict || it.Action == domain.ActionCreate {
		t.Errorf("agent subagent: action = %q (%s), want unchanged or update", it.Action, it.Reason)
	}
}

func TestTransformThenUpdateWithSkip_EveryTargetPathKeepsExactlyOneManifestEntry(t *testing.T) {
	r, spy := trFollowRun(t)

	trTransformThenUpdate(t, r, spy, domain.DecisionSkip)

	perPath := map[string]int{}
	for _, e := range trLoadManifest(t, r.ws).Entries {
		perPath[e.TargetPath]++
	}
	for _, key := range trAgentFixtures {
		if n := perPath[r.dest(key)]; n != 1 {
			t.Errorf("agent %s: %d manifest entries at %s, want exactly one", key, n, r.dest(key))
		}
	}
	skillFile := r.skillsDir(hOpen) + "/" + trSkillKey + "/SKILL.md"
	if n := perPath[skillFile]; n != 1 {
		t.Errorf("skill: %d manifest entries at %s, want exactly one", n, skillFile)
	}
	if n := perPath[""]; n != 0 {
		t.Errorf("%d manifest entries are recorded under an empty target path", n)
	}
	if got := trEntryByRef(trLoadManifest(t, r.ws), domain.ArtifactHook, trHookKey); len(got) == 0 {
		t.Error("the hook bundle has no manifest entry after transform then update")
	}
	for path, n := range perPath {
		if n != 1 {
			t.Errorf("%d manifest entries at %s, want exactly one", n, path)
		}
	}
}
