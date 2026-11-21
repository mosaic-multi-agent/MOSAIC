package app_test

// recorded_harnesses_test.go verifies StackService.RecordedHarnesses: it reports the harness
// IDs the raw manifest at the given path records, nothing for an unreadable manifest, nothing
// for a path without a manifest of its own, and it never modifies the manifest file.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func newRecordedHarnessesService() app.Service {
	return app.New(app.Deps{Manifest: manifest.NewStore()})
}

// rhEntry builds a manifest entry of harness whose deployed file does not exist on disk.
func rhEntry(harness, key string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:     harness,
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
		TargetPath:  filepath.ToSlash(filepath.Join("."+harness, "agents", key+".md")),
		Version:     "1.0.0",
		ContentHash: "sha256:00",
		DeployedAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

// rhSaveManifest saves a schema-current manifest with the given entries into a new workspace.
func rhSaveManifest(t *testing.T, entries ...domain.ManifestEntry) string {
	t.Helper()
	ws := t.TempDir()
	m := domain.Manifest{SchemaVersion: manifest.SchemaVersion, Entries: entries}
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.Harness] {
			seen[e.Harness] = true
			m.Harnesses = append(m.Harnesses, domain.HarnessSection{
				HarnessID: e.Harness,
				UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			})
		}
	}
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	return ws
}

// rhWriteRawManifest writes raw bytes as the manifest file of a new workspace.
func rhWriteRawManifest(t *testing.T, content string) string {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, manifest.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return ws
}

func rhManifestBytes(t *testing.T, ws string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return b
}

func TestRecordedHarnesses_TwoHarnesses_ReportsExactlyThoseSorted(t *testing.T) {
	// Arrange
	ws := rhSaveManifest(t, rhEntry("opencode", "a"), rhEntry("claude-code", "b"), rhEntry("claude-code", "c"))
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(ws)

	// Assert
	want := []string{"claude-code", "opencode"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RecordedHarnesses = %v, want %v", got, want)
	}
}

func TestRecordedHarnesses_DeployedFilesDeleted_HarnessStillReported(t *testing.T) {
	// Arrange: the entries point at files that do not exist in the workspace.
	ws := rhSaveManifest(t, rhEntry("claude-code", "gone"))
	if _, err := os.Stat(filepath.Join(ws, ".claude-code", "agents", "gone.md")); err == nil {
		t.Fatal("fixture error: deployed file unexpectedly exists")
	}
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(ws)

	// Assert
	if !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Errorf("RecordedHarnesses = %v, want [claude-code]", got)
	}
}

func TestRecordedHarnesses_UnreadableOrMissingManifest_ReportsNone(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"missing":       func(t *testing.T) string { return t.TempDir() },
		"empty":         func(t *testing.T) string { return rhWriteRawManifest(t, "") },
		"corrupt":       func(t *testing.T) string { return rhWriteRawManifest(t, "schema_version: [unclosed\n  - : {{{\n") },
		"future-schema": func(t *testing.T) string { return rhWriteRawManifest(t, futureSchemaManifest) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			ws := build(t)
			svc := newRecordedHarnessesService()

			// Act
			got := svc.RecordedHarnesses(ws)

			// Assert
			if len(got) != 0 {
				t.Errorf("RecordedHarnesses = %v, want none", got)
			}
		})
	}
}

// futureSchemaManifest is a manifest from a newer tool whose entries must not be reported.
const futureSchemaManifest = `schema_version: "99"
entries:
  - harness: claude-code
    ref:
      kind: agent
      key: future
    target_path: .claude-code/agents/future.md
    version: 1.0.0
    content_hash: sha256:00
    deployed_at: 2026-01-02T03:04:05Z
`

func TestRecordedHarnesses_SubfolderWithoutOwnManifest_ReportsNoneEvenWhenAncestorHasOne(t *testing.T) {
	// Arrange
	ws := rhSaveManifest(t, rhEntry("claude-code", "a"))
	sub := filepath.Join(ws, "nested", "project")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(sub)

	// Assert
	if len(got) != 0 {
		t.Errorf("RecordedHarnesses(subfolder) = %v, want none", got)
	}
}

// schemaOneManifest is an old single-harness manifest that the manifest store upgrades in memory.
const schemaOneManifest = `schema_version: "1"
harness_id: claude-code
updated_at: "2024-06-15T12:30:00Z"
entries:
  - ref:
      kind: agent
      key: legacy
    target_path: .claude/agents/legacy.md
    version: "1.0.0"
    content_hash: sha256:00
    deployed_at: "2024-06-15T12:00:00Z"
`

func TestRecordedHarnesses_SchemaOneManifest_ReportsItsHarness(t *testing.T) {
	// Arrange
	ws := rhWriteRawManifest(t, schemaOneManifest)
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(ws)

	// Assert
	if !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Errorf("RecordedHarnesses = %v, want [claude-code]", got)
	}
}

func TestRecordedHarnesses_SchemaOneManifest_FileLeftUntouched(t *testing.T) {
	// Arrange
	ws := rhWriteRawManifest(t, schemaOneManifest)
	before := rhManifestBytes(t, ws)
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(ws)

	// Assert: the in-memory upgrade is never saved, and the harness was really read.
	if len(got) == 0 {
		t.Fatal("RecordedHarnesses reported nothing for the schema-1 manifest")
	}
	if after := rhManifestBytes(t, ws); !bytes.Equal(before, after) {
		t.Errorf("schema-1 manifest was rewritten:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if _, err := os.Stat(filepath.Join(ws, manifest.Dir, manifest.BackupDir)); err == nil {
		t.Error("a backup folder was created")
	}
}

func TestRecordedHarnesses_SectionWithoutEntries_NotReported(t *testing.T) {
	// Arrange: a harness section exists for opencode, but no entry belongs to it.
	ws := t.TempDir()
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Entries:       []domain.ManifestEntry{rhEntry("claude-code", "a")},
		Harnesses: []domain.HarnessSection{
			{HarnessID: "claude-code", UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
			{HarnessID: "opencode", UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		},
	}
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	svc := newRecordedHarnessesService()

	// Act
	got := svc.RecordedHarnesses(ws)

	// Assert
	if !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Errorf("RecordedHarnesses = %v, want [claude-code]", got)
	}
}

func TestRecordedHarnesses_NeverModifiesManifest(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"current-schema": func(t *testing.T) string { return rhSaveManifest(t, rhEntry("claude-code", "a")) },
		"corrupt":        func(t *testing.T) string { return rhWriteRawManifest(t, "schema_version: [unclosed\n") },
		"future-schema":  func(t *testing.T) string { return rhWriteRawManifest(t, futureSchemaManifest) },
		"empty":          func(t *testing.T) string { return rhWriteRawManifest(t, "") },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			ws := build(t)
			before := rhManifestBytes(t, ws)
			svc := newRecordedHarnessesService()

			// Act
			svc.RecordedHarnesses(ws)

			// Assert
			if after := rhManifestBytes(t, ws); !bytes.Equal(before, after) {
				t.Errorf("manifest bytes changed:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			if _, err := os.Stat(filepath.Join(ws, manifest.Dir, manifest.BackupDir)); err == nil {
				t.Error("a backup folder was created")
			}
		})
	}
}
