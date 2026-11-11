package manifest_test

// Tests for loading old-format (single-harness) manifests: each entry gets its harness from
// its path or from the old top-level harness id, entries that cannot be attributed are
// reported as dropped, and loading never writes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// migrationHarnessPaths is the path knowledge the production wiring gives the shared store.
var migrationHarnessPaths = manifest.HarnessPaths{
	"claude-code": {".claude/agents", ".claude/skills", ".claude/hooks"},
	"opencode":    {".opencode/agents", ".opencode/skills", ".opencode/plugins"},
	"ghcp-cli":    {".github/agents", ".github/skills", ".github/hooks"},
	"vscode-ghcp": {".github/agents", ".github/skills", ".github/hooks"},
}

// loadOldFixture places the named testdata fixture in a fresh workspace and loads it through a
// store built with opts. It returns the snapshot, the workspace and the bytes placed on disk.
func loadOldFixture(t *testing.T, fixture string, opts ...manifest.StoreOption) (manifest.Snapshot, string, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, data)

	snap, err := manifest.NewStore(opts...).Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return snap, ws, data
}

// harnessOfPath returns the harness a loaded manifest assigns to the entry at targetPath, and
// whether such an entry exists.
func harnessOfPath(m domain.Manifest, targetPath string) (string, bool) {
	for _, e := range m.Entries {
		if e.TargetPath == targetPath {
			return e.Harness, true
		}
	}
	return "", false
}

func TestStoreMigration_OldFormatLoadsAsPresentUpgradedInMemory(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q (err %v), want present", snap.State, snap.Err)
	}
	if snap.MigratedFrom != manifest.LegacySchemaVersion {
		t.Errorf("MigratedFrom = %q, want %q", snap.MigratedFrom, manifest.LegacySchemaVersion)
	}
	if snap.Manifest.SchemaVersion != "2" {
		t.Errorf("Manifest.SchemaVersion = %q, want the new schema %q", snap.Manifest.SchemaVersion, "2")
	}
}

func TestStoreMigration_NewSchemaFileIsNotReportedAsMigrated(t *testing.T) {
	store := manifest.NewStore()
	ws := makeWorkspace(t)
	m := sampleManifest()
	m.SchemaVersion = "2"
	must(t, store.Save(ws, m))

	snap, err := store.Load(ws)

	must(t, err)
	if snap.MigratedFrom != "" || snap.MigrationDropped != nil {
		t.Errorf("MigratedFrom = %q, MigrationDropped = %+v, want both empty for a new-schema file",
			snap.MigratedFrom, snap.MigrationDropped)
	}
}

func TestStoreMigration_OpenCodePathsAreInferredFromThePath(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	cases := []struct{ path, want string }{
		{".opencode/agents/codebase-research.md", "opencode"},
		{".opencode/skills/lean-tdd/SKILL.md", "opencode"},
	}
	for _, c := range cases {
		got, ok := harnessOfPath(snap.Manifest, c.path)
		if !ok || got != c.want {
			t.Errorf("harness of %s = %q (found %v), want %q", c.path, got, ok, c.want)
		}
	}
}

func TestStoreMigration_PathPrefixWinsOverTheOldHarnessID(t *testing.T) {
	// The old harness id is opencode, yet the .claude/ entry belongs to claude-code.
	snap, _, _ := loadOldFixture(t, "old_format_non_ghcp_harness_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	got, ok := harnessOfPath(snap.Manifest, ".claude/skills/lean-tdd/SKILL.md")

	if !ok || got != "claude-code" {
		t.Errorf("harness of the .claude/ entry = %q (found %v), want claude-code", got, ok)
	}
}

func TestStoreMigration_GithubEntriesTakeTheOldHarnessIDWhenItDeploysThere(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	for _, p := range []string{
		".github/agents/codebase-research.agent.md",
		".github/agents/reviewer.agent.md",
		".github/hooks", // equals one of the harness's directories
	} {
		got, ok := harnessOfPath(snap.Manifest, p)
		if !ok || got != "ghcp-cli" {
			t.Errorf("harness of %s = %q (found %v), want ghcp-cli", p, got, ok)
		}
	}
}

func TestStoreMigration_OldHarnessIDOfTheOtherGhcpTargetIsHonoured(t *testing.T) {
	// Same file, but the descriptor knowledge says only vscode-ghcp deploys to .github.
	paths := manifest.HarnessPaths{
		"opencode":    migrationHarnessPaths["opencode"],
		"vscode-ghcp": migrationHarnessPaths["vscode-ghcp"],
	}
	data, err := os.ReadFile(filepath.Join("testdata", "old_format_opencode_and_ghcp_manifest.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(strings.Replace(string(data), "harness_id: ghcp-cli", "harness_id: vscode-ghcp", 1)))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(paths)).Load(ws)

	must(t, err)
	got, ok := harnessOfPath(snap.Manifest, ".github/agents/reviewer.agent.md")
	if !ok || got != "vscode-ghcp" {
		t.Errorf("harness of the .github entry = %q (found %v), want vscode-ghcp", got, ok)
	}
}

func TestStoreMigration_OneLoadKeepsTheGithubEntryAndDropsTheStrayEntry(t *testing.T) {
	// Only the path condition separates the two entries: both share the old harness id.
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(oldManifestText("harness_id: ghcp-cli",
		oldEntryText("agent", "a", ".github/agents/a.agent.md"),
		oldEntryText("agent", "b", "docs/b.md"))))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

	must(t, err)
	if got, ok := harnessOfPath(snap.Manifest, ".github/agents/a.agent.md"); !ok || got != "ghcp-cli" {
		t.Errorf("harness of the .github entry = %q (found %v), want ghcp-cli", got, ok)
	}
	if _, ok := harnessOfPath(snap.Manifest, "docs/b.md"); ok {
		t.Errorf("the docs/ entry was kept; no harness deploys there")
	}
	if len(snap.MigrationDropped) != 1 || snap.MigrationDropped[0].Entry.TargetPath != "docs/b.md" {
		t.Errorf("MigrationDropped = %+v, want only the docs/ entry", snap.MigrationDropped)
	}
}

func TestStoreMigration_EntryUnderAPathNoHarnessDeploysToIsDropped(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	if _, ok := harnessOfPath(snap.Manifest, "docs/stray/SKILL.md"); ok {
		t.Errorf("the docs/ entry is still in the manifest; it must be dropped")
	}
	if len(snap.MigrationDropped) != 1 {
		t.Fatalf("MigrationDropped = %+v, want exactly the docs/ entry", snap.MigrationDropped)
	}
	d := snap.MigrationDropped[0]
	if d.Entry.TargetPath != "docs/stray/SKILL.md" || d.Entry.Ref.Key != "stray" {
		t.Errorf("dropped entry = %+v, want the docs/stray entry as recorded", d.Entry)
	}
	if d.Reason != domain.DropReasonUnattributed {
		t.Errorf("dropped reason = %q, want %q", d.Reason, domain.DropReasonUnattributed)
	}
	if d.Entry.Harness != "" {
		t.Errorf("dropped entry harness = %q, want it empty (never attributed)", d.Entry.Harness)
	}
}

func TestStoreMigration_GithubEntriesAreDroppedWhenTheOldHarnessIsNotGhcp(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_non_ghcp_harness_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	if _, ok := harnessOfPath(snap.Manifest, ".github/agents/reviewer.agent.md"); ok {
		t.Errorf("the .github entry of an opencode-only manifest must not be attributed to any harness")
	}
	if len(snap.MigrationDropped) != 1 ||
		snap.MigrationDropped[0].Entry.TargetPath != ".github/agents/reviewer.agent.md" {
		t.Fatalf("MigrationDropped = %+v, want exactly the .github entry", snap.MigrationDropped)
	}
	if got, ok := harnessOfPath(snap.Manifest, ".opencode/agents/codebase-research.md"); !ok || got != "opencode" {
		t.Errorf("the opencode entry lost its harness: %q (found %v)", got, ok)
	}
}

func TestStoreMigration_DroppedEntriesAreListedInFileOrder(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(oldManifestText("harness_id: opencode",
		oldEntryText("agent", "b", "zeta/b.md"),
		oldEntryText("agent", "a", ".opencode/agents/a.md"),
		oldEntryText("agent", "c", "alpha/c.md"))))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

	must(t, err)
	if len(snap.MigrationDropped) != 2 ||
		snap.MigrationDropped[0].Entry.TargetPath != "zeta/b.md" ||
		snap.MigrationDropped[1].Entry.TargetPath != "alpha/c.md" {
		t.Errorf("MigrationDropped = %+v, want zeta/b.md then alpha/c.md (file order)", snap.MigrationDropped)
	}
}

func TestStoreMigration_NothingDroppedLeavesTheDroppedListNil(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(oldManifestText("harness_id: claude-code",
		oldEntryText("agent", "a", ".claude/agents/a.md"))))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

	must(t, err)
	if snap.MigrationDropped != nil {
		t.Errorf("MigrationDropped = %+v, want nil when nothing was dropped", snap.MigrationDropped)
	}
	if snap.MigratedFrom != "1" {
		t.Errorf("MigratedFrom = %q, want 1", snap.MigratedFrom)
	}
}

func TestStoreMigration_EmptyOldHarnessIDDropsEveryNonPrefixEntry(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(oldManifestText(`harness_id: ""`,
		oldEntryText("agent", "a", ".github/agents/a.agent.md"),
		oldEntryText("agent", "b", ".claude/agents/b.md"))))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

	must(t, err)
	if _, ok := harnessOfPath(snap.Manifest, ".github/agents/a.agent.md"); ok {
		t.Errorf("an entry outside the fixed prefixes was kept although the old manifest names no harness")
	}
	if got, ok := harnessOfPath(snap.Manifest, ".claude/agents/b.md"); !ok || got != "claude-code" {
		t.Errorf("the .claude/ entry has harness %q (found %v), want claude-code", got, ok)
	}
	if len(snap.MigrationDropped) != 1 {
		t.Errorf("MigrationDropped = %+v, want only the .github entry", snap.MigrationDropped)
	}
}

func TestStoreMigration_WithoutPathKnowledgeOnlyTheFixedPrefixesAreAttributed(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml")

	if got, ok := harnessOfPath(snap.Manifest, ".opencode/agents/codebase-research.md"); !ok || got != "opencode" {
		t.Errorf("the .opencode/ entry has harness %q (found %v), want opencode", got, ok)
	}
	if _, ok := harnessOfPath(snap.Manifest, ".github/agents/reviewer.agent.md"); ok {
		t.Errorf("a .github entry was attributed although the store has no path knowledge")
	}
	if len(snap.MigrationDropped) != 4 {
		t.Errorf("MigrationDropped has %d entries, want 4 (three .github entries and the docs/ entry): %+v",
			len(snap.MigrationDropped), snap.MigrationDropped)
	}
}

func TestStoreMigration_EntryFieldsSurviveTheUpgrade(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	e, ok := snap.Manifest.LookupAt(
		domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "codebase-research"},
		".opencode/agents/codebase-research.md")

	if !ok {
		t.Fatalf("opencode agent entry not found")
	}
	if e.Version != "1.0.0" || e.HarnessVersion != "2.0.0" || e.InjectionsVersion != "3.0.0" ||
		e.ContentHash != "sha256:1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("entry fields changed by the upgrade: %+v", e)
	}
	if e.Catalog != (domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}) {
		t.Errorf("entry Catalog = %+v, want the recorded supplying catalog", e.Catalog)
	}
	wantAt := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	if !e.DeployedAt.Equal(wantAt) {
		t.Errorf("DeployedAt = %v, want %v", e.DeployedAt, wantAt)
	}
}

func TestStoreMigration_SameRefAtTwoPathsKeepsBothEntriesWithTheirOwnHarness(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))
	ref := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "codebase-research"}

	open, okOpen := snap.Manifest.LookupAt(ref, ".opencode/agents/codebase-research.md")
	ghcp, okGhcp := snap.Manifest.LookupAt(ref, ".github/agents/codebase-research.agent.md")

	if !okOpen || !okGhcp || open.Harness != "opencode" || ghcp.Harness != "ghcp-cli" {
		t.Errorf("entries = %+v (found %v) / %+v (found %v), want opencode and ghcp-cli",
			open, okOpen, ghcp, okGhcp)
	}
}

func TestStoreMigration_SectionsExistOnlyForHarnessesThatReceivedEntries(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_non_ghcp_harness_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	var ids []string
	for _, s := range snap.Manifest.Harnesses {
		ids = append(ids, s.HarnessID)
	}
	if len(ids) != 2 || ids[0] != "claude-code" || ids[1] != "opencode" {
		t.Errorf("section harness ids = %v, want [claude-code opencode] sorted", ids)
	}
}

func TestStoreMigration_EverySectionGetsTheOldUpdatedAt(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))
	want := time.Date(2026, 11, 2, 9, 15, 0, 0, time.UTC)

	for _, id := range []string{"opencode", "ghcp-cli"} {
		s, ok := snap.Manifest.Section(id)
		if !ok || !s.UpdatedAt.Equal(want) {
			t.Errorf("section %s = %+v (found %v), want UpdatedAt %v", id, s, ok, want)
		}
	}
}

func TestStoreMigration_OnlyTheOldHarnessSectionGetsTheSources(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	ghcp, _ := snap.Manifest.Section("ghcp-cli")
	open, openOK := snap.Manifest.Section("opencode")

	if !openOK {
		t.Fatalf("no opencode section; opencode entries were attributed and must get one")
	}
	if ghcp.ProtocolSource != (domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default Catalog", Version: "0.7.0"}) ||
		ghcp.BundleSource != (domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}) {
		t.Errorf("ghcp-cli section sources = %+v / %+v, want the old top-level sources",
			ghcp.ProtocolSource, ghcp.BundleSource)
	}
	if open.ProtocolSource != (domain.CatalogIdentity{}) || open.BundleSource != (domain.CatalogIdentity{}) {
		t.Errorf("opencode section sources = %+v / %+v, want unknown", open.ProtocolSource, open.BundleSource)
	}
}

func TestStoreMigration_OldHarnessWithoutEntriesGetsNoSection(t *testing.T) {
	ws := makeWorkspace(t)
	writeManifestFile(t, ws, []byte(oldManifestText("harness_id: ghcp-cli",
		oldEntryText("agent", "a", ".opencode/agents/a.md"))))

	snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

	must(t, err)
	if _, ok := snap.Manifest.Section("ghcp-cli"); ok {
		t.Errorf("ghcp-cli has a section although no entry was attributed to it")
	}
}

func TestStoreMigration_ToolVersionAndCatalogStackStayTopLevel(t *testing.T) {
	snap, _, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	if snap.Manifest.ToolVersion != "1.6.0" {
		t.Errorf("ToolVersion = %q, want 1.6.0", snap.Manifest.ToolVersion)
	}
	stack := snap.Manifest.CatalogStack
	if len(stack) != 2 || stack[0].CatalogID != "user-cat" || !stack[1].Default {
		t.Errorf("CatalogStack = %+v, want the two recorded layers", stack)
	}
}

func TestStoreMigration_LoadNeverWrites(t *testing.T) {
	_, ws, original := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	after, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("Load rewrote the old-format manifest:\n%s", after)
	}
	entries, err := os.ReadDir(filepath.Join(ws, manifest.Dir))
	if err != nil || len(entries) != 1 {
		t.Errorf(".mosaic holds %d files (err %v), want only manifest.yaml", len(entries), err)
	}
}

func TestStoreMigration_SavingAMigratedManifestWritesTheNewSchema(t *testing.T) {
	store := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths))
	snap, ws, _ := loadOldFixture(t, "old_format_opencode_and_ghcp_manifest.yaml",
		manifest.WithHarnessPaths(migrationHarnessPaths))

	must(t, store.Save(ws, snap.Manifest))
	again, err := store.Load(ws)

	must(t, err)
	if again.MigratedFrom != "" {
		t.Errorf("MigratedFrom = %q after saving, want empty (file is in the new schema)", again.MigratedFrom)
	}
	if len(again.Manifest.Entries) != len(snap.Manifest.Entries) {
		t.Errorf("entries after save = %d, want %d", len(again.Manifest.Entries), len(snap.Manifest.Entries))
	}
	text := readManifestText(t, ws)
	if !strings.Contains(text, `schema_version: "2"`) {
		t.Errorf("saved manifest is not schema 2:\n%s", text)
	}
	if !strings.Contains(text, "harnesses:") {
		t.Errorf("saved manifest still has the single-harness shape:\n%s", text)
	}
}

func TestStoreMigration_UnknownSchemaIsStillFutureSchema(t *testing.T) {
	for _, version := range []string{"0", "3", "99"} {
		ws := makeWorkspace(t)
		writeManifestFile(t, ws, []byte("schema_version: \""+version+"\"\nharness_id: claude-code\nentries: []\n"))

		snap, err := manifest.NewStore(manifest.WithHarnessPaths(migrationHarnessPaths)).Load(ws)

		must(t, err)
		if snap.State != manifest.StateFuture || snap.Err == nil {
			t.Errorf("schema %q: State = %q, Err = %v, want future-schema with an error", version, snap.State, snap.Err)
		}
	}
}
