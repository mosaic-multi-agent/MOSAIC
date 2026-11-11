package plan

// build_hook.go classifies one deployed hook file into a PlanItem. A hook bundle is planned as
// one item per file, as a skill is.

import (
	"path"
	"strings"

	"mosaic-deploy/internal/domain"
)

const (
	// hookNewFileReason is the reason of a hook file that has no file at its target path.
	hookNewFileReason = "new hook file; no file at target path"
	// hookDamagedReason is the reason of a missing hook file that the manifest still records.
	hookDamagedReason = "recorded hook file is missing; the bundle is completed"
	// hookLegacyReason is the reason of a hook file covered only by a bundle-level record.
	hookLegacyReason = "hook bundle was recorded before per-file records; the file is rewritten and recorded"
)

// hookFilePaths returns the workspace-relative target path of every file of the hook plan, in
// the plan's file order.
func hookFilePaths(hp domain.HookPlan) []string {
	dir := strings.ReplaceAll(hp.TargetDir, "\\", "/")
	paths := make([]string, 0, len(hp.Files))
	for _, f := range hp.Files {
		paths = append(paths, path.Join(dir, strings.ReplaceAll(f.TargetName, "\\", "/")))
	}
	return paths
}

// hasBundleLevelEntry reports whether the manifest holds a bundle-level record of the hook
// bundle, written before hook files were recorded one by one.
func hasBundleLevelEntry(m domain.Manifest, ref domain.ArtifactRef) bool {
	for _, e := range m.Entries {
		if e.Ref == ref && e.IsBundleLevelHook() {
			return true
		}
	}
	return false
}

// classifyHookItem classifies one file of a hook bundle into a PlanItem with the appropriate
// action. hook is the bundle; sourcePath and targetPath are the file's own paths.
func classifyHookItem(
	hook domain.HookBundle,
	sourcePath string,
	targetPath string,
	manifestUsable bool,
	m domain.Manifest,
	deployed domain.DeployedArtifactState,
	supplier supplierLookup,
) domain.PlanItem {
	ref := domain.ArtifactRef{Kind: domain.ArtifactHook, Key: hook.Key}
	item := domain.PlanItem{
		Ref:        ref,
		SourcePath: sourcePath,
		TargetPath: targetPath,
	}

	// Step 1: No deployed file. A file the manifest still records is a damaged bundle.
	if !deployed.Present {
		item.Action = domain.ActionCreate
		if _, recorded := m.LookupAt(ref, targetPath); manifestUsable && recorded {
			item.AddReason(hookDamagedReason)
		} else {
			item.AddReason(hookNewFileReason)
		}
		return item
	}

	// Step 2: No usable manifest.
	if !manifestUsable {
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("deployed hook found but no manifest is available; cannot confirm origin")
		return item
	}

	// Step 3: Target-path-aware manifest lookup. A bundle-level record covers the file without
	// an edit check; the file is rewritten and recorded individually.
	entry, inManifest := m.LookupAt(ref, targetPath)
	if !inManifest {
		if hasBundleLevelEntry(m, ref) {
			item.Action = domain.ActionUpdate
			item.AddReason(hookLegacyReason)
			return item
		}
		item.Action = domain.ActionConflict
		item.Conflict = &domain.LocalModification{
			CurrentHash:     deployed.ContentHash,
			ManifestMissing: true,
		}
		item.AddReason("hook file exists on disk but has no manifest record for this target path; cannot confirm origin")
		return item
	}

	// Step 4: A local edit of the file (a hook file is Deploy-owned as a whole) makes it an
	// update with a backup.
	edited := markOwnedContentEdit(&item, entry, deployed)

	// Step 5: Staleness uses the manifest-recorded version as the deployed version signal: a
	// hook file carries no in-file version marker, so deployed.Version is always empty.
	deltas := HookStalenessFromRecorded(entry.Version, hook.Version)
	catDrift := entryCatalogDrift(supplier, ref, entry)
	item.AddWarning(catDrift.Warning())
	if edited || len(deltas) > 0 || catDrift.Stale() {
		item.Action = domain.ActionUpdate
		addVersionDeltaReasons(&item, deltas)
		item.Stale = withCatalogDrift(&item, catDrift, deltas)
		return item
	}

	item.Action = domain.ActionUnchanged
	return item
}
