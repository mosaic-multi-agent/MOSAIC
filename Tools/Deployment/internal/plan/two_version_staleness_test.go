package plan_test

// two_version_staleness_test.go specifies how the plan compares the two harness injection
// versions of deployed agents: HarnessInjections.md (the shared version, tagged on the
// HarnessConstraints region) against every agent that has that region, and
// HarnessInjectionsOrchestrator.md (the orchestrator version, tagged on the second harness
// region) against orchestrator-role agents that have that region.
//
// Fixtures are in-memory deployed states; per-tier discovery lives in
// two_version_tier_acceptance_test.go.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

const (
	tvShared = "1.0.0" // shared version the deployed regions were written with
	tvOrch   = "2.0.0" // orchestrator version the deployed second region was written with
)

// tvState returns a deployed state whose version stamps match the fake catalog and module,
// carrying the shared region tagged shared and, when orchRegion is true, the second region
// tagged orch.
func tvState(hash, shared, orch string, orchRegion bool) domain.DeployedArtifactState {
	s := deployedState(hash, "1.0", "1.0", shared)
	s.HasInjectionRegion = true
	s.OrchestratorInjectionsVersion = orch
	s.HasOrchestratorInjectionRegion = orchRegion
	return s
}

// tvStates returns the deployed state of both orchestrator files and the worker: the
// orchestrators carry both regions, the worker only the shared one.
func tvStates(shared, orch string) map[string]domain.DeployedArtifactState {
	return map[string]domain.DeployedArtifactState{
		infraOrchPath:   tvState("sha256:orch", shared, orch, true),
		infraScriptPath: tvState("sha256:script", shared, orch, true),
		infraWorkerPath: tvState("sha256:worker", shared, "", false),
	}
}

// tvPlan builds the update-workspace plan for a harness whose injection files carry the given
// versions ("" = no such file) over the given deployed states.
func tvPlan(t *testing.T, shared, orch string, states map[string]domain.DeployedArtifactState) domain.Plan {
	t.Helper()
	in := buildInfraInput(plan.InfrastructureDeclarationIntent{}, states)
	mod := in.Module.(*fakeModule)
	mod.desc.InjectionsVersion = shared
	mod.desc.OrchestratorInjectionsVersion = orch
	return buildInfraPlan(t, in)
}

// tvDelta returns the delta of item with the given field.
func tvDelta(item domain.PlanItem, field string) (domain.VersionDelta, bool) {
	for _, d := range item.Stale {
		if d.Field == field {
			return d, true
		}
	}
	return domain.VersionDelta{}, false
}

// tvInjectionFields returns the injection-related delta fields of item.
func tvInjectionFields(item domain.PlanItem) []string {
	var out []string
	for _, d := range item.Stale {
		if d.Field == "injections_version" || d.Field == "orchestrator_injections_version" {
			out = append(out, d.Field)
		}
	}
	return out
}

func tvAssertUnchanged(t *testing.T, p domain.Plan, keys ...string) {
	t.Helper()
	for _, key := range keys {
		item := mustFindItem(t, p, key)
		if item.Action != domain.ActionUnchanged {
			t.Errorf("%s: Action = %q (reason %q), want %q", key, item.Action, reasonText(item), domain.ActionUnchanged)
		}
	}
}

// ---------------------------------------------------------------------------
// Current versions
// ---------------------------------------------------------------------------

// A deployed two-region layout whose tags equal the two file versions is current for every
// agent. This is also the second run after an update, and the case of an empty second region
// that carries its version tag.
func TestBuild_TwoRegionLayoutMatchingBothVersions_AllUnchanged(t *testing.T) {
	p := tvPlan(t, tvShared, tvOrch, tvStates(tvShared, tvOrch))

	tvAssertUnchanged(t, p, "orchestrator", "orchestrator-script", "test-worker")
}

// ---------------------------------------------------------------------------
// Shared file bump
// ---------------------------------------------------------------------------

// Editing HarnessInjections.md marks subagents and orchestrators alike; the delta compares the
// deployed shared tag against the new shared version for both roles.
func TestBuild_SharedVersionBump_MarksSubagentAndOrchestrators(t *testing.T) {
	p := tvPlan(t, "1.1.0", tvOrch, tvStates(tvShared, tvOrch))

	for _, key := range []string{"orchestrator", "orchestrator-script", "test-worker"} {
		item := mustFindItem(t, p, key)
		if item.Action != domain.ActionUpdate {
			t.Errorf("%s: Action = %q, want %q", key, item.Action, domain.ActionUpdate)
		}
		d, ok := tvDelta(item, "injections_version")
		if !ok {
			t.Errorf("%s: no injections_version delta; deltas %v", key, item.Stale)
			continue
		}
		if d.Deployed != tvShared || d.Source != "1.1.0" {
			t.Errorf("%s: injections_version delta = %q -> %q, want %q -> %q", key, d.Deployed, d.Source, tvShared, "1.1.0")
		}
		if _, ok := tvDelta(item, "orchestrator_injections_version"); ok {
			t.Errorf("%s: orchestrator_injections_version delta present although only the shared file changed", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Orchestrator file bump
// ---------------------------------------------------------------------------

// Editing only HarnessInjectionsOrchestrator.md marks only the orchestrator-role files, through
// the orchestrator delta alone; the subagent stays current.
func TestBuild_OrchestratorVersionBump_MarksOnlyOrchestrators(t *testing.T) {
	p := tvPlan(t, tvShared, "2.1.0", tvStates(tvShared, tvOrch))

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		item := mustFindItem(t, p, key)
		if item.Action != domain.ActionUpdate {
			t.Errorf("%s: Action = %q, want %q", key, item.Action, domain.ActionUpdate)
		}
		fields := tvInjectionFields(item)
		if len(fields) != 1 || fields[0] != "orchestrator_injections_version" {
			t.Errorf("%s: injection deltas = %v, want exactly [orchestrator_injections_version]", key, fields)
		}
		if d, ok := tvDelta(item, "orchestrator_injections_version"); ok && (d.Deployed != tvOrch || d.Source != "2.1.0") {
			t.Errorf("%s: orchestrator delta = %q -> %q, want %q -> %q", key, d.Deployed, d.Source, tvOrch, "2.1.0")
		}
	}
	tvAssertUnchanged(t, p, "test-worker")
}

// An orchestrator-file bump marks the orchestrator even when the deployed file lacks the second
// region (tagged shared region only): the orchestrator content still has to be written.
func TestBuild_OrchestratorVersionBump_OrchestratorWithoutSecondRegion_IsStale(t *testing.T) {
	states := tvStates(tvShared, tvOrch)
	states[infraOrchPath] = tvState("sha256:orch", tvShared, "", false)

	p := tvPlan(t, tvShared, "2.1.0", states)

	if item := mustFindItem(t, p, "orchestrator"); item.Action != domain.ActionUpdate {
		t.Errorf("orchestrator: Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
}

// A subagent that carries a second region (hand-edited or tampered) is not an orchestrator-role
// agent: an orchestrator-file bump alone does not mark it.
func TestBuild_OrchestratorVersionBump_SubagentWithSecondRegion_Unchanged(t *testing.T) {
	states := tvStates(tvShared, tvOrch)
	states[infraWorkerPath] = tvState("sha256:worker", tvShared, tvOrch, true)

	p := tvPlan(t, tvShared, "2.1.0", states)

	tvAssertUnchanged(t, p, "test-worker")
}

// An orchestrator written by an old external module has merged content in the shared region and
// a present but untagged second region; a change of only the orchestrator file still marks it
// through the orchestrator delta, and a subagent stays current.
func TestBuild_OldExternalModuleOrchestrator_OrchestratorFileOnlyChange_IsStale(t *testing.T) {
	states := tvStates(tvShared, tvOrch)
	states[infraOrchPath] = tvState("sha256:orch", tvShared, "", true)

	p := tvPlan(t, tvShared, "2.1.0", states)

	item := mustFindItem(t, p, "orchestrator")
	if item.Action != domain.ActionUpdate {
		t.Fatalf("orchestrator: Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	fields := tvInjectionFields(item)
	if len(fields) != 1 || fields[0] != "orchestrator_injections_version" {
		t.Errorf("orchestrator: injection deltas = %v, want exactly [orchestrator_injections_version]", fields)
	}
	if d, ok := tvDelta(item, "orchestrator_injections_version"); ok && (d.Deployed != "" || d.Source != "2.1.0") {
		t.Errorf("orchestrator delta = %q -> %q, want %q -> %q", d.Deployed, d.Source, "", "2.1.0")
	}
	tvAssertUnchanged(t, p, "test-worker")
}

// ---------------------------------------------------------------------------
// Legacy merged region
// ---------------------------------------------------------------------------

// An orchestrator deployed with the single merged region (tagged with the orchestrator file
// version, no second region) is stale once through the shared delta, and compares clean after
// the update writes the two-region layout.
func TestBuild_LegacyMergedOrchestrator_StaleOnce_ThenClean(t *testing.T) {
	legacy := tvStates(tvShared, tvOrch)
	legacy[infraOrchPath] = tvState("sha256:orch", tvOrch, "", false)

	first := tvPlan(t, tvShared, tvOrch, legacy)

	item := mustFindItem(t, first, "orchestrator")
	if item.Action != domain.ActionUpdate {
		t.Fatalf("legacy orchestrator: Action = %q, want %q", item.Action, domain.ActionUpdate)
	}
	d, ok := tvDelta(item, "injections_version")
	if !ok || d.Deployed != tvOrch || d.Source != tvShared {
		t.Errorf("legacy orchestrator: injections_version delta = %+v (found %v), want %q -> %q", d, ok, tvOrch, tvShared)
	}
	tvAssertUnchanged(t, first, "orchestrator-script", "test-worker")
	tvAssertUnchanged(t, tvPlan(t, tvShared, tvOrch, tvStates(tvShared, tvOrch)), "orchestrator")
}

// The legacy merged region may carry a tag equal to both new file versions; the orchestrator
// is still stale once, because it lacks the second region, and clean after the two-region update.
func TestBuild_LegacyMergedOrchestrator_AllVersionsEqual_StaleOnce_ThenClean(t *testing.T) {
	legacy := tvStates("1.0.0", "1.0.0")
	legacy[infraOrchPath] = tvState("sha256:orch", "1.0.0", "", false)

	first := tvPlan(t, "1.0.0", "1.0.0", legacy)

	if item := mustFindItem(t, first, "orchestrator"); item.Action != domain.ActionUpdate {
		t.Errorf("legacy orchestrator: Action = %q (reason %q), want %q", item.Action, reasonText(item), domain.ActionUpdate)
	}
	tvAssertUnchanged(t, first, "orchestrator-script", "test-worker")
	tvAssertUnchanged(t, tvPlan(t, "1.0.0", "1.0.0", tvStates("1.0.0", "1.0.0")), "orchestrator")
}

// ---------------------------------------------------------------------------
// Region-presence gate and missing files
// ---------------------------------------------------------------------------

// Agents without any harness injection region stay unchanged whatever the file versions are.
func TestBuild_AgentsWithoutInjectionRegion_Unchanged(t *testing.T) {
	states := map[string]domain.DeployedArtifactState{
		infraOrchPath:   deployedState("sha256:orch", "1.0", "1.0", ""),
		infraScriptPath: deployedState("sha256:script", "1.0", "1.0", ""),
		infraWorkerPath: deployedState("sha256:worker", "1.0", "1.0", ""),
	}

	p := tvPlan(t, "1.1.0", "2.1.0", states)

	tvAssertUnchanged(t, p, "orchestrator", "orchestrator-script", "test-worker")
}

// A harness without HarnessInjections.md, without HarnessInjectionsOrchestrator.md, or without
// either leaves its untagged regions untagged; none of these reports an agent stale on a second
// run.
func TestBuild_MissingInjectionFiles_NeverStale(t *testing.T) {
	cases := []struct {
		name         string
		shared, orch string // file versions of the harness; "" = file missing
		deployedTags [2]string
	}{
		{"neither file", "", "", [2]string{"", ""}},
		{"orchestrator file missing", "1.0.0", "", [2]string{"1.0.0", ""}},
		{"shared file missing", "", "2.0.0", [2]string{"", "2.0.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tvPlan(t, tc.shared, tc.orch, tvStates(tc.deployedTags[0], tc.deployedTags[1]))

			tvAssertUnchanged(t, p, "orchestrator", "orchestrator-script", "test-worker")
		})
	}
}

// ---------------------------------------------------------------------------
// AgentStaleness gates
// ---------------------------------------------------------------------------

// An orchestrator without the second region is stale whenever the source has an orchestrator
// version, even if the shared tag and the orchestrator version happen to be equal.
func TestAgentStaleness_Orchestrator_WithoutSecondRegion_EqualVersions_IsStale(t *testing.T) {
	deployed := tvState("sha256:x", "1.0.0", "", false)
	agent := makeOrchestrator()
	agent.Version = deployed.Version
	stamps := domain.VersionStamps{
		Version:                       deployed.Version,
		HarnessVersion:                deployed.HarnessVersion,
		InjectionsVersion:             "1.0.0",
		OrchestratorInjectionsVersion: "1.0.0",
	}

	deltas := plan.AgentStaleness(deployed, agent, stamps)

	if len(deltas) == 0 {
		t.Error("orchestrator without the second region compared clean; it must be reported stale once")
	}
}

// The shared delta applies to orchestrators exactly as to subagents, independent of the
// orchestrator version.
func TestAgentStaleness_Orchestrator_SharedVersionDiffers_ProducesSharedDeltaOnly(t *testing.T) {
	deployed := tvState("sha256:x", "1.0.0", "2.0.0", true)
	agent := makeOrchestrator()
	agent.Version = deployed.Version
	stamps := domain.VersionStamps{
		Version:                       deployed.Version,
		HarnessVersion:                deployed.HarnessVersion,
		InjectionsVersion:             "1.1.0",
		OrchestratorInjectionsVersion: "2.0.0",
	}

	deltas := plan.AgentStaleness(deployed, agent, stamps)

	if len(deltas) != 1 || deltas[0].Field != "injections_version" {
		t.Fatalf("deltas = %v, want exactly one injections_version delta", deltas)
	}
	if deltas[0].Deployed != "1.0.0" || deltas[0].Source != "1.1.0" {
		t.Errorf("delta = %q -> %q, want 1.0.0 -> 1.1.0", deltas[0].Deployed, deltas[0].Source)
	}
}
