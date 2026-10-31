package screens_test

// The column legends of the ? overlay: every view with a table lists each column it
// renders, under "Columns" groups between "This screen" and "Navigation", with one
// sentence each. The completeness check compares the titles a view renders with the
// legend keys, so a column added without a description fails here.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const legendOverlayH = 400 // tall enough that the whole overlay fits

// columnLegendEntry is one "key  description" line of an overlay group.
type columnLegendEntry struct{ key, desc string }

// columnLegendGroup is one titled group of the overlay.
type columnLegendGroup struct {
	title   string
	entries []columnLegendEntry
}

var entryGapRE = regexp.MustCompile(`\s{2,}`)

// overlayGroupsOf parses the groups of a rendered overlay: a two-space indented line
// opens a group, a four-space indented line is an entry of the open group.
func overlayGroupsOf(view string) []columnLegendGroup {
	var groups []columnLegendGroup
	for _, l := range overlayLines(view) {
		switch {
		case strings.HasPrefix(l, "    "):
			if len(groups) == 0 {
				continue
			}
			parts := entryGapRE.Split(strings.TrimSpace(l), 2)
			e := columnLegendEntry{key: parts[0]}
			if len(parts) == 2 {
				e.desc = strings.TrimSpace(parts[1])
			}
			g := &groups[len(groups)-1]
			g.entries = append(g.entries, e)
		case strings.HasPrefix(l, "  ") && strings.TrimSpace(l) != "":
			groups = append(groups, columnLegendGroup{title: strings.TrimSpace(l)})
		}
	}
	return groups
}

// columnGroups are the legend groups of the overlay.
func columnGroups(groups []columnLegendGroup) []columnLegendGroup {
	var out []columnLegendGroup
	for _, g := range groups {
		if g.title == "Columns" || strings.HasPrefix(g.title, "Columns: ") {
			out = append(out, g)
		}
	}
	return out
}

// legendViews are the table views whose legends are checked: every table view plus
// the call table of the Session screen.
func legendViews() []headerView {
	views := headerViews()
	return append(views, headerView{"session call table", "time", func(t *testing.T, w, h int) screens.Screen {
		run := sampleRun(t)
		return screens.NewSessionScreen(run, run.Context.Orchestrator.Actor, screens.ScreenContext, w, h, paletteStyles())
	}})
}

// wideOverlay opens the overlay of the view on a terminal tall enough for all of it.
func (v headerView) wideOverlay(t *testing.T) (string, string) {
	t.Helper()
	colourOff(t)
	s := v.screen(t, headerW, legendOverlayH)
	body := s.View(headerW, legendOverlayH)
	return body, openOverlay(s, headerW, legendOverlayH)
}

func TestColumnLegend_EveryRenderedTitleHasAnEntryWithADescription(t *testing.T) {
	for _, v := range legendViews() {
		t.Run(v.name, func(t *testing.T) {
			body, overlay := v.wideOverlay(t)
			block := readHeader(t, body, v.anchor)
			keys := map[string]string{}
			for _, g := range columnGroups(overlayGroupsOf(overlay)) {
				for _, e := range g.entries {
					keys[e.key] = e.desc
				}
			}

			for _, title := range block.titles() {
				desc, ok := keys[title]
				if !ok {
					t.Errorf("no legend entry has the key %q\n%s", title, stripStyling(overlay))
				} else if len(strings.Fields(desc)) < 2 {
					t.Errorf("legend entry %q has no sentence as description: %q", title, desc)
				}
			}
		})
	}
}

func TestColumnLegend_GroupsSitBetweenThisScreenAndNavigation(t *testing.T) {
	for _, v := range legendViews() {
		t.Run(v.name, func(t *testing.T) {
			_, overlay := v.wideOverlay(t)
			groups := overlayGroupsOf(overlay)

			first, last, nav, screen := -1, -1, -1, -1
			for i, g := range groups {
				switch {
				case g.title == "Columns" || strings.HasPrefix(g.title, "Columns: "):
					if first < 0 {
						first = i
					}
					last = i
				case g.title == "Navigation":
					nav = i
				case g.title == "This screen":
					screen = i
				}
			}

			if first < 0 {
				t.Fatalf("overlay has no Columns group\n%s", stripStyling(overlay))
			}
			if nav < 0 || last > nav || last+1 != nav {
				t.Errorf("Columns groups must end right before Navigation (last %d, navigation %d)", last, nav)
			}
			if screen >= 0 && first < screen {
				t.Errorf("Columns groups must come after This screen (first %d, this screen %d)", first, screen)
			}
		})
	}
}

func TestColumnLegend_TitlesAreUniqueWithinOneGroup(t *testing.T) {
	for _, v := range legendViews() {
		t.Run(v.name, func(t *testing.T) {
			_, overlay := v.wideOverlay(t)

			for _, g := range columnGroups(overlayGroupsOf(overlay)) {
				seen := map[string]bool{}
				for _, e := range g.entries {
					if seen[e.key] {
						t.Errorf("group %q lists %q twice", g.title, e.key)
					}
					seen[e.key] = true
				}
			}
		})
	}
}

func TestColumnLegend_AViewWithSeveralTablesNamesEachGroup(t *testing.T) {
	_, overlay := namedView(t, "tools").wideOverlay(t)

	groups := columnGroups(overlayGroupsOf(overlay))

	for _, g := range groups {
		if g.title == "Columns" {
			t.Errorf("the Tools view has several tables, so no group may be the bare %q", g.title)
		}
	}
	if len(groups) < 2 {
		t.Errorf("the Tools view must have one Columns group per table, got %d", len(groups))
	}
}

func TestColumnLegend_ADescribedColumnTheViewDoesNotRenderIsNotRequired(t *testing.T) {
	// The completeness rule runs from the rendered titles to the legend: a narrow
	// terminal that drops columns must not make the legend incomplete for the rest.
	v := namedView(t, "dispatches")
	colourOff(t)
	s := v.screen(t, 90, legendOverlayH)
	block := readHeader(t, s.View(90, legendOverlayH), v.anchor)
	overlay := openOverlay(s, 90, legendOverlayH)

	keys := map[string]bool{}
	for _, g := range columnGroups(overlayGroupsOf(overlay)) {
		for _, e := range g.entries {
			keys[e.key] = true
		}
	}

	for _, title := range block.titles() {
		if !keys[title] {
			t.Errorf("the narrow view renders %q but the legend has no entry for it", title)
		}
	}
}

func TestContextLegend_ReplacesTheNotesUnderTheTable(t *testing.T) {
	colourOff(t)
	view := stripStyling(namedView(t, "context").view(t, headerW, headerH))

	assertNotContains(t, view, "first write is a heuristic", "recorded by the harness", "final context = ")
	assertNotContains(t, view, domain.FinalContextDefinition)
}

func TestContextLegend_CarriesTheContentOfTheFormerNotes(t *testing.T) {
	_, overlay := namedView(t, "context").wideOverlay(t)

	text := strings.Join(overlayLines(overlay), "\n")

	assertContains(t, text, domain.FinalContextDefinition)
	assertContains(t, text, "heuristic", "logged", "detected")
}

func TestColumnLegend_ATallLegendStaysReachableOnAShortTerminal(t *testing.T) {
	for _, v := range legendViews() {
		t.Run(v.name, func(t *testing.T) {
			colourOff(t)
			body := stripStyling(v.screen(t, headerW, legendOverlayH).View(headerW, legendOverlayH))
			titles := readHeader(t, body, v.anchor).titles()
			s := v.screen(t, 100, shortH)
			seen := map[string]bool{}
			// One "down" reveals one new line at the bottom, so the group an entry
			// belongs to is the last group title seen, even when its title has
			// scrolled out of the window.
			group := ""
			take := func(line string) {
				switch {
				case strings.HasPrefix(line, "    "):
					if group == "Columns" || strings.HasPrefix(group, "Columns: ") {
						seen[entryGapRE.Split(strings.TrimSpace(line), 2)[0]] = true
					}
				case strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "":
					group = strings.TrimSpace(line)
				}
			}

			prev := openOverlay(s, 100, shortH)
			assertWithinBounds(t, prev, 100, shortH)
			for _, l := range overlayLines(prev) {
				take(l)
			}
			for i := 0; i < 400; i++ {
				view := press(s, "down").View(100, shortH)
				if view == prev {
					break
				}
				assertWithinBounds(t, view, 100, shortH)
				prev = view
				lines := overlayLines(view)
				take(lines[len(lines)-2]) // the last line is the "more" marker
			}

			for _, title := range titles {
				if !seen[title] {
					t.Errorf("scrolling the overlay never showed the legend entry %q", title)
				}
			}
		})
	}
}
