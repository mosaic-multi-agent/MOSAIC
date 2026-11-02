package screens_test

// Tests for effort in the TUI: the V2 dispatch drill-down row and the
// orchestrator's row on the run-detail screen show an "effort: <levels>"
// segment, with the absent marker when no level was reported.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func effortDispatchRun(levels ...domain.EffortLevels) domain.RunReport {
	run := agentTypesRun()
	for i, l := range levels {
		run.ByAgentType[0].Dispatches[i].Effort = l
	}
	return run
}

// lineWith returns the first view line containing needle.
func viewLineWith(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("view has no line containing %q\n%s", needle, view)
	return ""
}

func TestAgentTypesDrill_DispatchRow_ShowsItsEffort(t *testing.T) {
	run := effortDispatchRun(domain.EffortLevels{{Level: "high", Records: 3}}, domain.EffortLevels{{Level: "low", Records: 1}})

	_, view := drilled(run, tallH, "enter")

	assertContains(t, viewLineWith(t, view, "builder#1"), "effort: high")
	assertContains(t, viewLineWith(t, view, "builder#2"), "effort: low")
}

func TestAgentTypesDrill_MixedLevels_ShowEveryLevelWithItsRecordCount(t *testing.T) {
	run := effortDispatchRun(domain.EffortLevels{{Level: "medium", Records: 12}, {Level: "high", Records: 3}})

	_, view := drilled(run, tallH, "enter")

	assertContains(t, viewLineWith(t, view, "builder#1"), "effort: medium (12), high (3)")
}

func TestAgentTypesDrill_AbsentEffort_ShowsTheNotProvidedMarker(t *testing.T) {
	_, view := drilled(effortDispatchRun(), tallH, "enter")

	assertContains(t, viewLineWith(t, view, "builder#1"), "effort: "+screens.AbsentMarker)
}

func effortDetailView(orch domain.EffortLevels) string {
	run := overviewRun()
	run.Orchestrator.Effort = orch
	return screens.NewRunDetailScreen(run, tableW, tallH, plain()).View(tableW, tallH)
}

func TestRunDetail_OrchestratorRow_ShowsItsEffort(t *testing.T) {
	view := effortDetailView(domain.EffortLevels{{Level: "medium", Records: 4}})

	assertContains(t, view, "effort: medium")
}

func TestRunDetail_OrchestratorMixedLevels_ShowEveryLevelWithItsRecordCount(t *testing.T) {
	view := effortDetailView(domain.EffortLevels{{Level: "medium", Records: 12}, {Level: "high", Records: 3}})

	assertContains(t, view, "effort: medium (12), high (3)")
}

func TestRunDetail_OrchestratorWithoutEffort_ShowsTheNotProvidedMarker(t *testing.T) {
	view := effortDetailView(nil)

	assertContains(t, view, "effort: "+screens.AbsentMarker)
}
