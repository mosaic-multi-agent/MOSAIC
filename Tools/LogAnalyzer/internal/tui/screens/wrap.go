package screens

import (
	"strings"

	"mosaic-common/tui/kit/textwidth"
)

// WrapText breaks s into lines no wider than width cells. Existing line breaks
// are kept, words are moved whole to the next line, and a word wider than width
// is cut. A width of zero or less yields no lines.
func WrapText(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	for _, paragraph := range strings.Split(s, "\n") {
		out = append(out, wrapParagraph(paragraph, width)...)
	}
	return out
}

func wrapParagraph(paragraph string, width int) []string {
	var out []string
	line := ""
	for _, word := range strings.Fields(paragraph) {
		for textwidth.Of(word) > width {
			head, rest := cutCells(word, width)
			if line != "" {
				out = append(out, line)
				line = ""
			}
			out = append(out, head)
			word = rest
		}
		switch {
		case line == "":
			line = word
		case textwidth.Of(line)+1+textwidth.Of(word) <= width:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	return append(out, line)
}

// cutCells splits word after the last rune that still fits width; at least one
// rune is always taken so the loop in the caller ends.
func cutCells(word string, width int) (head, rest string) {
	used := 0
	for i, r := range word {
		w := textwidth.Of(string(r))
		if used+w > width && i > 0 {
			return word[:i], word[i:]
		}
		used += w
	}
	return word, ""
}
