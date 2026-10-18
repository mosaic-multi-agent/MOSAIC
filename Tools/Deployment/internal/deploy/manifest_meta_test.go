package deploy_test

// manifest_meta_test.go covers the provenance the executor records in the manifest.
//
// Per-entry provenance:
//   - The catalog of a plan item's version stamp is recorded on its manifest entry.
//   - An item without a catalog in its stamp gets an entry with no catalog.
//   - Redeploying a file whose prior entry has no catalog records the catalog.
//
// Top-level suppliers (carry-over rule):
//   - A run whose request supplies the protocol and bundle suppliers records them.
//   - A later run that supplies none keeps the suppliers of the earlier run.
//   - A run that supplies different suppliers replaces them; a field the run leaves nil is kept.
//   - A reverted atomic run and a dry run leave the prior manifest, suppliers included, unchanged.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

var (
	catUser    = domain.CatalogIdentity{ID: "user-cat", Name: "User Catalog", Version: "1.10"}
	catDefault = domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default Catalog", Version: "0.7.0"}
	catOther   = domain.CatalogIdentity{ID: "other-cat", Name: "Other", Version: "3.0"}
)

func idPtr(id domain.CatalogIdentity) *domain.CatalogIdentity { return &id }

// runAgents executes a non-atomic run that creates the given agents, with the given stamps and
// manifest-level data, and returns the manifest the store holds afterwards.
func runAgents(t *testing.T, workspace string, meta deploy.ManifestMeta, stamps map[string]domain.VersionStamp, keys ...string) domain.Manifest {
	t.Helper()
	items := make([]domain.PlanItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, newAgentItem(k, "agents/"+k+".md", domain.ActionCreate))
	}
	req := deploy.ExecRequest{
		Plan:          newPlan(workspace, items...),
		MosaicRoot:    t.TempDir(),
		Content:       fixedContent([]byte("# content")),
		VersionStamps: stamps,
		ManifestMeta:  meta,
	}
	if _, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return loadManifest(t, workspace)
}

func loadManifest(t *testing.T, workspace string) domain.Manifest {
	t.Helper()
	snap, err := manifest.NewStore().Load(workspace)
	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("manifest load: state %q, err %v / %v", snap.State, snap.Err, err)
	}
	return snap.Manifest
}

func requireSuppliers(t *testing.T, m domain.Manifest, protocol, bundle domain.CatalogIdentity) {
	t.Helper()
	if m.ProtocolSource != protocol {
		t.Errorf("ProtocolSource = %+v, want %+v", m.ProtocolSource, protocol)
	}
	if m.BundleSource != bundle {
		t.Errorf("BundleSource = %+v, want %+v", m.BundleSource, bundle)
	}
}

// ---------------------------------------------------------------------------
// Per-entry provenance
// ---------------------------------------------------------------------------

func TestManifestProvenance_EntryRecordsCatalogOfStamp(t *testing.T) {
	ws := t.TempDir()
	stamps := map[string]domain.VersionStamp{
		"agents/a.md": {Version: "1.0", Catalog: catUser},
		"agents/b.md": {Version: "1.0", Catalog: catDefault},
	}

	m := runAgents(t, ws, deploy.ManifestMeta{}, stamps, "a", "b")

	for key, want := range map[string]domain.CatalogIdentity{"a": catUser, "b": catDefault} {
		e, ok := m.Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key})
		if !ok {
			t.Fatalf("no entry for %s", key)
		}
		if e.Catalog != want {
			t.Errorf("entry %s Catalog = %+v, want %+v", key, e.Catalog, want)
		}
	}
}

func TestManifestProvenance_StampWithoutCatalogLeavesEntryUnknown(t *testing.T) {
	ws := t.TempDir()

	m := runAgents(t, ws, deploy.ManifestMeta{}, map[string]domain.VersionStamp{"agents/a.md": {Version: "1.0"}}, "a", "b")

	// b has no stamp at all; a has a stamp without catalog.
	for _, key := range []string{"a", "b"} {
		e, _ := m.Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key})
		if e.Catalog != (domain.CatalogIdentity{}) {
			t.Errorf("entry %s Catalog = %+v, want zero (unknown)", key, e.Catalog)
		}
	}
}

// TestManifestProvenance_RedeployFillsCatalogOfEntryWithoutOne simulates a workspace deployed
// before provenance existed: the prior entry has no catalog, and the redeploy records one.
func TestManifestProvenance_RedeployFillsCatalogOfEntryWithoutOne(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{}, nil, "a")
	if e, _ := loadManifest(t, ws).Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "a"}); e.Catalog != (domain.CatalogIdentity{}) {
		t.Fatalf("precondition: prior entry Catalog = %+v, want zero", e.Catalog)
	}
	item := newAgentItem("a", "agents/a.md", domain.ActionUpdate)
	req := deploy.ExecRequest{
		Plan:          newPlan(ws, item),
		MosaicRoot:    t.TempDir(),
		Content:       fixedContent([]byte("# content v2")),
		VersionStamps: map[string]domain.VersionStamp{"agents/a.md": {Version: "2.0", Catalog: catUser}},
	}

	if _, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	e, _ := loadManifest(t, ws).Lookup(item.Ref)
	if e.Catalog != catUser {
		t.Errorf("entry Catalog after redeploy = %+v, want %+v", e.Catalog, catUser)
	}
}

// ---------------------------------------------------------------------------
// Top-level suppliers: carry-over rule
// ---------------------------------------------------------------------------

func TestManifestMeta_RunSuppliesSuppliers_RecordsThem(t *testing.T) {
	ws := t.TempDir()
	meta := deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}

	m := runAgents(t, ws, meta, nil, "a")

	requireSuppliers(t, m, catDefault, catUser)
}

func TestManifestMeta_RunSuppliesNone_KeepsPriorSuppliers(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{}, nil, "b")

	requireSuppliers(t, m, catDefault, catUser)
	if _, ok := m.Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "b"}); !ok {
		t.Error("the partial run did not record its own entry")
	}
}

func TestManifestMeta_RunSuppliesDifferent_ReplacesPrior(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catOther), BundleSource: idPtr(catOther)}, nil, "b")

	requireSuppliers(t, m, catOther, catOther)
}

// TestManifestMeta_EachFieldCarriesOverIndependently verifies the rule is per field: a run that
// supplies only the protocol keeps the prior bundle supplier, and the reverse.
func TestManifestMeta_EachFieldCarriesOverIndependently(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catOther)}, nil, "b")
	requireSuppliers(t, m, catOther, catUser)

	m = runAgents(t, ws, deploy.ManifestMeta{BundleSource: idPtr(catDefault)}, nil, "c")
	requireSuppliers(t, m, catOther, catDefault)
}

// TestManifestMeta_FirstRunWithoutSuppliers_LeavesThemUnknown verifies that nothing is invented
// when neither the request nor a prior manifest has a supplier.
func TestManifestMeta_FirstRunWithoutSuppliers_LeavesThemUnknown(t *testing.T) {
	ws := t.TempDir()

	m := runAgents(t, ws, deploy.ManifestMeta{}, nil, "a")

	requireSuppliers(t, m, domain.CatalogIdentity{}, domain.CatalogIdentity{})
}

// TestManifestMeta_ResultManifestCarriesSuppliers verifies the manifest returned in ExecResult
// matches what was saved.
func TestManifestMeta_ResultManifestCarriesSuppliers(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")
	req := deploy.ExecRequest{
		Plan:       newPlan(ws, newAgentItem("b", "agents/b.md", domain.ActionCreate)),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("# content")),
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	requireSuppliers(t, res.Manifest, catDefault, catUser)
}

// ---------------------------------------------------------------------------
// Journal: reverted and dry runs
// ---------------------------------------------------------------------------

func TestManifestMeta_RevertedAtomicRun_LeavesPriorSuppliersUnchanged(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")
	blockPath(t, filepath.Join(ws, "skills"))
	req := deploy.ExecRequest{
		Plan: newPlan(ws,
			newAgentItem("b", "agents/b.md", domain.ActionCreate),
			newAgentItem("c", "skills/c.md", domain.ActionCreate)),
		MosaicRoot:   t.TempDir(),
		Content:      fixedContent([]byte("# content")),
		Atomic:       true,
		ManifestMeta: deploy.ManifestMeta{ProtocolSource: idPtr(catOther), BundleSource: idPtr(catOther)},
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.Reverted {
		t.Fatal("run was not reverted; the fixture must force a mid-plan failure")
	}
	m := loadManifest(t, ws)
	requireSuppliers(t, m, catDefault, catUser)
	if _, ok := m.Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "b"}); ok {
		t.Error("reverted run left an entry for a file it created")
	}
}

func TestManifestMeta_DryRun_LeavesPriorSuppliersUnchanged(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")
	req := deploy.ExecRequest{
		Plan:         newPlan(ws, newAgentItem("b", "agents/b.md", domain.ActionCreate)),
		MosaicRoot:   t.TempDir(),
		Content:      fixedContent([]byte("# content")),
		DryRun:       true,
		ManifestMeta: deploy.ManifestMeta{ProtocolSource: idPtr(catOther), BundleSource: idPtr(catOther)},
	}

	if _, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	requireSuppliers(t, loadManifest(t, ws), catDefault, catUser)
}
