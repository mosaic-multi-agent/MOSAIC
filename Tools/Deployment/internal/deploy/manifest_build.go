package deploy

// manifest_build.go holds the helpers that turn a prior manifest and a run's results into the
// manifest the run writes. Entries are keyed per harness: a run changes only the entries and
// the section of its own harness and carries every other harness's data unchanged.

import (
	"sort"
	"strings"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// entryKey is the composite key used by buildEntryMap and buildManifest.
// Keying by (Harness, Ref, Target) ensures each file in a multi-file skill retains its own
// manifest entry, and that two harnesses sharing a path keep independent entries.
type entryKey struct {
	Harness string
	Ref     domain.ArtifactRef
	Target  string // normalised target path (forward slashes)
}

// runHarnessID returns the harness a run belongs to: the plan's harness, or, when the plan
// names none, the harness of a prior manifest that holds exactly one.
func runHarnessID(plan domain.Plan, prior domain.Manifest) string {
	if plan.Harness.ID != "" {
		return plan.Harness.ID
	}
	if ids := prior.HarnessIDs(); len(ids) == 1 {
		return ids[0]
	}
	return ""
}

// newManifestEntry builds a ManifestEntry for a successfully deployed item.
// stamp carries the source version fields for this deployment so the planner can detect
// staleness on subsequent runs.
func newManifestEntry(item domain.PlanItem, content []byte, stamp domain.VersionStamp, agentOwned func(domain.PlanItem, []byte) *domain.OwnedContent) domain.ManifestEntry {
	return domain.ManifestEntry{
		Owned:                         ownedOfEntry(item, content, agentOwned),
		Ref:                           item.Ref,
		TargetPath:                    item.TargetPath,
		Version:                       stamp.Version,
		HarnessVersion:                stamp.HarnessVersion,
		InjectionsVersion:             stamp.InjectionsVersion,
		OrchestratorInjectionsVersion: stamp.OrchestratorInjectionsVersion,
		ToolMappingsVersion:           stamp.ToolMappingsVersion,
		ContentHash:                   manifest.Hash(content),
		DeployedAt:                    time.Now(),
		Catalog:                       stamp.Catalog,
	}
}

// simulatedEntry builds the manifest entry a dry run reports for a created or updated item, so
// the previewed manifest reflects the run's changes. It returns nil for any other action or
// when the item's content cannot be rendered.
func simulatedEntry(item domain.PlanItem, req ExecRequest) *domain.ManifestEntry {
	if item.Action != domain.ActionCreate && item.Action != domain.ActionUpdate {
		return nil
	}
	content, err := itemContent(item, req)
	if err != nil {
		return nil
	}
	entry := newManifestEntry(item, content, resolveVersionStamp(item, req.VersionStamps), req.AgentOwnedContent)
	return &entry
}

// resolveVersionStamp derives the version stamp for one plan item.
// For ActionUpdate items, stale field deltas supply authoritative source values — they override
// the corresponding VersionStamps map entries. For all other actions, stamps come entirely from
// the VersionStamps map (a nil map or absent key yields zero-value stamps).
func resolveVersionStamp(item domain.PlanItem, stamps map[string]domain.VersionStamp) domain.VersionStamp {
	stamp := stamps[item.TargetPath] // zero value when stamps is nil or key absent
	// ActionUpdate: item.Stale lists every changed field with its new source value. These are
	// authoritative for the stale fields; non-stale fields retain their value from the map.
	if item.Action == domain.ActionUpdate {
		for _, delta := range item.Stale {
			switch delta.Field {
			case "version":
				stamp.Version = delta.Source
			case "harness_version":
				stamp.HarnessVersion = delta.Source
			case "injections_version":
				stamp.InjectionsVersion = delta.Source
			case "orchestrator_injections_version":
				stamp.OrchestratorInjectionsVersion = delta.Source
			case "tool_mappings_version":
				stamp.ToolMappingsVersion = delta.Source
			}
		}
	}
	return stamp
}

// buildEntryMap constructs a mutable map of manifest entries keyed by the composite
// (Harness, ArtifactRef, normalised TargetPath). The executor modifies this map during
// execution and converts it back to a Manifest via buildManifest.
//
// Target paths are normalised (backslashes to forward slashes) to match LookupAt semantics.
// Using a composite key ensures each file in a multi-file skill retains its own manifest
// entry and is not silently overwritten by a later file with the same ArtifactRef.
func buildEntryMap(m domain.Manifest) map[entryKey]domain.ManifestEntry {
	out := make(map[entryKey]domain.ManifestEntry, len(m.Entries))
	for _, e := range m.Entries {
		out[keyOf(e.Harness, e.Ref, e.TargetPath)] = e
	}
	return out
}

// keyOf builds the entry map key of one entry.
func keyOf(harness string, ref domain.ArtifactRef, targetPath string) entryKey {
	return entryKey{Harness: harness, Ref: ref, Target: strings.ReplaceAll(targetPath, "\\", "/")}
}

// buildManifest produces the final domain.Manifest from the entry map, sorted by TargetPath
// then harness. Sections of other harnesses are carried from prior; the run harness's section
// is filled from meta.
func buildManifest(prior domain.Manifest, entryMap map[entryKey]domain.ManifestEntry, plan domain.Plan, toolVersion string, meta ManifestMeta) domain.Manifest {
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		ToolVersion:   toolVersion,
	}
	applyManifestMeta(&m, prior, runHarnessID(plan, prior), meta)

	keys := make([]entryKey, 0, len(entryMap))
	for k := range entryMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := entryMap[keys[i]], entryMap[keys[j]]
		if a.TargetPath != b.TargetPath {
			return a.TargetPath < b.TargetPath
		}
		return a.Harness < b.Harness
	})
	for _, k := range keys {
		m.Entries = append(m.Entries, entryMap[k])
	}
	return m
}
