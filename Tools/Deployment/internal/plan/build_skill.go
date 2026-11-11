package plan

// build_skill.go classifies one skill file into a PlanItem.

import (
	"mosaic-deploy/internal/domain"
)

// classifySkillItem classifies one file within a skill into a PlanItem with the appropriate
// action. isEntryFile controls whether version-based staleness is evaluated (entry files only).
func classifySkillItem(
	skill domain.Skill,
	sourcePath string,
	targetPath string,
	isEntryFile bool,
	manifestUsable bool,
	m domain.Manifest,
	deployed domain.DeployedArtifactState,
	supplier supplierLookup,
) domain.PlanItem {
	ref := domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: skill.Key}
	item := domain.PlanItem{
		Ref:        ref,
		SourcePath: sourcePath,
		TargetPath: targetPath,
	}

	// Step 1: No deployed file.
	if !deployed.Present {
		item.Action = domain.ActionCreate
		item.AddReason("new skill; no file at target path")
		return item
	}

	// Step 2: No usable manifest.
	if !manifestUsable {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("deployed skill file found but no manifest is available; cannot confirm origin")
		return item
	}

	// Step 3: Target-path-aware manifest lookup.
	entry, inManifest := m.LookupAt(ref, targetPath)
	if !inManifest {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("skill file exists on disk but has no manifest record for this target path; cannot confirm origin")
		return item
	}

	// Step 4: A local edit of the file (a skill file is Deploy-owned as a whole) makes it an
	// update with a backup, for every file of the skill.
	edited := markOwnedContentEdit(&item, entry, deployed)

	// Step 5: Staleness check — entry files only. Non-entry files have no version frontmatter
	// so SkillStaleness would always produce a spurious delta (deployed.Version == "").
	var deltas []domain.VersionDelta
	versionStale := false
	if isEntryFile {
		deltas = SkillStaleness(deployed, skill)
		versionStale = len(deltas) > 0 || !deployed.HasVersionInfo()
	}
	// Step 7: version staleness (entry files) or a changed supplying catalog (every file).
	catDrift := entryCatalogDrift(supplier, ref, entry)
	item.AddWarning(catDrift.Warning())
	if edited || versionStale || catDrift.Stale() {
		item.Action = domain.ActionUpdate
		if versionStale {
			addVersionDeltaReasons(&item, deltas)
			if !deployed.HasVersionInfo() {
				item.AddReason(noVersionInfoReason)
			}
		}
		item.Stale = withCatalogDrift(&item, catDrift, deltas)
		return item
	}

	// Step 8.
	item.Action = domain.ActionUnchanged
	return item
}
