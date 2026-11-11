package domain_test

// manifest_harness_test.go covers the multi-harness manifest model: the set of harnesses a
// workspace holds is derived from its entries, and ForHarness gives a view in which lookups
// never return another harness's entry.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
)

var (
	scopedRef  = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "reviewer"}
	scopedPath = ".github/agents/reviewer.agent.md"
)

func scopedEntry(harness, path, hash string) domain.ManifestEntry {
	return domain.ManifestEntry{Harness: harness, Ref: scopedRef, TargetPath: path, ContentHash: hash}
}

// heldManifest holds ghcp-cli and opencode; both record the agent at the shared path, the
// ghcp-cli entry first.
func heldManifest() domain.Manifest {
	return domain.Manifest{
		SchemaVersion: "2",
		ToolVersion:   "1.7.0",
		Harnesses: []domain.HarnessSection{
			{HarnessID: "ghcp-cli", UpdatedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)},
			{HarnessID: "opencode", UpdatedAt: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)},
		},
		Entries: []domain.ManifestEntry{
			scopedEntry("ghcp-cli", scopedPath, "sha256:ghcp"),
			scopedEntry("opencode", scopedPath, "sha256:open-shared"),
			scopedEntry("opencode", ".opencode/agents/reviewer.md", "sha256:open"),
		},
		CatalogStack: []domain.PersistedCatalogLayer{{Default: true}},
	}
}

func TestManifestHarnessIDs_AreTheDistinctEntryHarnessesSorted(t *testing.T) {
	m := heldManifest()
	m.Entries = append(m.Entries, scopedEntry("claude-code", ".claude/agents/reviewer.md", "sha256:c"))

	got := m.HarnessIDs()

	want := []string{"claude-code", "ghcp-cli", "opencode"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HarnessIDs = %v, want %v", got, want)
	}
}

func TestManifestHarnessIDs_DoNotIncludeASectionWithoutEntries(t *testing.T) {
	m := heldManifest()
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: "claude-code"})

	for _, id := range m.HarnessIDs() {
		if id == "claude-code" {
			t.Errorf("HarnessIDs lists claude-code, which has a section but no entry")
		}
	}
}

func TestManifestHarnessIDs_EmptyManifestHoldsNothing(t *testing.T) {
	if got := (domain.Manifest{}).HarnessIDs(); len(got) != 0 {
		t.Errorf("HarnessIDs of an empty manifest = %v, want none", got)
	}
}

func TestManifestHolds_IsTrueOnlyForHarnessesWithAnEntry(t *testing.T) {
	m := heldManifest()

	cases := map[string]bool{"ghcp-cli": true, "opencode": true, "claude-code": false, "vscode-ghcp": false, "": false}
	for id, want := range cases {
		if got := m.Holds(id); got != want {
			t.Errorf("Holds(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestManifestForHarness_KeepsOnlyTheEntriesOfThatHarness(t *testing.T) {
	m := heldManifest()

	got := m.ForHarness("opencode")

	if len(got.Entries) != 2 {
		t.Fatalf("entries = %+v, want the 2 opencode entries", got.Entries)
	}
	for _, e := range got.Entries {
		if e.Harness != "opencode" {
			t.Errorf("view of opencode contains an entry of %q", e.Harness)
		}
	}
}

func TestManifestForHarness_KeepsOnlyTheSectionOfThatHarness(t *testing.T) {
	m := heldManifest()

	got := m.ForHarness("ghcp-cli")

	if len(got.Harnesses) != 1 || got.Harnesses[0].HarnessID != "ghcp-cli" {
		t.Errorf("Harnesses = %+v, want only the ghcp-cli section", got.Harnesses)
	}
}

func TestManifestForHarness_CopiesTheOtherTopLevelFields(t *testing.T) {
	m := heldManifest()

	got := m.ForHarness("opencode")

	if got.SchemaVersion != "2" || got.ToolVersion != "1.7.0" || len(got.CatalogStack) != 1 {
		t.Errorf("SchemaVersion = %q, ToolVersion = %q, CatalogStack = %+v; want them copied",
			got.SchemaVersion, got.ToolVersion, got.CatalogStack)
	}
}

func TestManifestForHarness_LookupAtNeverReturnsAnotherHarnessEntry(t *testing.T) {
	m := heldManifest()

	got, ok := m.ForHarness("opencode").LookupAt(scopedRef, scopedPath)

	if !ok || got.Harness != "opencode" || got.ContentHash != "sha256:open-shared" {
		t.Errorf("LookupAt in the opencode view = %+v (found %v), want the opencode entry at the shared path", got, ok)
	}
}

func TestManifestForHarness_LookupNeverReturnsAnotherHarnessEntry(t *testing.T) {
	m := heldManifest()

	got, ok := m.ForHarness("opencode").Lookup(scopedRef)

	if !ok || got.Harness != "opencode" {
		t.Errorf("Lookup in the opencode view = %+v (found %v), want an opencode entry", got, ok)
	}
}

func TestManifestForHarness_ViewOfAHarnessWithoutEntriesFindsNothing(t *testing.T) {
	m := heldManifest()

	view := m.ForHarness("claude-code")

	if _, ok := view.LookupAt(scopedRef, scopedPath); ok {
		t.Errorf("the claude-code view found an entry although claude-code has none")
	}
	if len(view.Entries) != 0 || len(view.Harnesses) != 0 {
		t.Errorf("view = %+v, want no entries and no sections", view)
	}
}

func TestManifestForHarness_DoesNotChangeTheOriginal(t *testing.T) {
	m := heldManifest()

	_ = m.ForHarness("opencode")

	if len(m.Entries) != 3 || len(m.Harnesses) != 2 {
		t.Errorf("original manifest has %d entries and %d sections after ForHarness, want 3 and 2",
			len(m.Entries), len(m.Harnesses))
	}
}
