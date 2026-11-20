package runflow

// progress_scroll_test.go verifies scrolling the step list with the keys up,
// down, pgup, pgdown, home and end: what each key does in the normal state, that
// they do nothing while a stop gate is open, and that no other key changes
// meaning. The screen is drawn at 200 columns so nothing wraps; the number of
// rows that fit is read from the view rather than assumed.

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const scrollRowCount = 60

var (
	scrollRowRe   = regexp.MustCompile(`row-(\d\d)#`)
	scrollOlderRe = regexp.MustCompile(`\x{2191} (\d+) older`)
	scrollNewerRe = regexp.MustCompile(`\x{2193} (\d+) newer`)
	ansiRe        = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

func scrollRowName(i int) string { return fmt.Sprintf("row-%02d#1", i) }

// newScrollScreen is a 200-column screen of the given height holding n finished
// step rows named row-00 .. row-(n-1), with no panel begun.
func newScrollScreen(height, n int) *ProgressScreen {
	s := NewProgressScreen(200, height, progressStyles())
	addScrollRows(s, 0, n)
	return s
}

// addScrollRows appends finished rows with indexes from..to-1.
func addScrollRows(s *ProgressScreen, from, to int) {
	for i := from; i < to; i++ {
		s.AppendRow(ProgressRow{AgentInstance: scrollRowName(i), Phase: "EXECUTION"})
		s.CompleteRow("SUCCESS")
	}
}

func pressScrollKey(s *ProgressScreen, typ tea.KeyType, times int) {
	for i := 0; i < times; i++ {
		s.Update(tea.KeyMsg{Type: typ})
	}
}

// visibleRows returns the indexes of the step rows shown in view, in order.
func visibleRows(view string) []int {
	var out []int
	for _, m := range scrollRowRe.FindAllStringSubmatch(view, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// firstVisible returns the index of the oldest step row shown, failing when no
// row is shown or the shown rows are not a contiguous ascending range.
func firstVisible(t *testing.T, s *ProgressScreen) int {
	t.Helper()
	rows := visibleRows(s.View())
	if len(rows) == 0 {
		t.Fatalf("no step row shown:\n%s", s.View())
	}
	for i := 1; i < len(rows); i++ {
		if rows[i] != rows[i-1]+1 {
			t.Fatalf("shown rows %v are not one contiguous range", rows)
		}
	}
	return rows[0]
}

func visibleCount(s *ProgressScreen) int { return len(visibleRows(s.View())) }

// lastVisible returns the index of the newest step row shown.
func lastVisible(t *testing.T, s *ProgressScreen) int {
	t.Helper()
	rows := visibleRows(s.View())
	if len(rows) == 0 {
		t.Fatalf("no step row shown:\n%s", s.View())
	}
	return rows[len(rows)-1]
}

// indicatorCount returns the count printed by the older (up) or newer (down)
// indicator of view, and whether the indicator is shown.
func indicatorCount(view string, older bool) (int, bool) {
	re := scrollNewerRe
	if older {
		re = scrollOlderRe
	}
	m := re.FindStringSubmatch(ansiRe.ReplaceAllString(view, ""))
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	return n, true
}

func TestScroll_AtBottom_ShowsTheNewestRows(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)

	if got := lastVisible(t, s); got != scrollRowCount-1 {
		t.Errorf("newest shown row = %d, want %d", got, scrollRowCount-1)
	}
}

func TestScroll_Up_MovesTheWindowOneRowTowardOlder(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	before := firstVisible(t, s)

	pressScrollKey(s, tea.KeyUp, 1)

	if got := firstVisible(t, s); got != before-1 {
		t.Errorf("oldest shown row = %d after up, want %d", got, before-1)
	}
	if got := lastVisible(t, s); got == scrollRowCount-1 {
		t.Errorf("newest row still shown after up from the bottom")
	}
}

func TestScroll_UpThenDown_ReturnsToTheBottom(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	bottomFirst := firstVisible(t, s)

	pressScrollKey(s, tea.KeyUp, 1)
	if got := firstVisible(t, s); got != bottomFirst-1 {
		t.Fatalf("precondition: oldest shown row = %d after up, want %d", got, bottomFirst-1)
	}
	pressScrollKey(s, tea.KeyDown, 1)

	if got := firstVisible(t, s); got != bottomFirst {
		t.Errorf("oldest shown row = %d after up then down, want %d", got, bottomFirst)
	}
	if got := lastVisible(t, s); got != scrollRowCount-1 {
		t.Errorf("newest shown row = %d, want %d", got, scrollRowCount-1)
	}
}

func TestScroll_Down_AtTheBottom_ChangesNothing(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	before := s.View()

	pressScrollKey(s, tea.KeyDown, 3)

	if after := s.View(); after != before {
		t.Errorf("view changed on down at the bottom:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestScroll_Up_ManyTimes_StopsAtTheOldestRow(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)

	pressScrollKey(s, tea.KeyUp, scrollRowCount+10)

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after scrolling past the top, want 0", got)
	}
	before := s.View()
	pressScrollKey(s, tea.KeyUp, 1)
	if after := s.View(); after != before {
		t.Errorf("view changed on up at the oldest row")
	}
}

func TestScroll_PageUp_MovesByTheRowsShownBeforeTheKey(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyUp, 1)
	first, shown := firstVisible(t, s), visibleCount(s)

	pressScrollKey(s, tea.KeyPgUp, 1)

	if got := firstVisible(t, s); got != first-shown {
		t.Errorf("oldest shown row = %d after pgup, want %d (%d - %d shown)", got, first-shown, first, shown)
	}
}

func TestScroll_PageUp_NearTheTop_ClampsAtTheOldestRow(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	pressScrollKey(s, tea.KeyPgDown, 1)

	pressScrollKey(s, tea.KeyPgUp, 3)

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after pgup past the top, want 0", got)
	}
}

func TestScroll_PageDown_MovesByTheRowsShownBeforeTheKey(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	shown := visibleCount(s)

	pressScrollKey(s, tea.KeyPgDown, 1)

	if got := firstVisible(t, s); got != shown {
		t.Errorf("oldest shown row = %d after pgdown from the top, want %d", got, shown)
	}
}

func TestScroll_PageDown_NearTheBottom_ClampsAtTheNewestRows(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	bottomFirst := firstVisible(t, s)
	pressScrollKey(s, tea.KeyUp, 2)
	if got := firstVisible(t, s); got != bottomFirst-2 {
		t.Fatalf("precondition: oldest shown row = %d after two ups, want %d", got, bottomFirst-2)
	}

	pressScrollKey(s, tea.KeyPgDown, 3)

	if got := firstVisible(t, s); got != bottomFirst {
		t.Errorf("oldest shown row = %d after pgdown past the bottom, want %d", got, bottomFirst)
	}
	if got := lastVisible(t, s); got != scrollRowCount-1 {
		t.Errorf("newest shown row = %d, want %d", got, scrollRowCount-1)
	}
}

func TestScroll_Home_JumpsToTheOldestRows(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)

	pressScrollKey(s, tea.KeyHome, 1)

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after home, want 0", got)
	}
	if got := lastVisible(t, s); got >= scrollRowCount-1 {
		t.Errorf("newest row shown after home on a long list")
	}
}

func TestScroll_End_JumpsToTheNewestRows(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	if got := firstVisible(t, s); got != 0 {
		t.Fatalf("precondition: oldest shown row = %d after home, want 0", got)
	}

	pressScrollKey(s, tea.KeyEnd, 1)

	if got := lastVisible(t, s); got != scrollRowCount-1 {
		t.Errorf("newest shown row = %d after end, want %d", got, scrollRowCount-1)
	}
}

func TestScroll_Down_FromTheTop_MovesTheWindowOneRowTowardNewer(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)

	pressScrollKey(s, tea.KeyDown, 1)

	if got := firstVisible(t, s); got != 1 {
		t.Errorf("oldest shown row = %d after home then down, want 1", got)
	}
}

func TestScroll_PageDown_FromTheMiddle_MovesByTheRowsShownBeforeTheKey(t *testing.T) {
	s := newScrollScreen(20, scrollRowCount)
	pressScrollKey(s, tea.KeyHome, 1)
	pressScrollKey(s, tea.KeyDown, 3)
	first, shown := firstVisible(t, s), visibleCount(s)
	if first != 3 {
		t.Fatalf("precondition: oldest shown row = %d, want 3", first)
	}

	pressScrollKey(s, tea.KeyPgDown, 1)

	if got := firstVisible(t, s); got != first+shown {
		t.Errorf("oldest shown row = %d after pgdown, want %d (%d + %d shown)", got, first+shown, first, shown)
	}
}

func TestScroll_EmptyList_KeysAreNoOps(t *testing.T) {
	s := newScrollScreen(20, 0)
	before := s.View()

	for _, k := range []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd} {
		pressScrollKey(s, k, 1)
	}

	if after := s.View(); after != before {
		t.Errorf("view changed on scroll keys with no step rows:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestScroll_ListThatFits_AllKeysAreNoOps(t *testing.T) {
	s := newScrollScreen(30, 3)
	before := s.View()

	for _, k := range []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd} {
		pressScrollKey(s, k, 1)
	}

	if after := s.View(); after != before {
		t.Errorf("view changed on scroll keys with every row visible")
	}
	if got := visibleRows(s.View()); len(got) != 3 {
		t.Errorf("shown rows = %v, want all 3", got)
	}
}

func TestScroll_Keys_DoNothingWhileAStopGateIsOpen(t *testing.T) {
	gates := []struct {
		name string
		open func(s *ProgressScreen)
		held func(s *ProgressScreen) bool
		shut string
	}{
		{"graceful stop", func(s *ProgressScreen) { pressProgressKey(s, "s") }, (*ProgressScreen).ConfirmPending, "n"},
		{"force stop", func(s *ProgressScreen) { pressProgressKey(s, "x") }, (*ProgressScreen).ForceStopConfirmPending, "n"},
	}
	keys := []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd}
	for _, g := range gates {
		t.Run(g.name, func(t *testing.T) {
			s := newScrollScreen(24, scrollRowCount)
			pressScrollKey(s, tea.KeyUp, 2)
			first := firstVisible(t, s)
			g.open(s)
			if !g.held(s) {
				t.Fatal("precondition: the gate did not open")
			}

			for _, k := range keys {
				pressScrollKey(s, k, 1)
			}

			if !g.held(s) {
				t.Error("a scroll key closed the gate")
			}
			if got := firstVisible(t, s); got != first {
				t.Errorf("oldest shown row = %d, want %d: scroll keys moved the list under the gate", got, first)
			}
			pressProgressKey(s, g.shut)
			if got := firstVisible(t, s); got != first {
				t.Errorf("oldest shown row = %d after closing the gate, want %d", got, first)
			}
			pressScrollKey(s, tea.KeyHome, 1)
			if got := firstVisible(t, s); got != 0 {
				t.Errorf("home did not scroll once the gate was closed: oldest shown row = %d", got)
			}
		})
	}
}

func TestScroll_Keys_WorkAfterAConfirmedStop(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)
	pressProgressKey(s, "s")
	pressProgressKey(s, "y")
	if !s.GracefulStop() {
		t.Fatal("precondition: stop not confirmed")
	}

	pressScrollKey(s, tea.KeyHome, 1)

	if got := firstVisible(t, s); got != 0 {
		t.Errorf("oldest shown row = %d after home following a confirmed stop, want 0", got)
	}
}

func TestScroll_Keys_DoNotReachRequestGuardOrArtifactHandling(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)

	for _, k := range []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd} {
		pressScrollKey(s, k, 1)
	}

	if ev := s.TakeRequestKeyEvents(); len(ev) != 0 {
		t.Errorf("scroll keys recorded request events %+v", ev)
	}
	if ev := s.TakeGuardKeyEvents(); len(ev) != 0 {
		t.Errorf("scroll keys recorded guard events %+v", ev)
	}
	if s.ArtifactViewRequested() || s.ConfirmPending() || s.ForceStopConfirmPending() || s.GracefulStop() {
		t.Error("a scroll key triggered a stop, force-stop or artifact action")
	}
}

func TestScroll_OtherKeys_KeepTheirMeaningAndDoNotScroll(t *testing.T) {
	s := newScrollScreen(24, scrollRowCount)
	pressScrollKey(s, tea.KeyUp, 2)
	first := firstVisible(t, s)

	pressProgressKey(s, "j")
	pressProgressKey(s, "k")
	if got := firstVisible(t, s); got != first {
		t.Errorf("j or k scrolled the list: oldest shown row = %d, want %d", got, first)
	}
	pressProgressKey(s, "a")
	if !s.ArtifactViewRequested() {
		t.Error("a no longer requests the artifact view")
	}
	pressProgressKey(s, "l")
	if s.ActivityVisible() {
		t.Error("l no longer toggles the activity panel")
	}
	pressProgressKey(s, "s")
	if !s.ConfirmPending() {
		t.Error("s no longer opens the graceful-stop gate")
	}
	if got := firstVisible(t, s); got != first {
		t.Errorf("existing keys moved the list: oldest shown row = %d, want %d", got, first)
	}
}
