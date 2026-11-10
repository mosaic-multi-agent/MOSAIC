package systemfile_test

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

func TestResolve_ProvenanceReportsSupplyingCatalog(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, protocolName)
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if !reflect.DeepEqual(res.Protocol.Supplier, high.Layer) {
		t.Errorf("Protocol.Supplier = %+v, want %+v", res.Protocol.Supplier, high.Layer)
	}
	if !reflect.DeepEqual(res.Bundle.Supplier, low.Layer) {
		t.Errorf("Bundle.Supplier = %+v, want %+v", res.Bundle.Supplier, low.Layer)
	}
}

func TestResolve_FileInTwoLayers_YieldsExactlyOneShadowRecord(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, protocolName)
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 1 {
		t.Fatalf("Shadows = %+v, want exactly one record (protocol only)", res.Shadows)
	}
	got := res.Shadows[0]
	if got.Item != systemFileItem(protocolName) {
		t.Errorf("Item = %+v, want system-file %s", got.Item, protocolName)
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

func TestResolve_DeclaredFlagFollowsWinnersDeclaration(t *testing.T) {
	declares := identity.NewOverrides(systemFileItem(protocolName))
	high := testLayerDir(t, "high", declares, protocolName, bundleName)
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 2 {
		t.Fatalf("Shadows = %+v, want one record per file", res.Shadows)
	}
	for _, s := range res.Shadows {
		want := s.Item == systemFileItem(protocolName)
		if s.Declared != want {
			t.Errorf("%s: Declared = %v, want %v", s.Item.Key, s.Declared, want)
		}
	}
}

func TestResolve_DeclarationOfShadowedLayerIsIgnored(t *testing.T) {
	declares := identity.NewOverrides(systemFileItem(bundleName))
	high := testLayerDir(t, "high", identity.Overrides{}, bundleName)
	low := testLayerDir(t, "low", declares, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if len(res.Shadows) != 1 || res.Shadows[0].Declared {
		t.Errorf("Shadows = %+v, want one record with Declared=false", res.Shadows)
	}
}

func TestResolve_ThreeLayers_ShadowListsEveryLowerLayerHighestFirst(t *testing.T) {
	top := testLayerDir(t, "top", identity.Overrides{}, bundleName)
	mid := testLayerDir(t, "mid", identity.Overrides{}, bundleName)
	none := testLayerDir(t, "none", identity.Overrides{})
	low := testLayerDir(t, "low", identity.Overrides{}, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{top, mid, none, low})

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

func TestResolve_FileInOneLayerOnly_HasNoShadowRecord(t *testing.T) {
	only := testLayerDir(t, "only", identity.Overrides{}, protocolName, bundleName)
	other := testLayerDir(t, "other", identity.Overrides{})

	res := systemfile.Resolve([]stackrec.LayerInput{other, only})

	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none", res.Shadows)
	}
	if !res.Protocol.Supplied || !res.Bundle.Supplied {
		t.Errorf("both files must resolve to the only layer that has them: %+v", res)
	}
}
