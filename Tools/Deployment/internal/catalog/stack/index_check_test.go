package stack_test

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
)

// writeLayerIndex writes <root>/Workflows/Index.md listing the given "id|category/file.md" rows.
func writeLayerIndex(t *testing.T, root string, rows ...string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("# Workflows Index\n\n| ID | Category | Version | Name | Description | Hint | File |\n")
	sb.WriteString("|----|----------|---------|------|--------------|------|------|\n")
	for _, r := range rows {
		parts := strings.SplitN(r, "|", 2)
		cat := strings.SplitN(parts[1], "/", 2)[0]
		sb.WriteString("| " + parts[0] + " | " + cat + " | 1.0.0 | N | D | H | `" + parts[1] + "` |\n")
	}
	mustWrite(t, root, filepath.Join("Workflows", "Index.md"), sb.String())
}

func TestCheckIndex_MismatchInOneLayer_ReportedAgainstThatLayerOnly(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Cat", "wf-high", "1.0.0")
	writeLayerIndex(t, high, "wf-high|Cat/wf-high.md")
	addTestWorkflow(t, low, "Cat", "wf-low", "1.0.0")
	addTestWorkflow(t, low, "Cat", "stray", "1.0.0") // on disk, missing from the low layer's index
	writeLayerIndex(t, low, "wf-low|Cat/wf-low.md")
	c := newTestStack(t, mosaic, high, low)

	reports, err := stack.CheckIndex(c)

	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want one per layer", len(reports))
	}
	if reports[0].Layer.Identity.ID != "high" || reports[1].Layer.Identity.ID != "low" {
		t.Fatalf("layer order = %q, %q; want high then low", reports[0].Layer.Identity.ID, reports[1].Layer.Identity.ID)
	}
	if !reports[0].Report.Clean() || reports[0].Report.CatalogRoot != high {
		t.Errorf("high report = %+v, want clean and rooted at %s", reports[0].Report, high)
	}
	low0 := reports[1].Report
	if low0.Clean() || len(low0.FileOrphans) != 1 || low0.FileOrphans[0].Subject != "Cat/stray.md" {
		t.Fatalf("low report = %+v, want exactly the stray file-orphan", low0)
	}
	if !underRoot(low0.FileOrphans[0].Path, low) {
		t.Errorf("orphan path %q is not under the low layer %q", low0.FileOrphans[0].Path, low)
	}
}

func TestCheckIndex_LayerWithoutIndex_IsCleanAndIndexAbsent(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Cat", "wf", "1.0.0")
	writeLayerIndex(t, high, "wf|Cat/wf.md")
	c := newTestStack(t, mosaic, high, low)

	reports, err := stack.CheckIndex(c)

	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}
	if len(reports) != 2 || !reports[1].Report.Clean() || reports[1].Report.IndexPresent {
		t.Fatalf("reports = %+v, want two with the low one clean and index absent", reports)
	}
	if !reports[0].Report.IndexPresent {
		t.Error("high layer report says its index is absent")
	}
}

func TestCheckIndex_UnreadableIndexInLayer_ErrorNamesTheLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	// A directory where Index.md should be makes the index unreadable.
	mustWrite(t, low, filepath.Join("Workflows", "Index.md", "keep"), "x")
	c := newTestStack(t, mosaic, high, low)

	reports, err := stack.CheckIndex(c)

	if err == nil {
		t.Fatalf("CheckIndex = %+v, want an error for the unreadable index", reports)
	}
	assertContains(t, err.Error(), low)
}
