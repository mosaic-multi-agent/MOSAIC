package runflow

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"mosaic-run/internal/domain"
)

// maxActivityItems bounds the items kept for the current invocation; older
// ones are dropped so an event flood cannot grow the screen's memory.
const maxActivityItems = 100

// activityPlaceholder is shown until the running invocation reports its first event.
const activityPlaceholder = "no live output available"

// ActivityKey identifies one invocation's activity on the screen.
type ActivityKey uint64

// activityPanel is the live activity of the current invocation.
type activityPanel struct {
	hidden  bool // zero value: the panel is shown
	begun   bool
	key     ActivityKey
	inv     domain.ActivityInvocation
	started time.Time
	endedAt time.Time // zero while the invocation runs
	events  int       // items received, usage included
	items   []domain.ActivityItem

	status    domain.ActivityStatus // latest stream status of the invocation
	statusAt  time.Time             // when status was received; zero before the first
	toolSince time.Time             // when the open tool call was first reported; zero when none
}

// BeginActivity makes inv the current invocation: clears the previous items,
// starts its elapsed clock, and shows the placeholder until the first item.
func (s *ProgressScreen) BeginActivity(key ActivityKey, inv domain.ActivityInvocation) {
	s.activity = activityPanel{
		hidden:  s.activity.hidden,
		begun:   true,
		key:     key,
		inv:     inv,
		started: s.activityNow(),
	}
}

// AppendActivity adds item to the current invocation; ignored when key is not current.
func (s *ProgressScreen) AppendActivity(key ActivityKey, item domain.ActivityItem) {
	a := &s.activity
	if !a.begun || a.key != key {
		return
	}
	a.events++
	if item.Kind == domain.ActivityKindUsage {
		return
	}
	a.items = append(a.items, item)
	if over := len(a.items) - maxActivityItems; over > 0 {
		a.items = append([]domain.ActivityItem(nil), a.items[over:]...)
	}
}

// EndActivity marks the current invocation as ended; ignored when key is not current.
func (s *ProgressScreen) EndActivity(key ActivityKey) {
	a := &s.activity
	if !a.begun || a.key != key || !a.endedAt.IsZero() {
		return
	}
	a.endedAt = s.activityNow()
}

// ActivityVisible reports whether the panel is shown.
func (s *ProgressScreen) ActivityVisible() bool { return !s.activity.hidden }

// toggleActivity shows or hides the panel.
func (s *ProgressScreen) toggleActivity() { s.activity.hidden = !s.activity.hidden }

// activityItemLine renders one item in short form.
func activityItemLine(it domain.ActivityItem) string {
	switch it.Kind {
	case domain.ActivityToolCall:
		return strings.TrimSpace("  " + it.Name + "  " + it.Summary)
	case domain.ActivitySubagent:
		return strings.TrimSpace("  subagent " + it.Name + "  " + it.Summary)
	default:
		return "  " + it.Summary
	}
}

// activityLines renders the panel within maxLines lines, or nil when there is
// nothing to show or no room for at least the header and one line.
func (s *ProgressScreen) activityLines(maxLines int) []string {
	a := &s.activity
	if s.activity.hidden || !a.begun || maxLines < 2 {
		return nil
	}
	lines := []string{s.activityHeader()}
	if a.events == 0 {
		return append(lines, s.styles.Muted.Render(fitWidth("  "+activityPlaceholder, s.width)))
	}
	items := a.items
	if room := maxLines - 1; len(items) > room {
		items = items[len(items)-room:]
	}
	for _, it := range items {
		lines = append(lines, s.styles.Muted.Render(fitWidth(activityItemLine(it), s.width)))
	}
	return lines
}

// fitWidth cuts text to at most width columns, marking a cut with an ellipsis.
func fitWidth(text string, width int) string {
	if width < 1 || lipgloss.Width(text) <= width {
		return text
	}
	r := []rune(text)
	for len(r) > 0 && lipgloss.Width(string(r)+"…") > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// withActivityPanel inserts the panel after the step rows (parts[3]) when it is
// shown and the screen has room left below the other lines.
func (s *ProgressScreen) withActivityPanel(parts []string) []string {
	if s.activity.hidden {
		return parts
	}
	used := strings.Count(strings.Join(parts, "\n"), "\n") + 1
	panel := s.activityLines(s.height - used)
	if len(panel) == 0 {
		return parts
	}
	out := append([]string(nil), parts[:4]...)
	out = append(out, strings.Join(panel, "\n"))
	return append(out, parts[4:]...)
}
