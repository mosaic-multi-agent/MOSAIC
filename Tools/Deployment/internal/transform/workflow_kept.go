package transform

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"mosaic-common/docformat"
)

const (
	availableWorkflowsRegion = "AvailableWorkflows"
	workflowNamePrefix       = "Workflow:"
	workflowTagName          = "Workflow"
)

// Whole-line tag patterns. Each matches the trimmed content of one line, so a tag that shares
// its line with other content is not a match.
var (
	wholeWorkflowOpenPattern  = regexp.MustCompile(`^<Workflow(?:\s(?:[^<>"']|"[^"]*"|'[^']*')*)?>$`)
	wholeWorkflowClosePattern = regexp.MustCompile(`^</Workflow\s*>$`)
	workflowTagStartPattern   = regexp.MustCompile(`^</?Workflow(?:[\s>/]|$)`)
	regionClosePattern        = regexp.MustCompile(`^</AvailableWorkflows\s*>$`)
)

// KeptWorkflowBlocks returns one WorkflowBlock per direct Workflow child of the
// AvailableWorkflows region in deployed, in document order, whatever the id.
// Kept is false on every returned block; the caller sets it.
//
// Block extents come from the docformat parse tree: each block's bytes are the node's raw bytes,
// from the start of its opening tag line through the line break of its closing tag line. A
// workflow tag in running text is ordinary text. A line that begins with a workflow tag and
// carries more content is an error naming its line number, as is a direct block that is never
// closed. A file with no region, or an empty one, yields no blocks. A file that cannot be parsed,
// or that breaks one of these rules, yields an error and no blocks.
func KeptWorkflowBlocks(deployed []byte) ([]WorkflowBlock, error) {
	doc, err := docformat.Parse(deployed)
	if err != nil {
		return nil, err
	}
	region, ok := doc.Body().Deployed(availableWorkflowsRegion)
	if !ok {
		return nil, nil
	}
	regionBytes := region.Bytes()
	firstLine, err := regionFirstLine(deployed, regionBytes)
	if err != nil {
		return nil, err
	}
	if err := checkSameLineWorkflowTags(regionBytes, firstLine); err != nil {
		return nil, err
	}
	// Blocks are checked first: an unclosed block lends its closing tag to the region, so the
	// region is only reported unclosed when no block is.
	blocks, err := directWorkflowBlocks(region)
	if err != nil {
		return nil, err
	}
	if !closedBy(region, regionClosePattern) {
		return nil, fmt.Errorf("deployed %s region is never closed", availableWorkflowsRegion)
	}
	return blocks, nil
}

// regionFirstLine returns the 1-based line of deployed on which regionBytes starts.
func regionFirstLine(deployed, regionBytes []byte) (int, error) {
	at := bytes.Index(deployed, regionBytes)
	if at < 0 {
		return 0, fmt.Errorf("deployed %s region cannot be located", availableWorkflowsRegion)
	}
	return 1 + bytes.Count(deployed[:at], []byte("\n")), nil
}

// checkSameLineWorkflowTags rejects a line whose trimmed content begins with a workflow tag and
// carries more content. The error names the 1-based line number of that line in the deployed file.
func checkSameLineWorkflowTags(region []byte, firstLine int) error {
	for i, raw := range bytes.Split(region, []byte("\n")) {
		trimmed := bytes.TrimSpace(raw)
		if workflowTagStartPattern.Match(trimmed) && !isWholeWorkflowTag(trimmed) {
			return fmt.Errorf("deployed %s line %d: a workflow tag shares its line with other content",
				availableWorkflowsRegion, firstLine+i)
		}
	}
	return nil
}

// directWorkflowBlocks returns the Workflow children of region in document order. A child that
// is not closed by its own whole-line closing tag, or whose workflow tags do not balance within
// its bytes, is an error naming its id.
func directWorkflowBlocks(region *docformat.Node) ([]WorkflowBlock, error) {
	var blocks []WorkflowBlock
	for _, child := range region.Children() {
		id, ok := workflowIDOf(child.Name())
		if !ok {
			continue
		}
		if !closedBy(child, wholeWorkflowClosePattern) {
			return nil, unclosedWorkflowError(id)
		}
		if !workflowTagsBalanced(child) {
			return nil, unbalancedWorkflowError(id)
		}
		blocks = append(blocks, WorkflowBlock{ID: id, Block: child.Bytes()})
	}
	return blocks, nil
}

// workflowTagsBalanced reports whether the whole-line workflow tags of n, matched by element name
// whatever their type, pair up so that the first closing tag that ends the block is its last line.
// docformat treats a tag with an unrecognised type as text, so its closing tag can end the
// enclosing block early; that shows here as an unbalanced run of tags.
func workflowTagsBalanced(n *docformat.Node) bool {
	lines := bytes.Split(bytes.TrimRight(n.Bytes(), "\r\n"), []byte("\n"))
	depth := 0
	for i, raw := range lines {
		trimmed := bytes.TrimSpace(raw)
		switch {
		case wholeWorkflowOpenPattern.Match(trimmed):
			depth++
		case wholeWorkflowClosePattern.Match(trimmed):
			depth--
		}
		if depth == 0 && i < len(lines)-1 {
			return false
		}
	}
	return depth == 0
}

// closedBy reports whether n has a closing tag whose line matches closer. The parser pairs a
// closing tag with the innermost open node, so a node whose closing tag was taken by a
// surrounding or nested node is caught by the check on its last line.
func closedBy(n *docformat.Node, closer *regexp.Regexp) bool {
	if !n.Closed() {
		return false
	}
	lines := bytes.Split(bytes.TrimRight(n.Bytes(), "\r\n"), []byte("\n"))
	return closer.Match(bytes.TrimSpace(lines[len(lines)-1]))
}

// workflowIDOf returns the workflow id carried by a node name, and whether the name is a
// workflow tag at all. The tag name must be exactly Workflow, so WorkflowExtras is not one.
func workflowIDOf(name string) (string, bool) {
	if name == workflowTagName {
		return "", true
	}
	if strings.HasPrefix(name, workflowNamePrefix) {
		return strings.TrimPrefix(name, workflowNamePrefix), true
	}
	return "", false
}

func isWholeWorkflowTag(trimmed []byte) bool {
	return wholeWorkflowOpenPattern.Match(trimmed) || wholeWorkflowClosePattern.Match(trimmed)
}

func unclosedWorkflowError(id string) error {
	return fmt.Errorf("deployed workflow %q is never closed", id)
}

func unbalancedWorkflowError(id string) error {
	return fmt.Errorf("deployed workflow %q holds a workflow tag that does not pair within it", id)
}
