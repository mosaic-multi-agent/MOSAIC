package injections_test

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

func TestResolve_ProvenanceReportsSupplyingCatalogPerHarness(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles(), ghcpDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if got := res.Lookup(claudeDir).Supplier; !reflect.DeepEqual(got, high.Layer) {
		t.Errorf("claude Supplier = %+v, want %+v", got, high.Layer)
	}
	if got := res.Lookup(ghcpDir).Supplier; !reflect.DeepEqual(got, low.Layer) {
		t.Errorf("ghcp Supplier = %+v, want %+v", got, low.Layer)
	}
}

func TestResolve_HarnessInTwoLayers_YieldsExactlyOneShadowRecord(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles(), ghcpDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 1 {
		t.Fatalf("Shadows = %+v, want exactly one record (claude only)", res.Shadows)
	}
	got := res.Shadows[0]
	if got.Item != injectionsItem(claudeDir) {
		t.Errorf("Item = %+v, want harness-injections %q", got.Item, claudeDir)
	}
	if !reflect.DeepEqual(got.Winner, high.Layer) {
		t.Errorf("Winner = %+v, want %+v", got.Winner, high.Layer)
	}
	if !reflect.DeepEqual(got.Shadowed, []domain.CatalogLayer{low.Layer}) {
		t.Errorf("Shadowed = %+v, want [%+v]", got.Shadowed, low.Layer)
	}
	if got.Declared {
		t.Error("Declared = true, want false: the winner declares nothing")
	}
}

func TestResolve_DeclaredFlagFollowsWinnersDeclarationPerHarness(t *testing.T) {
	declares := identity.NewOverrides(injectionsItem(claudeDir))
	high := testLayerDir(t, "high", declares, map[string][]string{claudeDir: bothFiles(), ghcpDir: bothFiles()})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles(), ghcpDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 2 {
		t.Fatalf("Shadows = %+v, want one record per shadowed harness", res.Shadows)
	}
	for _, s := range res.Shadows {
		want := s.Item == injectionsItem(claudeDir)
		if s.Declared != want {
			t.Errorf("%s: Declared = %v, want %v", s.Item.Key, s.Declared, want)
		}
	}
}

func TestResolve_DeclarationOfShadowedLayerIsIgnored(t *testing.T) {
	declares := identity.NewOverrides(injectionsItem(openDir))
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{openDir: bothFiles()})
	low := testLayerDir(t, "low", declares, map[string][]string{openDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 1 || res.Shadows[0].Declared {
		t.Errorf("Shadows = %+v, want one record with Declared=false", res.Shadows)
	}
}

func TestResolve_ThreeLayers_ShadowListsEveryLowerSupplierHighestFirst(t *testing.T) {
	top := testLayerDir(t, "top", identity.Overrides{}, map[string][]string{openDir: bothFiles()})
	mid := testLayerDir(t, "mid", identity.Overrides{}, map[string][]string{openDir: bothFiles()})
	none := testLayerDir(t, "none", identity.Overrides{}, nil)
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{openDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{top, mid, none, low})

	if len(res.Shadows) != 1 {
		t.Fatalf("Shadows = %+v, want one record", res.Shadows)
	}
	want := []domain.CatalogLayer{mid.Layer, low.Layer}
	if !reflect.DeepEqual(res.Shadows[0].Shadowed, want) {
		t.Errorf("Shadowed = %+v, want %+v", res.Shadows[0].Shadowed, want)
	}
	if !reflect.DeepEqual(res.Shadows[0].Winner, top.Layer) {
		t.Errorf("Winner = %+v, want %+v", res.Shadows[0].Winner, top.Layer)
	}
}

func TestResolve_HarnessInOneLayerOnly_HasNoShadowRecord(t *testing.T) {
	only := testLayerDir(t, "only", identity.Overrides{}, map[string][]string{claudeDir: bothFiles(), ghcpDir: bothFiles()})
	other := testLayerDir(t, "other", identity.Overrides{}, nil)

	res := injections.Resolve([]stackrec.LayerInput{other, only})

	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none", res.Shadows)
	}
	if !res.Lookup(claudeDir).Supplied || !res.Lookup(ghcpDir).Supplied {
		t.Errorf("both harnesses must resolve to the only layer that has them")
	}
}
