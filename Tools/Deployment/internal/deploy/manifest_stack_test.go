package deploy_test

// manifest_stack_test.go covers the catalog stack the executor records in the manifest, which
// follows the same carry-over rule as the other manifest-level data.
//
//   - A run whose request supplies a stack records it.
//   - A run that supplies a different stack replaces it as a whole; no layer of the earlier
//     stack is kept, and a stack of only the default layer replaces a user-catalog stack.
//   - A run that supplies no stack keeps the stack an earlier run recorded, including when it
//     supplies the protocol and bundle suppliers, and supplying only a stack keeps the suppliers.
//   - A workspace no run ever supplied a stack for has none.
//   - A reverted atomic run and a dry run leave the recorded stack unchanged.

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

var (
	stackUserOne = domain.PersistedCatalogLayer{Path: filepath.Join(string(filepath.Separator), "cats", "one"), CatalogID: "cat-one"}
	stackUserTwo = domain.PersistedCatalogLayer{Path: filepath.Join(string(filepath.Separator), "cats", "two"), CatalogID: "cat-two"}
	stackDefault = domain.PersistedCatalogLayer{Default: true}
)

func stackOf(layers ...domain.PersistedCatalogLayer) []domain.PersistedCatalogLayer { return layers }

func requireStack(t *testing.T, m domain.Manifest, want []domain.PersistedCatalogLayer) {
	t.Helper()
	if !reflect.DeepEqual(m.CatalogStack, want) {
		t.Errorf("CatalogStack = %+v, want %+v", m.CatalogStack, want)
	}
}

func TestManifestStack_SuppliedStackIsRecorded(t *testing.T) {
	ws := t.TempDir()

	m := runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne, stackDefault)}, nil, "a")

	requireStack(t, m, stackOf(stackUserOne, stackDefault))
}

func TestManifestStack_NeverSupplied_ManifestHasNoStack(t *testing.T) {
	ws := t.TempDir()

	m := runAgents(t, ws, deploy.ManifestMeta{}, nil, "a")
	m = runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault)}, nil, "b")

	if len(m.CatalogStack) != 0 {
		t.Errorf("CatalogStack = %+v, want none", m.CatalogStack)
	}
}

func TestManifestStack_AbsentStackKeepsPriorStack(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne, stackDefault)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{}, nil, "b")

	requireStack(t, m, stackOf(stackUserOne, stackDefault))
}

func TestManifestStack_SuppliersWithoutStack_KeepPriorStack(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "b")

	requireStack(t, m, stackOf(stackUserOne))
	requireSuppliers(t, m, catDefault, catUser)
}

func TestManifestStack_StackWithoutSuppliers_KeepsPriorSuppliers(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{ProtocolSource: idPtr(catDefault), BundleSource: idPtr(catUser)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserTwo)}, nil, "b")

	requireStack(t, m, stackOf(stackUserTwo))
	requireSuppliers(t, m, catDefault, catUser)
}

func TestManifestStack_SuppliedStackReplacesPriorStackAsAWhole(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne, stackDefault)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserTwo)}, nil, "b")

	requireStack(t, m, stackOf(stackUserTwo))
}

func TestManifestStack_DefaultOnlyStackReplacesUserCatalogStack(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne, stackUserTwo)}, nil, "a")

	m := runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackDefault)}, nil, "b")

	requireStack(t, m, stackOf(stackDefault))
}

// TestManifestStack_RevertedAtomicRun_KeepsPriorStack blocks a path so the second run fails
// mid-plan; the reversal restores the manifest, so the stack the run supplied is not recorded.
func TestManifestStack_RevertedAtomicRun_KeepsPriorStack(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne, stackDefault)}, nil, "a")
	good := newAgentItem("b", "agents/b.md", domain.ActionCreate)
	bad := newAgentItem("c", "blocked/c.md", domain.ActionCreate)
	blockPath(t, filepath.Join(ws, "blocked"))
	req := deploy.ExecRequest{
		Plan:         newPlan(ws, good, bad),
		MosaicRoot:   t.TempDir(),
		Content:      fixedContent([]byte("# content")),
		Atomic:       true,
		ManifestMeta: deploy.ManifestMeta{CatalogStack: stackOf(stackUserTwo)},
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.Reverted {
		t.Fatal("precondition: the run was expected to revert")
	}
	requireStack(t, loadManifest(t, ws), stackOf(stackUserOne, stackDefault))
}

func TestManifestStack_DryRun_KeepsPriorStack(t *testing.T) {
	ws := t.TempDir()
	runAgents(t, ws, deploy.ManifestMeta{CatalogStack: stackOf(stackUserOne)}, nil, "a")
	req := deploy.ExecRequest{
		Plan:         newPlan(ws, newAgentItem("b", "agents/b.md", domain.ActionCreate)),
		MosaicRoot:   t.TempDir(),
		Content:      fixedContent([]byte("# content")),
		DryRun:       true,
		ManifestMeta: deploy.ManifestMeta{CatalogStack: stackOf(stackUserTwo, stackDefault)},
	}

	if _, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	requireStack(t, loadManifest(t, ws), stackOf(stackUserOne))
}
