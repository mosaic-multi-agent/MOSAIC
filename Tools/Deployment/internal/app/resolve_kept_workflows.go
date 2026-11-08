package app

// resolve_kept_workflows.go holds the per-path workflow layout of an orchestrator-role file
// and the merge of its kept blocks into the run's catalog blocks.

import "mosaic-deploy/internal/transform"

// DeployedPathWorkflows is the workflow layout of one orchestrator-role target path.
type DeployedPathWorkflows struct {
	Order []string                  // every direct workflow id of the deployed file, in deployed order
	Kept  []transform.WorkflowBlock // this path's kept blocks (Kept true), in deployed order
}

// mergeKeptWorkflows returns the catalogBlocks of the run with one path's kept blocks placed
// into them. Catalog blocks keep their order. Each kept block is placed directly after the
// block of the nearest id that precedes it in layout.Order and is present in the result at
// that moment, or first when there is none. A kept id that is also the id of a catalog block
// is not written. Inputs are not mutated.
func mergeKeptWorkflows(catalogBlocks []transform.WorkflowBlock, layout DeployedPathWorkflows) []transform.WorkflowBlock {
	merged := make([]transform.WorkflowBlock, len(catalogBlocks), len(catalogBlocks)+len(layout.Kept))
	copy(merged, catalogBlocks)
	for _, kept := range layout.Kept {
		if indexOfBlock(merged, kept.ID) >= 0 {
			continue
		}
		at := keptInsertIndex(merged, layout.Order, kept.ID)
		merged = append(merged, transform.WorkflowBlock{})
		copy(merged[at+1:], merged[at:])
		merged[at] = kept
	}
	return merged
}

// keptInsertIndex returns the index in blocks at which the kept block id goes: directly after
// the block of the nearest id before it in order that blocks holds, or 0 when there is none.
func keptInsertIndex(blocks []transform.WorkflowBlock, order []string, id string) int {
	pos := -1
	for i, o := range order {
		if o == id {
			pos = i
			break
		}
	}
	for i := pos - 1; i >= 0; i-- {
		if at := indexOfBlock(blocks, order[i]); at >= 0 {
			return at + 1
		}
	}
	return 0
}

// indexOfBlock returns the index of the block with id, or -1.
func indexOfBlock(blocks []transform.WorkflowBlock, id string) int {
	for i, b := range blocks {
		if b.ID == id {
			return i
		}
	}
	return -1
}
