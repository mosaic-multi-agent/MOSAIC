package app

// ghcp_agent_migration.go moves agents an earlier deploy recorded as X.md to X.agent.md for
// the GHCP targets. The deploy flows call it on the plan they built, before conflicts are
// decided: each agent the run deploys whose X.md the manifest records gets a removal of that
// file (with its manifest entry), and an X.agent.md already on disk is planned as a replacement
// so the executor backs it up before the new one is written.

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// migrateLegacyGhcpAgents adds the legacy X.md migration to p for the agents p deploys. It does
// nothing for other harnesses, an unreadable manifest, or agents without a recorded X.md.
func migrateLegacyGhcpAgents(p *domain.Plan, snap manifest.Snapshot) {
	if p.Harness.ID != ghcpCLIHarnessID && p.Harness.ID != ghcpVSCodeHarnessID {
		return
	}
	if snap.State != manifest.StatePresent {
		return
	}
	recorded := map[string]domain.ManifestEntry{}
	for _, e := range snap.Manifest.Entries {
		if e.Ref.Kind == domain.ArtifactAgent {
			recorded[legacyLookupKey(e.Ref.Key, e.TargetPath)] = e
		}
	}
	for i := range p.Items {
		item := &p.Items[i]
		if item.Ref.Kind != domain.ArtifactAgent || !strings.HasSuffix(item.TargetPath, agentFileExtAgent) {
			continue
		}
		legacyPath := strings.TrimSuffix(item.TargetPath, agentFileExtAgent) + agentFileExtPlain
		entry, ok := recorded[legacyLookupKey(item.Ref.Key, legacyPath)]
		if !ok {
			continue
		}
		p.Removals = append(p.Removals, domain.Removal{
			Ref: entry.Ref, Path: entry.TargetPath, DropEntry: true,
			Reason: "the legacy agent file is replaced by " + item.TargetPath,
		})
		exists := false
		if _, err := os.Lstat(filepath.Join(p.WorkspacePath, filepath.FromSlash(item.TargetPath))); err == nil {
			exists = true
			p.Replacements = append(p.Replacements, domain.Replacement{
				Ref: item.Ref, Path: item.TargetPath,
				Reason: "the existing agent file is replaced by the migrated agent file",
			})
		}
		forceAgentWrite(item, exists)
	}
}

// forceAgentWrite makes a migrated agent item a write whatever its state was: the legacy file
// is removed, so the agent must not be skipped as unchanged or decided as a conflict. An
// existing X.agent.md is replaced after backup.
func forceAgentWrite(item *domain.PlanItem, exists bool) {
	if item.RenamedFrom != "" {
		return // the planner already planned the rename as an update and gave its reason
	}
	if item.Action == domain.ActionCreate || (item.Action == domain.ActionUpdate && exists) {
		return
	}
	item.Conflict = nil
	if exists {
		item.Action = domain.ActionUpdate
		item.Reason = "replaces the existing agent file after backup"
		return
	}
	item.Action = domain.ActionCreate
	item.Reason = "replaces the legacy agent file"
}

// legacyLookupKey identifies an agent file by key and case-folded slash path.
func legacyLookupKey(key, target string) string {
	return key + "|" + strings.ToLower(path.Clean(strings.ReplaceAll(target, "\\", "/")))
}
