package app_test

// catalog_change_update_test.go verifies the outcome on the write paths when the catalog that
// supplies the deployed files is no longer the catalog the manifest records: with the real
// planner, deploy and update plan the agent, the orchestrator, both skill files and the hook as
// updates that name both catalogs, hand them to the executor, and the stamps they get carry the
// identity of the catalog supplying them now. Fixtures live in a temporary workspace; nothing
// reads the live catalog.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	ccuAgentTarget     = "agents/test-runner.md"
	ccuOrchTarget      = "agents/orchestrator.md"
	ccuSkillEntry      = "skills/stub-skill/SKILL.md"
	ccuSkillCompanion  = "skills/stub-skill/notes.md"
	ccuHookTarget      = uhHookTarget
	ccuCatalogDelta    = "source_catalog"
	ccuUnmodifiedAgent = "---\nmosaic_id: \"1\"\nversion: \"1.0\"\n---\nBody.\n"
)

// ccuHookRef is the hook bundle of the harness hook fixtures.
var ccuHookRef = domain.ArtifactRef{Kind: domain.ArtifactHook, Key: uhHookKey}

// ccuSupplierOf returns the identity of the catalog that supplies ref in the fixture stack.
func ccuSupplierOf(ref domain.ArtifactRef) domain.CatalogIdentity {
	if ref == ccuHookRef {
		return provHookSource
	}
	return provExpectedByRf[ref]
}

// ccuRecorded is the catalog the manifest names; it is not in the stack of the run.
var ccuRecorded = domain.CatalogIdentity{ID: "orchonly", Name: "Orchestrator Only", Version: "1.0.0"}

// ccuModule maps agents, skill files and hooks to the fixture target paths.
type ccuModule struct {
	*stubHarnessModule
	hp domain.HookPlan
}

func (m ccuModule) HookPlan(_ domain.HookPlanRequest) (domain.HookPlan, error) { return m.hp, nil }

func (m ccuModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	switch req.Kind {
	case domain.ArtifactAgent:
		return "agents/" + req.Key + ".md", nil
	case domain.ArtifactSkill:
		return "skills/" + req.Key + "/" + req.FileName, nil
	case domain.ArtifactHook:
		return ccuHookTarget, nil
	}
	return m.stubHarnessModule.TargetPath(req)
}

// ccuTargets lists every planned target with the reference that owns it.
func ccuTargets() map[string]domain.ArtifactRef {
	return map[string]domain.ArtifactRef{
		ccuAgentTarget:    provAgentRef,
		ccuOrchTarget:     provOrchRef,
		ccuSkillEntry:     provSkillRef,
		ccuSkillCompanion: provSkillRef,
		ccuHookTarget:     ccuHookRef,
	}
}

// ccuEntries records every target under catalog rec.
func ccuEntries(rec domain.CatalogIdentity) []domain.ManifestEntry {
	var entries []domain.ManifestEntry
	for target, ref := range ccuTargets() {
		e := domain.ManifestEntry{Ref: ref, TargetPath: target, Version: "1.0", ContentHash: manifest.Hash(nil), Catalog: rec}
		if ref == ccuHookRef {
			e.Version = uhCatalogVersion
		}
		entries = append(entries, e)
	}
	return entries
}

// ccuWrite puts content at the workspace-relative path.
func ccuWrite(t *testing.T, ws, rel, content string) {
	t.Helper()
	full := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// ccuDeps returns deps running the real planner over a stack whose items are supplied by the
// fixed provenance table of provDeps, over a workspace holding every target, recorded in the
// manifest under rec.
func ccuDeps(t *testing.T, rec domain.CatalogIdentity) (app.Deps, string, *recordingExecutor) {
	t.Helper()
	deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	cat := deps.Catalog.(provCatalog).resolvedStackCatalog.Catalog.(*stubCatalog)
	cat.skills = []domain.Skill{{
		Key: "stub-skill", Version: "1.0", SourceDir: filepath.Join(ws, "catalog-skill"),
		EntryFile: "SKILL.md", Files: []string{"SKILL.md", "notes.md"},
	}}
	cat.hooks = []domain.HookBundle{{Key: uhHookKey, Version: uhCatalogVersion}}
	pc := deps.Catalog.(provCatalog)
	pc.byItem[domain.CatalogItem{Kind: domain.CatalogItemHook, Key: uhHookKey}] = domain.CatalogLayer{Identity: provHookSource}
	mod := ccuModule{newModuleWithAgentsDir("agents"), newTestHookPlan(t, uhFlat)}
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{mod.Ref()},
		modules: map[string]domain.HarnessModule{"stub-harness": mod},
	}
	deps.Planner = plan.New()
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{
		State:    manifest.StatePresent,
		Manifest: domain.Manifest{SchemaVersion: manifest.SchemaVersion, HarnessID: "stub-harness", Entries: ccuEntries(rec)},
	}}
	ccuWrite(t, ws, ccuAgentTarget, ccuUnmodifiedAgent)
	orch := strings.Replace(string(crDeployedOrchestrator("quick-fix")), "---\n", "---\nmosaic_id: \"0\"\n", 1)
	ccuWrite(t, ws, ccuOrchTarget, orch)
	ccuWrite(t, ws, ccuSkillEntry, "---\nname: stub-skill\nversion: \"1.0\"\n---\n")
	ccuWrite(t, ws, ccuSkillCompanion, "notes\n")
	for _, name := range uhFlat.files {
		ccuWrite(t, ws, uhHooksDir+"/"+name, uhFixture(t, uhFlat, name))
	}
	return deps, ws, exec
}

// ccuOperation is a write operation that reaches every fixture target.
type ccuOperation struct {
	name string
	run  func(ctx context.Context, svc app.Service, ws string) error
}

func ccuOperations() []ccuOperation {
	skip := map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true}
	return []ccuOperation{
		{"deploy", func(ctx context.Context, svc app.Service, ws string) error {
			_, err := svc.DeployNew(ctx, app.DeployRequest{HarnessID: "stub-harness", WorkspacePath: ws,
				WorkflowIDs: []string{"quick-fix"}, HookIDs: []string{uhHookKey}, SkipAll: skip,
				AutoConfirmPlan: true, ConflictDefault: domain.DecisionOverwrite})
			return err
		}},
		{"update", func(ctx context.Context, svc app.Service, ws string) error {
			_, err := svc.Update(ctx, app.UpdateRequest{HarnessID: "stub-harness", WorkspacePath: ws, SkipAll: skip,
				AutoConfirmPlan: true, ConflictDefault: domain.DecisionOverwrite})
			return err
		}},
	}
}

// ccuItemAt returns the plan item the executor received for target.
func ccuItemAt(t *testing.T, items []domain.PlanItem, target string) domain.PlanItem {
	t.Helper()
	for _, it := range items {
		if it.TargetPath == target {
			return it
		}
	}
	t.Fatalf("the executor plan has no item for %q", target)
	return domain.PlanItem{}
}

func TestWritePaths_RecordedCatalogOutsideStack_PlanItemsAsCatalogUpdates(t *testing.T) {
	for _, op := range ccuOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := ccuDeps(t, ccuRecorded)

			if err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			items := exec.last(t).Plan.Items
			for target, ref := range ccuTargets() {
				item := ccuItemAt(t, items, target)
				want := ccuSupplierOf(ref)
				if item.Action != domain.ActionUpdate {
					t.Errorf("%s: Action = %q (reason %q), want %q", target, item.Action, item.Reason, domain.ActionUpdate)
				}
				for _, id := range []string{ccuRecorded.ID, want.ID} {
					if !strings.Contains(item.Reason, id) {
						t.Errorf("%s: Reason = %q, want it to name catalog %q", target, item.Reason, id)
					}
				}
			}
		})
	}
}

func TestWritePaths_RecordedCatalogOutsideStack_StampsCarryCurrentSupplier(t *testing.T) {
	for _, op := range ccuOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := ccuDeps(t, ccuRecorded)

			if err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			req := exec.last(t)
			for target, ref := range ccuTargets() {
				// A stamp only describes a rewrite when the item is planned as an update.
				if item := ccuItemAt(t, req.Plan.Items, target); item.Action != domain.ActionUpdate {
					t.Errorf("%s: Action = %q, want %q so the supplier is written", target, item.Action, domain.ActionUpdate)
				}
				stamp, ok := req.VersionStamps[target]
				if !ok {
					t.Errorf("%s: no version stamp", target)
					continue
				}
				if want := ccuSupplierOf(ref); stamp.Catalog != want {
					t.Errorf("%s: stamp Catalog = %+v, want the supplier %+v", target, stamp.Catalog, want)
				}
			}
		})
	}
}

func TestWritePaths_RecordedCatalogInStack_PlansNoCatalogUpdate(t *testing.T) {
	for _, op := range ccuOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws, exec := ccuDeps(t, ccuRecorded)
			deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{
				State:    manifest.StatePresent,
				Manifest: domain.Manifest{SchemaVersion: manifest.SchemaVersion, HarnessID: "stub-harness", Entries: ccuEntriesBySupplier()},
			}}

			if err := op.run(context.Background(), app.New(deps), ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			for _, it := range exec.last(t).Plan.Items {
				for _, d := range it.Stale {
					if d.Field == ccuCatalogDelta {
						t.Errorf("%s: Stale = %+v, want no catalog delta when the recorded catalog supplies the item", it.TargetPath, it.Stale)
					}
				}
			}
		})
	}
}

// ccuEntriesBySupplier records every target under the catalog that supplies it now.
func ccuEntriesBySupplier() []domain.ManifestEntry {
	var entries []domain.ManifestEntry
	for target, ref := range ccuTargets() {
		entries = append(entries, domain.ManifestEntry{
			Ref: ref, TargetPath: target, Version: "1.0", ContentHash: manifest.Hash(nil), Catalog: ccuSupplierOf(ref),
		})
		if ref == ccuHookRef {
			entries[len(entries)-1].Version = uhCatalogVersion
		}
	}
	return entries
}
