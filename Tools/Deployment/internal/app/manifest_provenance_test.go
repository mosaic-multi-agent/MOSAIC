package app_test

// manifest_provenance_test.go verifies what the write paths hand the executor so the manifest
// records where each deployed file came from.
//
// Per-file provenance, in each of deploy, update, workflows, agents and hooks:
//   - Every stamped item carries the identity the catalog's provenance names for it; the two
//     orchestrators are looked up by their agent key.
//   - An item the catalog does not know, and any item of a catalog that reports no provenance,
//     is stamped with no catalog.
//
// Top-level suppliers:
//   - Full runs (deploy, update) supply the protocol and bundle suppliers of the resolved stack,
//     each from the layer that supplies that file.
//   - Partial runs (workflows, agents, hooks) supply neither, so a later full run's suppliers
//     are preserved; a plain catalog supplies none either.
//
// Old manifests: a workspace whose manifest predates provenance updates without error, and the
// run stamps provenance again.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestWritePaths_StampTheCatalogOfEachItem(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())

			if _, err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			req := exec.last(t)
			for _, ref := range op.stamped {
				stamp, ok := provStampFor(req, ref)
				if !ok {
					t.Errorf("no version stamp for %v", ref)
					continue
				}
				if want := provExpectedByRf[ref]; stamp.Catalog != want {
					t.Errorf("stamp of %v Catalog = %+v, want %+v", ref, stamp.Catalog, want)
				}
			}
		})
	}
}

func TestWritePaths_ItemUnknownToProvenance_IsStampedWithoutCatalog(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
			pc := deps.Catalog.(provCatalog)
			pc.byItem = map[domain.CatalogItem]domain.CatalogLayer{} // knows nothing
			deps.Catalog = pc

			if _, err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			req := exec.last(t)
			for _, ref := range op.stamped {
				stamp, ok := provStampFor(req, ref)
				if !ok {
					t.Errorf("no version stamp for %v: unknown provenance must not drop the stamp", ref)
					continue
				}
				if stamp.Catalog != (domain.CatalogIdentity{}) {
					t.Errorf("stamp of %v Catalog = %+v, want zero", ref, stamp.Catalog)
				}
			}
		})
	}
}

func TestWritePaths_PlainCatalog_StampsAndSuppliesNoProvenance(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
			deps.Catalog.(*stubCatalog).hooks = []domain.HookBundle{{Key: "stub-hook", Version: "1.0"}}
			deps.Planner = &stubPlanner{plan: provPlan(ws)}
			exec := &recordingExecutor{}
			deps.Executor = exec

			if _, err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			req := exec.last(t)
			if req.ManifestMeta.ProtocolSource != nil || req.ManifestMeta.BundleSource != nil {
				t.Errorf("suppliers = %v / %v, want nil for a catalog that is not a stack",
					req.ManifestMeta.ProtocolSource, req.ManifestMeta.BundleSource)
			}
			for _, ref := range op.stamped {
				if stamp, _ := provStampFor(req, ref); stamp.Catalog != (domain.CatalogIdentity{}) {
					t.Errorf("stamp of %v Catalog = %+v, want zero", ref, stamp.Catalog)
				}
			}
		})
	}
}

func TestWritePaths_SuppliersFollowFullAndPartialRuns(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())

			if _, err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			meta := exec.last(t).ManifestMeta
			if !op.full {
				if meta.ProtocolSource != nil || meta.BundleSource != nil {
					t.Errorf("partial run supplies suppliers %v / %v; it must supply none so the earlier full run's are kept",
						meta.ProtocolSource, meta.BundleSource)
				}
				return
			}
			if meta.ProtocolSource == nil || *meta.ProtocolSource != provDefaultIdentity {
				t.Errorf("ProtocolSource = %v, want the default layer %+v", meta.ProtocolSource, provDefaultIdentity)
			}
			if meta.BundleSource == nil || *meta.BundleSource != provUserIdentity {
				t.Errorf("BundleSource = %v, want the user layer %+v", meta.BundleSource, provUserIdentity)
			}
		})
	}
}

// TestPartialRunAfterFullRun_KeepsRecordedSuppliers drives the real executor and manifest store:
// a full deploy records the suppliers and the per-file catalogs, and a following agents-only run
// keeps the suppliers.
func TestPartialRunAfterFullRun_KeepsRecordedSuppliers(t *testing.T) {
	deps, ws, _ := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	store := manifest.NewStore()
	deps.Manifest = store
	deps.Executor = deploy.NewExecutor(store, &stubLogger{}, deps.Todo)
	svc := app.New(deps)
	ops := provOperations()

	if _, err := ops[0].run(context.Background(), svc, ws); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if _, err := ops[3].run(context.Background(), svc, ws); err != nil {
		t.Fatalf("agents: %v", err)
	}

	snap, err := store.Load(ws)
	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("manifest: state %q, err %v / %v", snap.State, snap.Err, err)
	}
	if snap.Manifest.ProtocolSource != provDefaultIdentity || snap.Manifest.BundleSource != provUserIdentity {
		t.Errorf("suppliers after the partial run = %+v / %+v, want %+v / %+v",
			snap.Manifest.ProtocolSource, snap.Manifest.BundleSource, provDefaultIdentity, provUserIdentity)
	}
	if e, ok := snap.Manifest.Lookup(provAgentRef); !ok || e.Catalog != provAgentSource {
		t.Errorf("agent entry Catalog = %+v (found %v), want %+v", e.Catalog, ok, provAgentSource)
	}
}

// TestUpdate_ManifestWithoutProvenance_UpdatesAndStampsProvenance verifies a workspace whose
// manifest predates provenance (entries without catalog) updates without error and the run
// supplies provenance again.
func TestUpdate_ManifestWithoutProvenance_UpdatesAndStampsProvenance(t *testing.T) {
	deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{
		State: manifest.StatePresent,
		Manifest: domain.Manifest{
			SchemaVersion: manifest.SchemaVersion,
			HarnessID:     "stub-harness",
			Entries:       []domain.ManifestEntry{{Ref: provOrchRef, TargetPath: "orchestrator.md", Version: "1.0", ContentHash: "sha256:00"}},
		},
	}}

	if err := provWriteDeployed(ws); err != nil {
		t.Fatalf("write deployed files: %v", err)
	}

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws, AutoConfirmPlan: true})

	if err != nil {
		t.Fatalf("Update on a pre-provenance manifest: %v", err)
	}
	req := exec.last(t)
	if stamp, _ := provStampFor(req, provOrchRef); stamp.Catalog != provOrchSource {
		t.Errorf("stamp Catalog = %+v after update, want %+v: provenance must become known on redeploy", stamp.Catalog, provOrchSource)
	}
	if req.ManifestMeta.ProtocolSource == nil {
		t.Error("update of a pre-provenance workspace did not supply the protocol supplier")
	}
}
