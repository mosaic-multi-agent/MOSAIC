package manifest

import (
	"fmt"
	"strings"
	"time"

	"mosaic-deploy/internal/domain"
)

// pathPrefixHarnesses are the fixed directory prefixes whose harness can be told from the path
// alone.
var pathPrefixHarnesses = []struct{ prefix, harness string }{
	{".claude/", "claude-code"},
	{".opencode/", "opencode"},
}

// upgradeLegacy converts an old single-harness manifest into the current model in memory.
// Entries under .claude/ and .opencode/ take their harness from the path; any other entry
// takes the old top-level harness when that harness deploys to the entry's path, and is
// dropped (and reported) otherwise. Nothing is written.
func (s *store) upgradeLegacy(snap Snapshot, raw yamlManifest) Snapshot {
	updatedAt, err := time.Parse(time.RFC3339, raw.UpdatedAt)
	if err != nil {
		return corruptSnapshot(snap, fmt.Errorf("invalid updated_at %q: %w", raw.UpdatedAt, err))
	}

	m := domain.Manifest{
		SchemaVersion: SchemaVersion,
		ToolVersion:   raw.ToolVersion,
		CatalogStack:  fromYAMLStack(raw.CatalogStack),
	}
	var dropped []domain.DroppedEntry
	for _, e := range raw.Entries {
		entry, err := entryFromYAML(e)
		if err != nil {
			return corruptSnapshot(snap, err)
		}
		harness := s.legacyHarnessOf(entry.TargetPath, raw.HarnessID)
		if harness == "" {
			dropped = append(dropped, domain.DroppedEntry{Entry: entry, Reason: domain.DropReasonUnattributed})
			continue
		}
		entry.Harness = harness
		m.Entries = append(m.Entries, entry)
	}

	for _, id := range m.HarnessIDs() {
		section := domain.HarnessSection{HarnessID: id, UpdatedAt: updatedAt}
		if id == raw.HarnessID {
			section.ProtocolSource = fromYAMLCatalog(raw.ProtocolSource)
			section.BundleSource = fromYAMLCatalog(raw.BundleSource)
		}
		m.Harnesses = append(m.Harnesses, section)
	}
	sortManifest(&m)

	snap.State = StatePresent
	snap.Manifest = m
	snap.MigratedFrom = LegacySchemaVersion
	snap.MigrationDropped = dropped
	return snap
}

// legacyHarnessOf returns the harness an old-format entry at targetPath belongs to, or "" when
// it cannot be attributed.
func (s *store) legacyHarnessOf(targetPath, oldHarnessID string) string {
	path := strings.ReplaceAll(targetPath, "\\", "/")
	for _, p := range pathPrefixHarnesses {
		if strings.HasPrefix(path, p.prefix) {
			return p.harness
		}
	}
	if oldHarnessID == "" {
		return ""
	}
	for _, dir := range s.opts.harnessPaths[oldHarnessID] {
		if path == dir || strings.HasPrefix(path, dir+"/") {
			return oldHarnessID
		}
	}
	return ""
}
