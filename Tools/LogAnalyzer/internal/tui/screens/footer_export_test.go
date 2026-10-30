package screens_test

// Every per-run view lists `e export` in its footer and drops it only after every
// other entry; `E` (export all) is only in the help overlay. The runs list has
// one quit entry.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// runViewFooterCase is one state of a per-run view.
type runViewFooterCase struct {
	name string
	view func(t *testing.T) screens.RunView
}

// footerCases lists every per-run view, with the states that have their own key
// list: Overview and Timeline in both versions, By type and Context drilled or not.
func footerCases() []runViewFooterCase {
	var cases []runViewFooterCase
	for _, fv := range allFrameViews() {
		cases = append(cases, runViewFooterCase{string(fv.id), func(t *testing.T) screens.RunView {
			colourOff(t)
			return fv.make(fv.run, wideW, tallH, paletteStyles())
		}})
	}
	cases = append(cases,
		runViewFooterCase{"overview-figures", func(t *testing.T) screens.RunView {
			colourOff(t)
			s := screens.NewOverviewScreen(manyModelsRun(40), wideW, tallH, paletteStyles())
			press(s, screens.KeyToggleView)
			return s
		}},
		runViewFooterCase{"timeline-list", func(t *testing.T) screens.RunView {
			colourOff(t)
			s := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles())
			press(s, screens.KeyToggleView)
			return s
		}},
		runViewFooterCase{"agenttype-drilled", func(t *testing.T) screens.RunView {
			colourOff(t)
			s := screens.NewAgentTypesScreen(agentTypesRun(), wideW, tallH, paletteStyles())
			press(s, "enter")
			return s
		}},
		runViewFooterCase{"context", func(t *testing.T) screens.RunView {
			return ctxOpen(t)
		}},
		runViewFooterCase{"context-drilled", func(t *testing.T) screens.RunView {
			return ctxDrill(t, contextRun(), "builder#1", tableW, tallH)
		}},
	)
	return cases
}

func hasEntry(line, entry string) bool {
	for _, e := range footerEntries(line) {
		if e == entry {
			return true
		}
	}
	return false
}

func TestRunViewFooters_ListExportAsAWholeEntry(t *testing.T) {
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.view(t)

			if line := footerLine(s.View(wideW, tallH)); !hasEntry(line, "e export") {
				t.Errorf("footer %q must list the entry %q", line, "e export")
			}
			if help := s.Help(); !hasEntry(help, "e export") || strings.Contains(help, footerEllipsis) {
				t.Errorf("Help() = %q must list %q and drop nothing", help, "e export")
			}
		})
	}
}

func TestRunViewFooters_KeepExportAtEightyColumns(t *testing.T) {
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			line := footerLine(c.view(t).View(80, tallH))

			if !hasEntry(line, "e export") || !hasEntry(line, footerHelp) {
				t.Errorf("80-column footer %q must list %q and %q", line, "e export", footerHelp)
			}
		})
	}
}

func TestRunViewFooters_ExportIsTheLastEntryDroppedBeforeHelp(t *testing.T) {
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.view(t)

			for width := 19; width <= 100; width++ {
				line := footerLine(s.View(width, tallH))
				if !hasEntry(line, "e export") {
					t.Fatalf("width %d: footer %q must keep %q", width, line, "e export")
				}
			}
			if line := footerLine(s.View(19, tallH)); line != "e export  …  ? help" {
				t.Errorf("width 19: footer = %q, want only the export entry, an ellipsis and the help entry", line)
			}
			if line := footerLine(s.View(18, tallH)); line != "…  ? help" {
				t.Errorf("width 18: footer = %q, want the export entry dropped last", line)
			}
		})
	}
}

func TestRunViewFooters_ExportAllIsOnlyInTheHelpOverlay(t *testing.T) {
	exportAllRE := regexp.MustCompile(`(^|\s)E\s+export all`)
	for _, c := range footerCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.view(t)

			if line := footerLine(s.View(wideW, tallH)); exportAllRE.MatchString(line) {
				t.Errorf("footer %q must not list the export all key", line)
			}
			overlay := strings.Join(overlayLines(openOverlay(s, wideW, tallH)), "\n")
			if !exportAllRE.MatchString(overlay) {
				t.Errorf("the help overlay must list the export all key\n%s", overlay)
			}
		})
	}
}

func TestRunsFooter_HasASingleQuitEntry(t *testing.T) {
	colourOff(t)
	s := newRuns(wideW, tallH)

	for name, text := range map[string]string{"Help()": s.Help(), "footer": footerLine(s.View(wideW, tallH))} {
		if n := strings.Count(text, "quit"); n != 1 {
			t.Errorf("%s = %q lists %d quit entries, want 1", name, text, n)
		}
		if !hasEntry(text, "esc quit") || strings.Contains(text, "ctrl+c") {
			t.Errorf("%s = %q must list %q and not %q", name, text, "esc quit", "ctrl+c quit")
		}
	}
}
