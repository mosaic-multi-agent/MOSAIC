package app

// transform_target_legacy.go applies the GHCP agent-name migration to a transform from a
// harness outside the GHCP pair into a GHCP target: an agent the transform writes or keeps as
// X.agent.md whose X.md the target recorded gets a removal of that file with its manifest
// entry, so no X.md entry of the target remains. The agent's own plan item is not changed.

import (
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// targetLegacyRemovals returns the removals of the recorded, present X.md files of the agents
// the plan writes or keeps as X.agent.md of the target. An agent whose build failed is not
// written, so its legacy file stays. It returns nil for a pair transform (planned separately)
// and for a target outside the GHCP pair.
func (r *transformRun) targetLegacyRemovals() []domain.Removal {
	if isGhcpPair(r.srcID, r.tgtID) || (r.tgtID != ghcpCLIHarnessID && r.tgtID != ghcpVSCodeHarnessID) {
		return nil
	}
	if r.targetManifest.State != manifest.StatePresent {
		return nil
	}
	recorded := map[string]domain.ManifestEntry{}
	for _, e := range r.targetManifest.Manifest.Entries {
		if e.Ref.Kind == domain.ArtifactAgent {
			recorded[legacyLookupKey(e.Ref.Key, e.TargetPath)] = e
		}
	}
	var out []domain.Removal
	for i, pa := range r.plan.Agents {
		dest := filepath.ToSlash(pa.DestinationPath)
		if !strings.HasSuffix(dest, agentFileExtAgent) || !r.agentWrittenOrKept(i, pa) {
			continue
		}
		legacy := strings.TrimSuffix(dest, agentFileExtAgent) + agentFileExtPlain
		entry, ok := recorded[legacyLookupKey(pa.Key, legacy)]
		if !ok || !pathExists(filepath.Join(r.workspace, filepath.FromSlash(entry.TargetPath))) {
			continue
		}
		out = append(out, domain.Removal{
			Ref: entry.Ref, Path: entry.TargetPath, DropEntry: true,
			Reason: "the legacy agent file is replaced by " + dest,
		})
	}
	return out
}

// agentWrittenOrKept reports whether agent i of the plan ends as an X.agent.md of the target:
// it is written and still active, or its destination exists and is kept unchanged.
func (r *transformRun) agentWrittenOrKept(i int, pa PlannedAgent) bool {
	if isWrite(pa) {
		return i < len(r.agents) && r.agents[i].active()
	}
	return pa.Action == PlanSkip && pa.Skip == SkipDestinationExists
}
