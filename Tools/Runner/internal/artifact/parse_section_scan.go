package artifact

import (
	"bytes"
	"errors"
	"strings"

	"mosaic-common/docformat"
	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
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

// refuse builds a refusal for this section. A table structure error
// carries the file line of its row; the cause is kept.
func (sc sectionScan) refuse(prefix string, cause error) error {
	re := &domain.RefusalError{
		Component: "artifact",
		Reason:    prefix + cause.Error(),
		Cause:     cause,
		Section:   sc.name,
	}
	var se *mdtable.StructureError
	if errors.As(cause, &se) {
		re.Line = sc.fileLine(se.Line)
	}
	return re
}

// requireSections checks the required sections are present and closed, in the
// order the original check used.
func requireSections(exec, arts sectionScan) error {
	for _, sc := range []sectionScan{exec, arts} {
		if !sc.openFound {
			return &domain.RefusalError{
				Component: "artifact",
				Reason:    "missing " + sc.openTag + " section",
				Section:   sc.name,
			}
		}
		if !sc.closed {
			return &domain.RefusalError{
				Component: "artifact",
				Reason:    "file appears truncated: missing </" + sc.name + "> closing tag",
				Section:   sc.name,
			}
		}
	}
	return nil
}
