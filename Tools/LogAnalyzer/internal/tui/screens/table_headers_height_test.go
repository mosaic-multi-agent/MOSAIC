package screens_test

// A taller header never pushes content off screen: the rows a table shows are
// counted after the real header height, so the position hint, the rows on screen
// and the cursor row agree at a short terminal height, and every row shows when
// the terminal is tall enough.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// heightW is a terminal width at which every table keeps its columns.
const heightW = 240

var shortHeights = []int{14, 20}

// hintOf reads "a-b of n" from the view's scroll hint.
func hintOf(t *testing.T, view string) (first, last, total int) {
	t.Helper()
	m := regexp.MustCompile(`(\d+)-(\d+) of (\d+)`).FindStringSubmatch(stripStyling(view))
	if m == nil {
		t.Fatalf("a table taller than the screen must show a position hint\n%s", stripStyling(view))
	}
	first, _ = strconv.Atoi(m[1])
	last, _ = strconv.Atoi(m[2])
	total, _ = strconv.Atoi(m[3])
	return first, last, total
}

// scrollView is a list view that scrolls under a cursor, at a short terminal height.
type scrollView struct {
	name      string
	anchor    string
	minHeader int // header lines the wrapped titles need
	screen    func(t *testing.T, w, h int) screens.Screen
}

func scrollViews() []scrollView {
	pick := func(name string) headerView {
		for _, v := range headerViews() {
			if v.name == name {
				return v
			}
		}
		return headerView{}
	}
	listed := func(name string, minHeader int) scrollView {
		v := pick(name)
		return scrollView{name, v.anchor, minHeader, v.screen}
	}
	return []scrollView{
		listed("dispatches", 2),
		listed("by type drill-down", 2),
		listed("context", 3),
		{"runs list", "compare|cmp", 1, func(t *testing.T, w, h int) screens.Screen {
			return screens.NewRunsScreen(manyRunsReport(40), w, h, paletteStyles())
		}},
		{"pricing", "model", 1, func(t *testing.T, w, h int) screens.Screen {
			return screens.NewPricingScreen(numberedModels(30), "", w, h, paletteStyles())
		}},
	}
}

// assertRowsMatchHint fails unless the rows on screen are the window the hint reports.
func assertRowsMatchHint(t *testing.T, v scrollView, view string) (last, total int) {
	t.Helper()
	first, last, total := hintOf(t, view)
	block := readHeader(t, view, v.anchor)
	if got, want := len(block.rows), last-first+1; got != want {
		t.Errorf("the hint reports rows %d-%d (%d rows) but %d rows are on screen\n%s", first, last, want, got, stripStyling(view))
	}
	if block.headerLines < v.minHeader {
		t.Errorf("the header has %d lines, want at least %d for its wrapped titles", block.headerLines, v.minHeader)
	}
	return last, total
}

func TestTableHeight_RowsOnScreenMatchTheHintAtTheTop(t *testing.T) {
	for _, v := range scrollViews() {
		for _, h := range shortHeights {
			t.Run(v.name+"/"+strconv.Itoa(h), func(t *testing.T) {
				colourOff(t)
				view := v.screen(t, heightW, h).View(heightW, h)

				assertWithinBounds(t, view, heightW, h)
				assertRowsMatchHint(t, v, view)
			})
		}
	}
}

func TestTableHeight_LastRowIsOnScreenUnderTheCursorAtTheBottom(t *testing.T) {
	for _, v := range scrollViews() {
		for _, h := range shortHeights {
			t.Run(v.name+"/"+strconv.Itoa(h), func(t *testing.T) {
				colourOff(t)
				s := v.screen(t, heightW, h)
				press(s, repeat("down", 500)...)
				view := s.View(heightW, h)

				assertWithinBounds(t, view, heightW, h)
				last, total := assertRowsMatchHint(t, v, view)
				if last != total {
					t.Errorf("the window ends at row %d of %d: the last row is off screen\n%s", last, total, stripStyling(view))
				}
				rows := readHeader(t, view, v.anchor).rows
				if len(rows) == 0 || !strings.HasPrefix(rows[len(rows)-1], ">") {
					t.Errorf("the cursor must sit on the last row on screen\n%s", stripStyling(view))
				}
			})
		}
	}
}

// rowCounts are the rows each table holds in its fixture, and tallerHeaders the
// header lines its wrapped titles need.
var (
	rowCounts = map[string]int{
		"dispatches": 66, "by type": 10, "by type drill-down": 15, "by status": 3, "tools": 9, "context": 66,
		"overview models": 3, "compare": 7, "across runs, runs": 3, "across runs, agent types": 3,
		"runs list": 5, "runs list with marked runs": 5, "pricing": 3, "timeline invocations": 5, "timeline compactions": 7,
	}
	tallerHeaders = map[string]int{
		"dispatches": 2, "by type": 3, "by type drill-down": 2, "tools": 2, "context": 3,
		"overview models": 2, "compare": 2, "across runs, agent types": 2,
	}
)

func TestTableHeight_EveryRowShowsWhenTheTerminalIsTallEnough(t *testing.T) {
	for _, v := range headerViews() {
		t.Run(v.name, func(t *testing.T) {
			block := readHeader(t, v.view(t, headerW, headerH), v.anchor)

			if got, want := len(block.rows), rowCounts[v.name]; got != want {
				t.Errorf("%d rows on screen, want all %d (a taller header must not cut the last rows)", got, want)
			}
			if need := tallerHeaders[v.name]; block.headerLines < need {
				t.Errorf("the header has %d lines, want at least %d for its wrapped titles", block.headerLines, need)
			}
		})
	}
}

// lastContentLine is the last body line before the help line, blanks ignored.
func lastContentLine(view string) string {
	lines := viewLines(view)
	for i := len(lines) - 2; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimRight(lines[i], " ")
		}
	}
	return ""
}

func TestTableHeight_ScrolledPagesEndOnTheirLastLineAtAShortHeight(t *testing.T) {
	for _, name := range []string{"by status", "tools", "overview models", "compare", "across runs, agent types", "timeline compactions"} {
		for _, h := range shortHeights {
			t.Run(name+"/"+strconv.Itoa(h), func(t *testing.T) {
				v := namedView(t, name)
				want := lastContentLine(v.view(t, heightW, listingHeight(name)))

				s := v.screen(t, heightW, h)
				press(s, repeat("down", 500)...)
				view := s.View(heightW, h)

				assertWithinBounds(t, view, heightW, h)
				if got := lastContentLine(view); got != want {
					t.Errorf("scrolled to the end the page stops at %q, want its last line %q", got, want)
				}
			})
		}
	}
}

// Tables whose wrapped titles make a taller header keep their last row on screen
// at a short height: the header is the real wrapped height and the row budget is
// counted after it, so the last row of the full listing is still drawn.
func TestTableHeight_TallerHeaderStillShowsTheLastRowAtAShortHeight(t *testing.T) {
	for _, name := range []string{"tools", "by type", "overview models", "compare", "across runs, agent types"} {
		for _, h := range shortHeights {
			t.Run(name+"/"+strconv.Itoa(h), func(t *testing.T) {
				v := namedView(t, name)
				// Last body line of the tall render: the last row of the last table, which
				// also holds for screens that stack several tables.
				wantLast := lastContentLine(v.view(t, heightW, listingHeight(name)))
				s := v.screen(t, heightW, h)
				press(s, repeat("down", 500)...)
				view := s.View(heightW, h)

				assertWithinBounds(t, view, heightW, h)
				block := readHeader(t, lastTableOf(view, v.anchor), v.anchor)
				if need := tallerHeaders[name]; block.headerLines < need {
					t.Errorf("the header has %d lines, want at least %d for its wrapped titles", block.headerLines, need)
				}
				// The cursor gutter differs between the renders (the cursor of a cursor
				// screen sits on another row), so the lines are compared without it.
				gotLast := string(runesOf(lastContentLine(view)))
				if wantLast = string(runesOf(wantLast)); gotLast != wantLast {
					t.Errorf("the last line on screen is %q, want the listing's last line %q", gotLast, wantLast)
				}
			})
		}
	}
}

// lastTableOf is the view from just below the previous table with that anchor, so
// the last table's header is read whole: a page of stacked tables that scrolls by
// lines can cut the top off the header of the first table on screen.
func lastTableOf(view, anchor string) string {
	lines := viewLines(view)
	from := 0
	for {
		next := bottomHeaderLine(lines[from:], anchor)
		if next < 0 {
			break
		}
		if from+next+1 >= len(lines) || bottomHeaderLine(lines[from+next+1:], anchor) < 0 {
			break
		}
		from += next + 1
	}
	return strings.Join(lines[from:], "\n")
}

// manyRunsReport lists n complete runs, so the runs list scrolls.
func manyRunsReport(n int) domain.Report {
	report := domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary(nil)}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("20260101T1200%02dZ-aaaa", i)
		report.Runs = append(report.Runs, listedRun(id, completeMoney(int64(i)), 10+i, 30+i))
	}
	return report
}

// listingHeight is a terminal height at which the whole listing of the named view
// shows. The tools listing is taller than headerH because its last line is the
// content-capture note below the tables.
func listingHeight(name string) int {
	if name == "tools" {
		return 3 * headerH
	}
	return headerH
}
