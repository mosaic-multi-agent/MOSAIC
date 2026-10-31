package screens_test

// The tools view growth columns: average, minimum and maximum growth with an
// always-visible sample count at the run, orchestrator and agent-type level, "not
// provided" for tools without samples and for older logs, the left-out counts, the
// VS Code content-capture note, and the shared layout of the stacked tables.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

var growthWidths = []int{80, 120, 180}

func TestToolsGrowth_HeaderNamesTheGrowthColumnsUnderTheirGroup(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	titles := growthTitles(view)
	for _, want := range []string{"average growth", "minimum growth", "maximum growth", "growth samples", "negative growth", "not measurable"} {
		if !hasTitle(titles, want) {
			t.Errorf("the header has no column titled %q; its columns are %q", want, titles)
		}
	}
	assertGroupOverGrowthColumns(t, view, 4)
}

func TestToolsGrowth_RunLevelRowShowsAverageMinimumMaximumAndSampleCountEachInItsColumn(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Read",
		"average growth", "1,234", "minimum growth", "777", "maximum growth", "2,468", "growth samples", "13")
}

func TestToolsGrowth_OrchestratorLevelRowShowsItsOwnGrowthInItsColumns(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Grep",
		"average growth", "4,321", "minimum growth", "321", "maximum growth", "9,876", "growth samples", "17")
	assertNoFields(t, rowFor(t, view, "Grep"), "1,234", "5,555")
}

func TestToolsGrowth_AgentTypeLevelRowShowsItsOwnGrowthInItsColumns(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Edit",
		"average growth", "5,555", "minimum growth", "505", "maximum growth", "6,666", "growth samples", "19")
	assertNoFields(t, rowFor(t, view, "Edit"), "1,234", "4,321")
}

func TestToolsGrowth_LeftOutCountsAreShownInTheirColumnsOnTheRowOfEveryLevel(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Read", "negative growth", "5", "not measurable", "7")
	assertGrowthCells(t, view, "Grep", "negative growth", "6", "not measurable", "8")
	assertGrowthCells(t, view, "Edit", "negative growth", "14", "not measurable", "11")
}

func TestToolsGrowth_ToolWithoutSamplesShowsNotProvidedAZeroSampleCountAndZeroLeftOut(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Write",
		"average growth", notProvided, "minimum growth", screens.AbsentMarker, "maximum growth", screens.AbsentMarker,
		"growth samples", "0", "negative growth", "0", "not measurable", "0")
}

func TestToolsGrowth_ToolWithOnlyLeftOutCallsShowsNotProvidedAndItsCounts(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Bash",
		"average growth", notProvided, "minimum growth", screens.AbsentMarker, "maximum growth", screens.AbsentMarker,
		"growth samples", "0", "negative growth", "23", "not measurable", "29")
}

func TestToolsGrowth_ToolWithSamplesOfZeroGrowthShowsZeroNotNotProvided(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	assertGrowthCells(t, view, "Glob",
		"average growth", "0", "minimum growth", "0", "maximum growth", "0", "growth samples", "3")
}

func TestToolsGrowth_OlderLogShowsNotProvidedForEveryToolAtEveryLevel(t *testing.T) {
	view := growthView(t, olderLogRun(), tableW)

	for _, tool := range []string{"Read", "Bash", "Grep", "Edit", "Write"} {
		assertContains(t, rowFor(t, view, tool), notProvided)
	}
	assertContains(t, flatText(view), contentCaptureNote)
}

func TestToolsGrowth_NoteExplainsVSCodeNeedsOTelContentCapture(t *testing.T) {
	for _, w := range growthWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := growthView(t, growthRun(), w)

			assertContains(t, flatText(view), "Context growth is measured only on calls that requested exactly one tool", contentCaptureNote)
			assertFitsWidth(t, view, w)
		})
	}
}

func TestToolsGrowth_NoteSaysTheGrowthIsTheTokensTheResultAdds(t *testing.T) {
	view := growthView(t, growthRun(), 120)

	assertContains(t, flatText(view), "the tokens its result added to the next call")
}

func TestToolsGrowth_NoteIsNotShownWhenTheRunHasNoToolCalls(t *testing.T) {
	view := growthView(t, overviewRun(), 120)

	assertNotContains(t, flatText(view), contentCaptureNote)
}

func TestToolsGrowth_SampleCountAndLeftOutCountsSurviveEveryWidth(t *testing.T) {
	for _, w := range growthWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := growthView(t, growthRun(), w)

			titles := growthTitles(view)
			for _, want := range []string{"growth samples", "negative growth", "not measurable"} {
				if !hasTitle(titles, want) {
					t.Errorf("width %d dropped the column %q; its columns are %q", w, want, titles)
				}
			}
			assertGrowthCells(t, view, "Read", "growth samples", "13", "negative growth", "5", "not measurable", "7")
			assertFitsWidth(t, view, w)
		})
	}
}

func TestToolsGrowth_TablesStayOneLinePerRowAtEveryWidth(t *testing.T) {
	for _, w := range growthWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := growthView(t, growthRun(), w)

			for _, tool := range []string{"Read", "Bash", "Grep", "Edit", "Write"} {
				if n := strings.Count(stripStyling(view), tool); n != 1 {
					t.Errorf("tool %q appears %d times, want its row once", tool, n)
				}
			}
		})
	}
}

func TestToolsGrowth_StackedTablesShareOneColumnLayoutAtEveryWidth(t *testing.T) {
	for _, w := range growthWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := growthView(t, growthRun(), w)

			assertTablesShareLayout(t, view, "tool", 4)
		})
	}
}

func TestToolsGrowth_NoteAndGroupHeaderStayInsideTheBodyBudgetAndTheNoteIsReachable(t *testing.T) {
	const h = 24
	for _, w := range growthWidths {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			colourOff(t)
			var s screens.Screen = screens.NewToolsScreen(growthRun(), w, h, paletteStyles())

			noteSeen := false
			for step := 0; step < 80; step++ {
				view := s.View(w, h)
				assertWithinBounds(t, view, w, h)
				assertContains(t, footerLine(view), footerHelp)
				noteSeen = noteSeen || strings.Contains(flatText(view), contentCaptureNote)
				s = press(s, "down")
			}
			if !noteSeen {
				t.Errorf("scrolling through the view at %dx%d never showed the whole note %q", w, h, contentCaptureNote)
			}
		})
	}
}
