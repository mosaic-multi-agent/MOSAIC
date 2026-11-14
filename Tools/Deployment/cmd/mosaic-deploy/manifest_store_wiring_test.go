package main

// manifest_store_wiring_test.go verifies that the manifest store built for production knows
// which directories each built-in harness deploys into, so an old-format manifest is upgraded
// through the real harness descriptors.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/manifest"
)

// wiringRegistry discovers the built-in harnesses against a temporary MOSAIC root.
func wiringRegistry(t *testing.T) registry.Registry {
	t.Helper()
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: makeE2EMosaicRoot(t),
		GOOS:       runtime.GOOS,
	})
	if err != nil {
		t.Fatalf("registry.Discover: %v", err)
	}
	return reg
}

// oldFormatManifest returns an old-format manifest whose top-level harness is oldHarness and
// that holds one entry at each of the given target paths.
func oldFormatManifest(oldHarness string, targetPaths ...string) string {
	out := "schema_version: \"1\"\nharness_id: " + oldHarness + "\nupdated_at: \"2026-11-02T09:15:00Z\"\n" +
		"tool_version: 1.6.0\nentries:\n"
	for _, p := range targetPaths {
		out += "  - ref:\n      kind: agent\n      key: sample\n    target_path: " + p + "\n" +
			"    version: \"1.0.0\"\n    deployed_at: \"2026-11-01T10:00:00Z\"\n    content_hash: sha256:" +
			"1111111111111111111111111111111111111111111111111111111111111111\n"
	}
	return out
}

// loadThroughWiring writes an old-format manifest into a fresh workspace and loads it through
// the store the production wiring builds.
func loadThroughWiring(t *testing.T, content string) manifest.Snapshot {
	t.Helper()
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".mosaic"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".mosaic", "manifest.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := newManifestStore(wiringRegistry(t)).Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.State != manifest.StatePresent {
		t.Fatalf("State = %q (err %v), want present", snap.State, snap.Err)
	}
	return snap
}

func entryHarness(m domain.Manifest, targetPath string) (string, bool) {
	for _, e := range m.Entries {
		if e.TargetPath == targetPath {
			return e.Harness, true
		}
	}
	return "", false
}

func droppedPaths(snap manifest.Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, d := range snap.MigrationDropped {
		out[d.Entry.TargetPath] = true
	}
	return out
}

func TestManifestStoreWiring_GithubEntriesOfOldGhcpCliManifestKeepTheirHarness(t *testing.T) {
	snap := loadThroughWiring(t, oldFormatManifest("ghcp-cli",
		".github/agents/a.agent.md", ".github/skills/s/SKILL.md", ".github/hooks/h.json", "docs/notes.md"))

	for _, p := range []string{".github/agents/a.agent.md", ".github/skills/s/SKILL.md", ".github/hooks/h.json"} {
		if got, ok := entryHarness(snap.Manifest, p); !ok || got != "ghcp-cli" {
			t.Errorf("entry %q harness = %q (present %v), want ghcp-cli: the registry descriptor says ghcp-cli deploys there", p, got, ok)
		}
	}
	// The sibling outside every harness directory makes the load depend on the path knowledge.
	if !droppedPaths(snap)["docs/notes.md"] {
		t.Errorf("MigrationDropped = %v, want docs/notes.md dropped by the same load", snap.MigrationDropped)
	}
}

func TestManifestStoreWiring_GithubEntriesOfOldVscodeGhcpManifestKeepTheirHarness(t *testing.T) {
	snap := loadThroughWiring(t, oldFormatManifest("vscode-ghcp", ".github/agents/a.agent.md", "docs/notes.md"))

	if got, ok := entryHarness(snap.Manifest, ".github/agents/a.agent.md"); !ok || got != "vscode-ghcp" {
		t.Errorf("entry harness = %q (present %v), want vscode-ghcp", got, ok)
	}
	if !droppedPaths(snap)["docs/notes.md"] {
		t.Errorf("MigrationDropped = %v, want docs/notes.md dropped by the same load", snap.MigrationDropped)
	}
}

func TestManifestStoreWiring_GithubEntriesAreDroppedWhenTheOldHarnessDoesNotDeployThere(t *testing.T) {
	snap := loadThroughWiring(t, oldFormatManifest("opencode", ".github/agents/a.agent.md"))

	if !droppedPaths(snap)[".github/agents/a.agent.md"] {
		t.Errorf("MigrationDropped = %v, want the .github entry dropped: opencode does not deploy to .github", snap.MigrationDropped)
	}
	if _, ok := entryHarness(snap.Manifest, ".github/agents/a.agent.md"); ok {
		t.Error("the .github entry stayed in the manifest although no harness named by the old manifest deploys there")
	}
}

func TestManifestStoreWiring_PathOutsideEveryHarnessDirectoryIsDropped(t *testing.T) {
	snap := loadThroughWiring(t, oldFormatManifest("ghcp-cli", ".github/agents/a.agent.md", "docs/notes.md"))

	if !droppedPaths(snap)["docs/notes.md"] {
		t.Errorf("MigrationDropped = %v, want docs/notes.md dropped", snap.MigrationDropped)
	}
	if got, ok := entryHarness(snap.Manifest, ".github/agents/a.agent.md"); !ok || got != "ghcp-cli" {
		t.Errorf("the .github entry harness = %q (present %v), want ghcp-cli kept in the same load", got, ok)
	}
}
