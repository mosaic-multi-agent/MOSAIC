package artifact

import (
	"bytes"
	"strings"

	"mosaic-common/docformat"
)

// bodyLine is one line of the document body with its 1-based line number in
// the whole file.
type bodyLine struct {
	text string
	no   int
}

// sectionScan is the result of looking for one tagged section in the body.
type sectionScan struct {
	name      string
	openTag   string // opening tag as written by the canonical renderer
	openFound bool
	openLine  int // file line of the opening tag; 0 when not found
	closed    bool
	lines     []int  // file line of each content line
	content   []byte // content lines, blank lines dropped, closing tag removed
}

// splitBodyLines splits an LF-normalised body into numbered lines; the first
// line has number first.
func splitBodyLines(body string, first int) []bodyLine {
	parts := strings.Split(body, "\n")
	lines := make([]bodyLine, len(parts))
	for i, p := range parts {
		lines[i] = bodyLine{text: p, no: first + i}
	}
	return lines
}

// scanSection finds section name in lines. The opening tag must stand alone on
// its line. A line closes the section when, ignoring trailing whitespace, it
// ends with the closing tag; any text before the tag is the last content line.
func scanSection(lines []bodyLine, name string) sectionScan {
	sc := sectionScan{name: name}
	open, err := docformat.RenderOpenTagLine(docformat.NodeSection, name, "")
	if err != nil {
		return sc
	}
	sc.openTag = string(bytes.TrimSpace(open))
	closeTag := "</" + name + ">"

	i := 0
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].text) == sc.openTag {
			sc.openFound = true
			sc.openLine = lines[i].no
			break
		}
	}
	if !sc.openFound {
		return sc
	}

	var buf bytes.Buffer
	add := func(l bodyLine, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		buf.WriteString(text)
		buf.WriteByte('\n')
		sc.lines = append(sc.lines, l.no)
	}
	for i++; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i].text, " \t\r")
		if strings.HasSuffix(trimmed, closeTag) {
			// Tolerated, not damage: the closing tag may follow content on the
			// same line; the text before it is the last content line.
			add(lines[i], strings.TrimSuffix(trimmed, closeTag))
			sc.closed = true
			break
		}
		add(lines[i], lines[i].text)
	}
	sc.content = buf.Bytes()
	return sc
}

// fileLine maps a 1-based line of the section content to its line in the file;
// 0 when out of range.
func (sc sectionScan) fileLine(contentLine int) int {
	if contentLine < 1 || contentLine > len(sc.lines) {
		return 0
	}
	return sc.lines[contentLine-1]
}
