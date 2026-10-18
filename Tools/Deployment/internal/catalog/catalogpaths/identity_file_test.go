package catalogpaths_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

func TestRelIdentityFile_Name(t *testing.T) {
	if catalogpaths.RelIdentityFile != "catalog-identity.yaml" {
		t.Fatalf("RelIdentityFile = %q", catalogpaths.RelIdentityFile)
	}
}

func TestIdentityFile_JoinsCatalogRoot(t *testing.T) {
	root := filepath.Join("some", "catalog")

	got := catalogpaths.IdentityFile(root)

	want := filepath.Join(root, "catalog-identity.yaml")
	if got != want {
		t.Fatalf("IdentityFile = %q, want %q", got, want)
	}
}
