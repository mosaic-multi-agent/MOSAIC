package app

// transform_plan_ghcp_pair.go plans the move between the two GHCP targets (ghcp-cli and
// vscode-ghcp, either direction). Both targets share folders and file names, so the target
// replaces the source instead of sitting beside it: the source agents MOSAIC deployed are
// planned for removal with their manifest entry, source-variant hook files the target does not
// write are planned for removal, and existing target files are planned for replacement. The
// pair decision lives in isGhcpPair only.

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"mosaic-deploy/internal/domain"
)

// isGhcpPair reports whether the transform runs between the two GHCP targets.
func isGhcpPair(srcID, tgtID string) bool {
	return (srcID == ghcpCLIHarnessID && tgtID == ghcpVSCodeHarnessID) ||
		(srcID == ghcpVSCodeHarnessID && tgtID == ghcpCLIHarnessID)
}

// inputIsGhcpPair is isGhcpPair over the modules of a plan input.
func inputIsGhcpPair(in transformPlanInput) bool {
	return isGhcpPair(in.Source.Descriptor().ID, in.Target.Descriptor().ID)
}

// refuseBothGhcpTargets refuses a pair transform on a workspace that holds both GHCP targets.
// It reads the manifest and writes nothing. Every other transform and workspace state passes.
func (r *transformRun) refuseBothGhcpTargets() error {
	if !isGhcpPair(r.srcID, r.tgtID) {
		return nil
	}
	held := r.s.ghcpTargetsOf(r.workspace)
	if !held.CLI || !held.VSCode {
		return nil
	}
	return &GhcpTargetConflictError{
		Workspace: r.workspace,
		Requested: r.s.ghcpHarnessRef(r.tgtID),
		Present:   []domain.HarnessRef{r.s.ghcpHarnessRef(ghcpCLIHarnessID), r.s.ghcpHarnessRef(ghcpVSCodeHarnessID)},
	}
}

// removedPaths is the set of lower-cased slash paths named by the removals.
func removedPaths(removals []domain.Removal) map[string]bool {
	out := map[string]bool{}
	for _, r := range removals {
		out[strings.ToLower(r.Path)] = true
	}
	return out
}

// planPairHookChanges adds the removals and replacements of one carried hook bundle: the
// source-variant files and registration targets the target does not write are removed, and the
// target-variant files and performable registration targets that already exist are replaced.
func planPairHookChanges(in transformPlanInput, bundle domain.HookBundle, hb PlannedHookBundle, plan *TransformPlan) {
	if hb.Action != PlanWriteCreate && hb.Action != PlanWriteOverwrite {
		return
	}
	ref := domain.ArtifactRef{Kind: domain.ArtifactHook, Key: bundle.Key}
	tgtFiles, tgtRegs := hookPlanWritePaths(hb.Plan)
	if srcPlan, err := in.Source.HookPlan(domain.HookPlanRequest{Bundle: bundle, Scope: domain.ScopeProject}); err == nil && srcPlan.Supported {
		srcFiles, srcRegs := hookPlanWritePaths(srcPlan)
		for _, p := range sortedKeys(srcFiles) {
			if !tgtFiles[p] && fileExists(in.Workspace, p) {
				plan.Removals = append(plan.Removals, domain.Removal{Ref: ref, Path: p,
					Reason: "the source harness hook file is not part of the target harness hooks"})
			}
		}
		for _, p := range sortedKeys(srcRegs) {
			if !tgtRegs[p] && fileExists(in.Workspace, p) {
				plan.Removals = append(plan.Removals, domain.Removal{Ref: ref, Path: p,
					Reason: "the source harness hook registration file is not written for the target harness"})
			}
		}
	}
	for _, p := range sortedKeys(tgtFiles) {
		if fileExists(in.Workspace, p) {
			plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: p,
				Reason: "the existing hook file is replaced by the target harness version"})
		}
	}
	for _, p := range sortedKeys(tgtRegs) {
		if fileExists(in.Workspace, p) {
			plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: p,
				Reason: "the shared hook registration file is replaced by the target harness version, even if edited"})
		}
	}
}

// hookPlanWritePaths returns the workspace-relative slash paths of the hook files a plan writes and
// of the registration targets it performs.
func hookPlanWritePaths(hp domain.HookPlan) (files, regs map[string]bool) {
	files, regs = map[string]bool{}, map[string]bool{}
	for _, f := range hp.Files {
		files[path.Join(filepath.ToSlash(hp.TargetDir), f.TargetName)] = true
	}
	for _, step := range hp.Registration {
		if step.Performable && step.TargetPath != "" {
			regs[path.Clean(filepath.ToSlash(step.TargetPath))] = true
		}
	}
	return files, regs
}

// fileExists reports whether a regular file exists at the workspace-relative slash path.
func fileExists(workspace, rel string) bool {
	info, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

// sortedKeys returns the keys of set in ascending order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
