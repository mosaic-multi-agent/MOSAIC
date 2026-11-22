package app

// promote_regions.go decides what happens to the regions of a source agent when it becomes a
// generic source: project regions are kept (or emptied) and are always present, managed
// regions are emptied, and custom regions are removed. The rules apply at any nesting depth.

import (
	"bytes"
	"errors"

	"mosaic-common/docformat"
)

// errRegionNotFound reports a child node whose bytes could not be located in its parent's
// content; it cannot happen for a document the parser produced.
var errRegionNotFound = errors.New("region node not found in its parent content")

// removedCustomRegionNames names every custom region of a generic source document, at any
// depth, in document order. Duplicates are kept.
func removedCustomRegionNames(doc *docformat.Document) []string {
	var names []string
	for _, n := range doc.Body().CustomRegions() {
		names = append(names, n.Name())
	}
	return names
}

// regionWalker rewrites a document body according to the promote region rules.
type regionWalker struct {
	// empty empties every project region instead of keeping its content.
	empty bool
	// err is the first failure met while walking; the result is then unusable.
	err error
}

// rewriteBody returns the promoted form of body, the serialised body of a document whose
// top-level nodes are given.
func (w *regionWalker) rewriteBody(body []byte, top []*docformat.Node) ([]byte, error) {
	out := w.sequence(body, top)
	return out, w.err
}

// sequence rewrites content, whose direct child nodes are children, copying the text between
// children verbatim and replacing each child by its promoted form.
func (w *regionWalker) sequence(content []byte, children []*docformat.Node) []byte {
	var out bytes.Buffer
	cur := 0
	for _, child := range children {
		raw := child.Bytes()
		start := indexAtLineStart(content, cur, raw)
		if start < 0 {
			w.fail(errRegionNotFound)
			return content
		}
		out.Write(content[cur:start])
		out.Write(w.node(child))
		cur = start + len(raw)
	}
	out.Write(content[cur:])
	return out.Bytes()
}

// indexAtLineStart finds raw in content at or after from, at the start of a line.
func indexAtLineStart(content []byte, from int, raw []byte) int {
	for from <= len(content) {
		i := bytes.Index(content[from:], raw)
		if i < 0 {
			return -1
		}
		pos := from + i
		if pos == 0 || content[pos-1] == '\n' {
			return pos
		}
		from = pos + 1
	}
	return -1
}

func (w *regionWalker) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// node returns the promoted form of one node.
func (w *regionWalker) node(n *docformat.Node) []byte {
	switch n.Kind() {
	case docformat.NodeCustom:
		return bytes.Join(w.salvage(n), nil)
	case docformat.NodeDeployed:
		return w.emptied(n)
	case docformat.NodeInjection:
		if w.empty {
			return w.emptied(n)
		}
	}
	return w.rewritten(n)
}

// rewritten keeps n's own tags and rewrites its content.
func (w *regionWalker) rewritten(n *docformat.Node) []byte {
	open, closeTag := splitTags(n)
	content := w.sequence(n.Content(), n.Children())
	return concat(open, content, closeTag)
}

// emptied returns n with its content erased apart from the project regions inside it, which
// become its direct children in document order.
func (w *regionWalker) emptied(n *docformat.Node) []byte {
	open, closeTag, err := emptiedTags(n)
	if err != nil {
		w.fail(err)
		return n.Bytes()
	}
	terminator := lineTerminator(open)
	var kids bytes.Buffer
	for _, region := range w.salvage(n) {
		kids.Write(region)
		if !bytes.HasSuffix(region, []byte("\n")) {
			kids.Write(terminator)
		}
	}
	return concat(open, kids.Bytes(), closeTag)
}

// salvage returns the promoted form of every outermost project region inside n, in document
// order. Everything else inside n is dropped.
func (w *regionWalker) salvage(n *docformat.Node) [][]byte {
	var regions [][]byte
	for _, child := range n.Children() {
		if child.Kind() == docformat.NodeInjection {
			regions = append(regions, w.node(child))
			continue
		}
		regions = append(regions, w.salvage(child)...)
	}
	return regions
}

// splitTags returns n's open tag line and the bytes of its close tag as they stand.
func splitTags(n *docformat.Node) (open, closeTag []byte) {
	raw := n.Bytes()
	open = firstLine(raw)
	closeTag = raw[len(open)+len(n.Content()):]
	return open, closeTag
}

// emptiedTags returns the tags of n as they read once n is emptied: the same as Node.Clear
// leaves them (a version attribute on the open tag is dropped).
func emptiedTags(n *docformat.Node) (open, closeTag []byte, err error) {
	doc, err := ParseGenericSource(n.Bytes())
	if err != nil {
		return nil, nil, err
	}
	nodes := doc.Body().TopLevelNodes()
	if len(nodes) == 0 {
		return nil, nil, errRegionNotFound
	}
	if err := nodes[0].Clear(); err != nil {
		return nil, nil, err
	}
	open, closeTag = splitTags(nodes[0])
	return open, closeTag, nil
}

// firstLine returns the first line of b including its terminator.
func firstLine(b []byte) []byte {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return b[:i+1]
	}
	return b
}

// lineTerminator returns the line terminator the line ends with, defaulting to LF.
func lineTerminator(line []byte) []byte {
	if bytes.HasSuffix(line, []byte("\r\n")) {
		return []byte("\r\n")
	}
	return []byte("\n")
}

func concat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

// removedCustomOf names the custom regions of the canonical source bytes; nil when the source
// does not parse or has none.
func removedCustomOf(source []byte) []string {
	doc, err := ParseGenericSource(source)
	if err != nil {
		return nil
	}
	return removedCustomRegionNames(doc)
}
