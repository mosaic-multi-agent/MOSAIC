package planstages_test

// Shared helpers for the block-schedule reader tests.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/planstages"
)

// requireIterations asserts that got equals want, in order.
func requireIterations(t *testing.T, got, want []domain.ScheduleIteration) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("iterations mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

// stageRequest builds a request for the block "Stage" in section "Stages".
// Pass approaches only for a grouped block.
func stageRequest(fixture string, grouped bool, approaches ...domain.Approach) planstages.ScheduleRequest {
	return planstages.ScheduleRequest{
		Path:       planstagesFixture(fixture),
		Source:     "Plan.md",
		Section:    "Stages",
		Block:      domain.BlockName("Stage"),
		Grouped:    grouped,
		Approaches: approaches,
	}
}

// groupedApproaches is the approach set declared for grouped fixtures.
var groupedApproaches = []domain.Approach{"TDD", "Implementation-First", "Implementation-Only", "Tests-Only"}

// requireDefect asserts that err is a *domain.ScheduleDefectError carrying the
// expected rule, row and token, and the block, source and section of req.
func requireDefect(t *testing.T, err error, req planstages.ScheduleRequest, rule string, row int, token string) *domain.ScheduleDefectError {
	t.Helper()
	var de *domain.ScheduleDefectError
	if !errors.As(err, &de) {
		t.Fatalf("want *domain.ScheduleDefectError, got %T: %v", err, err)
	}
	if de.Rule != rule {
		t.Errorf("Rule: want %q, got %q", rule, de.Rule)
	}
	if de.Block != req.Block {
		t.Errorf("Block: want %q, got %q", req.Block, de.Block)
	}
	if de.Source != req.Source {
		t.Errorf("Source: want %q, got %q", req.Source, de.Source)
	}
	if de.Section != req.Section {
		t.Errorf("Section: want %q, got %q", req.Section, de.Section)
	}
	if de.Row != row {
		t.Errorf("Row: want %d, got %d", row, de.Row)
	}
	if de.Token != token {
		t.Errorf("Token: want %q, got %q", token, de.Token)
	}
	return de
}
