package injections_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

func TestResolve_BothLayersSupplyHarness_HigherLayerDirectoryWins(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: {sharedFile}})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	got := res.Lookup(claudeDir)
	wantDir := injDir(high.Layer.Root, claudeDir)
	if !got.Supplied || got.Dir != wantDir {
		t.Fatalf("Lookup = %+v, want supplied from %q", got, wantDir)
	}
	if got.HarnessDir != claudeDir {
		t.Errorf("HarnessDir = %q, want %q", got.HarnessDir, claudeDir)
	}
	if got.Err() != nil {
		t.Errorf("Err() = %v, want nil for a supplied harness", got.Err())
	}
}

// The higher layer's directory is used whole: the orchestrator file that only the lower layer
// has is not reachable through the resolved directory.
func TestResolve_NothingIsMergedAcrossLayers(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: {sharedFile}})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})

	got := injections.Resolve([]stackrec.LayerInput{high, low}).Lookup(claudeDir)

	if _, err := os.Stat(filepath.Join(got.Dir, orchFile)); !os.IsNotExist(err) {
		t.Errorf("%s reachable through the resolved dir (err=%v); lower-layer files must not be selected", orchFile, err)
	}
	if _, err := os.Stat(filepath.Join(got.Dir, sharedFile)); err != nil {
		t.Errorf("%s missing from the winning dir: %v", sharedFile, err)
	}
}

func TestResolve_HarnessOnlyInLowerLayer_ResolvesToLowerLayer(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, nil)
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})

	got := injections.Resolve([]stackrec.LayerInput{high, low}).Lookup(claudeDir)

	if !got.Supplied || got.Dir != injDir(low.Layer.Root, claudeDir) {
		t.Errorf("Lookup = %+v, want the low layer's directory", got)
	}
}

func TestResolve_DifferentHarnessesResolveFromDifferentLayers(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{ghcpDir: bothFiles()})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{
		claudeDir: bothFiles(), ghcpDir: bothFiles(), openDir: bothFiles(),
	})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if got := res.Lookup(ghcpDir); got.Dir != injDir(high.Layer.Root, ghcpDir) {
		t.Errorf("ghcp Dir = %q, want the high layer's", got.Dir)
	}
	for _, name := range []string{claudeDir, openDir} {
		if got := res.Lookup(name); got.Dir != injDir(low.Layer.Root, name) {
			t.Errorf("%s Dir = %q, want the low layer's", name, got.Dir)
		}
	}
}

func TestResolve_NoLayerSupplies_YieldsNamedNotSuppliedResult(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})
	low := testLayerDir(t, "low", identity.Overrides{}, nil)

	got := injections.Resolve([]stackrec.LayerInput{high, low}).Lookup(openDir)

	if got.Supplied || got.Dir != "" || !reflect.DeepEqual(got.Supplier, domain.CatalogLayer{}) {
		t.Errorf("Lookup = %+v, want unsupplied with empty Dir and zero Supplier", got)
	}
	if got.HarnessDir != openDir {
		t.Errorf("HarnessDir = %q, want %q", got.HarnessDir, openDir)
	}
	err := got.Err()
	var nse *injections.NotSuppliedError
	if !errors.As(err, &nse) || nse.HarnessDir != openDir {
		t.Fatalf("Err() = %v, want *NotSuppliedError naming %q", err, openDir)
	}
	if !errors.Is(err, injections.ErrNotSupplied) {
		t.Errorf("Err() = %v, want errors.Is ErrNotSupplied", err)
	}
	if !strings.Contains(err.Error(), "HarnessInjections/"+openDir) {
		t.Errorf("error text %q must name the injection item HarnessInjections/%s", err, openDir)
	}
}

func TestResolve_EmptyLayerList_NothingSupplied(t *testing.T) {
	res := injections.Resolve(nil)

	var nse *injections.NotSuppliedError
	if !errors.As(res.Lookup(claudeDir).Err(), &nse) || nse.HarnessDir != claudeDir {
		t.Errorf("Err() = %v, want NotSuppliedError for %q", res.Lookup(claudeDir).Err(), claudeDir)
	}
	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none", res.Shadows)
	}
}

func TestResolve_EmptyHarnessDirectory_StillSuppliesAndHidesLowerLayer(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, map[string][]string{claudeDir: nil})
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	got := res.Lookup(claudeDir)
	if !got.Supplied || got.Dir != injDir(high.Layer.Root, claudeDir) {
		t.Errorf("Lookup = %+v, want the empty high directory to supply", got)
	}
	if len(res.Shadows) != 1 {
		t.Errorf("Shadows = %+v, want one record for the shadowed lower directory", res.Shadows)
	}
}

func TestResolve_NonDirectoryEntryInInjectionsDir_IsIgnored(t *testing.T) {
	high := testLayerDir(t, "high", identity.Overrides{}, nil)
	parent := filepath.Join(high.Layer.Root, "HarnessInjections")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, claudeDir), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	low := testLayerDir(t, "low", identity.Overrides{}, map[string][]string{claudeDir: bothFiles()})

	res := injections.Resolve([]stackrec.LayerInput{high, low})

	if got := res.Lookup(claudeDir); got.Dir != injDir(low.Layer.Root, claudeDir) {
		t.Errorf("Lookup = %+v, want the low layer: a plain file does not supply", got)
	}
	if len(res.Shadows) != 0 {
		t.Errorf("Shadows = %+v, want none", res.Shadows)
	}
}

func TestResolution_Err_SuppliedIsNilAndUnsuppliedNamesHarness(t *testing.T) {
	if err := (injections.Resolution{HarnessDir: ghcpDir, Supplied: true, Dir: "/x"}).Err(); err != nil {
		t.Errorf("supplied Err() = %v, want nil", err)
	}
	err := injections.Resolution{HarnessDir: ghcpDir}.Err()
	var nse *injections.NotSuppliedError
	if !errors.As(err, &nse) || nse.HarnessDir != ghcpDir {
		t.Errorf("unsupplied Err() = %v, want *NotSuppliedError naming %q", err, ghcpDir)
	}
}
