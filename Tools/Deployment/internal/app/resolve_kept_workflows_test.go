package app

// resolve_kept_workflows_test.go specifies the placement rule of mergeKeptWorkflows: catalog
// blocks keep their order, and each kept block goes directly after the nearest id that
// precedes it in the deployed order and is already in the result, or first when none is.

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/transform"
)

func mergeCatalogBlocks(ids ...string) []transform.WorkflowBlock {
	blocks := make([]transform.WorkflowBlock, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, transform.WorkflowBlock{ID: id, Block: []byte("catalog-" + id)})
	}
	return blocks
}

func mergeKeptBlocks(ids ...string) []transform.WorkflowBlock {
	blocks := make([]transform.WorkflowBlock, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, transform.WorkflowBlock{ID: id, Block: []byte("kept-" + id), Kept: true})
	}
	return blocks
}

func mergedIDs(blocks []transform.WorkflowBlock) string {
	ids := make([]string, 0, len(blocks))
	for _, b := range blocks {
		ids = append(ids, b.ID)
	}
	return strings.Join(ids, ",")
}

func requireMergedIDs(t *testing.T, got []transform.WorkflowBlock, want string) {
	t.Helper()
	if g := mergedIDs(got); g != want {
		t.Errorf("merged ids = %q, want %q", g, want)
	}
}

func TestMergeKeptWorkflows_PredecessorDeselected_PlacedAfterNearestEarlierIdInRun(t *testing.T) {
	// b was deselected from the run; k sat after b, so it follows a.
	layout := DeployedPathWorkflows{Order: []string{"a", "b", "k", "c"}, Kept: mergeKeptBlocks("k")}

	got := mergeKeptWorkflows(mergeCatalogBlocks("a", "c"), layout)

	requireMergedIDs(t, got, "a,k,c")
}

func TestMergeKeptWorkflows_NoPredecessorInRun_PlacedFirst(t *testing.T) {
	// x is neither in the run nor kept, so k has no usable predecessor.
	layout := DeployedPathWorkflows{Order: []string{"x", "k", "a"}, Kept: mergeKeptBlocks("k")}

	got := mergeKeptWorkflows(mergeCatalogBlocks("a"), layout)

	requireMergedIDs(t, got, "k,a")
}

func TestMergeKeptWorkflows_ConsecutiveKeptBlocks_KeepDeployedOrderAfterSameAnchor(t *testing.T) {
	layout := DeployedPathWorkflows{Order: []string{"a", "d", "k1", "k2", "c"}, Kept: mergeKeptBlocks("k1", "k2")}

	got := mergeKeptWorkflows(mergeCatalogBlocks("a", "c"), layout)

	requireMergedIDs(t, got, "a,k1,k2,c")
}

func TestMergeKeptWorkflows_ConsecutiveKeptBlocksWithNoAnchor_KeepDeployedOrderFirst(t *testing.T) {
	layout := DeployedPathWorkflows{Order: []string{"k1", "k2", "a"}, Kept: mergeKeptBlocks("k1", "k2")}

	got := mergeKeptWorkflows(mergeCatalogBlocks("a"), layout)

	requireMergedIDs(t, got, "k1,k2,a")
}

func TestMergeKeptWorkflows_KeptIdAlsoInRun_CatalogBlockWinsAndOtherKeptBlocksStillPlaced(t *testing.T) {
	// k is supplied by the catalog in this run; k2 stays kept and follows k.
	layout := DeployedPathWorkflows{Order: []string{"a", "k", "k2"}, Kept: mergeKeptBlocks("k", "k2")}

	got := mergeKeptWorkflows(mergeCatalogBlocks("a", "k"), layout)

	requireMergedIDs(t, got, "a,k,k2")
	for _, b := range got {
		switch b.ID {
		case "k":
			if b.Kept || !bytes.Equal(b.Block, []byte("catalog-k")) {
				t.Errorf("block k = %+v, want the catalog block", b)
			}
		case "k2":
			if !b.Kept || !bytes.Equal(b.Block, []byte("kept-k2")) {
				t.Errorf("block k2 = %+v, want the kept block with its deployed bytes", b)
			}
		}
	}
}

func TestMergeKeptWorkflows_DoesNotMutateInputs(t *testing.T) {
	catalogBlocks := mergeCatalogBlocks("a", "c")
	layout := DeployedPathWorkflows{Order: []string{"a", "b", "k", "c"}, Kept: mergeKeptBlocks("k")}
	wantCatalog := mergeCatalogBlocks("a", "c")
	wantLayout := DeployedPathWorkflows{Order: []string{"a", "b", "k", "c"}, Kept: mergeKeptBlocks("k")}

	got := mergeKeptWorkflows(catalogBlocks, layout)

	requireMergedIDs(t, got, "a,k,c")
	if !reflect.DeepEqual(catalogBlocks, wantCatalog) {
		t.Errorf("catalog blocks were mutated: %+v", catalogBlocks)
	}
	if !reflect.DeepEqual(layout, wantLayout) {
		t.Errorf("layout was mutated: %+v", layout)
	}
}
