package plan

// build_edit.go detects local edits of Deploy-owned content by comparing the content record
// stored in the manifest with the record recomputed from the file on disk.

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// editReason is the reason of an item whose Deploy-owned content differs from what Deploy
// recorded when it last wrote the file.
const editReason = "Deploy-owned content was edited locally"

// markOwnedContentEdit checks the deployed file's Deploy-owned content against the manifest
// entry's record. When they differ it adds the edit reason (naming the differing regions where
// they are known) and marks the item for a backup before it is overwritten; it reports whether
// an edit was found.
//
// No check is made for an entry without a record (written before records existed) or when the
// record could not be recomputed from disk.
func markOwnedContentEdit(item *domain.PlanItem, entry domain.ManifestEntry, deployed domain.DeployedArtifactState) bool {
	if entry.Owned == nil || deployed.Owned == nil || entry.Owned.Hash == deployed.Owned.Hash {
		return false
	}
	reason := editReason
	if regions := transform.DiffRegions(*entry.Owned, *deployed.Owned); len(regions) > 0 {
		reason += " (regions: " + strings.Join(regions, ", ") + ")"
	}
	item.AddReason(reason)
	item.BackupBeforeWrite = true
	return true
}
