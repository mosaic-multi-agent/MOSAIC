package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"mosaic-deploy/internal/domain"
)

// State describes the result of reading the manifest file.
// Each value is distinct so callers can tell "file missing" from "file empty" from
// "file corrupt" from "file written by a newer tool" (CD-8, AC14.2).
type State string

const (
	// StatePresent means the manifest file was found, read, and parsed without error.
	StatePresent State = "present"
	// StateAbsent means no manifest file exists at the expected path.
	StateAbsent State = "absent"
	// StateEmpty means the file exists but contains no content (zero bytes or whitespace only).
	StateEmpty State = "empty"
	// StateCorrupt means the file exists but cannot be parsed as a valid manifest.
	StateCorrupt State = "corrupt"
	// StateFuture means the manifest was written by a newer schema version that this build
	// does not know how to interpret.
	StateFuture State = "future-schema"
)

// Snapshot is the result of Store.Load. The five state values are always distinct;
// a missing or unreadable manifest is never silently returned as an empty record set
// (CD-8, AC14.2).
type Snapshot struct {
	// State is one of the five State constants and is always set.
	State State
	// Manifest is populated only when State == StatePresent.
	// For all other states it is the zero value of domain.Manifest.
	Manifest domain.Manifest
	// Path is the absolute path of the manifest file (or where it would be created if absent).
	Path string
	// Err carries the underlying error when State is StateCorrupt or StateFuture.
	// It is nil for all other states.
	Err error
	// MigratedFrom is LegacySchemaVersion when Load upgraded the file in memory; "" otherwise.
	MigratedFrom string
	// MigrationDropped lists entries the upgrade could not attribute to a harness, in file
	// order, each with Reason domain.DropReasonUnattributed. Nil when nothing was dropped.
	MigrationDropped []domain.DroppedEntry
	// DamagedHooks lists the hook bundles that reconciliation kept although some of their files
	// are missing. Nil unless the snapshot was reconciled.
	DamagedHooks []DamagedBundle
}

// ForHarness returns a copy of s whose Manifest is s.Manifest.ForHarness(harnessID); the
// damaged hook bundles are narrowed to that harness.
func (s Snapshot) ForHarness(harnessID string) Snapshot {
	s.Manifest = s.Manifest.ForHarness(harnessID)
	var damaged []DamagedBundle
	for _, d := range s.DamagedHooks {
		if d.Harness == harnessID {
			damaged = append(damaged, d)
		}
	}
	s.DamagedHooks = damaged
	return s
}

// Store reads and writes the deployment manifest stored at
// <workspaceRoot>/.mosaic/manifest.yaml.
//
// Provenance: each entry may record the catalog (id, name, version) its source came from, and
// the manifest records the catalogs that supplied the protocol and the bundle. These fields
// are additive within schema "1": they are optional (omitted when unknown), the loader
// ignores nothing it needs and treats absent keys as unknown, so manifests written before
// provenance existed load as present and older builds can still read newer manifests.
// The top-level catalog_stack field (the persisted catalog stack) follows the same decision.
//
// Workflow provenance is not recorded here. The manifest has no workflow entries: workflows
// are embedded as regions in the deployed orchestrator file, so the orchestrator entry records
// that file's source catalog. Per-workflow provenance is a property of each resolved item in
// the catalog stack, not of a manifest entry.
type Store interface {
	// Load reads the manifest for the given workspace root. It always returns a Snapshot;
	// only unexpected I/O errors (e.g. permission denied on an existing file) result in a
	// returned error. Missing, empty, corrupt, and future-schema manifests all produce a
	// Snapshot with a distinct State and a nil error.
	Load(workspaceRoot string) (Snapshot, error)

	// Save writes m to <workspaceRoot>/.mosaic/manifest.yaml, creating the .mosaic
	// directory if it does not exist.
	Save(workspaceRoot string, m domain.Manifest) error
}

// SchemaVersion is the manifest schema version written by this build into every manifest file.
const SchemaVersion = "2"

// LegacySchemaVersion is the single-harness schema that this build upgrades on load.
const LegacySchemaVersion = "1"

// Dir, FileName, and BackupDir are the fixed relative locations of the manifest and its
// backup folder within a workspace root.
const (
	Dir       = ".mosaic"
	FileName  = "manifest.yaml"
	BackupDir = "backups"
)

// ---------------------------------------------------------------------------
// YAML wire types
// ---------------------------------------------------------------------------

// yamlManifest is the YAML-serializable form of domain.Manifest. Timestamps are
// stored as RFC3339 strings for portability and human-readability.
//
// HarnessID, UpdatedAt, ProtocolSource and BundleSource exist only in the old single-harness
// schema; they are read for the upgrade and never written.
type yamlManifest struct {
	SchemaVersion  string               `yaml:"schema_version"`
	HarnessID      string               `yaml:"harness_id,omitempty"`
	UpdatedAt      string               `yaml:"updated_at,omitempty"`
	Harnesses      []yamlHarnessSection `yaml:"harnesses,omitempty"`
	Entries        []yamlManifestEntry  `yaml:"entries"`
	ToolVersion    string               `yaml:"tool_version,omitempty"`
	ProtocolSource *yamlCatalog         `yaml:"protocol_source,omitempty"`
	BundleSource   *yamlCatalog         `yaml:"bundle_source,omitempty"`
	CatalogStack   []yamlStackLayer     `yaml:"catalog_stack,omitempty"`
}

// yamlHarnessSection is the wire form of domain.HarnessSection.
type yamlHarnessSection struct {
	HarnessID      string       `yaml:"harness_id"`
	UpdatedAt      string       `yaml:"updated_at"`
	ProtocolSource *yamlCatalog `yaml:"protocol_source,omitempty"`
	BundleSource   *yamlCatalog `yaml:"bundle_source,omitempty"`
}

// yamlStackLayer is the wire form of domain.PersistedCatalogLayer.
type yamlStackLayer struct {
	Path      string `yaml:"path,omitempty"`
	CatalogID string `yaml:"catalog_id,omitempty"`
	Default   bool   `yaml:"default,omitempty"`
}

func toYAMLStack(layers []domain.PersistedCatalogLayer) []yamlStackLayer {
	var out []yamlStackLayer
	for _, l := range layers {
		out = append(out, yamlStackLayer{Path: l.Path, CatalogID: l.CatalogID, Default: l.Default})
	}
	return out
}

func fromYAMLStack(layers []yamlStackLayer) []domain.PersistedCatalogLayer {
	var out []domain.PersistedCatalogLayer
	for _, l := range layers {
		out = append(out, domain.PersistedCatalogLayer{Path: l.Path, CatalogID: l.CatalogID, Default: l.Default})
	}
	return out
}

// yamlCatalog is the wire form of domain.CatalogIdentity. Versions are strings so values
// such as 1.10 are not reformatted.
type yamlCatalog struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version"`
}

func toYAMLCatalog(c domain.CatalogIdentity) *yamlCatalog {
	if c == (domain.CatalogIdentity{}) {
		return nil
	}
	return &yamlCatalog{ID: c.ID, Name: c.Name, Version: c.Version}
}

func fromYAMLCatalog(c *yamlCatalog) domain.CatalogIdentity {
	if c == nil {
		return domain.CatalogIdentity{}
	}
	return domain.CatalogIdentity{ID: c.ID, Name: c.Name, Version: c.Version}
}

type yamlManifestEntry struct {
	Harness                       string          `yaml:"harness,omitempty"`
	Ref                           yamlArtifactRef `yaml:"ref"`
	TargetPath                    string          `yaml:"target_path"`
	Version                       string          `yaml:"version"`
	HarnessVersion                string          `yaml:"harness_version"`
	InjectionsVersion             string          `yaml:"injections_version"`
	OrchestratorInjectionsVersion string          `yaml:"orchestrator_injections_version,omitempty"`
	ContentHash                   string          `yaml:"content_hash"`
	OwnedContent                  *yamlOwned      `yaml:"owned_content,omitempty"`
	DeployedAt                    string          `yaml:"deployed_at"`
	Catalog                       *yamlCatalog    `yaml:"catalog,omitempty"`
}

// yamlOwned is the wire form of a Deploy-owned content record.
type yamlOwned struct {
	Hash    string           `yaml:"hash"`
	Regions []yamlRegionHash `yaml:"regions,omitempty"`
}

type yamlRegionHash struct {
	Name string `yaml:"name"`
	Hash string `yaml:"hash"`
}

func toYAMLOwned(o *domain.OwnedContent) *yamlOwned {
	if o == nil {
		return nil
	}
	out := &yamlOwned{Hash: o.Hash}
	for _, r := range o.Regions {
		out.Regions = append(out.Regions, yamlRegionHash{Name: r.Name, Hash: r.Hash})
	}
	return out
}

func fromYAMLOwned(o *yamlOwned) *domain.OwnedContent {
	if o == nil {
		return nil
	}
	out := &domain.OwnedContent{Hash: o.Hash}
	for _, r := range o.Regions {
		out.Regions = append(out.Regions, domain.RegionHash{Name: r.Name, Hash: r.Hash})
	}
	return out
}

type yamlArtifactRef struct {
	Kind string `yaml:"kind"`
	Key  string `yaml:"key"`
}

// ---------------------------------------------------------------------------
// store implementation
// ---------------------------------------------------------------------------

type store struct {
	opts storeOptions
}

// NewStore returns the production Store implementation.
func NewStore(opts ...StoreOption) Store {
	s := &store{}
	for _, o := range opts {
		o(&s.opts)
	}
	return s
}

func (s *store) manifestPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, Dir, FileName)
}

// Load reads the manifest for the given workspace root. Abnormal manifest states
// (absent, empty, corrupt, future-schema) are communicated via Snapshot.State and
// never result in a returned error. Only genuine I/O failures (e.g. permission denied
// on an existing file) return a non-nil error.
func (s *store) Load(workspaceRoot string) (Snapshot, error) {
	path := s.manifestPath(workspaceRoot)
	snap := Snapshot{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			snap.State = StateAbsent
			return snap, nil
		}
		// Unexpected I/O error — propagate as a real error.
		return Snapshot{}, err
	}

	if strings.TrimSpace(string(data)) == "" {
		snap.State = StateEmpty
		return snap, nil
	}

	var raw yamlManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		snap.State = StateCorrupt
		snap.Err = fmt.Errorf("parse error: %w", err)
		return snap, nil
	}

	switch raw.SchemaVersion {
	case LegacySchemaVersion:
		return s.upgradeLegacy(snap, raw), nil
	case SchemaVersion:
		return loadCurrent(snap, raw), nil
	}
	snap.State = StateFuture
	snap.Err = fmt.Errorf("manifest schema_version %q is not recognised by this build (supports %q and %q)", raw.SchemaVersion, SchemaVersion, LegacySchemaVersion)
	return snap, nil
}

// loadCurrent converts a manifest of the current schema into the domain model.
func loadCurrent(snap Snapshot, raw yamlManifest) Snapshot {
	m := domain.Manifest{
		SchemaVersion: raw.SchemaVersion,
		ToolVersion:   raw.ToolVersion,
		CatalogStack:  fromYAMLStack(raw.CatalogStack),
	}
	for _, h := range raw.Harnesses {
		updatedAt, err := time.Parse(time.RFC3339, h.UpdatedAt)
		if err != nil {
			return corruptSnapshot(snap, fmt.Errorf("harness %q: invalid updated_at %q: %w", h.HarnessID, h.UpdatedAt, err))
		}
		m.Harnesses = append(m.Harnesses, domain.HarnessSection{
			HarnessID:      h.HarnessID,
			UpdatedAt:      updatedAt,
			ProtocolSource: fromYAMLCatalog(h.ProtocolSource),
			BundleSource:   fromYAMLCatalog(h.BundleSource),
		})
	}
	for _, e := range raw.Entries {
		if e.Harness == "" {
			return corruptSnapshot(snap, fmt.Errorf("entry %q records no harness", e.TargetPath))
		}
		entry, err := entryFromYAML(e)
		if err != nil {
			return corruptSnapshot(snap, err)
		}
		entry.Harness = e.Harness
		m.Entries = append(m.Entries, entry)
	}
	sortManifest(&m)
	snap.State = StatePresent
	snap.Manifest = m
	return snap
}

func corruptSnapshot(snap Snapshot, err error) Snapshot {
	snap.State = StateCorrupt
	snap.Err = err
	return snap
}

// entryFromYAML converts one wire entry; the harness is not set.
func entryFromYAML(e yamlManifestEntry) (domain.ManifestEntry, error) {
	deployedAt, err := time.Parse(time.RFC3339, e.DeployedAt)
	if err != nil {
		return domain.ManifestEntry{}, fmt.Errorf("entry %q: invalid deployed_at %q: %w", e.TargetPath, e.DeployedAt, err)
	}
	return domain.ManifestEntry{
		Ref:                           domain.ArtifactRef{Kind: domain.ArtifactKind(e.Ref.Kind), Key: e.Ref.Key},
		TargetPath:                    e.TargetPath,
		Version:                       e.Version,
		HarnessVersion:                e.HarnessVersion,
		InjectionsVersion:             e.InjectionsVersion,
		OrchestratorInjectionsVersion: e.OrchestratorInjectionsVersion,
		ContentHash:                   e.ContentHash,
		Owned:                         fromYAMLOwned(e.OwnedContent),
		DeployedAt:                    deployedAt,
		Catalog:                       fromYAMLCatalog(e.Catalog),
	}, nil
}

// sortManifest enforces the sorted invariants regardless of the on-disk order: entries by
// target path then harness, sections by harness id.
func sortManifest(m *domain.Manifest) {
	sort.SliceStable(m.Entries, func(i, j int) bool {
		if m.Entries[i].TargetPath != m.Entries[j].TargetPath {
			return m.Entries[i].TargetPath < m.Entries[j].TargetPath
		}
		return m.Entries[i].Harness < m.Entries[j].Harness
	})
	sort.SliceStable(m.Harnesses, func(i, j int) bool {
		return m.Harnesses[i].HarnessID < m.Harnesses[j].HarnessID
	})
}

// Save writes m to <workspaceRoot>/.mosaic/manifest.yaml in the current schema, creating the
// .mosaic directory if it does not exist. A second Save call on the same workspace replaces
// the previous manifest file entirely. A section is written for every harness that has
// entries and for no other.
func (s *store) Save(workspaceRoot string, m domain.Manifest) error {
	dir := filepath.Join(workspaceRoot, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	raw := yamlManifest{
		SchemaVersion: SchemaVersion,
		ToolVersion:   m.ToolVersion,
		CatalogStack:  toYAMLStack(m.CatalogStack),
	}
	for _, id := range m.HarnessIDs() {
		section, _ := m.Section(id)
		raw.Harnesses = append(raw.Harnesses, yamlHarnessSection{
			HarnessID:      id,
			UpdatedAt:      section.UpdatedAt.UTC().Format(time.RFC3339),
			ProtocolSource: toYAMLCatalog(section.ProtocolSource),
			BundleSource:   toYAMLCatalog(section.BundleSource),
		})
	}

	for _, e := range m.Entries {
		raw.Entries = append(raw.Entries, yamlManifestEntry{
			Harness:                       e.Harness,
			Ref:                           yamlArtifactRef{Kind: string(e.Ref.Kind), Key: e.Ref.Key},
			TargetPath:                    e.TargetPath,
			Version:                       e.Version,
			HarnessVersion:                e.HarnessVersion,
			InjectionsVersion:             e.InjectionsVersion,
			OrchestratorInjectionsVersion: e.OrchestratorInjectionsVersion,
			ContentHash:                   e.ContentHash,
			OwnedContent:                  toYAMLOwned(e.Owned),
			DeployedAt:                    e.DeployedAt.UTC().Format(time.RFC3339),
			Catalog:                       toYAMLCatalog(e.Catalog),
		})
	}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
