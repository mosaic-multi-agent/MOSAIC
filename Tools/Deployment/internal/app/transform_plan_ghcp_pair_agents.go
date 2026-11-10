package app

// transform_plan_ghcp_pair_agents.go plans the agents of the move between the two GHCP targets.
// Both targets name agents X.agent.md in the same folder, so only agents the manifest records as
// MOSAIC deployed are touched: a recorded X.agent.md is replaced in place, a recorded legacy X.md
// is migrated to X.agent.md (the old file is removed with its entry, an existing X.agent.md is
// replaced). Everything else in the folder is left alone.

import (
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// pairAgentGroup holds the files of one agent key in the source agents folder.
type pairAgentGroup struct {
	key    string
	files  []sourceAgentFile
	legacy *sourceAgentFile // the recorded legacy X.md file, when there is one
}

// pairAgentKey returns the agent key of a file name, matching the extensions case-insensitively.
func pairAgentKey(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, agentFileExtAgent):
		return name[:len(name)-len(agentFileExtAgent)]
	case strings.HasSuffix(lower, agentFileExtPlain):
		return name[:len(name)-len(agentFileExtPlain)]
	}
	return name
}

// isAgentExtName reports whether the file name ends in .agent.md, ignoring case.
func isAgentExtName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), agentFileExtAgent)
}

// recordedAgentPaths returns the manifest-recorded agent files by lower-cased slash path. An
// unreadable or unusable manifest records nothing.
func (s *service) recordedAgentPaths(workspace string) map[string]domain.ArtifactRef {
	recorded := map[string]domain.ArtifactRef{}
	snap, err := s.deps.Manifest.Load(workspace)
	if err != nil || snap.State != manifest.StatePresent {
		return recorded
	}
	for _, e := range snap.Manifest.Entries {
		if e.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		p := strings.ToLower(path.Clean(strings.ReplaceAll(e.TargetPath, "\\", "/")))
		if _, seen := recorded[p]; !seen {
			recorded[p] = e.Ref
		}
	}
	return recorded
}

// planPairAgents fills plan.Agents, plan.Removals, plan.Replacements and the agent notes for a
// GHCP pair transform.
func (s *service) planPairAgents(in transformPlanInput, relDir string, files []sourceAgentFile, plan *TransformPlan) error {
	recorded := s.recordedAgentPaths(in.Workspace)
	isRecorded := func(f sourceAgentFile) bool {
		_, ok := recorded[strings.ToLower(slashJoin(relDir, f.name))]
		return ok
	}

	groups := map[string]*pairAgentGroup{}
	for _, f := range files {
		k := pairAgentKey(f.name)
		g := groups[k]
		if g == nil {
			g = &pairAgentGroup{key: k}
			groups[k] = g
		}
		g.files = append(g.files, f)
		if !isAgentExtName(f.name) && g.legacy == nil && isRecorded(f) {
			fc := f
			g.legacy = &fc
		}
	}

	recognised := 0
	done := map[string]bool{}
	for _, f := range files {
		k := pairAgentKey(f.name)
		g := groups[k]
		if g.legacy != nil {
			if done[k] {
				continue
			}
			done[k] = true
			recognised += s.planPairMigration(in, relDir, g, isRecorded, recorded, plan)
			continue
		}
		recognised += s.planPairSingle(in, relDir, f, isRecorded(f), recorded, plan)
	}

	if recognised == 0 {
		plan.Notes = append(plan.Notes, TransformNote{
			Kind:    NoteNoAgentsFound,
			Message: "no MOSAIC agents found in " + plan.SourceAgentsDir,
			Paths:   []string{plan.SourceAgentsDir},
		})
	}
	plan.Notes = append(plan.Notes, bothFormsNotes(in.Workspace, plan.Agents, removedPaths(plan.Removals))...)
	return nil
}

// planPairSingle plans one file that is not part of a legacy migration. It returns 1 when the
// file is a recognised MOSAIC agent, otherwise 0.
func (s *service) planPairSingle(in transformPlanInput, relDir string, f sourceAgentFile, isRec bool,
	recorded map[string]domain.ArtifactRef, plan *TransformPlan) int {
	a := PlannedAgent{SourcePath: f.path, FileName: f.name}
	if !f.recognised {
		a.Action, a.Skip, a.Reason = PlanSkip, SkipNotAgent, "not a MOSAIC agent: "+f.reason
		plan.Agents = append(plan.Agents, a)
		return 0
	}
	a.Key, a.SourceModel = pairAgentKey(f.name), f.model
	if !isRec || !isAgentExtName(f.name) {
		a.Action, a.Skip = PlanSkip, SkipNotRecorded
		a.Reason = "the manifest does not record this file as a MOSAIC agent, so it is left unchanged"
		plan.Agents = append(plan.Agents, a)
		return 1
	}
	s.planPairWrite(in, &a)
	if a.Action == PlanWriteOverwrite {
		ref := recorded[strings.ToLower(slashJoin(relDir, f.name))]
		plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: ref, Path: filepath.ToSlash(a.DestinationPath),
			Reason: "the existing agent file is replaced by the target harness version"})
	}
	plan.Agents = append(plan.Agents, a)
	return 1
}

// planPairMigration plans one agent key that has a recorded legacy X.md file as a single row:
// the target version is written as X.agent.md, the legacy file is removed with its manifest
// entry and an existing X.agent.md is replaced. It returns 1 when the agent has a source.
func (s *service) planPairMigration(in transformPlanInput, relDir string, g *pairAgentGroup,
	isRecorded func(sourceAgentFile) bool, recorded map[string]domain.ArtifactRef, plan *TransformPlan) int {
	if !g.legacy.recognised {
		for _, f := range g.files {
			plan.Agents = append(plan.Agents, PlannedAgent{SourcePath: f.path, FileName: f.name, Action: PlanSkip,
				Skip: SkipNotAgent, Reason: "not a MOSAIC agent: " + g.legacy.reason})
		}
		return 0
	}
	src := *g.legacy
	for _, f := range g.files {
		if isAgentExtName(f.name) && f.recognised && isRecorded(f) {
			src = f
			break
		}
	}
	a := PlannedAgent{SourcePath: src.path, FileName: src.name, Key: g.key, SourceModel: src.model}
	s.planPairWrite(in, &a)
	if a.Action == PlanWriteOverwrite {
		plan.Replacements = append(plan.Replacements, domain.Replacement{Ref: recorded[strings.ToLower(slashJoin(relDir, g.legacy.name))],
			Path: filepath.ToSlash(a.DestinationPath), Reason: "the existing agent file is replaced by the migrated target harness version"})
	}
	if a.Action == PlanWriteOverwrite || a.Action == PlanWriteCreate {
		legacyPath := slashJoin(relDir, g.legacy.name)
		plan.Removals = append(plan.Removals, domain.Removal{Ref: recorded[strings.ToLower(legacyPath)], Path: legacyPath, DropEntry: true,
			Reason: "the legacy agent file is replaced by " + filepath.ToSlash(a.DestinationPath) + " for the target harness"})
	}
	plan.Agents = append(plan.Agents, a)
	return 1
}

// planPairWrite resolves the destination of a pair agent and plans it as a write. A pair agent
// is always written: an existing destination is replaced whatever the overwrite flag says.
func (s *service) planPairWrite(in transformPlanInput, a *PlannedAgent) {
	dest, err := in.Target.TargetPath(domain.TargetPathRequest{Kind: domain.ArtifactAgent, Key: a.Key, Scope: domain.ScopeProject})
	if err != nil {
		a.Action, a.Skip, a.Reason = PlanSkip, SkipUnsupported, "target harness cannot place this agent: "+err.Error()
		return
	}
	a.DestinationPath = dest
	if pathExists(filepath.Join(in.Workspace, filepath.FromSlash(dest))) {
		a.Action = PlanWriteOverwrite
	} else {
		a.Action = PlanWriteCreate
	}
}
