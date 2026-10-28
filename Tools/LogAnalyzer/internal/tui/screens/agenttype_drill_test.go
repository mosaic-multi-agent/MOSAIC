package screens_test

// Behaviour of the by-agent-type drill-down: the dispatches of one type,
// their incompleteness labels, back handling and scrolling.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func drilled(run domain.RunReport, h int, keys ...string) (*screens.AgentTypesScreen, string) {
	s := screens.NewAgentTypesScreen(run, tableW, h, plain())
	press(s, keys...)
	return s, s.View(tableW, h)
}

func TestAgentTypesDrill_ShowsTheDispatchesOfTheSelectedType(t *testing.T) {
	s, view := drilled(agentTypesRun(), tallH, "enter")

	got, ok := s.DrilledType()
	if !ok || got != "builder" {
		t.Errorf("DrilledType() = (%q, %v), want (\"builder\", true)", got, ok)
	}
	assertContains(t, view, "builder#1", "$2.21", "builder#2", "$3.43")
	assertNotContains(t, view, "reviewer#1")
}

func TestAgentTypesDrill_SecondTypeAfterMovingDown(t *testing.T) {
	s, view := drilled(agentTypesRun(), tallH, "down", "enter")

	got, _ := s.DrilledType()
	if got != "reviewer" {
		t.Errorf("DrilledType() = %q, want reviewer", got)
	}
	assertContains(t, view, "reviewer#1", "$4.56", "reviewer#2", "$5.67")
	assertNotContains(t, view, "builder#1")
}

func TestAgentTypesDrill_UnknownTypeCanBeDrilledInto(t *testing.T) {
	s, view := drilled(agentTypesRun(), tallH, "down", "down", "enter")

	got, ok := s.DrilledType()
	if !ok || got != "" {
		t.Errorf("DrilledType() = (%q, %v), want (\"\", true)", got, ok)
	}
	assertContains(t, view, "mystery#1", "$7.77")
}

func TestAgentTypesDrill_EscReturnsToTypeListThenLeaves(t *testing.T) {
	s, _ := drilled(agentTypesRun(), tallH, "enter")

	press(s, "esc")
	if _, ok := s.DrilledType(); ok {
		t.Error("first esc must leave the drill-down")
	}
	if s.Back() {
		t.Error("first esc must not leave the per-run views")
	}
	view := s.View(tableW, tallH)
	assertContains(t, view, "builder", "reviewer")
	assertNotContains(t, view, "builder#1")

	press(s, "esc")
	if !s.Back() {
		t.Error("second esc must leave the per-run views")
	}
}

func TestAgentTypesDrill_ReturningKeepsTheCursorOnTheDrilledType(t *testing.T) {
	s, _ := drilled(agentTypesRun(), tallH, "down", "enter", "esc", "enter")

	got, _ := s.DrilledType()
	if got != "reviewer" {
		t.Errorf("DrilledType() = %q, want reviewer (cursor must survive the round trip)", got)
	}
}

func TestAgentTypesDrill_LabelsOpenNoStatusMissingUsageAndStatus(t *testing.T) {
	run := agentTypesRun()
	open := closedDispatch("reviewer#1", "", 0)
	open.Invocation.State = domain.InvocationOpen
	open.Invocation.EndEvents = 0
	open.MissingUsage = true
	open.Totals = domain.Totals{}
	noStatus := closedDispatch("reviewer#2", "", 567)
	blocked := closedDispatch("reviewer#3", "BLOCKED", 100)
	run.ByAgentType[1].Dispatches = []domain.Dispatch{open, noStatus, blocked}

	_, view := drilled(run, tallH, "down", "enter")

	if !strings.Contains(view, screens.OpenLabel) {
		t.Errorf("open dispatch must be labelled %q\n%s", screens.OpenLabel, view)
	}
	assertContains(t, view, screens.MissingUsageLabel, screens.NoStatusLabel, "BLOCKED")
}

func TestAgentTypesDrill_ClosedDispatchWithStatusHasNoOpenOrNoStatusLabel(t *testing.T) {
	_, view := drilled(agentTypesRun(), tallH, "enter")

	assertContains(t, view, "SUCCESS")
	assertNotContains(t, view, screens.NoStatusLabel, screens.MissingUsageLabel)
}

func TestAgentTypesDrill_LabelsUnpricedAndPartialDispatches(t *testing.T) {
	run := agentTypesRun()
	d := closedDispatch("builder#1", "SUCCESS", 0)
	d.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	d.Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	p := closedDispatch("builder#2", "SUCCESS", 343)
	p.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	p.Totals.Money.Complete = false
	run.ByAgentType[0].Dispatches = []domain.Dispatch{d, p}

	_, view := drilled(run, tallH, "enter")

	assertContains(t, view, screens.UnpricedMarker, "$3.43 (partial)")
}

func TestAgentTypesDrill_LabelsAssumedTierCacheWrites(t *testing.T) {
	run := agentTypesRun()
	run.ByAgentType[0].Dispatches[0].Totals.AssumedTierCacheWrite = 999

	_, view := drilled(run, tallH, "enter")

	if line := lineWith(view, "999"); !strings.Contains(line, screens.AssumedTierLabel) {
		t.Errorf("assumed-tier count line %q must carry %q", line, screens.AssumedTierLabel)
	}
}

func TestAgentTypesDrill_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s, first := drilled(manyDispatchesRun(30), shortH, "enter")

	assertFits(t, first, shortH)
	assertContains(t, first, "builder#01")
	assertNotContains(t, first, "builder#30")
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint in the drill-down\n%s", first)
	}

	press(s, repeat("down", 40)...)
	last := s.View(tableW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "builder#30")
	assertNotContains(t, last, "builder#01")
	if got, ok := s.DrilledType(); !ok || got != "builder" {
		t.Errorf("scrolling must keep the drill-down open, got (%q, %v)", got, ok)
	}
}
