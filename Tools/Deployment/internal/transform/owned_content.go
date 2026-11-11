package transform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

var (
	utf8BOMBytes = []byte{0xEF, 0xBB, 0xBF}
	crlfBytesSeq = []byte("\r\n")
	lfBytesSeq   = []byte("\n")
)

// OwnedContentOf computes the Deploy-owned content record of a file's bytes.
//
// kind ArtifactAgent: body only (frontmatter excluded); the content of every region whose
// boundary tag has type="project", and of the infrastructure declarations region, is excluded
// (its boundary tags are included); every custom
// region is excluded entirely, tags included.
// kind ArtifactSkill, ArtifactHook: the whole file.
// The result is identical for inputs that differ only in a leading UTF-8 BOM or in CRLF vs LF
// line endings. Error only for an agent whose body cannot be parsed.
func OwnedContentOf(kind domain.ArtifactKind, content []byte) (domain.OwnedContent, error) {
	normalised := normaliseForOwnedContent(content)
	if kind != domain.ArtifactAgent {
		return domain.OwnedContent{Hash: ownedHash(normalised)}, nil
	}
	return agentOwnedContent(normalised)
}

// DiffRegions returns the names of regions whose hash differs or that exist on one side only,
// in recorded order then current order, each once. Empty when either side has no Regions.
func DiffRegions(recorded, current domain.OwnedContent) []string {
	if len(recorded.Regions) == 0 || len(current.Regions) == 0 {
		return nil
	}
	recordedHash := regionHashIndex(recorded.Regions)
	currentHash := regionHashIndex(current.Regions)
	var names []string
	seen := make(map[string]bool)
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, r := range recorded.Regions {
		if h, ok := currentHash[r.Name]; !ok || h != recordedHash[r.Name] {
			add(r.Name)
		}
	}
	for _, r := range current.Regions {
		if h, ok := recordedHash[r.Name]; !ok || h != currentHash[r.Name] {
			add(r.Name)
		}
	}
	return names
}

func regionHashIndex(regions []domain.RegionHash) map[string]string {
	idx := make(map[string]string, len(regions))
	for _, r := range regions {
		idx[r.Name] = r.Hash
	}
	return idx
}

// normaliseForOwnedContent drops a leading BOM and rewrites CRLF to LF.
func normaliseForOwnedContent(content []byte) []byte {
	content = bytes.TrimPrefix(content, utf8BOMBytes)
	return bytes.ReplaceAll(content, crlfBytesSeq, lfBytesSeq)
}

func ownedHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// agentOwnedContent computes the record of a normalised agent file: the body with project
// region content and custom regions left out. Whitespace around text runs and region blocks
// is not part of the record, so adding or removing a custom region together with its blank
// lines does not register as an edit.
func agentOwnedContent(normalised []byte) (domain.OwnedContent, error) {
	_, body, err := docformat.SplitFrontmatter(normalised)
	if err != nil {
		return domain.OwnedContent{}, fmt.Errorf("split frontmatter: %w", err)
	}
	// A synthetic empty frontmatter keeps a body that starts with "---" from being
	// mistaken for frontmatter.
	doc, err := docformat.Parse(append([]byte("---\n---\n"), body...))
	if err != nil {
		return domain.OwnedContent{}, fmt.Errorf("parse body: %w", err)
	}
	pieces, err := splitPieces(body, doc.Body().TopLevelNodes())
	if err != nil {
		return domain.OwnedContent{}, err
	}

	var outside, all bytes.Buffer
	var regions []domain.RegionHash
	for _, p := range pieces {
		if p.node == nil {
			appendLine(&outside, p.text)
			appendLine(&all, p.text)
			continue
		}
		owned, err := ownedNodeBytes(p.node)
		if err != nil {
			return domain.OwnedContent{}, err
		}
		appendLine(&all, owned)
		if p.node.Kind() != docformat.NodeCustom {
			regions = append(regions, domain.RegionHash{Name: p.node.Name(), Hash: ownedHash(owned)})
		}
	}
	regions = append([]domain.RegionHash{{Name: domain.OutsideRegionsName, Hash: ownedHash(outside.Bytes())}}, regions...)
	return domain.OwnedContent{Hash: ownedHash(all.Bytes()), Regions: regions}, nil
}

// piece is a text run or a region node of a body or region content, in document order.
type piece struct {
	text []byte
	node *docformat.Node
}

// splitPieces interleaves the text between the given sibling nodes with the nodes, locating
// each node's bytes in content from left to right.
func splitPieces(content []byte, nodes []*docformat.Node) ([]piece, error) {
	var pieces []piece
	rest := content
	for _, n := range nodes {
		nb := n.Bytes()
		at := bytes.Index(rest, nb)
		if at < 0 {
			return nil, fmt.Errorf("region %q cannot be located in the body", n.Name())
		}
		if at > 0 {
			pieces = append(pieces, piece{text: rest[:at]})
		}
		pieces = append(pieces, piece{node: n})
		rest = rest[at+len(nb):]
	}
	if len(rest) > 0 {
		pieces = append(pieces, piece{text: rest})
	}
	return pieces, nil
}

// ownedNodeBytes returns the Deploy-owned bytes of one region: nothing for a custom region,
// the boundary tags only for a project region, and the tags with owned content otherwise.
func ownedNodeBytes(n *docformat.Node) ([]byte, error) {
	if !n.Closed() {
		return nil, fmt.Errorf("region %q has no closing tag", n.Name())
	}
	if n.Kind() == docformat.NodeCustom {
		return nil, nil
	}
	whole := n.Bytes()
	content := n.Content()
	open := whole[:bytes.IndexByte(whole, '\n')+1]
	closeTag := whole[len(open)+len(content):]

	var out bytes.Buffer
	appendLine(&out, open)
	if !contentExcluded(n) {
		pieces, err := splitPieces(content, n.Children())
		if err != nil {
			return nil, err
		}
		for _, p := range pieces {
			if p.node == nil {
				appendLine(&out, p.text)
				continue
			}
			owned, err := ownedNodeBytes(p.node)
			if err != nil {
				return nil, err
			}
			appendLine(&out, owned)
		}
	} else if err := requireClosedChildren(n); err != nil {
		return nil, err
	}
	appendLine(&out, closeTag)
	return out.Bytes(), nil
}

// requireClosedChildren fails when any region nested in n is unclosed.
func requireClosedChildren(n *docformat.Node) error {
	for _, c := range n.Children() {
		if !c.Closed() {
			return fmt.Errorf("region %q has no closing tag", c.Name())
		}
		if err := requireClosedChildren(c); err != nil {
			return err
		}
	}
	return nil
}

// appendLine writes the whitespace-trimmed text followed by a line feed; empty text is skipped.
func appendLine(buf *bytes.Buffer, text []byte) {
	trimmed := bytes.TrimSpace(text)
	if len(trimmed) == 0 {
		return
	}
	buf.Write(trimmed)
	buf.WriteByte('\n')
}

// infrastructureRegionName is the managed region that holds the infrastructure agent
// declarations.
const infrastructureRegionName = "InfrastructureAgents"

// contentExcluded reports whether the content of a region is left out of the Deploy-owned
// content (its boundary tags stay in): project regions, and the infrastructure declarations
// region, whose declarations Deploy merges with the ones already in the file instead of
// replacing them, so a declaration added there is kept and is not an edit to be undone.
func contentExcluded(n *docformat.Node) bool {
	return n.Kind() == docformat.NodeInjection || n.Name() == infrastructureRegionName
}
