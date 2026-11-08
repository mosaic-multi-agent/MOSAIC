package transform_test

// workflow_kept_inline_test.go covers KeptWorkflowBlocks for a workflow body that quotes a
// workflow opening tag inline, in prose rather than as a whole line. The quoted tag must not
// open an element: the host workflow still ends at its own closing tag, and the workflow that
// follows it is still returned.

import (
	"bytes"
	"testing"

	"mosaic-deploy/internal/transform"
)

// keptInlineTokenDeployed is a deployed workflow whose prose quotes a type-bearing workflow
// opening tag mid-line. Its own closing tag ends the block.
const keptInlineTokenDeployed = "<Workflow type=\"managed\" name=\"host-flow\" version=\"1.0.0\">\n" +
	"## Host flow\n" +
	"\n" +
	"Write the block as `<Workflow type=\"core\" name=\"inner-token\">` in the output.\n" +
	"</Workflow>\n"

func TestKeptWorkflowBlocks_InlineWorkflowTokenInBody_HostAndLaterWorkflowsReturned(t *testing.T) {
	deployed := deployedOrchestrator("", keptInlineTokenDeployed+keptSecondDeployed)

	blocks, err := transform.KeptWorkflowBlocks(deployed)
	if err != nil {
		t.Fatalf("KeptWorkflowBlocks returned error: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (host workflow and the workflow after it)", len(blocks))
	}
	if blocks[0].ID != "host-flow" || blocks[1].ID != "retired-flow" {
		t.Errorf("ids = [%s %s], want [host-flow retired-flow]", blocks[0].ID, blocks[1].ID)
	}
	if !bytes.Equal(blocks[0].Block, []byte(keptInlineTokenDeployed)) {
		t.Errorf("host block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[0].Block, keptInlineTokenDeployed)
	}
	if !bytes.Equal(blocks[1].Block, []byte(keptSecondDeployed)) {
		t.Errorf("following block bytes differ from deployed bytes:\ngot:  %q\nwant: %q", blocks[1].Block, keptSecondDeployed)
	}
}
