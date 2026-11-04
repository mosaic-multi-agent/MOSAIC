package screens

import (
	"strings"

	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// Indents of a reason block: its first line and the lines that continue it.
const (
	reasonIndent         = "  "
	reasonContinueIndent = "    "
)

// unpricedReasonLines renders, for each model of unpriced that has a reason, a
// block "<model>: <reason>" in the warning style, in the order of unpriced.
// Models without a reason get no block. No line is wider than width.
func unpricedReasonLines(st Styles, width int, unpriced []domain.PricingKey, reasons []domain.UnpricedReason) []string {
	byKey := make(map[domain.PricingKey]string, len(reasons))
	for _, r := range reasons {
		byKey[r.Key] = r.Reason
	}
	var lines []string
	for _, key := range unpriced {
		reason, ok := byKey[key]
		if !ok || reason == "" {
			continue
		}
		for _, l := range wrapIndented(key.String()+": "+reason, width) {
			lines = append(lines, st.Warning.Render(l))
		}
	}
	return lines
}

// wrapIndented breaks text into lines of at most width cells: the first indented
// by reasonIndent, the rest by reasonContinueIndent. A word wider than its line is
// cut with an ellipsis; the words after it are kept.
func wrapIndented(text string, width int) []string {
	var out []string
	line, indent := "", reasonIndent
	flush := func() {
		if line != "" {
			out = append(out, indent+line)
			line, indent = "", reasonContinueIndent
		}
	}
	for _, word := range strings.Fields(text) {
		avail := max(width-len(indent), 1)
		if textwidth.Of(word) > avail {
			word = textwidth.Truncate(word, avail)
		}
		switch {
		case line == "":
			line = word
		case textwidth.Of(line)+1+textwidth.Of(word) <= avail:
			line += " " + word
		default:
			flush()
			if avail = max(width-len(indent), 1); textwidth.Of(word) > avail {
				word = textwidth.Truncate(word, avail)
			}
			line = word
		}
	}
	flush()
	return out
}
