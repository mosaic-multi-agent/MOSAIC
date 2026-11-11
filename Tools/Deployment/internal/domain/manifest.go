package domain

import (
	"sort"
	"strings"
	"time"
)

// VersionStamp carries the source version fields for one deployed artifact.
// It is supplied per-item by the caller (typically the app layer) so the executor
// can record current source versions in ManifestEntry, enabling the planner to
// detect staleness on subsequent runs.
type VersionStamp struct {
	Version                       string // catalog version of the artifact (agents, skills, hooks)
	HarnessVersion                string // transform engine version (agents only); written as mosaic_harness_version
	InjectionsVersion             string // role-selected injection version (agents only)
	OrchestratorInjectionsVersion string // reserved; no longer populated after Stage 3
	ToolMappingsVersion           string // hash of the effective tool-destination mapping set; agents only; empty when no config mappings
	Catalog                       CatalogIdentity // source catalog of the item; zero = unknown
}

// Manifest is the deployment tool's bookkeeping record for every artifact it has written to
// a workspace. It records harness identity, known-entry listing, the content hash for each
// entry (used for local-modification conflict detection), and absent/corrupt-state detection.
// It is not the source of truth for version comparison — deployed file versions are read
// directly from the deployed files by the app layer (DeployedArtifactState).
type Manifest struct {
	SchemaVersion string
	ToolVersion   string           // version of the mosaic-deploy binary that wrote this manifest; empty means not recorded
	Harnesses     []HarnessSection // one per harness that has entries; sorted by HarnessID
	Entries       []ManifestEntry  // sorted by TargetPath, then Harness
	// CatalogStack is the catalog stack persisted with the deployment, highest layer first;
	// nil = none.
	CatalogStack []PersistedCatalogLayer
}

// HarnessSection holds the values that describe one harness's deployment.
type HarnessSection struct {
	HarnessID string
	UpdatedAt time.Time // time of the last run that wrote this harness
	// ProtocolSource and BundleSource are the catalogs that supplied the protocol and the
	// deployed-sections bundle of the last full run for this harness; zero = unknown.
	ProtocolSource CatalogIdentity
	BundleSource   CatalogIdentity
}

// DropReasonUnattributed is the reason shown for an old-format entry whose harness cannot be
// told from its path.
const DropReasonUnattributed = "recorded before per-harness manifests and its harness cannot be told from its path; the file is untouched and now counts as unrecorded"

// DropReasonMissing is the reason shown for an entry whose recorded files are all missing.
const DropReasonMissing = "file no longer exists"

// EmptyContentHash is manifest.Hash(nil). Bundle-level hook entries written before per-file
// hook entries carry it.
const EmptyContentHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// DroppedEntry is one manifest entry removed before planning, with the reason shown to the user.
type DroppedEntry struct {
	Entry  ManifestEntry // as recorded; Entry.Harness may be "" for an upgrade drop
	Reason string
}

// HarnessIDs returns the distinct Entry.Harness values, sorted: the set of harnesses the
// workspace holds.
func (m Manifest) HarnessIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, e := range m.Entries {
		if !seen[e.Harness] {
			seen[e.Harness] = true
			ids = append(ids, e.Harness)
		}
	}
	sort.Strings(ids)
	return ids
}

// Holds reports whether at least one entry belongs to harnessID.
func (m Manifest) Holds(harnessID string) bool {
	for _, e := range m.Entries {
		if e.Harness == harnessID {
			return true
		}
	}
	return false
}

// Section returns the section of harnessID; ok is false when there is none.
func (m Manifest) Section(harnessID string) (HarnessSection, bool) {
	for _, s := range m.Harnesses {
		if s.HarnessID == harnessID {
			return s, true
		}
	}
	return HarnessSection{}, false
}

// ForHarness returns a copy holding only the entries and the section of harnessID; every other
// field is copied.
func (m Manifest) ForHarness(harnessID string) Manifest {
	out := m
	out.Harnesses = nil
	out.Entries = nil
	for _, s := range m.Harnesses {
		if s.HarnessID == harnessID {
			out.Harnesses = append(out.Harnesses, s)
		}
	}
	for _, e := range m.Entries {
		if e.Harness == harnessID {
			out.Entries = append(out.Entries, e)
		}
	}
	return out
}

// ManifestEntry records one deployed artifact: its identity, the path it was written to,
// the version stamps at the time of deployment, and a content hash for conflict detection.
type ManifestEntry struct {
	Harness                       string    // id of the one harness this entry belongs to; never empty in a loaded or saved manifest
	Ref                          ArtifactRef
	TargetPath                    string    // relative to the deployment root
	Version                       string    // agents, skills, hook bundles
	HarnessVersion                string    // agents only; written as harness_version in manifest YAML
	InjectionsVersion             string    // agents only; role-selected injection version
	OrchestratorInjectionsVersion string    // reserved; no longer populated after Stage 3
	ToolMappingsVersion           string    // agents only; hash of the effective tool-destination mapping set at deploy time
	ContentHash                   string    // "sha256:<hex>" over the exact deployed bytes (CD-11)
	DeployedAt                    time.Time
	Catalog                       CatalogIdentity // source catalog of the deployed file; zero = unknown
	Owned                         *OwnedContent   // Deploy-owned content record; nil = recorded before this record existed (no edit check)
}

// Lookup finds the manifest entry for one artifact. The second return value is false when
// no entry for the given ref exists. Callers outside the planner (for example the executor's
// entry map) use this method; see LookupAt for the target-path-aware variant used by the
// planner's classifiers.
func (m Manifest) Lookup(ref ArtifactRef) (ManifestEntry, bool) {
	for _, e := range m.Entries {
		if e.Ref == ref {
			return e, true
		}
	}
	return ManifestEntry{}, false
}

// LookupAt finds the manifest entry for one artifact recorded at the given target path.
// It returns false when no entry matches both the ref and the path — including the case
// where an entry exists for the same Kind+Key but was deployed under a different harness
// (and therefore a different target path).
//
// Paths are compared after normalising backslashes to forward slashes; the comparison is
// otherwise exact and case-sensitive.
func (m Manifest) LookupAt(ref ArtifactRef, targetPath string) (ManifestEntry, bool) {
	normTarget := strings.ReplaceAll(targetPath, "\\", "/")
	for _, e := range m.Entries {
		if e.Ref == ref && strings.ReplaceAll(e.TargetPath, "\\", "/") == normTarget {
			return e, true
		}
	}
	return ManifestEntry{}, false
}
