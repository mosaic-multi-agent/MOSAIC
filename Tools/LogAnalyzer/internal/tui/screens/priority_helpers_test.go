package screens_test

// Helpers of the column-priority tests: they read which columns of a table a
// view shows from its header line, at every terminal width, and check the order
// in which columns disappear as the terminal narrows.

import (
	"regexp"
	"strings"
	"testing"
)

const (
	// priorityWidest is wide enough for every column of every table to show.
	priorityWidest = 400
	// priorityNarrowest is the narrowest width the drop order is checked at; below
	// it the last-resort cut can clip a title.
	priorityNarrowest = 40
)

var (
	headerGapRE    = regexp.MustCompile(`\s{2,}`)
	sortMarkerRE   = regexp.MustCompile(`\s[v^↑↓]$`)
	hiddenMarkerRE = regexp.MustCompile(`^\+\d+\s`)
)

// headerTitles are the column titles on the header line of the table whose first
// column is anchor, or nil when the view has no such header.
func headerTitles(view, anchor string) []string {
	for _, line := range viewLines(view) {
		segments := headerGapRE.Split(strings.TrimSpace(stripStyling(line)), -1)
		if len(segments) == 0 || sortMarkerRE.ReplaceAllString(segments[0], "") != anchor {
			continue
		}
		var titles []string
		for _, seg := range segments {
			if hiddenMarkerRE.MatchString(seg) {
				continue
			}
			titles = append(titles, sortMarkerRE.ReplaceAllString(seg, ""))
		}
		return titles
	}
	return nil
}

func hasTitle(titles []string, want string) bool {
	for _, t := range titles {
		if t == want {
			return true
		}
	}
	return false
}

// assertDropOrder renders the view at every width from the widest down to the
// narrowest checked and fails when a column outlasts a column that must outlast
// it. tiers lists the titles from the first dropped to the last dropped; titles
// within one tier have no required order. A title of an earlier tier may show
// only while every title of every later tier shows too.
func assertDropOrder(t *testing.T, render func(width int) string, anchor string, tiers ...[]string) {
	t.Helper()
	widest := headerTitles(render(priorityWidest), anchor)
	for _, tier := range tiers {
		for _, title := range tier {
			if !hasTitle(widest, title) {
				t.Fatalf("the widest view of the %q table has no %q column; its columns are %q", anchor, title, widest)
			}
		}
	}
	for w := priorityWidest; w >= priorityNarrowest; w-- {
		shown := headerTitles(render(w), anchor)
		if shown == nil {
			t.Fatalf("width %d: no header line starts with %q", w, anchor)
		}
		for i, early := range tiers {
			for _, a := range early {
				if !hasTitle(shown, a) {
					continue
				}
				for _, late := range tiers[i+1:] {
					for _, b := range late {
						if !hasTitle(shown, b) {
							t.Fatalf("width %d: %q still shows but %q, which must outlast it, is hidden; header shows %q", w, a, b, shown)
						}
					}
				}
			}
		}
	}
}
