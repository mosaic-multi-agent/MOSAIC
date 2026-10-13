package planstages_test

// Tests that the text of a schedule defect error carries the context the
// session places into its fix message. Presence checks only; wording is free.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/planstages"
)

func requireTextContains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("error text %q: want it to contain %q", text, p)
		}
	}
}

func TestReadBlockSchedule_RowDefectErrorText_NamesBlockSourceSectionRowAndToken(t *testing.T) {
	req := stageRequest("sched-depends-empty-element.md", false)
	req.Source = "Run-Plan.md"

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V4", 3, "1,,2")
	requireTextContains(t, err.Error(), "V4", "Stage", "Run-Plan.md", "Stages", "3", "1,,2")
}

func TestReadBlockSchedule_MissingFileErrorText_SaysMissingAndNamesSource(t *testing.T) {
	req := stageRequest("does-not-exist.md", false)
	req.Source = "Run-Plan.md"

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V1", 0, "Run-Plan.md")
	requireTextContains(t, err.Error(), "missing", "Run-Plan.md", "Stage")
}

func TestReadBlockSchedule_UnreadableFileErrorText_SaysUnreadable(t *testing.T) {
	req := stageRequest("irrelevant.md", false)
	req.Path = t.TempDir()
	req.Source = "Run-Plan.md"

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "", 0, "Run-Plan.md")
	requireTextContains(t, err.Error(), "unreadable", "Run-Plan.md")
	if strings.Contains(err.Error(), "missing") {
		t.Errorf("error text %q: unreadable defect must not say missing", err.Error())
	}
}

func TestReadBlockSchedule_HeadingDefectErrorText_NamesRuleNotMissing(t *testing.T) {
	req := stageRequest("sched-no-heading.md", false)

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V1", 0, stagesHeading)
	requireTextContains(t, err.Error(), "V1", "Plan.md", "Stages")
	if strings.Contains(err.Error(), "missing") {
		t.Errorf("error text %q: heading defect must read as its rule, not missing", err.Error())
	}
}

func TestReadBlockSchedule_PaddedCells_AreTrimmed(t *testing.T) {
	req := stageRequest("sched-padded-cells.md", true, groupedApproaches...)

	got, err := planstages.ReadBlockSchedule(req)
	if err != nil {
		t.Fatalf("ReadBlockSchedule: %v", err)
	}

	requireIterations(t, got.Iterations, []domain.ScheduleIteration{
		{ID: "1", HITL: true, DependsOn: nil, Approach: "TDD"},
		{ID: "2", HITL: false, DependsOn: []domain.IterationID{"1"}, Approach: "Tests-Only"},
	})
}
