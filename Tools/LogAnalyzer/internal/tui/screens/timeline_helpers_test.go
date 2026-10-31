package screens_test

// Helpers of the timeline tests: opening the screen, finding the lane of an
// invocation among the lines of a view, reading glyph columns off a lane and
// matching the lines of the detail strip.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// Glyphs the Gantt draws in Unicode mode with colour off.
const (
	barGlyph      = '█' // interval cell
	loggedGlyph   = '│' // logged compaction marker
	detectedGlyph = '┊' // detected compaction marker
	openEndGlyph  = '>' // last cell of an open-ended bar
)

// openTimeline switches colour off and opens the timeline of a run.
func openTimeline(t *testing.T, run domain.RunReport, w, h int) *screens.TimelineScreen {
	t.Helper()
	colourOff(t)
	return screens.NewTimelineScreen(run, w, h, paletteStyles())
}

// timelineView renders the timeline of a run with colour off.
func timelineView(t *testing.T, run domain.RunReport, w, h int) string {
	t.Helper()
	return openTimeline(t, run, w, h).View(w, h)
}

// lineIndex returns the index of the first line containing needle, or -1.
func lineIndex(view, needle string) int {
	for i, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return i
		}
	}
	return -1
}

var openLabelToken = regexp.MustCompile(`(^|[^0-9A-Za-z#])` + regexp.QuoteMeta(screens.OpenLabel) + `($|[^0-9A-Za-z#])`)

var timeLabelRE = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)

// timeLabels are the clock labels (hh:mm:ss) of a line.
func timeLabels(line string) []string { return timeLabelRE.FindAllString(line, -1) }

// isLaneLine reports whether the styling-free line is the lane of the invocation
// id: it starts with the id as its label and does not carry the type, which only
// the detail strip's first line does.
func isLaneLine(line, id, typ string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, id) {
		return false
	}
	if rest := trimmed[len(id):]; rest != "" && rest[0] != ' ' {
		return false
	}
	return typ == "" || !strings.Contains(line, typ)
}

// laneIndex is the index of the lane of invocation id in the view, or -1.
func laneIndex(view, id, typ string) int {
	for i, l := range viewLines(view) {
		if isLaneLine(l, id, typ) {
			return i
		}
	}
	return -1
}

// laneLine is the styling-free lane of invocation id; the test stops when it is
// missing.
func laneLine(t *testing.T, view, id, typ string) string {
	t.Helper()
	lines := viewLines(view)
	i := laneIndex(view, id, typ)
	if i < 0 {
		t.Fatalf("no lane for %s\n%s", id, stripStyling(view))
	}
	return lines[i]
}

// rawLaneLine is the lane of invocation id with its escape sequences.
func rawLaneLine(t *testing.T, view, id, typ string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if isLaneLine(stripStyling(l), id, typ) {
			return l
		}
	}
	t.Fatalf("no lane for %s\n%s", id, stripStyling(view))
	return ""
}

// orchestratorLaneIndex is the index of the orchestrator lane, or -1.
func orchestratorLaneIndex(view string) int {
	for i, l := range viewLines(view) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "orchestrator") {
			return i
		}
	}
	return -1
}

// axisLineIndex is the index of the first line carrying at least three clock
// labels: the time axis (the detail strip never has more than two).
func axisLineIndex(view string) int {
	for i, l := range viewLines(view) {
		if len(timeLabels(l)) >= 3 {
			return i
		}
	}
	return -1
}

// glyphCols are the display columns of a glyph on a styling-free line.
func glyphCols(line string, glyph rune) []int {
	var cols []int
	for i, r := range []rune(line) {
		if r == glyph {
			cols = append(cols, i)
		}
	}
	return cols
}

func countGlyph(line string, glyph rune) int { return len(glyphCols(line, glyph)) }

// lastCol is the display column of the last visible cell of a line, or -1.
func lastCol(line string) int { return len([]rune(strings.TrimRight(line, " "))) - 1 }

// lineWithAll returns the first styling-free line holding every needle, or "".
func lineWithAll(view string, needles ...string) string {
next:
	for _, l := range viewLines(view) {
		for _, n := range needles {
			if !strings.Contains(l, n) {
				continue next
			}
		}
		return l
	}
	return ""
}

// stripLine is lineWithAll that fails the test when no line holds every needle.
func stripLine(t *testing.T, view string, needles ...string) string {
	t.Helper()
	l := lineWithAll(view, needles...)
	if l == "" {
		t.Fatalf("no line holds all of %q\n%s", needles, stripStyling(view))
	}
	return l
}

// withoutID removes the instance id from a row so its digits are not read as figures.
func withoutID(row, id string) string { return strings.ReplaceAll(row, id, "") }

// laneLinesOf are the lane lines of the given instances in the view.
func laneLinesOf(view string, ids map[string]string) []string {
	var out []string
	for id, typ := range ids {
		if i := laneIndex(view, id, typ); i >= 0 {
			out = append(out, viewLines(view)[i])
		}
	}
	return out
}

// timelineTypes are the entries of timelineRun with their types.
var timelineTypes = map[string]string{
	"solo#1": "planner", "alpha#2": "coder", "beta#3": "tester", "rev#4": "reviewer",
	"delta#5": screens.UnknownTypeLabel,
}

// laneAndOrchestratorLines are the orchestrator lane and every entry lane of
// timelineRun.
func laneAndOrchestratorLines(view string) []string {
	lines := laneLinesOf(view, timelineTypes)
	if i := orchestratorLaneIndex(view); i >= 0 {
		lines = append(lines, viewLines(view)[i])
	}
	return lines
}

// assertLaneOrder fails unless the lanes of the invocations appear top to bottom
// in the given order.
func assertLaneOrder(t *testing.T, view string, types map[string]string, ids ...string) {
	t.Helper()
	prev := -1
	for _, id := range ids {
		i := laneIndex(view, id, types[id])
		if i < 0 {
			t.Fatalf("no lane for %s\n%s", id, stripStyling(view))
		}
		if i <= prev {
			t.Errorf("lane of %s is out of order, want %v\n%s", id, ids, stripStyling(view))
		}
		prev = i
	}
}

// newTimelineScreen opens the timeline of a run at the wide test size.
func newTimelineScreen(run domain.RunReport) *screens.TimelineScreen {
	return screens.NewTimelineScreen(run, wideW, tallH, paletteStyles())
}

// orchestratorLane is the styling-free orchestrator lane; the test stops when the
// view has none.
func orchestratorLane(t *testing.T, view string) string {
	t.Helper()
	i := orchestratorLaneIndex(view)
	if i < 0 {
		t.Fatalf("the view has no orchestrator lane\n%s", stripStyling(view))
	}
	return viewLines(view)[i]
}

// firstCol is the first display column of a glyph on a lane; the test stops when
// the lane has none.
func firstCol(t *testing.T, line string, glyph rune) int {
	t.Helper()
	cols := glyphCols(line, glyph)
	if len(cols) == 0 {
		t.Fatalf("the lane has no %q: %q", string(glyph), line)
	}
	return cols[0]
}

// stripBlock finds the three-line detail strip of the invocation id: the first
// line holds the id and its type, the second the needles of line2, the third the
// needles of line3 (an empty needle is skipped). The block must not name the
// type of any other entry, so it can only be the strip of the cursor's entry and
// not a row of a table listing every entry. It returns the three lines.
func stripBlock(t *testing.T, view, id, typ, line2, line3 string) []string {
	t.Helper()
	lines := viewLines(view)
	for i, l := range lines {
		if !strings.Contains(l, id) || !strings.Contains(l, typ) || isLaneLine(l, id, typ) || i+2 >= len(lines) {
			continue
		}
		block := lines[i : i+3]
		if !strings.Contains(block[1], line2) || !strings.Contains(block[2], line3) {
			continue
		}
		joined := strings.Join(block, "\n")
		foreign := false
		for otherID, otherTyp := range timelineTypes {
			if otherID != id && strings.Contains(joined, otherTyp) && !strings.Contains(typ, otherTyp) {
				foreign = true
			}
		}
		if !foreign {
			return block
		}
	}
	t.Fatalf("no detail strip for %s (%s) with %q on its second and %q on its third line\n%s", id, typ, line2, line3, stripStyling(view))
	return nil
}
