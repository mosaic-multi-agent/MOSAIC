package plan_test

// orchestrator_region_staleness_test.go specifies how the plan compares the orchestrator
// injections version when the caller marks orchestrator keys as undeliverable (the source has no
// orchestrator marker and the harness has no orchestrator content for it):
//
//   - an undeliverable key expects no orchestrator region, so a deployed file without the region
//     compares clean;
//   - every key that is not marked compares exactly as before.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// undeliverablePlan builds the tvPlan scenario with the given keys marked undeliverable.
func undeliverablePlan(t *testing.T, undeliverable map[string]bool, states map[string]domain.DeployedArtifactState) domain.Plan {
	t.Helper()
	in := buildInfraInput(plan.InfrastructureDeclarationIntent{}, states)
	mod := in.Module.(*fakeModule)
	mod.desc.InjectionsVersion = tvShared
	mod.desc.OrchestratorInjectionsVersion = tvOrch
	in.OrchestratorRegionUndeliverable = undeliverable
	return buildInfraPlan(t, in)
}

// regionlessOrchestrators returns deployed orchestrators that carry the tagged shared region
// only.
func regionlessOrchestrators() map[string]domain.DeployedArtifactState {
	states := tvStates(tvShared, "")
	states[infraOrchPath] = tvState("sha256:orch", tvShared, "", false)
	states[infraScriptPath] = tvState("sha256:script", tvShared, "", false)
	return states
}

func bothUndeliverable() map[string]bool {
	return map[string]bool{"orchestrator": true, "orchestrator-script": true}
}

// An undeliverable orchestrator without the second region is current: the second update after
// the first write reports it unchanged.
func TestBuild_UndeliverableOrchestratorRegion_DeployedWithoutRegion_Unchanged(t *testing.T) {
	p := undeliverablePlan(t, bothUndeliverable(), regionlessOrchestrators())

	tvAssertUnchanged(t, p, "orchestrator", "orchestrator-script", "test-worker")
}

// The same deployed layout is stale when the keys are not marked (the harness has orchestrator
// content, or the source carries the marker): today's behaviour.
func TestBuild_DeliverableOrchestratorRegion_DeployedWithoutRegion_StaleOnOrchestratorVersion(t *testing.T) {
	cases := map[string]map[string]bool{
		"nil set":       nil,
		"empty set":     {},
		"other key set": {"test-worker": true},
		"false values":  {"orchestrator": false, "orchestrator-script": false},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			p := undeliverablePlan(t, set, regionlessOrchestrators())

			for _, key := range []string{"orchestrator", "orchestrator-script"} {
				item := mustFindItem(t, p, key)
				if item.Action != domain.ActionUpdate {
					t.Errorf("%s: Action = %q, want %q", key, item.Action, domain.ActionUpdate)
				}
				if _, ok := tvDelta(item, "orchestrator_injections_version"); !ok {
					t.Errorf("%s: no orchestrator_injections_version delta; deltas %v", key, item.Stale)
				}
			}
		})
	}
}

// The marking is per key: an unmarked orchestrator stays stale while a marked one is clean.
func TestBuild_UndeliverableOrchestratorRegion_MarksOnlyTheNamedKey(t *testing.T) {
	p := undeliverablePlan(t, map[string]bool{"orchestrator": true}, regionlessOrchestrators())

	tvAssertUnchanged(t, p, "orchestrator", "test-worker")
	if item := mustFindItem(t, p, "orchestrator-script"); item.Action != domain.ActionUpdate {
		t.Errorf("orchestrator-script: Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
}

// Marking a key does not hide a shared-version change on it.
func TestBuild_UndeliverableOrchestratorRegion_SharedVersionBump_StillStale(t *testing.T) {
	in := buildInfraInput(plan.InfrastructureDeclarationIntent{}, regionlessOrchestrators())
	mod := in.Module.(*fakeModule)
	mod.desc.InjectionsVersion = "1.1.0"
	mod.desc.OrchestratorInjectionsVersion = tvOrch
	in.OrchestratorRegionUndeliverable = bothUndeliverable()

	p := buildInfraPlan(t, in)

	item := mustFindItem(t, p, "orchestrator")
	if _, ok := tvDelta(item, "injections_version"); !ok || item.Action != domain.ActionUpdate {
		t.Errorf("orchestrator: Action %q, deltas %v; want an update with an injections_version delta", item.Action, item.Stale)
	}
	if _, ok := tvDelta(item, "orchestrator_injections_version"); ok {
		t.Errorf("orchestrator: unexpected orchestrator_injections_version delta for an undeliverable region")
	}
}

// A key with the marker (not marked) and an empty tagged region compares clean.
func TestBuild_DeliverableOrchestratorRegion_EmptyTaggedRegion_Unchanged(t *testing.T) {
	p := undeliverablePlan(t, map[string]bool{}, tvStates(tvShared, tvOrch))

	tvAssertUnchanged(t, p, "orchestrator", "orchestrator-script", "test-worker")
}

// A key with the marker (not marked) whose deployed file lacks the region is stale once, then
// clean after the update wrote the tagged region.
func TestBuild_DeliverableOrchestratorRegion_MissingRegion_StaleOnce_ThenClean(t *testing.T) {
	first := undeliverablePlan(t, map[string]bool{}, regionlessOrchestrators())

	if item := mustFindItem(t, first, "orchestrator"); item.Action != domain.ActionUpdate {
		t.Errorf("orchestrator: Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	second := undeliverablePlan(t, map[string]bool{}, tvStates(tvShared, tvOrch))
	tvAssertUnchanged(t, second, "orchestrator", "orchestrator-script")
}
