package screens_test

// Every per-run view renders through the shared frame: header with badges, tab bar
// with the current view highlighted, short help with the ? hint, and nothing
// drawn past the terminal.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var invocationBadgeRE = regexp.MustCompile(`\d+ invocations`)

// withHeaderFigures gives a run a known total and a 90 second span.
func withHeaderFigures(run domain.RunReport) domain.RunReport {
	run.Totals.Money = domain.CategoryMoney{Total: known(1000), Complete: true}
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	run.Timeline.Axis = domain.TimeInterval{Start: start, End: start.Add(framedRunSpan)}
	run.Timeline.HasAxis = true
	return run
}

func TestPerRunViews_ShowHeaderTabBarAndShortHelp(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			run := withHeaderFigures(fv.run)

			lines := framedLines(t, fv.make(run, 140, 40, paletteStyles()).View(140, 40))

			assertContains(t, lines[0], run.Run.Label(), "$10.00", screens.FormatDuration(framedRunSpan, true))
			if !invocationBadgeRE.MatchString(lines[0]) {
				t.Errorf("header must show the invocation count: %q", lines[0])
			}
			assertContains(t, lines[1], "["+fv.tab+"]")
			if n := strings.Count(lines[1], "["); n != 1 {
				t.Errorf("only the current view's tab is highlighted, got %d in %q", n, lines[1])
			}
			help := lines[len(lines)-1]
			assertContains(t, help, "? help")
			assertContains(t, help, "e export")
		})
	}
}

func TestPerRunViews_HeaderMarksProvisionalRun(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		run := fv.run
		run.Provisional = true

		lines := framedLines(t, fv.make(run, 140, 40, paletteStyles()).View(140, 40))

		assertContains(t, lines[0], run.Run.Label(), screens.ProvisionalLabel)
	}
}

func TestPerRunViews_HeaderShowsFindingsOnceTheQualityIsSet(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			v := fv.make(fv.run, 140, 40, paletteStyles())
			setter, ok := v.(qualitySetter)
			if !ok {
				t.Fatalf("%T must offer SetRunQuality", v)
			}
			before := framedLines(t, v.View(140, 40))[0]

			setter.SetRunQuality(screens.RunQuality{Findings: 3, DataLoss: true})
			after := framedLines(t, v.View(140, 40))[0]

			assertNotContains(t, strings.ToLower(before), "finding")
			assertContains(t, after, "3 findings")
		})
	}
}

func TestPerRunViews_StayWithinWidthAndHeightAndFillTheFrame(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			run := fv.run
			run.Provisional = true
			for _, width := range []int{120, 80, 60, 40, 30} {
				for _, height := range []int{5, 8, 12, 24, 40} {
					v := fv.make(run, width, height, paletteStyles())
					setter, ok := v.(qualitySetter)
					if !ok {
						t.Fatalf("%T must offer SetRunQuality", v)
					}
					setter.SetRunQuality(screens.RunQuality{Findings: 7, DataLoss: true})

					assertWithinBounds(t, v.View(width, height), width, height)
				}
			}
		})
	}
}

func TestOverview_LongBodyFillsExactlyTheTerminalHeight(t *testing.T) {
	colourOff(t)
	run := manyModelsRun(60)
	for _, height := range []int{6, 8, 12, 24} {
		view := screens.NewOverviewScreen(run, 100, height, paletteStyles()).View(100, height)

		if n := len(viewLines(view)); n != height {
			t.Errorf("height %d: view has %d lines, want the body to fill all of them\n%s", height, n, view)
		}
	}
}

func TestPerRunViews_HelpAdvertisesTheHelpKey(t *testing.T) {
	for _, fv := range allFrameViews() {
		help := fv.make(fv.run, wideW, tallH, plain()).Help()

		assertContains(t, help, screens.KeyHelp)
	}
}

func TestPerRunViews_NoEscapeSequencesWithColourOff(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		view := fv.make(fv.run, 100, 30, paletteStyles()).View(100, 30)

		if strings.Contains(view, "\x1b") {
			t.Errorf("%s: output must carry no escape sequence with colour off\n%q", fv.id, view)
		}
	}
}
