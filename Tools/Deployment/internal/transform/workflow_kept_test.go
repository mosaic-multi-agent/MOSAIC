package transform_test

// workflow_kept_test.go covers transform.KeptWorkflowBlocks: one block per direct Workflow
// child of the deployed AvailableWorkflows region, holding its exact bytes in document
// order. Nested custom regions stay inside their block; direct custom children and
// non-workflow content are ignored; a missing or empty region yields no entries; a file
// that cannot be parsed returns an error and no entries.

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"mosaic-deploy/internal/transform"
)

// keptFirstDeployed is a deployed workflow with a nested custom region. Its bytes must come
// back whole, including the nested region and the final line break.
const keptFirstDeployed = `<Workflow type="managed" name="quick-fix" version="0.0.1-local">
## Quick Fix Workflow

| Phase | Subagent |
|-------|----------|
| PLANNING | planner |

<QuickFixNotes type="custom">
Nested notes inside the first workflow.
</QuickFixNotes>
</Workflow>
`

// keptSecondDeployed is a deployed workflow whose id is in no catalog.
const keptSecondDeployed = `<Workflow type="managed" name="retired-flow" version="9.9">
## A workflow that is no longer in any catalog
</Workflow>
`

// keptDirectCustomChild is a custom region directly under AvailableWorkflows, not a workflow.
const keptDirectCustomChild = `<WorkflowExtras type="custom">
Direct custom child, not a workflow.
</WorkflowExtras>
`

const keptFreeText = "Free text between sections.\n"

func TestKeptWorkflowBlocks_OneBlockPerWorkflowChild_ExactBytesInDocumentOrder(t *testing.T) {
	deployed := deployedOrchestrator("", keptFirstDeployed+keptDirectCustomChild+keptFreeText+keptSecondDeployed)

	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (direct workflow children only)", len(blocks))
	}
	if blocks[0].ID != "quick-fix" || blocks[1].ID != "retired-flow" {
		t.Errorf("ids = [%s %s], want [quick-fix retired-flow]", blocks[0].ID, blocks[1].ID)
	}
	if !bytes.Equal(blocks[0].Block, []byte(keptFirstDeployed)) {
		t.Errorf("first block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, keptFirstDeployed)
	}
	if !bytes.Equal(blocks[1].Block, []byte(keptSecondDeployed)) {
		t.Errorf("second block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[1].Block, keptSecondDeployed)
	}
}

func TestKeptWorkflowBlocks_NestedCustomRegion_KeptInsideItsBlock(t *testing.T) {
	deployed := deployedOrchestrator("", keptFirstDeployed)

	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if !bytes.Contains(blocks[0].Block, []byte("Nested notes inside the first workflow.")) {
		t.Errorf("nested custom region missing from block bytes: %q", blocks[0].Block)
	}
}

func TestKeptWorkflowBlocks_ReturnedBlocksNotMarkedKept(t *testing.T) {
	deployed := deployedOrchestrator("", keptFirstDeployed+keptSecondDeployed)

	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 so that the Kept check below has blocks to inspect", len(blocks))
	}

	for _, b := range blocks {
		if b.Kept {
			t.Errorf("block %q returned with Kept=true; the caller sets Kept", b.ID)
		}
	}
}

// keptEmptyCase pairs a deployed file that holds no workflow child with a control file that
// differs only by one direct workflow child. The control proves the reader parses the region,
// so an empty result alone cannot pass.
type keptEmptyCase struct {
	deployed []byte
	control  []byte
}

func TestKeptWorkflowBlocks_NoWorkflowChildren_ReturnsNoEntries(t *testing.T) {
	cases := map[string]keptEmptyCase{
		"region is empty": {
			deployed: deployedOrchestrator("", ""),
			control:  deployedOrchestrator("", keptSecondDeployed),
		},
		"region is absent": {
			deployed: []byte(deployedFrontmatter + "\n<Identity type=\"core\">\nOld file without the region.\n</Identity>\n"),
			control:  []byte(deployedFrontmatter + "\n<Identity type=\"core\">\n<AvailableWorkflows type=\"managed\">\n" + keptSecondDeployed + "</AvailableWorkflows>\n</Identity>\n"),
		},
		"only custom child": {
			deployed: deployedOrchestrator("", keptDirectCustomChild),
			control:  deployedOrchestrator("", keptDirectCustomChild+keptSecondDeployed),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertKeptIDs(t, tc.control, []string{"retired-flow"})
			assertKeptIDs(t, tc.deployed, nil)
		})
	}
}

// assertKeptIDs checks that KeptWorkflowBlocks returns no error and exactly the given ids, in order.
func assertKeptIDs(t *testing.T, deployed []byte, want []string) {
	t.Helper()
	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(want))
	}
	for i, id := range want {
		if blocks[i].ID != id {
			t.Errorf("block %d id = %q, want %q", i, blocks[i].ID, id)
		}
	}
}

func TestKeptWorkflowBlocks_CRLFDeployedFile_ReturnsExactCRLFBytes(t *testing.T) {
	crlfBlock := toCRLF(keptFirstDeployed)
	deployed := deployedOrchestrator("", crlfBlock)

	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if !bytes.Equal(blocks[0].Block, []byte(crlfBlock)) {
		t.Errorf("CRLF block bytes changed:\ngot:  %q\nwant: %q", blocks[0].Block, crlfBlock)
	}
}

// TestKeptWorkflowBlocks_IndentedTagsAndTrailingWhitespace_BlockKeepsThem covers the raw line
// extent of a block: indentation before the opening tag and whitespace after the closing tag,
// before its line break, are part of the block's bytes under LF and CRLF line endings.
func TestKeptWorkflowBlocks_IndentedTagsAndTrailingWhitespace_BlockKeepsThem(t *testing.T) {
	lfBlock := "  <Workflow type=\"managed\" name=\"indented-flow\" version=\"1.0.0\">\n" +
		"## Indented flow\n" +
		"  </Workflow>  \n"

	cases := map[string]string{
		"LF":   lfBlock,
		"CRLF": toCRLF(lfBlock),
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			deployed := deployedOrchestrator("", block)

			blocks := keptBlocksOrFail(t, deployed)

			if len(blocks) != 1 {
				t.Fatalf("got %d blocks, want 1", len(blocks))
			}
			if blocks[0].ID != "indented-flow" {
				t.Errorf("id = %q, want indented-flow", blocks[0].ID)
			}
			if !bytes.Equal(blocks[0].Block, []byte(block)) {
				t.Errorf("block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, block)
			}
		})
	}
}

// TestKeptWorkflowBlocks_WorkflowTagFollowedOnSameLine_ReturnsErrorAndNoEntries covers the
// boundary rule: a line whose trimmed content begins with a workflow tag and carries more content
// on that line is not read as a boundary, so the call fails and returns no entries.
func TestKeptWorkflowBlocks_WorkflowTagFollowedOnSameLine_ReturnsErrorAndNoEntries(t *testing.T) {
	t.Run("closing tag followed by opening tag", func(t *testing.T) {
		firstBlock := strings.TrimSuffix(keptFirstDeployed, "\n")
		deployed := deployedOrchestrator("", firstBlock+keptSecondDeployed)
		assertKeptBlocksErrorAtLine(t, deployed, "</Workflow><Workflow")
	})

	t.Run("opening tag followed by its own closing tag", func(t *testing.T) {
		oneLine := "<Workflow type=\"core\" name=\"one-line-flow\" version=\"1.0\">## One line</Workflow>\n"
		deployed := deployedOrchestrator("", oneLine)
		assertKeptBlocksErrorAtLine(t, deployed, "<Workflow type=\"core\" name=\"one-line-flow\"")
	})

	t.Run("opening tag followed by trailing text", func(t *testing.T) {
		trailing := "<Workflow type=\"core\" name=\"trailing-open-flow\" version=\"1.0\"> trailing text\n## Body\n</Workflow>\n"
		deployed := deployedOrchestrator("", trailing)
		assertKeptBlocksErrorAtLine(t, deployed, "<Workflow type=\"core\" name=\"trailing-open-flow\"")
	})

	t.Run("closing tag followed by plain text", func(t *testing.T) {
		trailing := "<Workflow type=\"core\" name=\"trailing-close-flow\" version=\"1.0\">\n## Body\n</Workflow> trailing text\n"
		deployed := deployedOrchestrator("", trailing)
		assertKeptBlocksErrorAtLine(t, deployed, "</Workflow> trailing text")
	})
}

// assertKeptBlocksErrorAtLine asserts that KeptWorkflowBlocks fails and returns no entries, and
// that the error carries the 1-based line number of the offending line: the first line of deployed
// that contains marker. The line number is checked by presence only, not by error wording.
func assertKeptBlocksErrorAtLine(t *testing.T, deployed []byte, marker string) {
	t.Helper()
	wantLine := lineNumberOf(t, deployed, marker)
	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err == nil {
		t.Fatal("expected error for a workflow tag followed by more content on its line, got nil")
	}
	if blocks != nil {
		t.Errorf("expected nil result with error, got %d blocks", len(blocks))
	}
	if !strings.Contains(err.Error(), strconv.Itoa(wantLine)) {
		t.Errorf("error %q does not carry line number %d of the offending line", err, wantLine)
	}
}

// lineNumberOf returns the 1-based line number of the first line of deployed that contains marker.
func lineNumberOf(t *testing.T, deployed []byte, marker string) int {
	t.Helper()
	idx := bytes.Index(deployed, []byte(marker))
	if idx < 0 {
		t.Fatalf("marker %q not found in deployed bytes", marker)
	}
	return 1 + bytes.Count(deployed[:idx], []byte("\n"))
}

// keptBlocksOrFail returns the blocks of deployed, failing the test if KeptWorkflowBlocks errors.
func keptBlocksOrFail(t *testing.T, deployed []byte) []transform.WorkflowBlock {
	t.Helper()
	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}
	return blocks
}

// TestKeptWorkflowBlocks_OpeningTagWithoutClosingTag_ReturnsErrorNamingID covers an opening
// workflow tag with no matching whole-line closing tag before the end of the region.
func TestKeptWorkflowBlocks_OpeningTagWithoutClosingTag_ReturnsErrorNamingID(t *testing.T) {
	orphan := "<Workflow type=\"custom\" name=\"orphan-flow\" version=\"1.0\">\n## Never closed\n"
	deployed := deployedOrchestrator("", orphan+keptSecondDeployed)

	blocks, err := transform.KeptWorkflowBlocks(deployed)

	if err == nil {
		t.Fatal("expected error for opening tag without closing tag, got nil")
	}
	if !strings.Contains(err.Error(), "orphan-flow") {
		t.Errorf("error %q does not name the id orphan-flow", err)
	}
	if blocks != nil {
		t.Errorf("expected nil result with error, got %d blocks", len(blocks))
	}
}

func TestKeptWorkflowBlocks_UnparseableDeployedFile_ReturnsErrorAndNoEntries(t *testing.T) {
	unclosed := []byte("---\nversion: 6.0.0\n<Identity type=\"core\">\n" + keptFirstDeployed)

	blocks, err := transform.KeptWorkflowBlocks(unclosed)

	if err == nil {
		t.Fatal("expected error for unparseable deployed file, got nil")
	}
	if blocks != nil {
		t.Errorf("expected nil result with error, got %d blocks", len(blocks))
	}
}
