package transform

// workflow_kept_assembly_test.go covers assembleWorkflowBlocks with kept blocks: a kept
// block is emitted byte for byte (no retype, no Row column, ragged table and nested custom
// region untouched), a catalog block in the same list is still retyped and gains a Row
// column, and emitted order equals list order with kept and catalog blocks interleaved.

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// keptRaggedBlock is a deployed block that a rebuild must not touch: its routing table has
// a short row and a long row, and a custom region is nested inside it.
const keptRaggedBlock = `<Workflow type="core" name="kept-flow" version="2.1">
## Kept Flow

| Phase | Subagent | HITL |
|-------|----------|:----:|
| PLANNING | planner |
| EXECUTION.Build.[StageNumber] | implementer | FALSE | extra |

<ProjectNotes type="custom">
Local notes inside a kept workflow.
</ProjectNotes>
</Workflow>
`

// keptSecondBlock is a second kept block, used to check that kept blocks keep their order.
const keptSecondBlock = `<Workflow type="core" name="kept-second" version="0.3">
## Second Kept Flow

| Phase | Subagent |
|-------|----------|
| FINALIZATION | summarizer |
</Workflow>
`

// catalogFlowBlock is a catalog workflow, written with the core boundary tag and a routing
// table that has no Row column yet.
const catalogFlowBlock = `<Workflow type="core" name="catalog-flow" version="1.0">
## Catalog Flow

| Phase | Subagent |
|-------|----------|
| PLANNING | planner |

</Workflow>
`

func TestAssembleWorkflowBlocks_KeptBlock_EmittedByteForByte(t *testing.T) {
	blocks := []WorkflowBlock{{ID: "kept-flow", Block: []byte(keptRaggedBlock), Kept: true}}

	assembled, _ := assembleWorkflowBlocks(blocks)

	if !bytes.Equal(assembled, []byte(keptRaggedBlock)) {
		t.Errorf("kept block changed on emission:\ngot:  %q\nwant: %q", assembled, keptRaggedBlock)
	}
}

func TestAssembleWorkflowBlocks_CatalogBlock_StillRetypedWithRowColumn(t *testing.T) {
	blocks := []WorkflowBlock{{ID: "catalog-flow", Block: []byte(catalogFlowBlock)}}

	assembled, _ := assembleWorkflowBlocks(blocks)
	got := string(assembled)

	if !strings.HasPrefix(got, `<Workflow type="managed" name="catalog-flow" version="1.0">`) {
		t.Errorf("catalog opening tag not retyped to managed: %q", got)
	}
	if strings.Contains(got, `type="core"`) {
		t.Errorf("catalog block still carries type=core: %q", got)
	}
	if !strings.Contains(got, "| Row | Phase | Subagent |") {
		t.Errorf("catalog routing table has no leading Row column: %q", got)
	}
	if !strings.Contains(got, "| 1 | PLANNING | planner |") {
		t.Errorf("catalog data row not numbered 1: %q", got)
	}
}

func TestAssembleWorkflowBlocks_InputBlocks_NotMutated(t *testing.T) {
	keptBefore := bytes.Clone([]byte(keptRaggedBlock))
	catalogBefore := bytes.Clone([]byte(catalogFlowBlock))
	blocks := []WorkflowBlock{
		{ID: "kept-flow", Block: []byte(keptRaggedBlock), Kept: true},
		{ID: "catalog-flow", Block: []byte(catalogFlowBlock)},
	}
	listBefore := slices.Clone(blocks)

	assembleWorkflowBlocks(blocks)

	if !slices.EqualFunc(blocks, listBefore, func(a, b WorkflowBlock) bool {
		return a.ID == b.ID && a.Kept == b.Kept && bytes.Equal(a.Block, b.Block)
	}) {
		t.Error("input block list changed during assembly")
	}
	if !bytes.Equal(blocks[0].Block, keptBefore) {
		t.Errorf("kept block bytes mutated:\ngot:  %q\nwant: %q", blocks[0].Block, keptBefore)
	}
	if !bytes.Equal(blocks[1].Block, catalogBefore) {
		t.Errorf("catalog block bytes mutated:\ngot:  %q\nwant: %q", blocks[1].Block, catalogBefore)
	}
}

func TestAssembleWorkflowBlocks_KeptAndCatalogInterleaved_EmittedInListOrder(t *testing.T) {
	blocks := []WorkflowBlock{
		{ID: "kept-flow", Block: []byte(keptRaggedBlock), Kept: true},
		{ID: "catalog-flow", Block: []byte(catalogFlowBlock)},
		{ID: "kept-second", Block: []byte(keptSecondBlock), Kept: true},
	}

	assembled, ids := assembleWorkflowBlocks(blocks)

	if !slices.Equal(ids, []string{"kept-flow", "catalog-flow", "kept-second"}) {
		t.Errorf("emitted ids = %v, want list order [kept-flow catalog-flow kept-second]", ids)
	}
	if !bytes.HasPrefix(assembled, []byte(keptRaggedBlock)) {
		t.Error("first kept block is not emitted at the head of the output")
	}
	catalogAt := bytes.Index(assembled, []byte(`name="catalog-flow"`))
	secondAt := bytes.Index(assembled, []byte(keptSecondBlock))
	if catalogAt < 0 || secondAt < 0 {
		t.Fatalf("catalog or second kept block missing from output: %q", assembled)
	}
	if !(catalogAt < secondAt) {
		t.Errorf("catalog block (at %d) not emitted before second kept block (at %d)", catalogAt, secondAt)
	}
	if !bytes.HasSuffix(assembled, []byte(keptSecondBlock)) {
		t.Error("second kept block is not emitted byte for byte at the end of the output")
	}
}
