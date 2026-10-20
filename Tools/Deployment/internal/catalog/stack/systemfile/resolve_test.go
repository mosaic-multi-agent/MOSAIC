package systemfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

func TestResolve_BothLayersHaveBoth_HigherLayerSuppliesEachFile(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, protocolName, bundleName)
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if !res.Protocol.Supplied || res.Protocol.Path != filepath.Join(high.Layer.Root, protocolName) {
		t.Errorf("Protocol = %+v, want the high layer's file", res.Protocol)
	}
	if !res.Bundle.Supplied || res.Bundle.Path != filepath.Join(high.Layer.Root, bundleName) {
		t.Errorf("Bundle = %+v, want the high layer's file", res.Bundle)
	}
	if res.Protocol.Name != protocolName || res.Bundle.Name != bundleName {
		t.Errorf("names = %q, %q, want %q, %q", res.Protocol.Name, res.Bundle.Name, protocolName, bundleName)
	}
}

func TestResolve_FileOnlyInLowerLayer_ResolvesToLowerLayer(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{})
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if res.Protocol.Path != filepath.Join(low.Layer.Root, protocolName) || !res.Protocol.Supplied {
		t.Errorf("Protocol = %+v, want the low layer's file", res.Protocol)
	}
	if res.Bundle.Path != filepath.Join(low.Layer.Root, bundleName) || !res.Bundle.Supplied {
		t.Errorf("Bundle = %+v, want the low layer's file", res.Bundle)
	}
}

func TestResolve_ProtocolAndBundleCanResolveFromDifferentLayers(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, bundleName)
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if res.Protocol.Path != filepath.Join(low.Layer.Root, protocolName) {
		t.Errorf("Protocol.Path = %q, want the low layer's file", res.Protocol.Path)
	}
	if res.Bundle.Path != filepath.Join(high.Layer.Root, bundleName) {
		t.Errorf("Bundle.Path = %q, want the high layer's file", res.Bundle.Path)
	}
	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none: each file exists in one layer only", res.Shadows)
	}
}

func TestResolve_NoLayerHasFile_YieldsNamedNotSuppliedResult(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, bundleName)
	low := testLayerDir(t, "low", identity.Overrides{}, bundleName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if res.Protocol.Supplied || res.Protocol.Path != "" || !reflect.DeepEqual(res.Protocol.Supplier, domain.CatalogLayer{}) {
		t.Errorf("Protocol = %+v, want unsupplied with empty path and zero supplier", res.Protocol)
	}
	err := res.Protocol.Err()
	var nse *systemfile.NotSuppliedError
	if !errors.As(err, &nse) || nse.Name != protocolName {
		t.Fatalf("Protocol.Err() = %v, want *NotSuppliedError naming %s", err, protocolName)
	}
	if !errors.Is(err, systemfile.ErrNotSupplied) {
		t.Errorf("Protocol.Err() = %v, want errors.Is ErrNotSupplied", err)
	}
	if res.Bundle.Err() != nil {
		t.Errorf("Bundle.Err() = %v, want nil for a supplied file", res.Bundle.Err())
	}
}

func TestResolve_EmptyLayerList_NeitherFileSupplied(t *testing.T) {
	res := systemfile.Resolve(nil)

	var nse *systemfile.NotSuppliedError
	if !errors.As(res.Protocol.Err(), &nse) || nse.Name != protocolName {
		t.Errorf("Protocol.Err() = %v, want NotSuppliedError for %s", res.Protocol.Err(), protocolName)
	}
	if !errors.As(res.Bundle.Err(), &nse) || nse.Name != bundleName {
		t.Errorf("Bundle.Err() = %v, want NotSuppliedError for %s", res.Bundle.Err(), bundleName)
	}
}

func TestResolve_DirectoryWithFileName_DoesNotSupplyAndLowerLayerIsChecked(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{})
	if err := os.Mkdir(filepath.Join(high.Layer.Root, protocolName), 0o755); err != nil {
		t.Fatal(err)
	}
	low := testLayerDir(t, "low", identity.Overrides{}, protocolName)

	res := systemfile.Resolve([]stackrec.LayerInput{high, low})

	if res.Protocol.Path != filepath.Join(low.Layer.Root, protocolName) {
		t.Errorf("Protocol.Path = %q, want the low layer's file", res.Protocol.Path)
	}
	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none: the directory is not a supplier", res.Shadows)
	}
}

func TestResolution_Err_SuppliedIsNilAndUnsuppliedNamesFile(t *testing.T) {
	if err := (systemfile.Resolution{Name: bundleName, Supplied: true, Path: "/x/" + bundleName}).Err(); err != nil {
		t.Errorf("supplied Err() = %v, want nil", err)
	}
	err := systemfile.Resolution{Name: bundleName}.Err()
	var nse *systemfile.NotSuppliedError
	if !errors.As(err, &nse) || nse.Name != bundleName {
		t.Errorf("unsupplied Err() = %v, want *NotSuppliedError naming %s", err, bundleName)
	}
}
