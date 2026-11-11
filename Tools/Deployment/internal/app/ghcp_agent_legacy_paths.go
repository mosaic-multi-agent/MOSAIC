package app

// ghcp_agent_legacy_paths.go relates a legacy X.md agent file to the X.agent.md file that
// replaces it, for the id-based lookup of deployed agents.

import (
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

// legacySiblingOf returns the X.md path that is the legacy form of the X.agent.md path p, or
// "" when p is not an X.agent.md path.
func legacySiblingOf(p string) string {
	if !strings.HasSuffix(p, agentFileExtAgent) {
		return ""
	}
	return strings.TrimSuffix(p, agentFileExtAgent) + agentFileExtPlain
}

// isLegacySiblingPath reports whether resolved is the legacy X.md form of the planned X.agent.md.
func isLegacySiblingPath(resolved, planned string) bool {
	legacy := legacySiblingOf(strings.ReplaceAll(planned, "\\", "/"))
	return legacy != "" && strings.EqualFold(strings.ReplaceAll(resolved, "\\", "/"), legacy)
}

// dropLegacySiblings removes from entries each X.md that has its X.agent.md sibling in entries,
// so an agent held in both forms resolves to the X.agent.md alone.
func dropLegacySiblings(entries []DeployedAgentEntry) []DeployedAgentEntry {
	if len(entries) < 2 {
		return entries
	}
	kept := make([]DeployedAgentEntry, 0, len(entries))
	for _, e := range entries {
		legacy := false
		for _, other := range entries {
			if isLegacySiblingPath(e.TargetPath, other.TargetPath) {
				legacy = true
				break
			}
		}
		if !legacy {
			kept = append(kept, e)
		}
	}
	return kept
}

// probeAgentWithRecordedLegacy probes the deployed agent at target. When nothing is there on a
// GHCP target and the manifest records the agent's legacy X.md, the legacy file is probed
// instead, so the agent counts as deployed while it awaits migration.
func probeAgentWithRecordedLegacy(
	workspace string, module domain.HarnessModule, snap manifest.Snapshot, key, target string,
) domain.DeployedArtifactState {
	modelKey := module.Descriptor().Frontmatter.ModelKey
	state := probeDeployedArtifactOf(workspace, target, modelKey, domain.ArtifactAgent)
	if state.Present {
		return state
	}
	if pathExists(filepath.Join(workspace, filepath.FromSlash(target))) {
		return state // something is at the target path (not a readable agent file): not a rename
	}
	if legacy := recordedLegacyPath(module, snap, key, target); legacy != "" {
		return probeDeployedArtifactOf(workspace, legacy, modelKey, domain.ArtifactAgent)
	}
	return state
}

// recordedLegacyPath returns the X.md path the manifest records for the agent whose planned
// X.agent.md path is target, or "" when there is none or the harness is not a GHCP target.
func recordedLegacyPath(module domain.HarnessModule, snap manifest.Snapshot, key, target string) string {
	if id := module.Ref().ID; id != ghcpCLIHarnessID && id != ghcpVSCodeHarnessID {
		return ""
	}
	legacy := legacySiblingOf(strings.ReplaceAll(target, "\\", "/"))
	if legacy == "" || snap.State != manifest.StatePresent {
		return ""
	}
	want := legacyLookupKey(key, legacy)
	for _, e := range snap.Manifest.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && legacyLookupKey(e.Ref.Key, e.TargetPath) == want {
			return e.TargetPath
		}
	}
	return ""
}

// applyRecordedLegacyAgents finds the migrating agents among the planned paths: agents whose
// X.agent.md is absent while the manifest records their X.md and that file is present. The
// deployed state of each is replaced by the state of its X.md. It returns planned target path
// -> recorded legacy path for every migrating agent (also those whose state was seeded from the
// legacy file), or nil when there is none. An X.md the manifest does not record is never used,
// and an agent whose X.agent.md exists is not migrating. It works from planned paths, so agents
// with and without a numeric id are treated alike.
func applyRecordedLegacyAgents(
	workspace string, module domain.HarnessModule, snap manifest.Snapshot,
	paths plan.PlannedPaths, state map[string]domain.DeployedArtifactState,
) map[string]string {
	modelKey := module.Descriptor().Frontmatter.ModelKey
	var migrating map[string]string
	for _, pp := range paths {
		if pp.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		legacy := recordedLegacyPath(module, snap, pp.Ref.Key, pp.TargetPath)
		if legacy == "" || pathExists(filepath.Join(workspace, filepath.FromSlash(pp.TargetPath))) {
			continue
		}
		legacyState := probeDeployedArtifactOf(workspace, legacy, modelKey, domain.ArtifactAgent)
		if !legacyState.Present {
			continue
		}
		if cur := state[pp.TargetPath]; cur.Present && cur.ContentHash != legacyState.ContentHash {
			continue // resolved to another file of the same agent (renamed by the user)
		}
		state[pp.TargetPath] = legacyState
		if migrating == nil {
			migrating = map[string]string{}
		}
		migrating[pp.TargetPath] = legacy
	}
	return migrating
}

// splitMigratingAgents separates agents whose planned path is a key of migrating from the rest.
func splitMigratingAgents(
	agents []domain.Agent, paths plan.PlannedPaths, migrating map[string]string,
) (others, moved []domain.Agent) {
	for _, a := range agents {
		target, _ := paths.Path(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: a.Key})
		if _, ok := migrating[target]; ok {
			moved = append(moved, a)
		} else {
			others = append(others, a)
		}
	}
	return others, moved
}

// resolveMigratingModels records, without asking, the model the run supplies for each migrating
// agent: an agent pre-answer, else the tier mapping in tiers. An agent with neither stays
// unresolved, so the model of its legacy file is kept afterwards.
func resolveMigratingModels(
	models map[string]domain.ModelSelection, moved []domain.Agent, module domain.HarnessModule,
	agentModels map[string]string, tiers map[domain.Tier]string,
) {
	for _, a := range moved {
		if id := agentModels[a.Key]; id != "" {
			origin := domain.OriginCustom
			for _, known := range module.Descriptor().Models.IDs {
				if known == id {
					origin = domain.OriginHarnessList
					break
				}
			}
			models[a.Key] = domain.ModelSelection{ModelID: id, Origin: origin}
		} else if id := tiers[a.RecommendedTier]; id != "" {
			models[a.Key] = domain.ModelSelection{ModelID: id, Origin: domain.OriginTierMapping}
		}
	}
}
