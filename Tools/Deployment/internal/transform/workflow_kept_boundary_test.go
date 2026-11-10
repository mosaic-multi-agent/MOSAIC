package transform_test

// workflow_kept_boundary_test.go covers the boundary rules of transform.KeptWorkflowBlocks.
// A workflow tag in running text is ordinary text and neither opens nor closes a block. A
// whole-line workflow tag pair inside a fenced example is part of the enclosing block. A direct
// workflow opening tag with no closing tag, or an unpaired tag nested in a kept block, is an
// error that names the block left open, with no entries returned.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-deploy/internal/transform"
)

// keptProseClosingTagDeployed quotes a closing workflow tag in running text. The quoted tag must
// not end the block; only the whole-line closing tag does.
const keptProseClosingTagDeployed = "<Workflow type=\"managed\" name=\"prose-flow\" version=\"1.0.0\">\n" +
	"## Prose flow\n" +
	"\n" +
	"Use </Workflow> to close a block\n" +
	"</Workflow>\n"

// keptProseOpeningTagDeployed quotes an opening workflow tag inline. The quoted tag must not open
// an element; the host block still ends at its own whole-line closing tag.
const keptProseOpeningTagDeployed = "<Workflow type=\"managed\" name=\"prose-open-flow\" version=\"1.0.0\">\n" +
	"## Prose opener\n" +
	"\n" +
	"Quote the opener <Workflow type=\"core\" name=\"quoted-inner\"> here.\n" +
	"</Workflow>\n"

// keptFencedPairDeployed holds a whole-line workflow tag pair inside a fenced example. The pair is
// part of the enclosing block's bytes and is not a boundary.
const keptFencedPairDeployed = "<Workflow type=\"managed\" name=\"host-with-example\" version=\"1.0.0\">\n" +
	"## Host with example\n" +
	"\n" +
	"```\n" +
	"<Workflow type=\"core\" name=\"example-inner\">\n" +
	"## Inner example\n" +
	"</Workflow>\n" +
	"```\n" +
	"\n" +
	"</Workflow>\n"

func TestKeptWorkflowBlocks_ClosingTagInRunningText_BlockReturnedWhole(t *testing.T) {
	deployed := deployedOrchestrator("", keptProseClosingTagDeployed+keptSecondDeployed)

	blocks := keptBlocksOrFail(t, deployed)

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].ID != "prose-flow" || blocks[1].ID != "retired-flow" {
		t.Errorf("ids = [%s %s], want [prose-flow retired-flow]", blocks[0].ID, blocks[1].ID)
	}
	if !bytes.Equal(blocks[0].Block, []byte(keptProseClosingTagDeployed)) {
		t.Errorf("prose block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, keptProseClosingTagDeployed)
	}
}

func TestKeptWorkflowBlocks_OpeningTagInRunningText_DoesNotOpenBlock(t *testing.T) {
	deployed := deployedOrchestrator("", keptProseOpeningTagDeployed+keptSecondDeployed)

	blocks := keptBlocksOrFail(t, deployed)

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].ID != "prose-open-flow" || blocks[1].ID != "retired-flow" {
		t.Errorf("ids = [%s %s], want [prose-open-flow retired-flow]", blocks[0].ID, blocks[1].ID)
	}
	if !bytes.Equal(blocks[0].Block, []byte(keptProseOpeningTagDeployed)) {
		t.Errorf("prose block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, keptProseOpeningTagDeployed)
	}
}

func TestKeptWorkflowBlocks_WholeLineTagPairInFence_IsPartOfEnclosingBlock(t *testing.T) {
	deployed := deployedOrchestrator("", keptFencedPairDeployed)

	blocks := keptBlocksOrFail(t, deployed)

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 (the fenced pair is not an entry)", len(blocks))
	}
	if blocks[0].ID != "host-with-example" {
		t.Errorf("id = %q, want host-with-example", blocks[0].ID)
	}
	if !bytes.Equal(blocks[0].Block, []byte(keptFencedPairDeployed)) {
		t.Errorf("enclosing block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, keptFencedPairDeployed)
	}
}

func TestKeptWorkflowBlocks_UnclosedDirectOpeningTag_ErrorNamesThatBlock(t *testing.T) {
	unclosed := "<Workflow type=\"custom\" name=\"unclosed-flow\" version=\"1.0\">\n## Never closed\n"
	complete := "<Workflow type=\"core\" name=\"complete-flow\" version=\"1.0\">\n## Complete\n</Workflow>\n"
	deployed := deployedOrchestrator("", unclosed+complete)

	assertKeptErrorNames(t, deployed, "unclosed-flow", "complete-flow")
}

func TestKeptWorkflowBlocks_UnpairedTagNestedInKeptBlock_ErrorNamesEnclosingBlock(t *testing.T) {
	host := "<Workflow type=\"managed\" name=\"nested-host\" version=\"1.0\">\n" +
		"```\n" +
		"<Workflow type=\"core\" name=\"lone-inner\">\n" +
		"```\n" +
		"</Workflow>\n"
	deployed := deployedOrchestrator("", host+keptSecondDeployed)

	assertKeptErrorNames(t, deployed, "nested-host", "lone-inner")
}

func TestKeptWorkflowBlocks_UnrecognisedTypePairNested_ErrorNamesEnclosingBlock(t *testing.T) {
	host := "<Workflow type=\"managed\" name=\"bogus-host\" version=\"1.0\">\n" +
		"```\n" +
		"<Workflow type=\"bogus\" name=\"bogus-inner\">\n" +
		"## Inner\n" +
		"</Workflow>\n" +
		"```\n" +
		"</Workflow>\n"
	deployed := deployedOrchestrator("", host+keptSecondDeployed)

	assertKeptErrorNames(t, deployed, "bogus-host", "bogus-inner")
}

func TestKeptWorkflowBlocks_NamedPairWithoutTypeNested_ErrorNamesEnclosingBlock(t *testing.T) {
	host := "<Workflow type=\"managed\" name=\"untyped-host\" version=\"1.0\">\n" +
		"```\n" +
		"<Workflow name=\"untyped-inner\">\n" +
		"## Inner\n" +
		"</Workflow>\n" +
		"```\n" +
		"</Workflow>\n"
	deployed := deployedOrchestrator("", host+keptSecondDeployed)

	assertKeptErrorNames(t, deployed, "untyped-host", "untyped-inner")
}

func TestKeptWorkflowBlocks_BarePairNested_ErrorNamesEnclosingBlock(t *testing.T) {
	host := "<Workflow type=\"managed\" name=\"bare-host\" version=\"1.0\">\n" +
		"```\n" +
		"<Workflow>\n" +
		"## Inner\n" +
		"</Workflow>\n" +
		"```\n" +
		"</Workflow>\n"
	deployed := deployedOrchestrator("", host+keptSecondDeployed)

	assertKeptErrorNames(t, deployed, "bare-host", "no-such-nested-name")
}

// assertKeptErrorNames asserts that KeptWorkflowBlocks fails with no entries, that the error names
// wantID, and that it does not name skipID.
func assertKeptErrorNames(t *testing.T, deployed []byte, wantID, skipID string) {
	t.Helper()
	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err == nil {
		t.Fatalf("expected error naming %q, got nil", wantID)
	}
	if blocks != nil {
		t.Errorf("expected nil result with error, got %d blocks", len(blocks))
	}
	if !strings.Contains(err.Error(), wantID) {
		t.Errorf("error %q does not name %q", err, wantID)
	}
	if strings.Contains(err.Error(), skipID) {
		t.Errorf("error %q names %q, which is not the block left open", err, skipID)
	}
}
