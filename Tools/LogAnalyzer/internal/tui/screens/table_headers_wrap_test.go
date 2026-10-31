package screens_test

// Titles wrap at their spaces, so a column is as narrow as its widest value or
// its longest title word (with the sort marker when it is the sort column), and
// the header grows taller instead of wider.

import (
	"testing"
)

// wrappedColumns are multi-word titles whose column is not a flex, share or
// sparkline column, so its width follows its values and its title words alone.
var wrappedColumns = map[string][]string{
	"dispatches":               {"peak context", "cache hit", "cache write", "write cost", "net cache", "unread cache writes"},
	"by type":                  {"average peak context", "maximum peak context", "cache hit", "cache write", "write cost", "repeat spend", "likely rework", "rework cost"},
	"tools":                    {"average duration", "minimum duration", "maximum duration"},
	"context":                  {"API calls", "first context", "context at first write", "peak context", "final context", "detected only"},
	"overview models":          {"API calls"},
	"across runs, agent types": {"average per run", "invocations per run"},
}

func TestTableWrap_ColumnIsNoWiderThanItsWidestValueOrLongestTitleWord(t *testing.T) {
	for name, titles := range wrappedColumns {
		t.Run(name, func(t *testing.T) {
			block := readHeader(t, viewNamed(t, name), viewAnchor(t, name))

			for _, title := range titles {
				col, ok := block.col(title)
				if !ok {
					t.Errorf("no column is titled %q; the titles are %q", title, block.titles())
					continue
				}
				if width, need := col.end-col.start, col.neededWidth(); width != need {
					t.Errorf("column %q is %d cells wide, want %d (widest value %d, longest title word %d)",
						title, width, need, col.cellWidth, col.longestWord())
				}
			}
		})
	}
}

func TestTableWrap_MultiWordTitleSitsOnOneLinePerWordWhenTheValuesAreNarrower(t *testing.T) {
	block := readHeader(t, viewNamed(t, "context"), "instance")

	col, ok := block.col("API calls")
	if !ok {
		t.Fatalf("no column is titled \"API calls\"; the titles are %q", block.titles())
	}
	if col.titleLines != 2 {
		t.Errorf("\"API calls\" fills %d header lines, want 2 (a column of one-digit counts is narrower than the title)", col.titleLines)
	}
	if block.headerLines < 3 {
		t.Errorf("the Context header has %d lines, want its longest title and the compaction group label (at least 3)", block.headerLines)
	}
}

func TestTableWrap_TitlesAreBottomAlignedSoTheLastLineSitsAboveTheRows(t *testing.T) {
	block := readHeader(t, viewNamed(t, "dispatches"), "instance")

	peak, ok := block.col("peak context")
	if !ok {
		t.Fatalf("no column is titled \"peak context\"; the titles are %q", block.titles())
	}
	instance, _ := block.col("instance")
	if peak.titleLines < 2 {
		t.Errorf("\"peak context\" fills %d header line, want 2: a column of numbers is narrower than the title", peak.titleLines)
	}
	if !peak.onBottom || !instance.onBottom {
		t.Errorf("the last line of every title must be the bottom header line, right above the rows: peak context %v, instance %v",
			peak.onBottom, instance.onBottom)
	}
}

func TestTableWrap_WrappedHeadersStayWithinTheTerminalAtEveryWidth(t *testing.T) {
	for _, w := range []int{80, 120, 180} {
		for _, name := range []string{"dispatches", "by type", "context", "tools", "by status"} {
			view := narrowView(t, name, w)

			assertWithinBounds(t, view, w, headerH)
		}
	}
}
