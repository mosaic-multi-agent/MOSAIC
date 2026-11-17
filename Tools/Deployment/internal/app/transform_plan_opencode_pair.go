package app

// transform_plan_opencode_pair.go plans the move between the two OpenCode targets (opencode and
// opencode-v2, either direction). Both targets write the same paths under .opencode/, so the
// target replaces the source in place: a recorded agent is overwritten, a recorded skill folder
// is carried in place, and every source-variant hook file the target does not write is planned
// for removal, so no source plugin lingers in .opencode/plugins (V2 drops a V1-form plugin with
// a warning). Only files the source harness's manifest entries record are touched; everything
// else is left alone. The pair decision lives in isOpenCodePair only.

import (
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// isOpenCodePair reports whether the transform runs between the two OpenCode targets.
func isOpenCodePair(srcID, tgtID string) bool {
	return (srcID == openCodeHarnessID && tgtID == openCodeV2HarnessID) ||
		(srcID == openCodeV2HarnessID && tgtID == openCodeHarnessID)
}

// inputIsOpenCodePair is isOpenCodePair over the modules of a plan input.
func inputIsOpenCodePair(in transformPlanInput) bool {
	return isOpenCodePair(in.Source.Descriptor().ID, in.Target.Descriptor().ID)
}

// refuseBothOpenCodeTargets refuses a pair transform on a workspace that holds both OpenCode
// targets. It reads the manifest and writes nothing. Every other transform and workspace state
// passes.
func (r *transformRun) refuseBothOpenCodeTargets() error {
	if !isOpenCodePair(r.srcID, r.tgtID) {
		return nil
	}
	held := r.s.openCodeTargetsOf(r.workspace)
	if !held.V1 || !held.V2 {
		return nil
	}
	return &OpenCodeTargetConflictError{
		Workspace: r.workspace,
		Requested: r.s.ghcpHarnessRef(r.tgtID),
		Present:   []domain.HarnessRef{r.s.ghcpHarnessRef(openCodeHarnessID), r.s.ghcpHarnessRef(openCodeV2HarnessID)},
	}
}

// planOpenCodeAgents fills plan.Agents, plan.Replacements and the agent notes for an OpenCode
// pair transform. A recorded agent is overwritten in place; any other file is left unchanged.
func (s *service) planOpenCodeAgents(in transformPlanInput, relDir string, files []sourceAgentFile, plan *TransformPlan) {
	recorded := s.recordedAgentPaths(in.Workspace, in.Source.Ref().ID)
	srcExt := in.Source.Descriptor().Extensions[domain.ArtifactAgent]

	recognised := 0
	for _, f := range files {
		a := PlannedAgent{SourcePath: f.path, FileName: f.name}
		if !f.recognised {
			a.Action, a.Skip, a.Reason = PlanSkip, SkipNotAgent, "not a MOSAIC agent: "+f.reason
			plan.Agents = append(plan.Agents, a)
			continue
		}
		recognised++
		a.Key, a.SourceModel = agentkey.FromFileNameExt(f.name, srcExt), f.model
		ref, isRecorded := recorded[strings.ToLower(slashJoin(relDir, f.name))]
		if !isRecorded {
			a.Action, a.Skip = PlanSkip, SkipNotRecorded
			a.Reason = "the manifest does not record this file as a MOSAIC agent, so it is left unchanged"
			plan.Agents = append(plan.Agents, a)
			continue
		}
		s.planPairWrite(in, &a)
		if a.Action == PlanWriteOverwrite {
			plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: filepath.ToSlash(a.DestinationPath),
				Reason: "the existing agent file is replaced by the target harness version"})
		}
		plan.Agents = append(plan.Agents, a)
	}
	if recognised == 0 {
		plan.Notes = append(plan.Notes, TransformNote{
			Kind:    NoteNoAgentsFound,
			Message: "no MOSAIC agents found in " + plan.SourceAgentsDir,
			Paths:   []string{plan.SourceAgentsDir},
		})
	}
}

// recordedSkillFolder reports whether a manifest entry of the source harness records a file
// below the skill folder dir (workspace-relative slash path).
func recordedSkillFolder(snap manifest.Snapshot, dir string) bool {
	if snap.State != manifest.StatePresent {
		return false
	}
	prefix := strings.ToLower(path.Clean(dir)) + "/"
	for _, e := range snap.Manifest.Entries {
		if e.Ref.Kind != domain.ArtifactSkill {
			continue
		}
		p := strings.ToLower(path.Clean(strings.ReplaceAll(e.TargetPath, "\\", "/")))
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// planOpenCodeSkill plans one skill folder of an OpenCode pair transform. Both targets keep
// skills in the same folder, so a recorded skill is carried in place whatever the overwrite flag
// says and an unrecorded one is left unchanged.
func (s *service) planOpenCodeSkill(sk PlannedSkill, snap manifest.Snapshot) PlannedSkill {
	if !recordedSkillFolder(snap, sk.DestinationDir) {
		sk.Action, sk.Skip = PlanSkip, SkipNotRecorded
		sk.Reason = "the manifest does not record this skill folder as a MOSAIC skill, so it is left unchanged"
		return sk
	}
	sk.Action, sk.Skip, sk.Reason = PlanWriteOverwrite, "", ""
	return sk
}

// planOpenCodeHookChanges adds the removals and replacements of one hook bundle of an OpenCode
// pair transform. Every source-variant file and performable registration target the target does
// not write is removed, also when the target carries nothing of the bundle; the target-variant
// files and registration targets that already exist are replaced.
func planOpenCodeHookChanges(in transformPlanInput, bundle domain.HookBundle, hb PlannedHookBundle, plan *TransformPlan) {
	ref := domain.ArtifactRef{Kind: domain.ArtifactHook, Key: bundle.Key}
	tgtFiles, tgtRegs := map[string]bool{}, map[string]bool{}
	if hb.Action == PlanWriteCreate || hb.Action == PlanWriteOverwrite {
		tgtFiles, tgtRegs = hookPlanWritePaths(hb.Plan)
	}
	if srcPlan, err := in.Source.HookPlan(domain.HookPlanRequest{Bundle: bundle, Scope: domain.ScopeProject}); err == nil && srcPlan.Supported {
		srcFiles, srcRegs := hookPlanWritePaths(srcPlan)
		for _, p := range sortedKeys(srcFiles) {
			if !tgtFiles[p] && fileExists(in.Workspace, p) {
				plan.Removals = append(plan.Removals, domain.Removal{Ref: ref, Path: p,
					Reason: "the source harness plugin file is not part of the target harness plugin"})
			}
		}
		for _, p := range sortedKeys(srcRegs) {
			if !tgtRegs[p] && fileExists(in.Workspace, p) {
				plan.Removals = append(plan.Removals, domain.Removal{Ref: ref, Path: p,
					Reason: "the source harness registration file is not written for the target harness"})
			}
		}
	}
	for _, p := range sortedKeys(tgtFiles) {
		if fileExists(in.Workspace, p) {
			plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: p,
				Reason: "the existing plugin file is replaced by the target harness version"})
		}
	}
	for _, p := range sortedKeys(tgtRegs) {
		if fileExists(in.Workspace, p) {
			plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: p,
				Reason: "the registration file is replaced by the target harness version, even if edited"})
		}
	}
}

// refuseBothTargets refuses a pair transform on a workspace that holds both targets of its pair.
func (r *transformRun) refuseBothTargets() error {
	if err := r.refuseBothGhcpTargets(); err != nil {
		return err
	}
	return r.refuseBothOpenCodeTargets()
}
