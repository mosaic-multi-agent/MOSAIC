package runflow

import (
	"fmt"
	"strings"
)

// stepScroll is the scroll position of the step list. The zero value follows
// the newest rows. After the user scrolls up, off is the index of the oldest
// shown row and stays put while rows are appended or change status.
type stepScroll struct {
	scrolled bool
	off      int
}

// stepWindow is the part of the step list shown for one line budget.
type stepWindow struct {
	start int  // index of the oldest shown row
	count  int // rows shown
	older  int // rows hidden above the window
	newer  int // rows hidden below the window
	bottom int // start index of the window at the bottom
}

// stepWindowFor computes the window of the step list for budget lines. All
// rows fit without indicators; otherwise the window has budget-1 rows at the
// top and the bottom (one indicator line) and budget-2 rows in between (two
// indicator lines), never fewer than one.
func (s *ProgressScreen) stepWindowFor(budget int) stepWindow {
	n := len(s.rows)
	if n <= budget {
		return stepWindow{count: n}
	}
	edge := max(1, budget-1)
	w := stepWindow{bottom: n - edge}
	w.start = w.bottom
	if s.scroll.scrolled && s.scroll.off < w.bottom {
		w.start = max(0, s.scroll.off)
	}
	w.count = edge
	if w.start > 0 && w.start < w.bottom {
		w.count = max(1, budget-2)
	}
	w.older = w.start
	w.newer = n - w.start - w.count
	return w
}

// currentWindow is the window for the screen's present line budget.
func (s *ProgressScreen) currentWindow() stepWindow {
	return s.stepWindowFor(s.stepLineBudget(s.optionalParts()))
}

// scrollStepList handles the step-list scroll keys and reports whether key was
// one of them. It moves the view relative to the window currently shown and
// turns following on when the view reaches the newest rows.
func (s *ProgressScreen) scrollStepList(key string) bool {
	switch key {
	case "up", "down", "pgup", "pgdown", "home", "end":
	default:
		return false
	}
	w := s.currentWindow()
	maxOff := w.bottom
	page := w.count
	if page < 1 {
		page = 1
	}
	off := w.start
	switch key {
	case "up":
		off--
	case "down":
		off++
	case "pgup":
		off -= page
	case "pgdown":
		off += page
	case "home":
		off = 0
	case "end":
		off = maxOff
	}
	if off > maxOff {
		off = maxOff
	}
	if off < 0 {
		off = 0
	}
	s.scroll = stepScroll{scrolled: off < maxOff, off: off}
	return true
}

// settleScroll turns following on when the scrolled view already shows the
// newest row, so rows added later (or a larger screen) keep it there.
func (s *ProgressScreen) settleScroll() {
	if !s.scroll.scrolled {
		return
	}
	if s.scroll.off >= s.currentWindow().bottom {
		s.scroll = stepScroll{}
	}
}

// stepRowsBlock renders the step rows of the window with the indicator lines.
func (s *ProgressScreen) stepRowsBlock(budget int) string {
	w := s.stepWindowFor(budget)
	var lines []string
	if w.older > 0 {
		lines = append(lines, s.scrollIndicator("↑", w.older, "older"))
	}
	for i := w.start; i < w.start+w.count; i++ {
		lines = append(lines, s.stepRowLine(i))
	}
	if w.newer > 0 {
		lines = append(lines, s.scrollIndicator("↓", w.newer, "newer"))
	}
	return strings.Join(lines, "\n")
}

// scrollIndicator renders the count of rows hidden in one direction.
func (s *ProgressScreen) scrollIndicator(arrow string, hidden int, word string) string {
	return s.styles.Muted.Render(fitWidth(fmt.Sprintf("  %s %d %s", arrow, hidden, word), s.width))
}

// runningRowLabel is the label of a row that is running or has no status yet;
// the iteration is shown only for a running row.
func runningRowLabel(row ProgressRow, running bool) string {
	label := fmt.Sprintf("%s  phase=%s", row.AgentInstance, row.Phase)
	if running && row.Stage != "" {
		label += fmt.Sprintf(" iteration=%s", row.Stage)
	}
	return label
}

// stepRowLine renders row i as one line no wider than the screen.
func (s *ProgressScreen) stepRowLine(i int) string {
	row := s.rows[i]
	if isFailedInfraRow(row) && row.Status != "running" && s.runningIdx != i {
		return s.renderInfraFailureRow(row)
	}
	labelWidth := s.width - 2
	if row.Status == "running" || s.runningIdx == i {
		return "▶ " + s.styles.Selected.Render(fitWidth(runningRowLabel(row, true), labelWidth))
	}
	if row.Status == "" {
		return "  " + s.styles.Body.Render(fitWidth(runningRowLabel(row, false), labelWidth))
	}
	label := fitWidth(fmt.Sprintf("%s  %s", row.AgentInstance, row.Status), labelWidth)
	switch {
	case row.Status == "SUCCESS":
		return "  " + s.styles.Success.Render(label)
	case isRunnerRecordedStatus(row.Status):
		return "  " + s.runnerRecordedStyle().Render(label)
	case row.Status == "BLOCKED" || row.Status == "CAPABILITY_EXCEEDED":
		return "  " + s.styles.Error.Render(label)
	default:
		return "  " + s.styles.Warning.Render(label)
	}
}
