package deploy_test

// manifest_harness_test.go covers how the executor writes the multi-harness manifest.
//
//   - Entries a run writes belong to the run's harness.
//   - Entries and the section of any other harness are carried unchanged.
//   - The same path under two harnesses is two independent entries.
//   - Only the run harness's section takes the run's updated_at and catalog sources.
//   - tool_version stays top level; a dry run saves nothing.
//   - An old-format manifest is attributed by the executor's own load exactly as by any other
//     load through the same store.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	runHarness   = "opencode"
	otherHarness = "ghcp-cli"
	sharedPath   = ".github/agents/reviewer.agent.md"
)

var (
	openSectionAt  = time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	otherSectionAt = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	reviewerRef    = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "reviewer"}
)

// harnessPathsForTests is the path knowledge the production wiring gives the shared store.
var harnessPathsForTests = manifest.HarnessPaths{
	"claude-code": {".claude/agents", ".claude/skills", ".claude/hooks"},
	"opencode":    {".opencode/agents", ".opencode/skills", ".opencode/plugins"},
	"ghcp-cli":    {".github/agents", ".github/skills", ".github/hooks"},
}

func priorEntry(harness, path string, ref domain.ArtifactRef, hash string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:     harness,
		Ref:         ref,
		TargetPath:  path,
		Version:     "1.0.0",
		ContentHash: hash,
		DeployedAt:  time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Catalog:     catUser,
	}
}

// twoHarnessManifest holds an opencode and a ghcp-cli deployment; the reviewer agent sits at
// the shared .github path for ghcp-cli only.
func twoHarnessManifest() domain.Manifest {
	return domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		ToolVersion:   "1.6.0",
		Harnesses: []domain.HarnessSection{
			{HarnessID: otherHarness, UpdatedAt: otherSectionAt, ProtocolSource: catDefault, BundleSource: catUser},
			{HarnessID: runHarness, UpdatedAt: openSectionAt, ProtocolSource: catOther},
		},
		Entries: []domain.ManifestEntry{
			priorEntry(otherHarness, sharedPath, reviewerRef, "sha256:ghcp"),
			priorEntry(runHarness, ".opencode/agents/planner.md",
				domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "planner"}, "sha256:open"),
		},
	}
}

func seedManifest(t *testing.T, store manifest.Store, ws string, m domain.Manifest) {
	t.Helper()
	saveManifestWithFiles(t, store, ws, m)
}

// executeFor runs the executor for harnessID over items and returns the result.
func executeFor(t *testing.T, store manifest.Store, ws, harnessID string, mutate func(*deploy.ExecRequest), items ...domain.PlanItem) deploy.ExecResult {
	t.Helper()
	plan := newPlan(ws, items...)
	plan.Harness = domain.HarnessRef{ID: harnessID}
	req := deploy.ExecRequest{
		Plan:        plan,
		MosaicRoot:  t.TempDir(),
		Content:     fixedContent([]byte("# written by the run")),
		ToolVersion: "1.7.0",
	}
	if mutate != nil {
		mutate(&req)
	}
	res, err := deploy.NewExecutor(store, newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

func entriesAt(m domain.Manifest, path string) map[string]domain.ManifestEntry {
	out := map[string]domain.ManifestEntry{}
	for _, e := range m.Entries {
		if e.TargetPath == path {
			out[e.Harness] = e
		}
	}
	return out
}

func TestManifestHarness_EntriesTheRunWritesBelongToTheRunHarness(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()

	res := executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionCreate))

	saved := loadManifest(t, ws)
	for name, m := range map[string]domain.Manifest{"result": res.Manifest, "saved": saved} {
		e, ok := m.LookupAt(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "planner"}, ".opencode/agents/planner.md")
		if !ok || e.Harness != runHarness {
			t.Errorf("%s manifest: entry = %+v (found %v), want harness %q", name, e, ok, runHarness)
		}
	}
}

func TestManifestHarness_AnotherHarnessEntriesAreCarriedUnchanged(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	saved := loadManifest(t, ws)
	got := entriesAt(saved, sharedPath)[otherHarness]
	want := priorEntry(otherHarness, sharedPath, reviewerRef, "sha256:ghcp")
	if got != want {
		t.Errorf("ghcp-cli entry after an opencode run = %+v, want it unchanged: %+v", got, want)
	}
}

func TestManifestHarness_AnotherHarnessSectionIsCarriedUnchanged(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	executeFor(t, store, ws, runHarness, func(r *deploy.ExecRequest) {
		r.ManifestMeta = deploy.ManifestMeta{ProtocolSource: idPtr(catUser), BundleSource: idPtr(catUser)}
	}, newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	saved := loadManifest(t, ws)
	got, ok := saved.Section(otherHarness)
	if !ok {
		t.Fatalf("saved manifest lost the %s section", otherHarness)
	}
	want := domain.HarnessSection{HarnessID: otherHarness, UpdatedAt: otherSectionAt, ProtocolSource: catDefault, BundleSource: catUser}
	if got.HarnessID != want.HarnessID || !got.UpdatedAt.Equal(want.UpdatedAt) ||
		got.ProtocolSource != want.ProtocolSource || got.BundleSource != want.BundleSource {
		t.Errorf("%s section after an opencode run = %+v, want it unchanged: %+v", otherHarness, got, want)
	}
}

func TestManifestHarness_SavedManifestShowsBothHarnesses(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	res := executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	for name, m := range map[string]domain.Manifest{"result": res.Manifest, "saved": loadManifest(t, ws)} {
		for _, id := range []string{runHarness, otherHarness} {
			if _, ok := m.Section(id); !ok {
				t.Errorf("%s manifest has no section for %s", name, id)
			}
		}
		if _, ok := entriesAt(m, sharedPath)[otherHarness]; !ok {
			t.Errorf("%s manifest lost the %s entry", name, otherHarness)
		}
	}
}

func TestManifestHarness_SamePathUnderTwoHarnessesStaysTwoIndependentEntries(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	executeFor(t, store, ws, runHarness, func(r *deploy.ExecRequest) {
		r.Content = fixedContent([]byte("# opencode view of the shared file"))
	}, newAgentItem("reviewer", sharedPath, domain.ActionCreate))

	at := entriesAt(loadManifest(t, ws), sharedPath)
	if len(at) != 2 {
		t.Fatalf("entries at %s = %+v, want one per harness", sharedPath, at)
	}
	if at[otherHarness].ContentHash != "sha256:ghcp" {
		t.Errorf("ghcp-cli entry hash = %q, want its own recorded hash kept", at[otherHarness].ContentHash)
	}
	if h := at[runHarness].ContentHash; h == "" || h == "sha256:ghcp" {
		t.Errorf("opencode entry hash = %q, want the hash of the bytes this run wrote", h)
	}
}

func TestManifestHarness_UnchangedItemKeepsOnlyTheRunHarnessEntryAtASharedPath(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	m := twoHarnessManifest()
	m.Entries = append(m.Entries, priorEntry(runHarness, sharedPath, reviewerRef, "sha256:open-shared"))
	seedManifest(t, store, ws, m)

	executeFor(t, store, ws, runHarness, nil, newAgentItem("reviewer", sharedPath, domain.ActionUnchanged))

	at := entriesAt(loadManifest(t, ws), sharedPath)
	if at[runHarness].ContentHash != "sha256:open-shared" || at[otherHarness].ContentHash != "sha256:ghcp" {
		t.Errorf("entries at the shared path = %+v, want each harness to keep its own recorded hash", at)
	}
}

func TestManifestHarness_OnlyTheRunHarnessSectionTakesTheRunValues(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())
	before := time.Now().Add(-time.Second)

	res := executeFor(t, store, ws, runHarness, func(r *deploy.ExecRequest) {
		r.ManifestMeta = deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}
	}, newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	got, ok := res.Manifest.Section(runHarness)
	if !ok {
		t.Fatalf("no section for the run harness")
	}
	if got.ProtocolSource != catDefault || got.BundleSource != catUser {
		t.Errorf("run harness sources = %+v / %+v, want the sources the run supplied", got.ProtocolSource, got.BundleSource)
	}
	if got.UpdatedAt.Before(before) {
		t.Errorf("run harness UpdatedAt = %v, want the time of this run", got.UpdatedAt)
	}
}

func TestManifestHarness_RunWithoutSuppliersKeepsThePreviousSourcesOfItsOwnHarness(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	res := executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	open, _ := res.Manifest.Section(runHarness)
	other, _ := res.Manifest.Section(otherHarness)
	if open.ProtocolSource != catOther {
		t.Errorf("run harness ProtocolSource = %+v, want its own previous value %+v kept", open.ProtocolSource, catOther)
	}
	if other.ProtocolSource != catDefault || other.BundleSource != catUser {
		t.Errorf("other harness sources = %+v / %+v, want them untouched", other.ProtocolSource, other.BundleSource)
	}
}

func TestManifestHarness_FirstRunOfAHarnessDoesNotInheritAnotherHarnessSources(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	m := twoHarnessManifest()
	m.Harnesses = m.Harnesses[:1] // only ghcp-cli has a section
	m.Entries = m.Entries[:1]
	seedManifest(t, store, ws, m)

	res := executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionCreate))

	other, otherOK := res.Manifest.Section(otherHarness)
	if !otherOK || other.ProtocolSource != catDefault || other.BundleSource != catUser {
		t.Errorf("ghcp-cli section = %+v (found %v), want it kept with its own sources", other, otherOK)
	}
	open, ok := res.Manifest.Section(runHarness)
	if !ok {
		t.Fatalf("no section was created for the run harness")
	}
	if open.ProtocolSource != (domain.CatalogIdentity{}) || open.BundleSource != (domain.CatalogIdentity{}) {
		t.Errorf("new section sources = %+v / %+v, want unknown (not ghcp-cli's)", open.ProtocolSource, open.BundleSource)
	}
}

func TestManifestHarness_ToolVersionStaysAtTheTopLevel(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())

	executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	data, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("manifest is not valid YAML: %v", err)
	}
	if doc["tool_version"] != "1.7.0" {
		t.Errorf("top-level tool_version = %v, want 1.7.0", doc["tool_version"])
	}
	if _, found := doc["harness_id"]; found {
		t.Errorf("saved manifest still has a top-level harness_id")
	}
	sections, _ := doc["harnesses"].([]any)
	for _, raw := range sections {
		if s, _ := raw.(map[string]any); s["tool_version"] != nil {
			t.Errorf("a harness section carries tool_version: %v", s)
		}
	}
}

func TestManifestHarness_DryRunSavesNothingAndKeepsTheFileBytes(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	seedManifest(t, store, ws, twoHarnessManifest())
	path := filepath.Join(ws, manifest.Dir, manifest.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded manifest: %v", err)
	}

	executeFor(t, store, ws, runHarness, func(r *deploy.ExecRequest) { r.DryRun = true },
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest after dry run: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("dry run changed the manifest file:\n%s", after)
	}
}

func TestManifestHarness_DryRunOnAWorkspaceWithoutManifestCreatesNone(t *testing.T) {
	ws := t.TempDir()

	executeFor(t, manifest.NewStore(), ws, runHarness, func(r *deploy.ExecRequest) { r.DryRun = true },
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionCreate))

	if _, err := os.Stat(filepath.Join(ws, manifest.Dir, manifest.FileName)); !os.IsNotExist(err) {
		t.Errorf("a dry run created a manifest (stat err %v)", err)
	}
}

const oldFormatTwoHarnessManifest = `schema_version: "1"
harness_id: ghcp-cli
updated_at: "2026-11-02T09:15:00Z"
tool_version: 1.6.0
entries:
  - ref:
      kind: agent
      key: planner
    target_path: .opencode/agents/planner.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:open
    deployed_at: "2026-11-01T10:00:00Z"
  - ref:
      kind: agent
      key: reviewer
    target_path: .github/agents/reviewer.agent.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:ghcp
    deployed_at: "2026-11-02T09:00:00Z"
  - ref:
      kind: skill
      key: stray
    target_path: docs/stray/SKILL.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:stray
    deployed_at: "2026-11-02T09:00:00Z"
`

func writeOldFormat(t *testing.T, ws string) {
	t.Helper()
	dir := filepath.Join(ws, manifest.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), []byte(oldFormatTwoHarnessManifest), 0o644); err != nil {
		t.Fatalf("write old manifest: %v", err)
	}
	// The executor reconciles with the disk, so the files the old manifest records must exist.
	for _, p := range []string{".opencode/agents/planner.md", sharedPath, "docs/stray/SKILL.md"} {
		writeWorkspaceFile(t, ws, p, "deployed content")
	}
}

func TestManifestHarness_ExecutorLoadKeepsOldGithubEntriesAttributedLikeTheFlowLoad(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore(manifest.WithHarnessPaths(harnessPathsForTests))
	writeOldFormat(t, ws)
	flowView, err := store.Load(ws) // what a flow's load through the shared store attributes
	if err != nil {
		t.Fatalf("flow load: %v", err)
	}
	flowEntry, ok := flowView.Manifest.LookupAt(reviewerRef, sharedPath)
	if !ok || flowEntry.Harness != otherHarness {
		t.Fatalf("flow load attributes the .github entry to %q (found %v), want %s", flowEntry.Harness, ok, otherHarness)
	}

	executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	saved := loadManifest(t, ws)
	got, ok := saved.LookupAt(reviewerRef, sharedPath)
	if !ok || got.Harness != otherHarness || got.ContentHash != "sha256:ghcp" {
		t.Errorf("after the run the .github entry is %+v (found %v), want it kept for %s as the flow load saw it",
			got, ok, otherHarness)
	}
	if _, ok := saved.Section(otherHarness); !ok {
		t.Errorf("the old harness section of %s was lost", otherHarness)
	}
}

func TestManifestHarness_ExecutorLoadDropsTheEntriesTheFlowLoadDrops(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore(manifest.WithHarnessPaths(harnessPathsForTests))
	writeOldFormat(t, ws)

	executeFor(t, store, ws, runHarness, nil,
		newAgentItem("planner", ".opencode/agents/planner.md", domain.ActionUpdate))

	for _, e := range loadManifest(t, ws).Entries {
		if strings.HasPrefix(e.TargetPath, "docs/") {
			t.Errorf("the unattributable entry %+v was written back to the manifest", e)
		}
	}
}

func TestManifestHarness_RunForTheOldHarnessLeavesOpencodeEntriesOfTheOtherHarness(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore(manifest.WithHarnessPaths(harnessPathsForTests))
	writeOldFormat(t, ws)

	executeFor(t, store, ws, otherHarness, nil,
		newAgentItem("reviewer", sharedPath, domain.ActionUpdate))

	saved := loadManifest(t, ws)
	planner, ok := saved.LookupAt(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "planner"}, ".opencode/agents/planner.md")
	if !ok || planner.Harness != runHarness || planner.ContentHash != "sha256:open" {
		t.Errorf("opencode entry after a ghcp-cli run = %+v (found %v), want it kept for opencode", planner, ok)
	}
}
