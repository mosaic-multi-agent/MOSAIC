package app

// ghcp_agent_legacy_paths.go relates a legacy X.md agent file to the X.agent.md file that
// replaces it, for the id-based lookup of deployed agents.

import (
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
	state := probeDeployedArtifact(workspace, target, modelKey)
	if state.Present {
		return state
	}
	if legacy := recordedLegacyPath(module, snap, key, target); legacy != "" {
		return probeDeployedArtifact(workspace, legacy, modelKey)
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

// applyRecordedLegacyState replaces the absent state of each planned agent whose legacy X.md the
// manifest records with the state of that legacy file.
func applyRecordedLegacyState(
	workspace string, module domain.HarnessModule, snap manifest.Snapshot,
	paths plan.PlannedPaths, state map[string]domain.DeployedArtifactState,
) {
	for _, pp := range paths {
		if pp.Ref.Kind != domain.ArtifactAgent || state[pp.TargetPath].Present {
			continue
		}
		state[pp.TargetPath] = probeAgentWithRecordedLegacy(workspace, module, snap, pp.Ref.Key, pp.TargetPath)
	}
}
