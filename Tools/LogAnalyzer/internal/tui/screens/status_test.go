package screens_test

// Behaviour of the by-returned-status-code view: groups per run and per agent
// type, the separate no-status and open groups, labels and scrolling.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func statusView(run domain.RunReport, w, h int) string {
	return screens.NewStatusScreen(run, w, h, plain()).View(w, h)
}

func TestStatus_ShowsEachReportedCodeWithCountAndCost(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	cases := []struct{ amount, code, count string }{
		{"$12.34", "SUCCESS", "7"},
		{"$2.50", "BLOCKED", "2"},
		{"$0.77", "WEIRD_CODE", "1"},
	}
	for _, c := range cases {
		line := rowFor(t, view, c.amount)
		if !strings.Contains(line, c.code) || !hasToken(line, c.count) {
			t.Errorf("row %q must show code %q and count %s", line, c.code, c.count)
		}
	}
}

func TestStatus_KeepsGroupOrder(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	order := []string{"$12.34", "$2.50", "$0.77", "$4.56", "$1.11"}
	last := -1
	for _, amount := range order {
		idx := strings.Index(view, amount)
		if idx < 0 {
			t.Fatalf("view does not contain %q\n%s", amount, view)
		}
		if idx <= last {
			t.Errorf("group with %s is out of order (reported codes, no status, open)\n%s", amount, view)
		}
		last = idx
	}
}

func TestStatus_NoStatusAndOpenAreSeparateLabelledGroups(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	none := rowFor(t, view, "$4.56")
	open := rowFor(t, view, "$1.11")
	if none == open {
		t.Fatalf("no-status and open groups share the row %q", none)
	}
	if !strings.Contains(none, screens.NoStatusLabel) || !hasToken(none, "3") {
		t.Errorf("no-status row %q must carry %q and its count 3", none, screens.NoStatusLabel)
	}
	if strings.Contains(none, screens.OpenLabel) {
		t.Errorf("no-status row %q must not be labelled open", none)
	}
	if !strings.Contains(open, screens.OpenLabel) || !hasToken(open, "2") {
		t.Errorf("open row %q must carry %q and its count 2", open, screens.OpenLabel)
	}
	if strings.Contains(open, screens.NoStatusLabel) {
		t.Errorf("open row %q must not be labelled no status", open)
	}
}

func TestStatus_ReportedRowsAreNeverLabelledNoStatusOrOpen(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	for _, amount := range []string{"$12.34", "$2.50", "$0.77"} {
		line := rowFor(t, view, amount)
		if strings.Contains(line, screens.NoStatusLabel) || strings.Contains(line, screens.OpenLabel) {
			t.Errorf("reported row %q must not carry the no-status or open label", line)
		}
	}
}

func TestStatus_ShowsStatusGroupsPerAgentType(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	builder := strings.Index(view, "builder")
	reviewer := strings.Index(view, "reviewer")
	unknown := strings.Index(view, screens.UnknownTypeLabel)
	if builder < 0 || reviewer < 0 || unknown < 0 {
		t.Fatalf("view must name builder, reviewer and %q\n%s", screens.UnknownTypeLabel, view)
	}
	if !(builder < reviewer && reviewer < unknown) {
		t.Errorf("agent types must keep the report order, unknown type last\n%s", view)
	}
	sections := []struct {
		start, end int
		amounts    []string
	}{
		{builder, reviewer, []string{"$7.07", "$0.40"}},
		{reviewer, unknown, []string{"$3.03", "$0.09"}},
		{unknown, len(view), []string{"$0.88"}},
	}
	for _, sec := range sections {
		for _, amount := range sec.amounts {
			idx := strings.Index(view, amount)
			if idx < sec.start || idx >= sec.end {
				t.Errorf("amount %s is not listed under its agent type\n%s", amount, view)
			}
		}
	}
}

func TestStatus_AgentTypeNoStatusAndOpenAreSeparateLabelledGroups(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	none := rowFor(t, view, "$0.40")
	open := rowFor(t, view, "$0.09")
	if !strings.Contains(none, screens.NoStatusLabel) || strings.Contains(none, screens.OpenLabel) {
		t.Errorf("builder no-status row %q must carry only %q", none, screens.NoStatusLabel)
	}
	if !strings.Contains(open, screens.OpenLabel) || strings.Contains(open, screens.NoStatusLabel) {
		t.Errorf("reviewer open row %q must carry only %q", open, screens.OpenLabel)
	}
}

func TestStatus_UnpricedModelsMarkTheGroupPartial(t *testing.T) {
	run := statusRun()
	g := &run.ByStatus.Run[0]
	g.Figures.UnpricedModels = []domain.ModelID{"model-x"}
	g.Figures.Totals.Money = domain.CategoryMoney{Total: known(1234), Complete: false}

	line := rowFor(t, statusView(run, wideW, tallH), "$12.34")

	if !strings.Contains(line, "$12.34 (partial)") || !strings.Contains(line, "model-x") {
		t.Errorf("row %q must mark the amount partial and name the unpriced model", line)
	}
}

func TestStatus_LabelsAssumedTierCacheWrites(t *testing.T) {
	run := statusRun()
	run.ByStatus.Run[1].Figures.Totals.AssumedTierCacheWrite = 12345

	line := rowFor(t, statusView(run, wideW, tallH), "$2.50")

	if !strings.Contains(line, screens.AssumedTierLabel) || !strings.Contains(line, "12,345") {
		t.Errorf("row %q must carry %q and the grouped count", line, screens.AssumedTierLabel)
	}
}

func TestStatus_LabelsMissingTokenUsage(t *testing.T) {
	run := statusRun()
	run.ByStatus.Run[3].Figures.MissingUsage = 2

	line := rowFor(t, statusView(run, wideW, tallH), "$4.56")

	if !strings.Contains(line, screens.MissingUsageLabel) {
		t.Errorf("row %q must carry %q", line, screens.MissingUsageLabel)
	}
}

func TestStatus_CompleteRunHasNoIncompletenessLabels(t *testing.T) {
	view := statusView(statusRun(), wideW, tallH)

	assertContains(t, view, "SUCCESS", "$12.34")
	assertNotContains(t, view, "(partial)", screens.UnpricedMarker, screens.AssumedTierLabel, screens.MissingUsageLabel)
}

func TestStatus_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := screens.NewStatusScreen(manyStatusTypesRun(20), wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertNotContains(t, first, "type-20")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 80)...)
	last := scrolled.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "type-20")
	assertNotContains(t, last, "type-01")

	back := press(scrolled, repeat("up", 80)...).View(wideW, shortH)
	if back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestStatus_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := statusView(manyStatusTypesRun(20), wideW, 200)

	assertContains(t, view, "type-01", "type-20")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestStatus_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40
	run := statusRun()
	run.ByStatus.Run[0].Figures.UnpricedModels = []domain.ModelID{"a-very-long-unpriced-model-name"}

	view := statusView(run, narrow, tallH)

	assertContains(t, view, "SUCCESS")
	assertFitsWidth(t, view, narrow)
}

func TestStatus_RunWithoutStatusDataStillRenders(t *testing.T) {
	view := statusView(overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without status groups must still render a page")
	}
}

func TestStatus_AgentTypeSectionLabelsOpenAndMissingUsage(t *testing.T) {
	run := statusRun()
	run.ByStatus.ByAgentType[1].Groups[1].Figures.MissingUsage = 1

	line := rowFor(t, statusView(run, wideW, tallH), "$0.09")

	if !strings.Contains(line, screens.OpenLabel) || !strings.Contains(line, screens.MissingUsageLabel) {
		t.Errorf("agent-type open row %q must carry %q and %q", line, screens.OpenLabel, screens.MissingUsageLabel)
	}
}
