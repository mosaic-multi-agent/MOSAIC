package runflow

// progress_scroll_follow_test.go verifies follow behaviour of the scrolled step
// list (stable view after scrolling up, End or returning to the bottom turning
// following on, SetHistory and Resize) and the older/newer indicators, which
// appear exactly when rows are hidden in that direction and show the count.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// assertIndicatorsMatchWindow checks the indicators of s against the rows
// shown: the older count is the rows above the window, the newer count the rows
// below it, and each indicator is present exactly when its count is positive.
func assertIndicatorsMatchWindow(t *testing.T, s *ProgressScreen, total int) {
	t.Helper()
	view := s.View()
	rows := visibleRows(view)
	if len(rows) == 0 {
		t.Fatalf("no step row shown:\n%s", view)
	}
	wantOlder, wantNewer := rows[0], total-1-rows[len(rows)-1]
	gotOlder, hasOlder := indicatorCount(view, true)
	gotNewer, hasNewer := indicatorCount(view, false)
	if hasOlder != (wantOlder > 0) || (hasOlder && gotOlder != wantOlder) {
		t.Errorf("older indicator shown=%v count=%d, want shown=%v count=%d:\n%s", hasOlder, gotOlder, wantOlder > 0, wantOlder, view)
	}
	if hasNewer != (wantNewer > 0) || (hasNewer && gotNewer != wantNewer) {
		t.Errorf("newer indicator shown=%v count=%d, want shown=%v count=%d:\n%s", hasNewer, gotNewer, wantNewer > 0, wantNewer, view)
	}
}

func TestScrollIndicators_ListThatFits_ShowsNeither(t *testing.T) {
	s := newScrollScreen(30, 3)

	view := s.View()

	if _, ok := indicatorCount(view, true); ok {
		t.Errorf("older indicator shown with every row visible:\n%s", view)
	}
	if _, ok := indicatorCount(view, false); ok {
		t.Errorf("newer indicator shown with every row visible:\n%s", view)
	}
}

func TestScrollIndicators_MatchTheHiddenRowsAtEveryPosition(t *testing.T) {
	steps := []struct {
		name string
		keys []tea.KeyType
	}{
		{"following at the bottom", nil},
		{"one row up", []tea.KeyType{tea.KeyUp}},
		{"a page up", []tea.KeyType{tea.KeyUp, tea.KeyPgUp}},
		{"top", []tea.KeyType{tea.KeyHome}},
		{"one page down from the top", []tea.KeyType{tea.KeyHome, tea.KeyPgDown}},
		{"down from the top", []tea.KeyType{tea.KeyHome, tea.KeyDown}},
		{"back at the bottom", []tea.KeyType{tea.KeyHome, tea.KeyEnd}},
	}
	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			s := newScrollScreen(20, scrollRowCount)
			for _, k := range st.keys {
				pressScrollKey(s, k, 1)
			}

			assertIndicatorsMatchWindow(t, s, scrollRowCount)
		})
	}
}

func TestScrollIndicators_AtTheTop_ShowOnlyNewer_AtTheBottom_OnlyOlder(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)

	if _, ok := indicatorCount(s.View(), true); !ok {
		t.Error("older indicator missing at the bottom of a long list")
	}
	if _, ok := indicatorCount(s.View(), false); ok {
		t.Error("newer indicator shown at the bottom")
	}
	pressScrollKey(s, tea.KeyHome, 1)
	if _, ok := indicatorCount(s.View(), true); ok {
		t.Error("older indicator shown at the top")
	}
	if _, ok := indicatorCount(s.View(), false); !ok {
		t.Error("newer indicator missing at the top of a long list")
	}
}

func TestScrollIndicators_UseArrowsAndWords(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyUp, 1)

	view := ansiRe.ReplaceAllString(s.View(), "")

	if !strings.Contains(view, "\u2191 ") || !strings.Contains(view, " older") {
		t.Errorf("view lacks the older indicator:\n%s", view)
	}
	if !strings.Contains(view, "\u2193 ") || !strings.Contains(view, " newer") {
		t.Errorf("view lacks the newer indicator:\n%s", view)
	}
}

func TestScrollIndicators_NewerLineFollowsTheLastRowBeforeConsultationLine(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)
	s.StartConsultation(routingNoKind)
	pressScrollKey(s, tea.KeyHome, 1)

	lines := strings.Split(ansiRe.ReplaceAllString(s.View(), ""), "\n")
	last, newer, consult := -1, -1, -1
	for i, l := range lines {
		switch {
		case scrollRowRe.MatchString(l):
			last = i
		case scrollNewerRe.MatchString(l):
			newer = i
		case strings.Contains(l, "consultation: routing"):
			consult = i
		}
	}

	if newer != last+1 {
		t.Errorf("newer indicator on line %d, want the line after the last step row (%d)", newer, last)
	}
	if consult <= newer {
		t.Errorf("consultation line %d is not below the newer indicator %d", consult, newer)
	}
}

func TestScrollIndicators_OlderLineIsTheFirstLineOfTheRowsBlock(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)

	lines := strings.Split(ansiRe.ReplaceAllString(s.View(), ""), "\n")
	older, first := -1, -1
	for i, l := range lines {
		if older < 0 && scrollOlderRe.MatchString(l) {
			older = i
		}
		if first < 0 && scrollRowRe.MatchString(l) {
			first = i
		}
	}

	if older < 0 || first != older+1 {
		t.Errorf("older indicator on line %d, first step row on line %d; want the row right after the indicator", older, first)
	}
}

func TestScrollFollow_AtTheBottom_NewRowsStayInView(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)

	addScrollRows(s, scrollRowCount, scrollRowCount+5)

	if got := lastVisible(t, s); got != scrollRowCount+4 {
		t.Errorf("newest shown row = %d, want %d", got, scrollRowCount+4)
	}
	assertIndicatorsMatchWindow(t, s, scrollRowCount+5)
}

func TestScrollFollow_AfterScrollingUp_AppendedRowsDoNotMoveTheView(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyUp, 3)
	first, shown := firstVisible(t, s), visibleCount(s)
	newerBefore, _ := indicatorCount(s.View(), false)

	addScrollRows(s, scrollRowCount, scrollRowCount+5)

	if got := firstVisible(t, s); got != first {
		t.Errorf("oldest shown row = %d after appends, want %d", got, first)
	}
	if got := visibleCount(s); got != shown {
		t.Errorf("rows shown = %d after appends, want %d", got, shown)
	}
	if got, _ := indicatorCount(s.View(), false); got != newerBefore+5 {
		t.Errorf("newer count = %d after 5 appends, want %d", got, newerBefore+5)
	}
}

func TestScrollFollow_AfterScrollingUp_CompletingTheRunningRowDoesNotMoveTheView(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	s.AppendRow(ProgressRow{AgentInstance: scrollRowName(scrollRowCount), Phase: "EXECUTION", Status: "running"})
	bottomFirst := firstVisible(t, s)
	pressScrollKey(s, tea.KeyUp, 3)
	first, shown := firstVisible(t, s), visibleCount(s)
	if first != bottomFirst-3 {
		t.Fatalf("precondition: oldest shown row = %d after three ups, want %d", first, bottomFirst-3)
	}

	s.CompleteRow("SUCCESS")

	if got := firstVisible(t, s); got != first || visibleCount(s) != shown {
		t.Errorf("window changed to first=%d shown=%d after completing a row, want first=%d shown=%d", got, visibleCount(s), first, shown)
	}
}

func TestScrollFollow_End_TurnsFollowingBackOn(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	if got := firstVisible(t, s); got != 0 {
		t.Fatalf("precondition: oldest shown row = %d after home, want 0", got)
	}
	addScrollRows(s, scrollRowCount, scrollRowCount+3)
	if got := firstVisible(t, s); got != 0 {
		t.Fatalf("precondition: appends moved the scrolled view to oldest row %d", got)
	}

	pressScrollKey(s, tea.KeyEnd, 1)
	addScrollRows(s, scrollRowCount+3, scrollRowCount+8)

	if got := lastVisible(t, s); got != scrollRowCount+7 {
		t.Errorf("newest shown row = %d, want %d: end did not turn following on", got, scrollRowCount+7)
	}
}

func TestScrollFollow_ScrollingDownToTheBottom_TurnsFollowingBackOn(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	bottomFirst := firstVisible(t, s)
	pressScrollKey(s, tea.KeyUp, 2)
	if got := firstVisible(t, s); got != bottomFirst-2 {
		t.Fatalf("precondition: oldest shown row = %d after two ups, want %d", got, bottomFirst-2)
	}
	pressScrollKey(s, tea.KeyDown, 2)

	addScrollRows(s, scrollRowCount, scrollRowCount+4)

	if got := lastVisible(t, s); got != scrollRowCount+3 {
		t.Errorf("newest shown row = %d, want %d: returning to the bottom did not turn following on", got, scrollRowCount+3)
	}
}

func TestScrollFollow_PageDownReachingTheBottom_TurnsFollowingBackOn(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	bottomFirst := firstVisible(t, s)
	pressScrollKey(s, tea.KeyUp, 2)
	if got := firstVisible(t, s); got != bottomFirst-2 {
		t.Fatalf("precondition: oldest shown row = %d after two ups, want %d", got, bottomFirst-2)
	}
	pressScrollKey(s, tea.KeyPgDown, 1)

	addScrollRows(s, scrollRowCount, scrollRowCount+4)

	if got := lastVisible(t, s); got != scrollRowCount+3 {
		t.Errorf("newest shown row = %d, want %d: pgdown to the bottom did not turn following on", got, scrollRowCount+3)
	}
}

func TestScrollFollow_SetHistory_ShowsTheNewestRowsAgain(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	var history []ProgressRow
	for i := 0; i < 40; i++ {
		history = append(history, ProgressRow{AgentInstance: scrollRowName(i), Phase: "EXECUTION", Status: "SUCCESS"})
	}

	s.SetHistory(history)

	if got := lastVisible(t, s); got != 39 {
		t.Errorf("newest shown row = %d after SetHistory, want 39", got)
	}
	assertIndicatorsMatchWindow(t, s, 40)
	addScrollRows(s, 40, 42)
	if got := lastVisible(t, s); got != 41 {
		t.Errorf("newest shown row = %d after an append following SetHistory, want 41", got)
	}
}

func TestScrollFollow_SetHistoryWithFewerRows_LeavesAValidView(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)

	s.SetHistory([]ProgressRow{{AgentInstance: scrollRowName(0), Status: "SUCCESS"}, {AgentInstance: scrollRowName(1), Status: "SUCCESS"}})

	if got := visibleRows(s.View()); len(got) != 2 {
		t.Errorf("shown rows = %v, want both rows of the new history", got)
	}
	assertIndicatorsMatchWindow(t, s, 2)
}

func TestScrollResize_KeepsTheTopRowWhenItCan(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)

	s.Resize(200, 28)

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after growing the screen from the top, want 0", got)
	}
	assertIndicatorsMatchWindow(t, s, scrollRowCount)
}

func TestScrollResize_ShrinkingKeepsAValidWindow(t *testing.T) {
	s := newScrollScreen(30, scrollRowCount)
	pressScrollKey(s, tea.KeyUp, 5)

	s.Resize(200, 14)

	assertIndicatorsMatchWindow(t, s, scrollRowCount)
	if got := lastVisible(t, s); got >= scrollRowCount {
		t.Errorf("shown row %d is outside the list", got)
	}
}

func TestScrollResize_GrowingUntilEveryRowFits_ShowsAllRowsAndFollowsAgain(t *testing.T) {
	s := newScrollScreen(20, 30)
	pressScrollKey(s, tea.KeyUp, 1)

	s.Resize(200, 60)

	if got := visibleRows(s.View()); len(got) != 30 {
		t.Fatalf("shown rows = %d, want all 30 after the resize", len(got))
	}
	addScrollRows(s, 30, 70)
	if got := lastVisible(t, s); got != 69 {
		t.Errorf("newest shown row = %d, want 69: the clamp to the bottom did not turn following on", got)
	}
}

func TestScrollFollow_BudgetChange_KeepsTheScrolledView(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)

	s.SetPendingRequests(PendingRequests{ManualDispatch: true})
	s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Accepted: true})

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after the budget shrank, want 0", got)
	}
	assertIndicatorsMatchWindow(t, s, scrollRowCount)
}
