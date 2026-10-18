package stack_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

func TestStack_Layers_OrderedHighestFirstWithIdentities(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")

	c := newTestStack(t, mosaic, high, low)

	want := []domain.CatalogLayer{
		{Identity: domain.CatalogIdentity{ID: "high", Name: "Catalog high", Version: "1.0.0"}, Root: high},
		{Identity: domain.CatalogIdentity{ID: "low", Name: "Catalog low", Version: "1.0.0"}, Root: low},
	}
	if got := c.Layers(); !reflect.DeepEqual(got, want) {
		t.Errorf("Layers() = %+v, want %+v", got, want)
	}
}

func TestStack_Layers_DefaultLayerIsFlaggedAndLast(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	user := newTestCatalogRoot(t, "user")
	defRoot := filepath.Join(mosaic, "Catalog")
	writeTestIdentity(t, defRoot, "mosaic")

	c, err := stack.New(mosaic, []stack.Layer{{Root: user}, {Root: defRoot, Default: true}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}

	layers := c.Layers()
	if len(layers) != 2 || layers[0].Default || !layers[1].Default {
		t.Fatalf("Layers() = %+v, want [user, default]", layers)
	}
	if layers[1].Root != defRoot || layers[1].Identity.ID != "mosaic" {
		t.Errorf("default layer = %+v", layers[1])
	}
}

func TestStack_Roots_RootIsMosaicRootAndCatalogRootIsHighestLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")

	c := newTestStack(t, mosaic, high, low)

	if c.Root() != mosaic {
		t.Errorf("Root() = %q, want the MOSAIC root %q", c.Root(), mosaic)
	}
	if c.CatalogRoot() != high {
		t.Errorf("CatalogRoot() = %q, want the highest layer's root %q", c.CatalogRoot(), high)
	}
}

func TestStack_Layer_IsNotMergedWithDefaultCatalog(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	defRoot := filepath.Join(mosaic, "Catalog")
	writeTestIdentity(t, defRoot, "mosaic")
	addTestSkill(t, defRoot, "default-skill", "1.0.0")
	addTestHook(t, defRoot, "default-hook", "h1")
	user := newTestCatalogRoot(t, "user")
	addTestSkill(t, user, "user-skill", "1.0.0")

	c := newTestStack(t, mosaic, user)

	if _, ok := c.Skill("default-skill"); ok {
		t.Error("Skill(default-skill) resolved although the default catalog is not a layer")
	}
	if _, ok := c.Hook("default-hook"); ok {
		t.Error("Hook(default-hook) resolved although the default catalog is not a layer")
	}
	if _, ok := c.Skill("user-skill"); !ok {
		t.Error("Skill(user-skill) missing")
	}
}

func TestStack_DefaultLayer_ContributesWhenRequestedAsLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	defRoot := filepath.Join(mosaic, "Catalog")
	writeTestIdentity(t, defRoot, "mosaic")
	addTestSkill(t, defRoot, "default-skill", "1.0.0")
	user := newTestCatalogRoot(t, "user")

	c, err := stack.New(mosaic, []stack.Layer{{Root: user}, {Root: defRoot, Default: true}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}

	if _, ok := c.Skill("default-skill"); !ok {
		t.Error("Skill(default-skill) missing although the default layer is in the stack")
	}
}

func TestStack_Issues_ConcatenateLayerIssues(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestTamperedHook(t, high, "bad-high")
	addTestTamperedHook(t, low, "bad-low")

	c := newTestStack(t, mosaic, high, low)

	issues := c.Issues()
	if len(issues) != 2 {
		t.Fatalf("Issues() = %+v, want one issue from each layer", issues)
	}
	for i, id := range []string{"bad-high", "bad-low"} {
		if issues[i].Code != "hook-hash-mismatch" || !strings.Contains(issues[i].Subject+issues[i].Path, id) {
			t.Errorf("Issues()[%d] = %+v, want hook-hash-mismatch for %s (higher layer first)", i, issues[i], id)
		}
	}
}

func TestStack_Catalog_ExposesLayersThroughCompanionInterfaceOnly(t *testing.T) {
	mosaic, catRoot := newTestMosaicShapedFixture(t)
	plain, err := catalog.Load(mosaic, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	st := newTestStack(t, mosaic, catRoot)

	var composed catalog.Catalog = st
	lister, ok := composed.(catalog.LayerLister)
	if !ok {
		t.Fatal("stack catalog held as catalog.Catalog does not implement catalog.LayerLister")
	}
	if got := lister.Layers(); len(got) != 1 || got[0].Root != catRoot || got[0].Identity.ID != "mosaic" {
		t.Errorf("Layers() = %+v, want the one layer at %s", got, catRoot)
	}
	if _, ok := plain.(catalog.LayerLister); ok {
		t.Error("catalog.Load result implements catalog.LayerLister, want it unchanged")
	}
}
