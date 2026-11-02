// Package guidance reads and writes a run's standing-guidance file.
package guidance

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// FileName is the guidance file's name inside the run folder
// (Orchestration-{run_id}/RunnerGuidance.txt).
const FileName = "RunnerGuidance.txt"

const (
	headerTask        = "[task_description]"
	headerConstraints = "[constraints]"
)

const fileComment = "# MOSAIC Runner standing guidance for this run.\n" +
	"# Appended to every engine-routed dispatch. Edit the text under each heading;\n" +
	"# the Runner reads this file when a session starts.\n"

// FormatError reports malformed guidance file content.
type FormatError struct {
	Line   int // 1-based line number
	Reason string
}

// Error returns "line {Line}: {Reason}".
func (e *FormatError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Reason) }

// Format renders g in the guidance file format.
func Format(g domain.Guidance) []byte {
	g = g.Normalized()
	var b strings.Builder
	b.WriteString(fileComment)
	b.WriteString(headerTask + "\n")
	writeField(&b, g.TaskDescription)
	b.WriteString("\n")
	b.WriteString(headerConstraints + "\n")
	writeField(&b, g.Constraints)
	return []byte(b.String())
}

// writeField writes the lines of a field, escaping lines that would otherwise
// read as a header or as an escaped line.
func writeField(b *strings.Builder, field string) {
	if field == "" {
		return
	}
	for _, line := range strings.Split(field, "\n") {
		if isHeader(line) || strings.HasPrefix(line, `\`) {
			b.WriteString(`\`)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// headerOf returns the header name a line stands for (ignoring trailing spaces
// and tabs), or "" when the line is not a header.
func headerOf(line string) string {
	trimmed := strings.TrimRight(line, " \t")
	if trimmed == headerTask || trimmed == headerConstraints {
		return trimmed
	}
	return ""
}

func isHeader(line string) bool { return headerOf(line) != "" }

// Parse reads the guidance file format. On malformed input it returns a
// *FormatError.
func Parse(data []byte) (domain.Guidance, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	fields := map[string][]string{}
	current := ""
	for i, line := range strings.Split(text, "\n") {
		if header := headerOf(line); header != "" {
			if _, seen := fields[header]; seen {
				return domain.Guidance{}, &FormatError{Line: i + 1, Reason: "duplicate " + header + " heading"}
			}
			fields[header] = nil
			current = header
			continue
		}
		if current == "" {
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				continue
			}
			return domain.Guidance{}, &FormatError{Line: i + 1, Reason: "text before the first heading; put it under [task_description] or [constraints]"}
		}
		line = strings.TrimPrefix(line, `\`)
		fields[current] = append(fields[current], line)
	}
	return domain.Guidance{
		TaskDescription: strings.Join(fields[headerTask], "\n"),
		Constraints:     strings.Join(fields[headerConstraints], "\n"),
	}.Normalized(), nil
}
