package app_test

// check_index_catalog_test.go covers the per-catalog reporting of Service.CheckWorkflowIndex:
// one entry per layer for a stack catalog, exactly one identity-less entry for a plain catalog.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
)

func writeIndexFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

const indexHeader = "# Workflows Index\n\n| ID | Category | Version | Name | Description | Hint | File |\n" +
	"|----|----------|---------|------|--------------|------|------|\n"

func writeIndexedWorkflow(t *testing.T, root, id string, listed bool) {
	t.Helper()
	writeIndexFixtureFile(t, root, filepath.Join("Workflows", "Cat", id+".md"),
		"---\nid: "+id+"\nversion: \"1.0.0\"\nname: "+id+"\ndescription: d.\nhint: h.\n---\nBody.\n")
	if listed {
		appendIndexRow(t, root, id)
	}
}

func appendIndexRow(t *testing.T, root, id string) {
	t.Helper()
	p := filepath.Join(root, "Workflows", "Index.md")
	prev, _ := os.ReadFile(p)
	if len(prev) == 0 {
		prev = []byte(indexHeader)
	}
	row := "| " + id + " | Cat | 1.0.0 | " + id + " | d | h | `Cat/" + id + ".md` |\n"
	writeIndexFixtureFile(t, root, filepath.Join("Workflows", "Index.md"), string(prev)+row)
}

func writeLayerIdentity(t *testing.T, root, id string) {
	t.Helper()
	writeIndexFixtureFile(t, root, "catalog-identity.yaml", "id: "+id+"\nname: Name "+id+"\nversion: \"2.0.0\"\n")
}

func newMosaicRootWithCatalogDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeIndexFixtureFile(t, root, filepath.Join("Catalog", "CatalogFilesFormat.md"), "# format\n")
	return root
}

func TestCheckWorkflowIndex_StackCatalog_ReportsOneEntryPerLayerAgainstItsCatalog(t *testing.T) {
	mosaic := newMosaicRootWithCatalogDir(t)
	high, low := t.TempDir(), t.TempDir()
	writeLayerIdentity(t, high, "high")
	writeLayerIdentity(t, low, "low")
	writeIndexedWorkflow(t, high, "wf-high", true)
	writeIndexedWorkflow(t, low, "wf-low", true)
	writeIndexedWorkflow(t, low, "stray", false)
	cat, err := stack.New(mosaic, []stack.Layer{{Root: high}, {Root: low}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	svc := app.New(app.Deps{Catalog: cat, MosaicRoot: mosaic})

	res, err := svc.CheckWorkflowIndex(context.Background())

	if err != nil {
		t.Fatalf("CheckWorkflowIndex: %v", err)
	}
	if len(res.Catalogs) != 2 {
		t.Fatalf("Catalogs has %d entries, want 2 (one per layer): %+v", len(res.Catalogs), res.Catalogs)
	}
	h, l := res.Catalogs[0], res.Catalogs[1]
	if h.Catalog.ID != "high" || h.CatalogRoot != high || !h.Clean {
		t.Errorf("high entry = %+v, want id high, root %s, clean", h, high)
	}
	if l.Catalog.ID != "low" || l.Catalog.Version != "2.0.0" || l.CatalogRoot != low || l.Clean {
		t.Errorf("low entry = %+v, want id low version 2.0.0, root %s, not clean", l, low)
	}
	if len(h.FileOrphans) != 0 || len(l.FileOrphans) != 1 || l.FileOrphans[0].Subject != "Cat/stray.md" {
		t.Errorf("orphans high=%v low=%v, want the stray file reported against low only", h.FileOrphans, l.FileOrphans)
	}
}

func TestCheckWorkflowIndex_StackCatalog_TopLevelFieldsAggregateTheEntries(t *testing.T) {
	mosaic := newMosaicRootWithCatalogDir(t)
	high, low := t.TempDir(), t.TempDir()
	writeLayerIdentity(t, high, "high")
	writeLayerIdentity(t, low, "low")
	writeIndexedWorkflow(t, high, "wf-high", true)
	writeIndexedWorkflow(t, low, "wf-low", true)
	writeIndexedWorkflow(t, low, "stray", false)
	cat, err := stack.New(mosaic, []stack.Layer{{Root: high}, {Root: low}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	svc := app.New(app.Deps{Catalog: cat, MosaicRoot: mosaic})

	res, err := svc.CheckWorkflowIndex(context.Background())

	if err != nil {
		t.Fatalf("CheckWorkflowIndex: %v", err)
	}
	if res.Clean {
		t.Error("Clean = true, want false: drift in a non-top layer makes the aggregate unclean")
	}
	if res.CatalogRoot != high || res.IndexPath != filepath.Join(high, "Workflows", "Index.md") {
		t.Errorf("CatalogRoot/IndexPath = %q / %q, want the highest layer's", res.CatalogRoot, res.IndexPath)
	}
	if res.DiskCount != 3 || res.IndexCount != 2 {
		t.Errorf("DiskCount/IndexCount = %d/%d, want sums 3/2", res.DiskCount, res.IndexCount)
	}
	if len(res.FileOrphans) != 1 || !res.IndexPresent {
		t.Errorf("FileOrphans=%v IndexPresent=%v, want the concatenated orphan and presence true", res.FileOrphans, res.IndexPresent)
	}
}

func TestCheckWorkflowIndex_StackCatalog_TopLayerWithoutIndex_IndexPresentIfAnyLayerHasOne(t *testing.T) {
	mosaic := newMosaicRootWithCatalogDir(t)
	high, low := t.TempDir(), t.TempDir()
	writeLayerIdentity(t, high, "high")
	writeLayerIdentity(t, low, "low")
	writeIndexedWorkflow(t, low, "wf-low", true)
	cat, err := stack.New(mosaic, []stack.Layer{{Root: high}, {Root: low}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	svc := app.New(app.Deps{Catalog: cat, MosaicRoot: mosaic})

	res, err := svc.CheckWorkflowIndex(context.Background())

	if err != nil {
		t.Fatalf("CheckWorkflowIndex: %v", err)
	}
	if res.Catalogs[0].IndexPresent || !res.Catalogs[1].IndexPresent {
		t.Fatalf("entry presence = %v/%v, want false/true", res.Catalogs[0].IndexPresent, res.Catalogs[1].IndexPresent)
	}
	if !res.IndexPresent {
		t.Error("IndexPresent = false, want true: a lower layer has an index")
	}
	if res.IndexPath != filepath.Join(high, "Workflows", "Index.md") {
		t.Errorf("IndexPath = %q, want the highest layer's", res.IndexPath)
	}
}

func TestCheckWorkflowIndex_PlainCatalog_YieldsOneIdentitylessEntryWithTodaysFindings(t *testing.T) {
	mosaic := newMosaicRootWithCatalogDir(t)
	catRoot := filepath.Join(mosaic, "Catalog")
	writeIndexedWorkflow(t, catRoot, "listed", true)
	writeIndexedWorkflow(t, catRoot, "stray", false)
	appendIndexRow(t, catRoot, "ghost") // index row without a file: an index-orphan
	cat, err := catalog.Load(mosaic, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	svc := app.New(app.Deps{Catalog: cat, MosaicRoot: mosaic})

	res, err := svc.CheckWorkflowIndex(context.Background())

	if err != nil {
		t.Fatalf("CheckWorkflowIndex: %v", err)
	}
	if len(res.Catalogs) != 1 {
		t.Fatalf("Catalogs has %d entries, want exactly 1: %+v", len(res.Catalogs), res.Catalogs)
	}
	e := res.Catalogs[0]
	if e.Catalog.ID != "" || e.Catalog.Name != "" || e.Catalog.Version != "" {
		t.Errorf("Catalog identity = %+v, want zero", e.Catalog)
	}
	if e.CatalogRoot != cat.CatalogRoot() {
		t.Errorf("entry CatalogRoot = %q, want %q", e.CatalogRoot, cat.CatalogRoot())
	}
	if e.Clean || len(e.FileOrphans) != 1 || e.FileOrphans[0].Subject != "Cat/stray.md" {
		t.Errorf("entry = %+v, want the stray file-orphan", e)
	}
	if len(e.IndexOrphans) != 1 || e.IndexOrphans[0].Subject != "ghost" {
		t.Errorf("entry IndexOrphans = %+v, want the ghost index-orphan", e.IndexOrphans)
	}
	if e.IndexPath != filepath.Join(catRoot, "Workflows", "Index.md") || !e.IndexPresent {
		t.Errorf("entry IndexPath/IndexPresent = %q/%v, want the catalog's index, present", e.IndexPath, e.IndexPresent)
	}
	if res.Clean != e.Clean || res.CatalogRoot != e.CatalogRoot || res.DiskCount != e.DiskCount ||
		res.IndexCount != e.IndexCount || res.IndexPath != e.IndexPath || res.IndexPresent != e.IndexPresent ||
		!reflect.DeepEqual(res.FileOrphans, e.FileOrphans) || !reflect.DeepEqual(res.IndexOrphans, e.IndexOrphans) {
		t.Errorf("top-level result %+v differs from the single entry %+v", res, e)
	}
}

func TestCheckWorkflowIndex_StackCatalog_UnreadableLayerIndex_ErrorNamesTheLayer(t *testing.T) {
	mosaic := newMosaicRootWithCatalogDir(t)
	high, low := t.TempDir(), t.TempDir()
	writeLayerIdentity(t, high, "high")
	writeLayerIdentity(t, low, "low")
	writeIndexedWorkflow(t, high, "wf-high", true)
	// A directory where Index.md should be makes the low layer's index unreadable.
	writeIndexFixtureFile(t, low, filepath.Join("Workflows", "Index.md", "keep"), "x")
	cat, err := stack.New(mosaic, []stack.Layer{{Root: high}, {Root: low}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	svc := app.New(app.Deps{Catalog: cat, MosaicRoot: mosaic})

	_, err = svc.CheckWorkflowIndex(context.Background())

	if err == nil {
		t.Fatal("CheckWorkflowIndex returned no error for an unreadable layer index")
	}
	if !strings.Contains(err.Error(), low) {
		t.Errorf("error %q does not name the failing layer %s", err, low)
	}
}
