package config

import (
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// yamlScalar renders s as a single-line YAML scalar, quoting it when plain style is unsafe.
func yamlScalar(s string) string {
	data, err := yaml.Marshal(s)
	out := strings.TrimRight(string(data), "\n")
	if err != nil || out == "" || strings.Contains(out, "\n") {
		return strconv.Quote(s)
	}
	return out
}

// quotedList renders names as a flow sequence of double-quoted strings.
func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// entryLines renders one tool_destinations list entry in block style with its dash at indent.
// Destination keys are written in the documented order: to, field, format, separator, names.
func entryLines(w wireToolMapping, indent int) []string {
	pad := strings.Repeat(" ", indent)
	out := []string{pad + "- generic: " + yamlScalar(w.Generic), pad + "  destinations:"}
	for _, d := range w.Destinations {
		keyPad := pad + "      "
		out = append(out, pad+"    - to: "+yamlScalar(d.To))
		if d.Field != "" {
			out = append(out, keyPad+"field: "+yamlScalar(d.Field))
		}
		if d.Format != "" {
			out = append(out, keyPad+"format: "+yamlScalar(d.Format))
		}
		if d.Separator != "" {
			out = append(out, keyPad+"separator: "+strconv.Quote(d.Separator))
		}
		out = append(out, keyPad+"names: "+quotedList(d.Names))
	}
	return out
}

// upsertWireMapping replaces the entry with w's generic in list, or appends w when absent.
func upsertWireMapping(list []wireToolMapping, w wireToolMapping) []wireToolMapping {
	out := append([]wireToolMapping{}, list...)
	for i := range out {
		if out[i].Generic == w.Generic {
			out[i] = w
			return out
		}
	}
	return append(out, w)
}

// leadingSpaces returns the number of leading spaces of l.
func leadingSpaces(l string) int {
	return len(l) - len(strings.TrimLeft(l, " "))
}

// isContentLine reports whether l carries YAML content (is neither blank nor a comment).
func isContentLine(l string) bool {
	return !isBlankOrComment(l)
}

// lastContentIndex returns the index of the last content line in [from, to), or -1.
func lastContentIndex(lines []string, from, to int) int {
	for i := to - 1; i >= from; i-- {
		if isContentLine(lines[i]) {
			return i
		}
	}
	return -1
}

// dedent removes up to n leading spaces from every line and joins the lines with newlines.
func dedent(lines []string, n int) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		cut := leadingSpaces(l)
		if cut > n {
			cut = n
		}
		out[i] = l[cut:]
	}
	return strings.Join(out, "\n")
}
