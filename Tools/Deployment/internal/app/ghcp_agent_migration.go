package app

// ghcp_agent_migration.go moves agents an earlier deploy recorded as X.md to X.agent.md for
// the GHCP targets. The deploy flows call it on the plan they built, before conflicts are
// decided: each agent the run deploys whose recorded X.md is present on disk gets a removal of
// that file (with its manifest entry). The agent item keeps the action, conflict, backup marker
// and reasons the planner gave it; the migration only adds its own reason, and lists an existing
// X.agent.md as a replacement only where the plan writes it over without a backup.

import (
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// migrateLegacyGhcpAgents adds the legacy X.md migration to p for the agents p deploys. It does
// nothing for other harnesses, an unreadable manifest, or agents whose recorded X.md is absent.
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
		if !ok || !pathExists(filepath.Join(p.WorkspacePath, filepath.FromSlash(entry.TargetPath))) {
			continue
		}
		p.Removals = append(p.Removals, domain.Removal{
			Ref: entry.Ref, Path: entry.TargetPath, DropEntry: true,
			Reason: "the legacy agent file is replaced by " + item.TargetPath,
		})
		planMigrationReplacement(p, item)
		if item.RenamedFrom == "" {
			// The migration reason joins the item's own reasons; it never replaces them. A
			// planner-detected rename already carries its own rename reason.
			item.AddReason("migrates from the legacy file " + legacyPath)
		}
	}
}

// planMigrationReplacement lists the existing X.agent.md as a replacement only when the plan
// writes it over without a backup of its own: an unchanged or conflicting agent keeps its
// planned state, and an edited one is backed up through its own marker.
func planMigrationReplacement(p *domain.Plan, item *domain.PlanItem) {
	if (item.Action != domain.ActionUpdate && item.Action != domain.ActionCreate) || item.BackupBeforeWrite || item.RenamedFrom != "" {
		return
	}
	if !pathExists(filepath.Join(p.WorkspacePath, filepath.FromSlash(item.TargetPath))) {
		return
	}
	p.Replacements = append(p.Replacements, domain.Replacement{
		Ref: item.Ref, Path: item.TargetPath,
		Reason: "the existing agent file is replaced by the migrated agent file",
	})
}

// legacyLookupKey identifies an agent file by key and case-folded slash path.
func legacyLookupKey(key, target string) string {
	return key + "|" + strings.ToLower(path.Clean(strings.ReplaceAll(target, "\\", "/")))
}
