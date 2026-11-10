package runflow

// The first line of the live activity panel: harness and model, context size
// of the latest model call, the stall indicator (running tool / quiet, with a
// highlight from five minutes), and the end state. Durations are driven by an
// injected clock and explicit status receipt times, never by waiting.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"mosaic-run/internal/domain"
)

var headerT0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

// headerClock is a controllable time source.
type headerClock struct{ now time.Time }

func (c *headerClock) Now() time.Time { return c.now }

func (c *headerClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// newHeaderScreen is a progress screen with a controllable clock at headerT0
// and invocation 1 (inv) begun at headerT0.
func newHeaderScreen(t *testing.T, inv domain.ActivityInvocation) (*ProgressScreen, *headerClock) {
	t.Helper()
	clock := &headerClock{now: headerT0}
	s := NewProgressScreen(120, 30, progressStyles())
	s.SetClock(clock.Now)
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#3", Phase: "EXECUTION", Status: "running"})
	s.BeginActivity(1, inv)
	return s, clock
}

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// headerLine returns the raw (styled) panel header line of the screen.
func headerLine(t *testing.T, s *ProgressScreen) string {
	t.Helper()
	for _, line := range strings.Split(s.View(), "\n") {
		plain := ansiSeq.ReplaceAllString(line, "")
		if strings.Contains(plain, "Activity") && strings.Contains(plain, "agent-a#3") ||
			strings.Contains(plain, "Activity (consultation)") {
			return line
		}
	}
	t.Fatalf("no activity header in view:\n%s", s.View())
	return ""
}

// headerText returns the panel header with styling removed.
func headerText(t *testing.T, s *ProgressScreen) string {
	t.Helper()
	return ansiSeq.ReplaceAllString(headerLine(t, s), "")
}

func statusWith(mutate func(*domain.ActivityStatus)) domain.ActivityStatus {
	var st domain.ActivityStatus
	mutate(&st)
	return st
}

func TestActivityHeader_Model_IsShownNextToTheHarness(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "haiku-5-5"}, headerT0)

	h := headerText(t, s)

	if !strings.Contains(h, "(claude-code, haiku-5-5)") {
		t.Errorf("header lacks the model next to the harness: %q", h)
	}
}

func TestActivityHeader_Model_IsOmittedWhenNotReported(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, headerT0)

	h := headerText(t, s)

	if !strings.Contains(h, "(claude-code)") {
		t.Errorf("header should show the harness alone when no model is known: %q", h)
	}
}

func TestActivityHeader_Model_LatestStatusWins(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "model-old"}, headerT0)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "model-new"}, headerT0)

	h := headerText(t, s)

	if !strings.Contains(h, "model-new") || strings.Contains(h, "model-old") {
		t.Errorf("header should show only the latest model: %q", h)
	}
}

func TestActivityHeader_Context_IsShownCompactFromTheLatestCall(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 50000, HasContext: true}, headerT0)
	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 112000, HasContext: true}, headerT0)

	h := headerText(t, s)

	if !strings.Contains(h, "ctx 112k") {
		t.Errorf("header lacks the latest context size as 'ctx 112k': %q", h)
	}
	if strings.Contains(h, "50k") || strings.Contains(h, "%") {
		t.Errorf("header shows a superseded size or a percentage: %q", h)
	}
}

func TestActivityHeader_Context_IsOmittedWithoutUsage(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "m", ContextTokens: 112000}, headerT0)

	h := headerText(t, s)

	if strings.Contains(h, "ctx") {
		t.Errorf("header shows a context size although the harness reported no usage: %q", h)
	}
}

func TestActivityHeader_NeverShowsTokenInOutFigures(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.AppendActivity(1, domain.ActivityItem{Kind: domain.ActivityKindUsage,
		Usage: domain.ActivityUsage{InputTokens: 1234, OutputTokens: 567, CostUSD: 0.5, HasCost: true}})
	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 9000, HasContext: true}, headerT0)

	h := headerText(t, s)

	if strings.Contains(h, "tokens") || strings.Contains(h, "1234") || strings.Contains(h, "567") {
		t.Errorf("header shows token in/out figures: %q", h)
	}
}

func TestActivityHeader_StatusAlone_KeepsThePlaceholder(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)

	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "m"}, headerT0)

	if !strings.Contains(s.View(), placeholderText) {
		t.Errorf("a status without any item ended the placeholder:\n%s", s.View())
	}
}

func TestActivityHeader_Stall_ShowsTheRunningToolWhileOneIsOpen(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{RunningTool: "Bash", RunningToolID: "t1"}, headerT0)
	clock.advance(30 * time.Second)

	h := headerText(t, s)

	if !strings.Contains(h, "running Bash") {
		t.Errorf("header lacks 'running Bash' while the tool is open: %q", h)
	}
	if strings.Contains(h, "quiet") {
		t.Errorf("header says quiet while a tool is running: %q", h)
	}
}

func TestActivityHeader_Stall_ShowsQuietWhenNoToolIsOpen(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{RunningTool: "Bash", RunningToolID: "t1"}, headerT0)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, headerT0.Add(10*time.Second))
	clock.advance(40 * time.Second)

	h := headerText(t, s)

	if !strings.Contains(h, "quiet") {
		t.Errorf("header lacks 'quiet' once the tool finished: %q", h)
	}
	if strings.Contains(h, "running Bash") {
		t.Errorf("header still shows the finished tool: %q", h)
	}
}

func TestActivityHeader_Stall_QuietBeforeAnyStatus(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	clock.advance(20 * time.Second)

	h := headerText(t, s)

	if !strings.Contains(h, "quiet") {
		t.Errorf("header lacks 'quiet' before the first status: %q", h)
	}
}

func TestActivityHeader_Stall_QuietDurationRestartsWithEachStatus(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	clock.advance(10 * time.Second)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, clock.now)
	clock.advance(15 * time.Second)
	first := headerText(t, s)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, clock.now)
	second := headerText(t, s)

	quietPart := func(h string) string { return h[strings.Index(h, "quiet"):] }

	if !strings.Contains(first, "quiet") || !strings.Contains(second, "quiet") {
		t.Fatalf("quiet missing: %q / %q", first, second)
	}
	if quietPart(first) == quietPart(second) {
		t.Errorf("quiet duration did not restart on a new status: %q vs %q", first, second)
	}
}

func TestActivityHeader_Stall_ConsultationBehavesLikeADispatch(t *testing.T) {
	s, clock := newHeaderScreen(t, consultInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{RunningTool: "Read", RunningToolID: "t9"}, headerT0)
	clock.advance(5 * time.Second)
	running := headerText(t, s)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, clock.now)
	quiet := headerText(t, s)

	if !strings.Contains(running, "Activity (consultation)") || !strings.Contains(running, "running Read") {
		t.Errorf("consultation header lacks label or running tool: %q", running)
	}
	if !strings.Contains(quiet, "Activity (consultation)") || !strings.Contains(quiet, "quiet") {
		t.Errorf("consultation header lacks label or quiet: %q", quiet)
	}
}

func TestActivityHeader_Stall_HarnessWithoutToolStartsOnlyShowsQuiet(t *testing.T) {
	inv := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "agent-a#3", Harness: "opencode"}
	s, clock := newHeaderScreen(t, inv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 8000, HasContext: true}, headerT0)
	clock.advance(12 * time.Second)

	h := headerText(t, s)

	if !strings.Contains(h, "quiet") || strings.Contains(h, "running") {
		t.Errorf("header for a harness without tool-start events should show quiet only: %q", h)
	}
}

// highlightMarks counts the style sequences of the raw header line; a
// highlighted stall indicator adds sequences to the header's normal styling.
func highlightMarks(t *testing.T, s *ProgressScreen) int {
	t.Helper()
	return len(ansiSeq.FindAllString(headerLine(t, s), -1))
}

func TestActivityHeader_Highlight_QuietStartsAtFiveMinutes(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{}, headerT0)
	clock.advance(1 * time.Minute)
	normal := highlightMarks(t, s)
	clock.advance(3*time.Minute + 59*time.Second)
	justBefore := highlightMarks(t, s)
	clock.advance(1 * time.Second)
	atLimit := highlightMarks(t, s)

	if justBefore != normal {
		t.Errorf("quiet highlighted before 5 minutes (%d vs %d style sequences)", justBefore, normal)
	}
	if atLimit == normal {
		t.Errorf("quiet not highlighted at 5 minutes (%d style sequences, same as normal)", atLimit)
	}
}

func TestActivityHeader_Highlight_RunningToolCountsFromWhenItWasFirstReported(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	tool := domain.ActivityStatus{RunningTool: "Bash", RunningToolID: "t1"}
	s.UpdateActivityStatus(1, tool, headerT0)
	clock.advance(1 * time.Minute)
	normal := highlightMarks(t, s)
	s.UpdateActivityStatus(1, tool, headerT0.Add(4*time.Minute+30*time.Second))
	clock.advance(4 * time.Minute)

	marks := highlightMarks(t, s)

	if marks == normal {
		t.Errorf("running tool not highlighted 5 minutes after it started although later events arrived (%d sequences)", marks)
	}
}

func TestActivityHeader_Highlight_NewToolRestartsTheDuration(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{RunningTool: "Bash", RunningToolID: "t1"}, headerT0)
	clock.advance(1 * time.Minute)
	normal := highlightMarks(t, s)
	s.UpdateActivityStatus(1, domain.ActivityStatus{RunningTool: "Read", RunningToolID: "t2"}, headerT0.Add(4*time.Minute))
	clock.advance(5 * time.Minute)

	marks := highlightMarks(t, s)

	if marks != normal {
		t.Errorf("a newly started tool inherited the previous tool's duration (%d vs %d sequences)", marks, normal)
	}
}

func TestActivityHeader_Highlight_QuietIsNotHighlightedWhileEventsKeepArriving(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	normal := highlightMarks(t, s)
	for i := 0; i < 8; i++ {
		clock.advance(1 * time.Minute)
		s.UpdateActivityStatus(1, domain.ActivityStatus{}, clock.now)
	}

	marks := highlightMarks(t, s)

	if marks != normal {
		t.Errorf("header highlighted although the last event was just received (%d vs %d sequences)", marks, normal)
	}
}

func endedHeader(t *testing.T, inv domain.ActivityInvocation, st domain.ActivityStatus) string {
	t.Helper()
	s, clock := newHeaderScreen(t, inv)
	s.UpdateActivityStatus(1, st, headerT0)
	clock.advance(36 * time.Second)
	s.EndActivity(1)
	clock.advance(10 * time.Minute)
	return headerText(t, s)
}

func TestActivityHeader_End_ShowsElapsedEndedContextAndCost(t *testing.T) {
	st := domain.ActivityStatus{Model: "haiku-5-5", ContextTokens: 112000, HasContext: true,
		CostUSD: 1.5, HasCost: true, RunningTool: "Bash", RunningToolID: "t1"}

	h := endedHeader(t, dispatchInv, st)

	for _, want := range []string{"36s ended", "ctx 112k", "$1.50"} {
		if !strings.Contains(h, want) {
			t.Errorf("ended header lacks %q: %q", want, h)
		}
	}
	if strings.Contains(h, "quiet") || strings.Contains(h, "running") {
		t.Errorf("ended header still shows the stall indicator: %q", h)
	}
}

func TestActivityHeader_End_WithoutReportedCost_ShowsNoCost(t *testing.T) {
	inv := domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "agent-a#3", Harness: "ghcp-cli"}
	st := domain.ActivityStatus{ContextTokens: 90000, HasContext: true, CostUSD: 3, HasCost: false}

	h := endedHeader(t, inv, st)

	if strings.Contains(h, "$") {
		t.Errorf("ended header shows a cost although none was reported: %q", h)
	}
	if !strings.Contains(h, "ctx 90k") || !strings.Contains(h, "ended") {
		t.Errorf("ended header lacks context size or ended mark: %q", h)
	}
}

func TestActivityHeader_End_WithoutUsage_OmitsContext(t *testing.T) {
	h := endedHeader(t, dispatchInv, domain.ActivityStatus{})

	if strings.Contains(h, "ctx") || strings.Contains(h, "$") {
		t.Errorf("ended header shows context or cost without any report: %q", h)
	}
}

func TestActivityHeader_End_ConsultationShowsTheSameEndState(t *testing.T) {
	st := domain.ActivityStatus{ContextTokens: 30000, HasContext: true, CostUSD: 0.25, HasCost: true}

	h := endedHeader(t, consultInv, st)

	for _, want := range []string{"Activity (consultation)", "36s ended", "ctx 30k", "$0.25"} {
		if !strings.Contains(h, want) {
			t.Errorf("ended consultation header lacks %q: %q", want, h)
		}
	}
}

func TestActivityHeader_End_LateStatusIsIgnored(t *testing.T) {
	s, clock := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 10000, HasContext: true}, headerT0)
	clock.advance(5 * time.Second)
	s.EndActivity(1)

	s.UpdateActivityStatus(1, domain.ActivityStatus{ContextTokens: 99000, HasContext: true, CostUSD: 9, HasCost: true}, clock.now)
	h := headerText(t, s)

	if !strings.Contains(h, "ctx 10k") || strings.Contains(h, "99k") || strings.Contains(h, "$") {
		t.Errorf("a status received after the end changed the header: %q", h)
	}
}

func TestActivityHeader_StatusOfASupersededInvocation_IsIgnored(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.BeginActivity(2, dispatchInv)

	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "stale-model", ContextTokens: 77000, HasContext: true}, headerT0)
	h := headerText(t, s)

	if strings.Contains(h, "stale-model") || strings.Contains(h, "77k") {
		t.Errorf("a stale invocation's status reached the header: %q", h)
	}
}

func TestActivityHeader_NewInvocation_StartsWithoutTheOldModelAndContext(t *testing.T) {
	s, _ := newHeaderScreen(t, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "old-model", ContextTokens: 77000, HasContext: true}, headerT0)

	s.BeginActivity(2, dispatchInv)
	h := headerText(t, s)

	if strings.Contains(h, "old-model") || strings.Contains(h, "ctx") {
		t.Errorf("new invocation inherits the previous header state: %q", h)
	}
}

func TestActivityHeader_NarrowScreen_CutsTheHeaderToTheWidth(t *testing.T) {
	const width = 40
	clock := &headerClock{now: headerT0}
	s := NewProgressScreen(width, 30, progressStyles())
	s.SetClock(clock.Now)
	s.BeginActivity(1, dispatchInv)
	s.UpdateActivityStatus(1, domain.ActivityStatus{Model: "haiku-5-5", ContextTokens: 112000, HasContext: true,
		RunningTool: "Bash", RunningToolID: "t1"}, headerT0)
	clock.advance(6 * time.Minute)

	line := headerLine(t, s)

	if w := lipgloss.Width(line); w > width {
		t.Errorf("header is %d columns wide, want <= %d: %q", w, width, line)
	}
}
