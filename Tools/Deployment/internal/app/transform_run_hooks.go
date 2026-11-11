package app

// transform_run_hooks.go carries the planned hook bundles of a transform run: the bundles of
// the source deployment are installed from the target harness's variant through the deploy
// executor, which records them in the manifest and reports registration steps in the TODO
// file. A bundle that is not carried is named with its reason.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// hookState is the working state of one planned hook bundle during a run.
type hookState struct {
	// version is the catalog version of the bundle, recorded with each of its files.
	version string
}

// prepareHooks resolves the catalog version of every bundle the plan writes.
func (r *transformRun) prepareHooks() {
	r.hooks = make([]hookState, len(r.plan.Hooks))
	versions := map[string]string{}
	for _, b := range r.s.deps.Catalog.Hooks() {
		versions[b.Key] = b.Version
	}
	for i, hb := range r.plan.Hooks {
		if writes(hb.Action) {
			r.hooks[i].version = versions[hb.Key]
		}
	}
}

// hookWrites is the number of hook bundles that will be written.
func (r *transformRun) hookWrites() int {
	n := 0
	for _, hb := range r.plan.Hooks {
		if writes(hb.Action) {
			n++
		}
	}
	return n
}

// hookTargetPath is the workspace-relative slash path a hook file is written to.
func hookTargetPath(hp domain.HookPlan, f domain.HookFile) string {
	return path.Join(filepath.ToSlash(hp.TargetDir), filepath.ToSlash(f.TargetName))
}

// hookItems returns one plan item per file of every hook bundle that will be written. The
// executor writes each file from its own source and records it in the manifest.
func (r *transformRun) hookItems() []domain.PlanItem {
	var items []domain.PlanItem
	for _, hb := range r.plan.Hooks {
		if !writes(hb.Action) {
			continue
		}
		action, reason := domain.ActionCreate, "transform from "+r.srcID
		if hb.Action == PlanWriteOverwrite {
			action, reason = domain.ActionUpdate, "transform from "+r.srcID+" (overwrites the existing files)"
		}
		for _, f := range hb.Plan.Files {
			items = append(items, domain.PlanItem{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: hb.Key},
				SourcePath: f.SourcePath, TargetPath: hookTargetPath(hb.Plan, f),
				Action: action, Reasons: []string{reason},
			})
		}
	}
	return items
}

// hookPlans returns the target hook plans of the bundles that will be written, which supply the
// registration steps, with the version stamp of every hook file added to stamps.
func (r *transformRun) hookPlans(stamps map[string]domain.VersionStamp) []domain.HookPlan {
	var plans []domain.HookPlan
	for i, hb := range r.plan.Hooks {
		if !writes(hb.Action) {
			continue
		}
		plans = append(plans, hb.Plan)
		for _, f := range hb.Plan.Files {
			stamps[hookTargetPath(hb.Plan, f)] = domain.VersionStamp{Version: r.hooks[i].version, Catalog: r.sourceBundleCatalog(hb.Key)}
		}
	}
	return plans
}

// checkHookDestinations fails before anything is written when a hook file cannot be written
// because its destination is not a regular file. The executor reports such a hook file as a gap
// and keeps going; a transform run is all-or-nothing and must stop instead.
func (r *transformRun) checkHookDestinations() error {
	for _, hb := range r.plan.Hooks {
		if !writes(hb.Action) {
			continue
		}
		for _, f := range hb.Plan.Files {
			dest := filepath.Join(r.workspace, filepath.FromSlash(hb.Plan.TargetDir), filepath.FromSlash(f.TargetName))
			if info, err := os.Stat(dest); err == nil && !info.Mode().IsRegular() {
				return fmt.Errorf("hook %s: %s exists and is not a file", hb.Key, dest)
			}
		}
	}
	return nil
}

// hookGaps names every bundle of the source deployment that is not carried and why. A bundle
// that already exists in the target is kept, not dropped, and is not named.
func (r *transformRun) hookGaps() []domain.Gap {
	var gaps []domain.Gap
	for _, hb := range r.plan.Hooks {
		if hb.Action == PlanSkip && hb.Skip != SkipDestinationExists {
			gaps = append(gaps, domain.Gap{
				Kind: domain.GapSkippedFile, Subject: "hook " + hb.Key,
				Detail: "hook bundle " + hb.Key + " was not carried: " + hb.Reason,
			})
		}
	}
	return gaps
}

// hookOutcome is the outcome of one planned hook bundle.
func (r *transformRun) hookOutcome(i int) TransformFileOutcome {
	hb := r.plan.Hooks[i]
	out := TransformFileOutcome{
		Kind: domain.ArtifactHook, Key: hb.Key, DestinationPath: hb.Plan.TargetDir, Action: hb.Action,
	}
	switch {
	case hb.Action == PlanSkip:
		out.Status, out.Skip, out.Reason = StatusSkipped, hb.Skip, hb.Reason
	default:
		out.Status = StatusTransformed
	}
	return out
}
