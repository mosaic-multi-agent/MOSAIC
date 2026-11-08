package transform

// workflow_kept_region.go holds the parts of workflow region assembly that keep a kept block's
// bytes whole: line-ending conversion skips kept blocks, and a custom region nested inside a
// kept block is not placed a second time.

import "mosaic-common/docformat"

// assembleWorkflowRegionBytes assembles blocks and converts line endings to eol. Blocks marked
// Kept are emitted exactly as given; every other block is converted. When no block is kept the
// whole region is converted at once.
func assembleWorkflowRegionBytes(blocks []WorkflowBlock, eol string) (assembled []byte, ids []string) {
	if !hasKeptBlock(blocks) {
		assembled, ids = assembleWorkflowBlocks(blocks)
		return ConvertLineEndings(assembled, eol), ids
	}
	ids = make([]string, 0, len(blocks))
	for _, b := range blocks {
		part, partIDs := assembleWorkflowBlocks([]WorkflowBlock{b})
		if !b.Kept {
			part = ConvertLineEndings(part, eol)
		}
		assembled = append(assembled, part...)
		ids = append(ids, partIDs...)
	}
	return assembled, ids
}

func hasKeptBlock(blocks []WorkflowBlock) bool {
	for _, b := range blocks {
		if b.Kept {
			return true
		}
	}
	return false
}

// holdsCustomRegion reports whether the output body already contains a custom region named name,
// as when a kept workflow block carries it inside its own bytes.
func holdsCustomRegion(body *docformat.Body, name string) bool {
	_, ok := body.Custom(name)
	return ok
}
