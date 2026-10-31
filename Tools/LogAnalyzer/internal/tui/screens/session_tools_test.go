package screens_test

// The tools section of the Session screen: the actor's tools with the figures of
// the Tools view. A tool call without a start or an end counts and shows no
// duration, a tool without growth samples shows "not provided", and the section
// sits below the call table in one-line rows.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/tui/screens"
)

func TestSessionTools_TableHasTheColumnsOfTheToolsView(t *testing.T) {
	session := sessionToolsTitles(t, "builder#1")

	want := headerTitles(growthView(t, growthRun(), headerW), "tool")

	if strings.Join(session, "|") != strings.Join(want, "|") {
		t.Errorf("session tool columns = %q\nTools view columns = %q", session, want)
	}
}

func sessionToolsTitles(t *testing.T, id string) []string {
	t.Helper()
	return sessToolsBlock(t, sessFindingsView(t, id)).titles()
}

func TestSessionTools_RowShowsCallsErrorsDurationsAndUnpairedOfTheTool(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	sessToolCells(t, view, "Bash",
		"calls", "5", "errors", "4",
		"average duration", screens.FormatDuration(2*time.Second, true), "minimum duration", screens.FormatDuration(time.Second, true),
		"maximum duration", screens.FormatDuration(3*time.Second, true), "unpaired", "1 "+screens.UnpairedLabel)
	sessToolCells(t, view, "Edit", "calls", "3", "errors", "2", "unpaired", "")
}

func TestSessionTools_ShareHasThePercentOfTheCallsNextToTheBar(t *testing.T) {
	view := sessFindingsView(t, "builder#1")
	block := sessToolsBlock(t, view)
	share, ok := block.col("share")
	if !ok {
		t.Fatalf("no share column; columns %q", block.titles())
	}

	for tool, pct := range map[string]string{"Bash": "42%", "Edit": "25%", "Glob": "0%"} {
		for _, row := range block.rows {
			r := runesOf(row)
			if name, _ := block.col("tool"); strings.TrimSpace(fragment(r, name.start, name.end)) != tool {
				continue
			}
			if got := fragment(r, share.start, share.end); !strings.Contains(got, pct) {
				t.Errorf("tool %q share cell %q, want the percent %q", tool, got, pct)
			}
		}
	}
}

func TestSessionTools_ToolCallsWithoutAStartOrAnEndCountAndShowNoDuration(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	sessToolCells(t, view, screens.StartMissingLabel,
		"calls", "2", "unpaired", "2 "+screens.UnpairedLabel,
		"average duration", screens.AbsentMarker, "minimum duration", screens.AbsentMarker, "maximum duration", screens.AbsentMarker)
}

func TestSessionTools_GrowthFiguresAreShownWithSampleCount(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	sessToolCells(t, view, "Edit",
		"average growth", "1,234", "minimum growth", "777", "maximum growth", "1,691",
		"growth samples", "2", "negative growth", "0", "not measurable", "0")
}

func TestSessionTools_ToolWithoutGrowthSamplesShowsNotProvidedAndItsLeftOutCounts(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	sessToolCells(t, view, "Bash",
		"average growth", notProvided, "minimum growth", screens.AbsentMarker, "maximum growth", screens.AbsentMarker,
		"growth samples", "0", "negative growth", "2", "not measurable", "1")
}

func TestSessionTools_ToolSeenOnlyThroughGrowthHasARowWithoutCalls(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	sessToolCells(t, view, "Glob",
		"calls", "0", "errors", "0", "average duration", screens.AbsentMarker,
		"average growth", "4,321", "growth samples", "1")
}

func TestSessionTools_OlderLogWithoutRequestedToolsShowsNotProvidedForTheTools(t *testing.T) {
	view := sessFindingsView(t, "coder#1")

	sessToolCells(t, view, "Bash", "calls", "1", "average growth", notProvided, "growth samples", "0")
}

func TestSessionTools_EachActorShowsOnlyItsOwnTools(t *testing.T) {
	orch := sessToolsBlock(t, sessFindingsView(t, "orchestrator"))
	builder := sessToolsBlock(t, sessFindingsView(t, "builder#1"))

	if len(orch.rows) != 1 || !strings.Contains(orch.rows[0], "Agent") {
		t.Errorf("orchestrator tool rows %q, want only Agent", orch.rows)
	}
	if len(builder.rows) != 5 {
		t.Errorf("builder#1 shows %d tool rows, want its 5 (one line each)\n%s", len(builder.rows), strings.Join(builder.rows, "\n"))
	}
	for _, row := range builder.rows {
		if strings.Contains(row, "Agent") {
			t.Errorf("builder#1 shows the orchestrator's tool: %q", row)
		}
	}
}

func TestSessionTools_ActorsWithoutToolsOrFiguresShowNoToolsTable(t *testing.T) {
	for _, id := range []string{"reviewer#1", "tester#1", "ghost#1"} {
		view := sessFindingsView(t, id)

		if _, ok := parseHeader(view, "tool"); ok {
			t.Errorf("%s: a tools table is drawn for an actor without tool figures\n%s", id, view)
		}
		assertWithinBounds(t, view, headerW, legendOverlayH)
	}
}

func TestSessionTools_TableComesAfterTheCallTable(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	lastCall := sessLineIndex(view, ctxAt(190).Format("15:04:05"), "model-b") // builder#1's last call
	header := sessLineIndex(view, "errors", "calls")

	if lastCall < 0 || header < 0 || header <= lastCall {
		t.Errorf("tools header at line %d, last call row at %d: the tools come after the calls", header, lastCall)
	}
}

func TestSessionTools_GrowthColumnsStandUnderTheirGroupLabel(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	if n := strings.Count(view, growthGroup); n != 1 {
		t.Errorf("the group label %q appears %d times, want once over the growth columns", growthGroup, n)
	}
}

func TestSessionTools_SampleCountAndLeftOutCountsSurviveEveryWidth(t *testing.T) {
	for _, w := range []int{80, 120, 180} {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			view := sessFindingsViewOf(t, sessionFindingsRun(), "builder#1", w)

			titles := sessToolsBlock(t, view).titles()

			for _, want := range []string{"tool", "calls", "errors", "growth samples", "negative growth", "not measurable"} {
				if !hasTitle(titles, want) {
					t.Errorf("width %d: the tools table lost the column %q; its columns are %q", w, want, titles)
				}
			}
			assertWithinBounds(t, view, w, legendOverlayH)
		})
	}
}
