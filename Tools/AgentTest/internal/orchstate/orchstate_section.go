package orchstate

import (
	"bytes"
	"strings"
)

// executionLogScan is the located ExecutionLog section of a document.
type executionLogScan struct {
	openFound bool
	closed    bool
	openLine  int    // 1-based file line of the opening tag
	content   []byte // lines between the tags; the text before a trailing closing tag is the last line
}

// scanExecutionLog finds the ExecutionLog section in the document text. The
// opening tag stands alone on its line. A line closes the section when,
// ignoring trailing whitespace, it ends with the closing tag; any text before
// the tag is the last content line. Line endings may be LF or CRLF.
func scanExecutionLog(data []byte) executionLogScan {
	closeTag := "</" + executionLogSection + ">"
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	var sc executionLogScan
	var buf bytes.Buffer
	for i, line := range lines {
		if !sc.openFound {
			if isExecutionLogOpenTag(line) {
				sc.openFound = true
				sc.openLine = i + 1
			}
			continue
		}
		trimmed := strings.TrimRight(line, " \t\r")
		if strings.HasSuffix(trimmed, closeTag) {
			buf.WriteString(strings.TrimSuffix(trimmed, closeTag))
			buf.WriteByte('\n')
			sc.closed = true
			break
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	sc.content = buf.Bytes()
	return sc
}

// isExecutionLogOpenTag reports whether line is exactly an ExecutionLog
// opening tag (attributes allowed), ignoring surrounding whitespace.
func isExecutionLogOpenTag(line string) bool {
	t := strings.TrimSpace(line)
	prefix := "<" + executionLogSection
	if !strings.HasPrefix(t, prefix) || !strings.HasSuffix(t, ">") {
		return false
	}
	rest := t[len(prefix):]
	return rest == ">" || rest[0] == ' ' || rest[0] == '\t'
}

// rowFileLine returns the file line of data row index (0 = first row after the
// separator) of the first table in the section content, or 0 when unknown.
func (sc executionLogScan) rowFileLine(index int) int {
	pipeLines := 0
	for i, line := range strings.Split(string(sc.content), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		if pipeLines == index+2 {
			return sc.openLine + 1 + i
		}
		pipeLines++
	}
	return 0
}
