package app_test

// active_stack_test.go verifies the active-stack method of the stack service interface: the
// layers of the catalog in effect, highest first, with identity, root and default-layer flag,
// and the effective origin of the stack.
//
// The effective origin follows one rule, shared with the persistence decision of the write
// runs: a set StackOrigin wins; otherwise StackFromInput true means stack input and false means
// default. Only the stack-input origin is persisted.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// activeStackFixture is a real two-layer stack: a user catalog over the default catalog.
type activeStackFixture struct {
	deps        app.Deps
	user, dflt  domain.CatalogLayer
	userAndDflt []domain.CatalogLayer
}

func newActiveStackFixture(t *testing.T, withUser bool) activeStackFixture {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	mosaic := newSysMosaicRoot(t, sysLayer{id: "act-default"})
	f := activeStackFixture{}
	f.dflt = domain.CatalogLayer{
		Identity: domain.CatalogIdentity{ID: "act-default", Name: "Fixture act-default", Version: "1.0.0"},
		Root:     defaultStackLayer(mosaic).Root,
		Default:  true,
	}
	layers := []stack.Layer{defaultStackLayer(mosaic)}
	f.userAndDflt = []domain.CatalogLayer{f.dflt}
	if withUser {
		userRoot := newSysUserLayer(t, sysLayer{id: "act-user"})
		f.user = domain.CatalogLayer{
			Identity: domain.CatalogIdentity{ID: "act-user", Name: "Fixture act-user", Version: "1.0.0"},
			Root:     userRoot,
		}
		layers = append([]stack.Layer{{Root: userRoot}}, layers...)
		f.userAndDflt = []domain.CatalogLayer{f.user, f.dflt}
	}
	useResolvedStack(t, &deps, mosaic, layers...)
	f.deps = deps
	return f
}

func requireActiveStack(t *testing.T, got stackconfig.ActiveStack, layers []domain.CatalogLayer, origin stackconfig.Origin) {
	t.Helper()
	if got.Origin != origin {
		t.Errorf("Origin = %q, want %q", got.Origin, origin)
	}
	if len(got.Layers) != len(layers) {
		t.Fatalf("Layers = %+v, want %+v", got.Layers, layers)
	}
	for i := range layers {
		if got.Layers[i] != layers[i] {
			t.Errorf("Layers[%d] = %+v, want %+v", i, got.Layers[i], layers[i])
		}
	}
}

func TestActiveStack_UserLayerOverDefault_ReportsBothHighestFirstWithStackInputOrigin(t *testing.T) {
	f := newActiveStackFixture(t, true)
	f.deps.StackOrigin = stackconfig.OriginStackInput
	f.deps.StackFromInput = true

	got := app.New(f.deps).ActiveStack()

	requireActiveStack(t, got, f.userAndDflt, stackconfig.OriginStackInput)
}

func TestActiveStack_DefaultLayerFlagAlone_ReportsTheDefaultLayerWithStackInputOrigin(t *testing.T) {
	f := newActiveStackFixture(t, false)
	f.deps.StackOrigin = stackconfig.OriginStackInput
	f.deps.StackFromInput = true

	got := app.New(f.deps).ActiveStack()

	requireActiveStack(t, got, []domain.CatalogLayer{f.dflt}, stackconfig.OriginStackInput)
}

func TestActiveStack_ReusedPersistedStack_ReportsPersistedOrigin(t *testing.T) {
	f := newActiveStackFixture(t, true)
	f.deps.StackOrigin = stackconfig.OriginPersisted

	got := app.New(f.deps).ActiveStack()

	requireActiveStack(t, got, f.userAndDflt, stackconfig.OriginPersisted)
}

func TestActiveStack_NoInputNoPersistedStack_ReportsDefaultOrigin(t *testing.T) {
	f := newActiveStackFixture(t, false)
	f.deps.StackOrigin = stackconfig.OriginDefault

	got := app.New(f.deps).ActiveStack()

	requireActiveStack(t, got, []domain.CatalogLayer{f.dflt}, stackconfig.OriginDefault)
}

func TestActiveStack_EffectiveOrigin(t *testing.T) {
	cases := []struct {
		name      string
		origin    stackconfig.Origin
		fromInput bool
		want      stackconfig.Origin
	}{
		{"both zero is default", "", false, stackconfig.OriginDefault},
		{"stack input flag alone is stack input", "", true, stackconfig.OriginStackInput},
		{"set origin wins over the input flag", stackconfig.OriginPersisted, true, stackconfig.OriginPersisted},
		{"set default wins over the input flag", stackconfig.OriginDefault, true, stackconfig.OriginDefault},
		{"set stack input without the flag", stackconfig.OriginStackInput, false, stackconfig.OriginStackInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newActiveStackFixture(t, false)
			f.deps.StackOrigin = c.origin
			f.deps.StackFromInput = c.fromInput

			got := app.New(f.deps).ActiveStack()

			if got.Origin != c.want {
				t.Errorf("Origin = %q, want %q", got.Origin, c.want)
			}
		})
	}
}

func TestActiveStack_PlainCatalog_ReportsOneLayerOfTheCatalogRoot(t *testing.T) {
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	deps.Catalog.(*stubCatalog).catalogRoot = "/plain/catalog"

	got := app.New(deps).ActiveStack()

	requireActiveStack(t, got, []domain.CatalogLayer{{Root: "/plain/catalog"}}, stackconfig.OriginDefault)
}

// TestWritePaths_PersistOnlyTheStackInputOrigin covers the persistence decision across the
// effective-origin rule: a set origin decides, the input flag alone still persists.
func TestWritePaths_PersistOnlyTheStackInputOrigin(t *testing.T) {
	cases := []struct {
		name      string
		origin    stackconfig.Origin
		fromInput bool
		persists  bool
	}{
		{"stack input origin alone", stackconfig.OriginStackInput, false, true},
		{"input flag alone", "", true, true},
		{"persisted origin is not persisted again", stackconfig.OriginPersisted, true, false},
		{"default origin is not persisted", stackconfig.OriginDefault, true, false},
		{"neither set", "", false, false},
	}
	for _, c := range cases {
		for _, op := range provOperations() {
			t.Run(c.name+"/"+op.name, func(t *testing.T) {
				f := newSPFixture(t)
				deps := f.withInput(f.descA())
				deps.StackOrigin = c.origin
				deps.StackFromInput = c.fromInput

				if _, err := op.run(context.Background(), app.New(deps), f.ws); err != nil {
					t.Fatalf("%s: %v", op.name, err)
				}

				got := f.exec.last(t).ManifestMeta.CatalogStack
				if c.persists {
					requireStackEqual(t, got, f.layersA())
				} else if got != nil {
					t.Errorf("CatalogStack = %+v, want none", got)
				}
			})
		}
	}
}
