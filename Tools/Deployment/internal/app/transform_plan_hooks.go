package app

// transform_plan_hooks.go plans hook bundles: those present in the source deployment, each
// either carried with its target variant or named as not carried with a reason.

import (
	"path"
	"strings"

	"mosaic-deploy/internal/domain"
)

// sharedHooksRegistrationFile is the registration file of the shared hooks folder, which stays
// as it is when the harnesses of a transform share that folder.
const sharedHooksRegistrationFile = "mosaic-logger.json"

// planHooks fills plan.Hooks.
func (s *service) planHooks(in transformPlanInput, plan *TransformPlan) {
	bundles := map[string]domain.HookBundle{}
	for _, h := range s.deps.Catalog.Hooks() {
		bundles[h.Key] = h
	}
	srcHooks := in.Source.Descriptor().Paths.Hooks.Project
	tgtHooks := in.Target.Descriptor().Paths.Hooks.Project
	for _, key := range s.deployedHookIDs(in.Source, in.Workspace, domain.ScopeProject) {
		plan.Hooks = append(plan.Hooks, planHookBundle(in, bundles[key], srcHooks == tgtHooks))
	}
}

// planHookBundle decides the action for one hook bundle of the source deployment.
func planHookBundle(in transformPlanInput, bundle domain.HookBundle, sharedFolder bool) PlannedHookBundle {
	hb := PlannedHookBundle{Key: bundle.Key, Action: PlanSkip}
	if sharedFolder {
		folder := in.Source.Descriptor().Paths.Hooks.Project
		hb.Skip = SkipSharedHooksFolder
		hb.Reason = "both harnesses use the hooks folder " + folder + " and its existing registration file " +
			path.Join(folder, sharedHooksRegistrationFile) + "; the hooks are already in place and are not carried"
		return hb
	}
	hp, err := in.Target.HookPlan(domain.HookPlanRequest{Bundle: bundle, Scope: domain.ScopeProject})
	if err != nil {
		hb.Skip, hb.Reason = SkipUnsupported, "target hook plan failed: "+err.Error()
		return hb
	}
	hb.Plan = hp
	switch {
	case !hp.Supported:
		hb.Skip = SkipUnsupported
		hb.Reason = strings.TrimSpace("target harness does not carry hooks: " + hp.Reason)
	case len(hp.Files) == 0:
		hb.Skip = SkipNoTargetVariant
		hb.Reason = "the bundle has no variant for the target harness"
	default:
		present := probeHookPlanPresence(in.Workspace, hp).Present
		switch {
		case present && !in.Overwrite:
			hb.Skip = SkipDestinationExists
			hb.Reason = "the bundle is already present in " + hp.TargetDir + " and is kept unchanged"
		case present:
			hb.Action = PlanWriteOverwrite
		default:
			hb.Action = PlanWriteCreate
		}
	}
	return hb
}
