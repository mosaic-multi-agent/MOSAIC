package screens_test

// Fixture for the tools view growth columns. Every growth figure is distinct, so a
// figure found on a row can only come from that row's tool.

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mosaic-log-analyzer/internal/domain"
)

// notProvided is the text of a figure that the log does not carry.
const notProvided = "not provided"

// contentCaptureNote is the sentence the view gives about the content capture
// that VS Code needs for the growth figures.
const contentCaptureNote = "VS Code needs OTel content capture for these figures."

func growthFigures(samples int, avg, min, max int64, negative, notMeasurable int) domain.ToolGrowth {
	return domain.ToolGrowth{
		Samples: samples, Average: domain.Tokens(avg), Min: domain.Tokens(min), Max: domain.Tokens(max),
		Negative: negative, NotMeasurable: notMeasurable,
	}
}

// leftOutOnly is the growth of a tool whose calls were all left out.
func leftOutOnly(negative, notMeasurable int) domain.ToolGrowth {
	return domain.ToolGrowth{Negative: negative, NotMeasurable: notMeasurable}
}

// growthRun is toolsRun with growth figures: Read, Grep, Edit have samples; Bash has
// only left-out calls; Write has no growth at all.
func growthRun() domain.RunReport {
	run := toolsRun()
	run.Tools.Run[0].Growth = leftOutOnly(23, 29) // Bash
	run.Tools.Run[1].Growth = growthFigures(13, 1234, 777, 2468, 5, 7)
	// Glob has samples whose growth is zero: it must show 0, not "not provided".
	run.Tools.Run = append(run.Tools.Run, domain.ToolFigures{Tool: "Glob", Calls: 2, Growth: growthFigures(3, 0, 0, 0, 0, 0)})
	run.Tools.Orchestrator[0].Growth = growthFigures(17, 4321, 321, 9876, 6, 8)
	run.Tools.ByAgentType[0].Tools[0].Growth = growthFigures(19, 5555, 505, 6666, 14, 11)
	return run
}

// olderLogRun is toolsRun as an older log without requested tools gives it: no
// tool has any growth figure.
func olderLogRun() domain.RunReport { return toolsRun() }

// growthView renders the tools view of the run at the width.
func growthView(t *testing.T, run domain.RunReport, w int) string {
	t.Helper()
	return toolsView(t, run, w, layoutH)
}

// flatText is the view without styling, its lines joined by single spaces, so a
// sentence wrapped over lines can be searched as one.
func flatText(view string) string {
	return strings.Join(strings.Fields(stripStyling(view)), " ")
}

func growthTitles(view string) []string { return headerTitles(view, "tool") }

// growthGroup is the label of the group of growth columns, growthColumns its columns.
const growthGroup = "context growth (tokens)"

var growthColumns = []string{"average growth", "minimum growth", "maximum growth", "growth samples", "negative growth", "not measurable"}

// growthHeader is the header of the first table of the view; the stacked tables
// share its column positions.
func growthHeader(t *testing.T, view string) headerBlock {
	t.Helper()
	block, ok := parseHeader(view, "tool")
	if !ok {
		t.Fatalf("no table has \"tool\" as its first column\n%s", stripStyling(view))
	}
	return block
}

// assertGrowthCells fails unless, in the row of the tool, each named column holds
// exactly the wanted text. pairs are column title, wanted cell text, ...
func assertGrowthCells(t *testing.T, view, tool string, pairs ...string) {
	t.Helper()
	block := growthHeader(t, view)
	row := runesOf(rowFor(t, stripStyling(view), tool))
	for i := 0; i+1 < len(pairs); i += 2 {
		col, ok := block.col(pairs[i])
		if !ok {
			t.Errorf("the header has no column %q; its columns are %q", pairs[i], block.titles())
			continue
		}
		if got := fragment(row, col.start, col.end); got != pairs[i+1] {
			t.Errorf("tool %q, column %q = %q, want %q", tool, pairs[i], got, pairs[i+1])
		}
	}
}

// assertGroupOverGrowthColumns fails unless the group label stands over the growth
// columns of the first table and every stacked table carries the label.
func assertGroupOverGrowthColumns(t *testing.T, view string, tables int) {
	t.Helper()
	if n := strings.Count(stripStyling(view), growthGroup); n != tables {
		t.Errorf("the group label %q appears %d times, want once per table (%d)", growthGroup, n, tables)
	}
	block := growthHeader(t, view)
	first, last := -1, 0
	for _, title := range growthColumns {
		if col, ok := block.col(title); ok {
			if first < 0 || col.start < first {
				first = col.start
			}
			last = max(last, col.end)
		}
	}
	for _, l := range viewLines(view) {
		i := strings.Index(l, growthGroup)
		if i < 0 {
			continue
		}
		centre := utf8.RuneCountInString(l[:i]) - gutterWidth + utf8.RuneCountInString(growthGroup)/2
		if centre < first || centre >= last {
			t.Errorf("the group label is centred at column %d, outside the growth columns [%d, %d): %q", centre, first, last, l)
		}
		return
	}
}
