package artifact

import (
	"bytes"
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// sanitizeCells returns the values made safe to write as table cells: line
// breaks become spaces, surrounding whitespace is trimmed and pipes are escaped.
func sanitizeCells(values ...string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = mdtable.SanitizeCell(v)
	}
	return out
}

// cellText returns the text a table cell carries: trimmed, with escaped pipes
// restored to plain pipes.
func cellText(cell string) string {
	return mdtable.UnescapeCell(strings.TrimSpace(cell))
}

// refuseSection wraps a section parse failure as a refusal that names the
// section and keeps the underlying error as its cause.
func refuseSection(prefix string, cause error) error {
	return &domain.RefusalError{
		Component: "artifact",
		Reason:    prefix + cause.Error(),
		Cause:     cause,
		Section:   "frontmatter",
	}
}

// dropBlankLines removes whitespace-only lines, so a row appended after the
// blank line that precedes a close tag still belongs to the table.
func dropBlankLines(content []byte) []byte {
	var out bytes.Buffer
	for _, line := range bytes.SplitAfter(content, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			out.Write(line)
		}
	}
	return out.Bytes()
}
