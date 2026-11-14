package manifest_test

// reconcile_test.go specifies manifest reconciliation: which entries survive a comparison of
// the manifest with the disk. Reconcile decides presence only, for every harness, and is driven
// here through a presence function so no files are needed.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// present returns a presence function that reports exactly the given paths as files.
func present(paths ...string) func(string) bool {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rcEntry(harness string, kind domain.ArtifactKind, key, path string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:     harness,
		Ref:         domain.ArtifactRef{Kind: kind, Key: key},
		TargetPath:  path,
		Version:     "1.0.0",
		ContentHash: manifest.Hash([]byte(path)),
	}
}

// rcBundleLevelHook is a hook entry recorded before per-file hook entries: one entry per
// bundle whose path does not name a file and whose content hash is the empty-content hash.
func rcBundleLevelHook(harness, key, path string) domain.ManifestEntry {
	e := rcEntry(harness, domain.ArtifactHook, key, path)
	e.ContentHash = manifest.Hash(nil)
	return e
}

func rcManifest(entries ...domain.ManifestEntry) domain.Manifest {
	seen := map[string]bool{}
	var sections []domain.HarnessSection
	for _, e := range entries {
		if !seen[e.Harness] {
			seen[e.Harness] = true
			sections = append(sections, domain.HarnessSection{
				HarnessID: e.Harness, UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
		}
	}
	return domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		ToolVersion:   "1.7.0",
		Harnesses:     sections,
		Entries:       entries,
	}
}

func rcPaths(m domain.Manifest) []string {
	var out []string
	for _, e := range m.Entries {
		out = append(out, e.TargetPath)
	}
	return out
}

func rcDroppedPaths(r manifest.Reconciliation) []string {
	var out []string
	for _, d := range r.Dropped {
		out = append(out, d.Entry.TargetPath)
	}
	return out
}

func TestReconcile_AllFilesPresent_KeepsEveryEntryAndDropsNothing(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("opencode", domain.ArtifactSkill, "s", ".opencode/skills/s/SKILL.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/agents/a.md", ".opencode/skills/s/SKILL.md"))

	if len(r.Dropped) != 0 {
		t.Errorf("Dropped = %+v, want none", r.Dropped)
	}
	if !reflect.DeepEqual(r.Manifest, m) {
		t.Errorf("manifest changed although every file is present:\n got %+v\nwant %+v", r.Manifest, m)
	}
}

func TestReconcile_AgentFileMissing_IsDroppedWithTheMissingReason(t *testing.T) {
	gone := rcEntry("ghcp-cli", domain.ArtifactAgent, "x", ".github/agents/X.md")
	kept := rcEntry("ghcp-cli", domain.ArtifactAgent, "y", ".github/agents/y.agent.md")

	r := manifest.Reconcile(rcManifest(gone, kept), present(".github/agents/y.agent.md"))

	if got := rcPaths(r.Manifest); !reflect.DeepEqual(got, []string{".github/agents/y.agent.md"}) {
		t.Errorf("remaining entries = %v, want only the present agent", got)
	}
	if len(r.Dropped) != 1 {
		t.Fatalf("Dropped = %+v, want exactly the missing agent", r.Dropped)
	}
	if r.Dropped[0].Entry != gone {
		t.Errorf("dropped entry = %+v, want the entry as recorded %+v", r.Dropped[0].Entry, gone)
	}
	if r.Dropped[0].Reason != domain.DropReasonMissing {
		t.Errorf("reason = %q, want %q", r.Dropped[0].Reason, domain.DropReasonMissing)
	}
}

func TestReconcile_SkillFileMissing_IsDroppedPerFile(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactSkill, "s", ".opencode/skills/s/SKILL.md"),
		rcEntry("opencode", domain.ArtifactSkill, "s", ".opencode/skills/s/refs/one.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/skills/s/SKILL.md"))

	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, []string{".opencode/skills/s/refs/one.md"}) {
		t.Errorf("dropped = %v, want only the missing skill file", got)
	}
	if got := rcPaths(r.Manifest); !reflect.DeepEqual(got, []string{".opencode/skills/s/SKILL.md"}) {
		t.Errorf("remaining = %v, want only the present skill file", got)
	}
}

func TestReconcile_DroppedEntriesFollowInputOrder(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactAgent, "z", ".opencode/agents/z.md"),
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("opencode", domain.ArtifactAgent, "m", ".opencode/agents/m.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/agents/a.md"))

	want := []string{".opencode/agents/z.md", ".opencode/agents/m.md"}
	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, want) {
		t.Errorf("dropped order = %v, want input order %v", got, want)
	}
}

func TestReconcile_HookBundleWithSomeFilesMissing_IsKeptWhole(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.ps1"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
	)

	r := manifest.Reconcile(m, present(".github/hooks/logger.json"))

	if len(r.Dropped) != 0 {
		t.Errorf("Dropped = %+v, want none: a damaged bundle is kept", r.Dropped)
	}
	if got := rcPaths(r.Manifest); len(got) != 3 {
		t.Errorf("remaining = %v, want all three entries of the damaged bundle", got)
	}
}

func TestReconcile_HookBundleWithEveryFileMissing_IsDroppedWhole(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.sh"),
		rcEntry("ghcp-cli", domain.ArtifactAgent, "x", ".github/agents/x.agent.md"),
	)

	r := manifest.Reconcile(m, present(".github/agents/x.agent.md"))

	want := []string{".github/hooks/logger.json", ".github/hooks/logger.sh"}
	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, want) {
		t.Errorf("dropped = %v, want both hook files %v", got, want)
	}
	for _, d := range r.Dropped {
		if d.Reason != domain.DropReasonMissing {
			t.Errorf("reason of %s = %q, want %q", d.Entry.TargetPath, d.Reason, domain.DropReasonMissing)
		}
	}
}

func TestReconcile_BundleLevelHookEntryWithAbsentPath_IsKept(t *testing.T) {
	m := rcManifest(
		rcBundleLevelHook("ghcp-cli", "logger", ".github/hooks/logger"),
		rcEntry("ghcp-cli", domain.ArtifactAgent, "x", ".github/agents/x.agent.md"),
	)

	r := manifest.Reconcile(m, present(".github/agents/x.agent.md"))

	if len(r.Dropped) != 0 {
		t.Errorf("Dropped = %+v, want none: a bundle-level hook entry is not dropped for its absent path", r.Dropped)
	}
	if got := rcPaths(r.Manifest); len(got) != 2 {
		t.Errorf("remaining = %v, want the bundle-level entry and the agent", got)
	}
}

func TestReconcile_BundleLevelEntryDoesNotCountTowardsThePresenceOfItsBundle(t *testing.T) {
	m := rcManifest(
		rcBundleLevelHook("ghcp-cli", "logger", ".github/hooks/logger"),
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
	)

	r := manifest.Reconcile(m, present())

	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, []string{".github/hooks/logger.json"}) {
		t.Errorf("dropped = %v, want the per-file entry whose file is missing", got)
	}
	if got := rcPaths(r.Manifest); !reflect.DeepEqual(got, []string{".github/hooks/logger"}) {
		t.Errorf("remaining = %v, want the bundle-level entry kept", got)
	}
}

func TestReconcile_HookBundlesAreGroupedPerHarness(t *testing.T) {
	m := rcManifest(
		rcEntry("ghcp-cli", domain.ArtifactHook, "logger", ".github/hooks/logger.json"),
		rcEntry("opencode", domain.ArtifactHook, "logger", ".opencode/plugins/logger.ts"),
	)

	r := manifest.Reconcile(m, present(".opencode/plugins/logger.ts"))

	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, []string{".github/hooks/logger.json"}) {
		t.Errorf("dropped = %v, want only the bundle of the harness whose files are all missing", got)
	}
}

func TestReconcile_HarnessWithEveryEntryDropped_IsNoLongerHeld(t *testing.T) {
	m := rcManifest(
		rcEntry("vscode-ghcp", domain.ArtifactAgent, "x", ".github/agents/x.agent.md"),
		rcEntry("vscode-ghcp", domain.ArtifactAgent, "y", ".github/agents/y.agent.md"),
		rcEntry("opencode", domain.ArtifactAgent, "x", ".opencode/agents/x.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/agents/x.md"))

	if r.Manifest.Holds("vscode-ghcp") {
		t.Error("vscode-ghcp is still held after all its entries were dropped")
	}
	if got := r.Manifest.HarnessIDs(); !reflect.DeepEqual(got, []string{"opencode"}) {
		t.Errorf("HarnessIDs = %v, want [opencode]", got)
	}
	if _, ok := r.Manifest.Section("vscode-ghcp"); ok {
		t.Error("the section of the harness without entries is still present")
	}
	if _, ok := r.Manifest.Section("opencode"); !ok {
		t.Error("the section of the harness that keeps entries was removed")
	}
}

func TestReconcile_EntriesOfEveryHarnessAreReconciled(t *testing.T) {
	m := rcManifest(
		rcEntry("claude-code", domain.ArtifactAgent, "a", ".claude/agents/a.md"),
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("ghcp-cli", domain.ArtifactAgent, "a", ".github/agents/a.agent.md"),
		rcEntry("vscode-ghcp", domain.ArtifactSkill, "s", ".github/skills/s/SKILL.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/agents/a.md"))

	harnesses := map[string]bool{}
	for _, d := range r.Dropped {
		harnesses[d.Entry.Harness] = true
	}
	for _, h := range []string{"claude-code", "ghcp-cli", "vscode-ghcp"} {
		if !harnesses[h] {
			t.Errorf("no entry of harness %s was dropped although its file is missing; dropped = %+v", h, r.Dropped)
		}
	}
	if harnesses["opencode"] {
		t.Error("the entry of opencode was dropped although its file is present")
	}
}

func TestReconcile_KeepsTheTopLevelFieldsOfTheManifest(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("opencode", domain.ArtifactAgent, "b", ".opencode/agents/b.md"),
	)
	m.CatalogStack = []domain.PersistedCatalogLayer{{}}

	r := manifest.Reconcile(m, present(".opencode/agents/a.md"))

	if len(r.Manifest.Entries) != 1 {
		t.Fatalf("entries = %v, want only the present one", rcPaths(r.Manifest))
	}
	if r.Manifest.ToolVersion != "1.7.0" || r.Manifest.SchemaVersion != manifest.SchemaVersion {
		t.Errorf("tool/schema version = %q/%q, want them kept", r.Manifest.ToolVersion, r.Manifest.SchemaVersion)
	}
	if len(r.Manifest.CatalogStack) != 1 {
		t.Errorf("CatalogStack = %+v, want it kept", r.Manifest.CatalogStack)
	}
}

func TestReconcile_ContentEditsAreNotExamined(t *testing.T) {
	e := rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md")
	e.ContentHash = "sha256:recorded-but-not-the-disk-content"

	r := manifest.Reconcile(rcManifest(e), present(".opencode/agents/a.md"))

	if len(r.Dropped) != 0 || len(r.Manifest.Entries) != 1 {
		t.Errorf("a present file was dropped or changed: %+v", r)
	}
}

func TestReconcile_EntryOfAnotherKind_IsKeptWhenPresentAndDroppedWhenMissing(t *testing.T) {
	const kind = domain.ArtifactKind("workflow")
	m := rcManifest(
		rcEntry("opencode", kind, "kept", ".opencode/other/kept.md"),
		rcEntry("opencode", kind, "gone", ".opencode/other/gone.md"),
	)

	r := manifest.Reconcile(m, present(".opencode/other/kept.md"))

	if got := rcDroppedPaths(r); !reflect.DeepEqual(got, []string{".opencode/other/gone.md"}) {
		t.Errorf("dropped = %v, want only the entry of the missing file", got)
	}
	if got := rcPaths(r.Manifest); !reflect.DeepEqual(got, []string{".opencode/other/kept.md"}) {
		t.Errorf("remaining = %v, want only the present entry", got)
	}
}

func TestReconcile_DoesNotChangeItsInputManifest(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("vscode-ghcp", domain.ArtifactAgent, "b", ".github/agents/b.agent.md"),
		rcEntry("opencode", domain.ArtifactAgent, "c", ".opencode/agents/c.md"),
	)
	snapshot := rcManifest(m.Entries...)
	snapshot.Entries = append([]domain.ManifestEntry(nil), m.Entries...)
	snapshot.Harnesses = append([]domain.HarnessSection(nil), m.Harnesses...)

	manifest.Reconcile(m, present(".opencode/agents/c.md"))

	if !reflect.DeepEqual(m, snapshot) {
		t.Errorf("Reconcile changed the manifest it was given:\n got %+v\nwant %+v", m, snapshot)
	}
}

func TestReconcile_RunTwice_DropsNothingTheSecondTime(t *testing.T) {
	m := rcManifest(
		rcEntry("opencode", domain.ArtifactAgent, "a", ".opencode/agents/a.md"),
		rcEntry("opencode", domain.ArtifactAgent, "b", ".opencode/agents/b.md"),
	)
	exists := present(".opencode/agents/a.md")

	first := manifest.Reconcile(m, exists)
	second := manifest.Reconcile(first.Manifest, exists)

	if len(first.Dropped) != 1 {
		t.Fatalf("first run dropped %v, want the one missing entry", rcDroppedPaths(first))
	}
	if len(second.Dropped) != 0 || !reflect.DeepEqual(second.Manifest, first.Manifest) {
		t.Errorf("second run changed something: dropped %v", rcDroppedPaths(second))
	}
}

func TestFileExists_FalseForAPathUnderAMissingDirectory(t *testing.T) {
	exists := manifest.FileExists(t.TempDir())

	if exists("no/such/dir/file.md") {
		t.Error("a path under a missing directory is reported present")
	}
}

func TestFileExists_FollowsASymlinkToAFile(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "real/file.md", "x")
	if err := os.Symlink(filepath.Join(ws, "real", "file.md"), filepath.Join(ws, "link.md")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	if !manifest.FileExists(ws)("link.md") {
		t.Error("a symlink to a regular file is reported missing")
	}
}

func TestFileExists_TrueOnlyForARegularFileUnderTheWorkspace(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "dir/file.md", "x")
	exists := manifest.FileExists(ws)

	if !exists("dir/file.md") {
		t.Error("an existing regular file is reported missing")
	}
	if exists("dir/other.md") {
		t.Error("a missing file is reported present")
	}
	if exists("dir") {
		t.Error("a directory is reported as a regular file")
	}
}
