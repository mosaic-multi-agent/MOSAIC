package runflow

// The live activity panel of the progress screen: shown at first appearance,
// toggled with 'l', a placeholder until the first item of the running
// invocation, short one-line items, and the rule that a hidden panel leaves
// today's screen untouched.
//
// Drawn at 120 columns so no help bar or item line wraps.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"mosaic-run/internal/domain"
)

const placeholderText = "no live output available"

var dispatchInv = domain.ActivityInvocation{
	Source: domain.ActivitySourceDispatch, Agent: "agent-a#3", Harness: "claude-code",
}

var consultInv = domain.ActivityInvocation{
	Source: domain.ActivitySourceConsultation, Agent: "orchestrator-x", Harness: "claude-code",
}

func toolItem(name, summary string) domain.ActivityItem {
	return domain.ActivityItem{Kind: domain.ActivityToolCall, Name: name, Summary: summary}
}

func textItem(text string) domain.ActivityItem {
	return domain.ActivityItem{Kind: domain.ActivityText, Summary: text}
}

func subagentItem(name, summary string) domain.ActivityItem {
	return domain.ActivityItem{Kind: domain.ActivitySubagent, Name: name, Summary: summary}
}

// newActivityScreen is a progress screen with one running step and invocation
// 1 begun.
func newActivityScreen(t *testing.T) *ProgressScreen {
	t.Helper()
	s := NewProgressScreen(120, 30, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#3", Phase: "EXECUTION", Status: "running"})
	s.BeginActivity(1, dispatchInv)
	return s
}

// withoutTitle drops the first line, which carries the elapsed-time clock.
func withoutTitle(view string) string {
	if i := strings.Index(view, "\n"); i >= 0 {
		return view[i+1:]
	}
	return view
}

func TestActivityPanel_IsVisibleWhenTheScreenIsFirstConstructed(t *testing.T) {
	s := NewProgressScreen(120, 30, progressStyles())

	if !s.ActivityVisible() {
		t.Error("ActivityVisible() = false on a new screen, want true")
	}
}

func TestActivityPanel_ToggleKey_HidesAndShowsThePanel(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, toolItem("Read", "a.txt"))

	pressProgressKey(s, "l")
	hidden := !s.ActivityVisible() && !strings.Contains(s.View(), "a.txt")
	pressProgressKey(s, "L")
	shown := s.ActivityVisible() && strings.Contains(s.View(), "a.txt")

	if !hidden {
		t.Error("'l' did not hide the panel (flag or its content still shown)")
	}
	if !shown {
		t.Error("'L' did not show the panel again with its content")
	}
}

func TestActivityPanel_ToggleKey_IsNotARequestOrAnyOtherBinding(t *testing.T) {
	s := newActivityScreen(t)

	pressProgressKey(s, "l")

	if s.ActivityVisible() {
		t.Error("'l' did not toggle the panel")
	}
	if ev := s.TakeRequestKeyEvents(); len(ev) != 0 {
		t.Errorf("'l' recorded request key events %+v, want none", ev)
	}
	if ev := s.TakeStopGateEvents(); len(ev) != 0 || s.ConfirmPending() || s.GracefulStop() {
		t.Errorf("'l' touched the stop gate: events=%+v pending=%v stop=%v", ev, s.ConfirmPending(), s.GracefulStop())
	}
	if s.ArtifactViewRequested() {
		t.Error("'l' requested the artifact view")
	}
}

func TestActivityPanel_ToggleKey_WorksAfterAGracefulStopIsArmed(t *testing.T) {
	s := newActivityScreen(t)
	pressProgressKey(s, "s")
	pressProgressKey(s, "y")
	if !s.ActivityVisible() {
		t.Fatal("panel not visible before the toggle key was pressed")
	}

	pressProgressKey(s, "l")

	if s.ActivityVisible() {
		t.Error("'l' did not hide the panel after a graceful stop was armed")
	}
}

func TestActivityPanel_ToggleKey_IsIgnoredWhileTheStopGateIsOpen(t *testing.T) {
	s := newActivityScreen(t)
	pressProgressKey(s, "s")

	pressProgressKey(s, "l")

	if !s.ActivityVisible() {
		t.Error("'l' toggled the panel while the stop gate was open")
	}
	if !s.ConfirmPending() {
		t.Error("'l' closed the stop gate")
	}
}

func TestActivityPanel_Visibility_SurvivesResetResizeAndNewInvocations(t *testing.T) {
	s := newActivityScreen(t)
	if !s.ActivityVisible() {
		t.Fatal("panel not visible before the toggle key was pressed")
	}
	pressProgressKey(s, "l")
	if s.ActivityVisible() {
		t.Fatal("'l' did not hide the panel")
	}

	s.ResetStopState()
	s.Resize(100, 20)
	s.BeginActivity(2, dispatchInv)
	s.AppendActivity(2, toolItem("Read", "persisted.txt"))

	if s.ActivityVisible() || strings.Contains(s.View(), "persisted.txt") {
		t.Error("hidden panel became visible again; visibility must persist for the screen's lifetime")
	}
	pressProgressKey(s, "l")
	if !s.ActivityVisible() || !strings.Contains(s.View(), "persisted.txt") {
		t.Error("toggling back did not show the panel with the current invocation's item")
	}
}

func TestActivityPanel_HelpBar_ListsTheToggleKey(t *testing.T) {
	s := NewProgressScreen(200, 30, progressStyles())

	if view := s.View(); !strings.Contains(view, "l activity") {
		t.Errorf("View() help bar lacks %q:\n%s", "l activity", view)
	}
}

func TestActivityPanel_Hidden_ViewEqualsTheScreenWithoutAnyActivity(t *testing.T) {
	plain := NewProgressScreen(120, 30, progressStyles())
	plain.AppendRow(ProgressRow{AgentInstance: "agent-a#3", Phase: "EXECUTION", Status: "running"})
	active := newActivityScreen(t)
	active.AppendActivity(1, toolItem("Read", "a.txt"))
	if withoutTitle(active.View()) == withoutTitle(plain.View()) {
		t.Fatal("visible panel did not change the screen; nothing to hide")
	}
	pressProgressKey(active, "l")
	pressProgressKey(plain, "l")

	got, want := withoutTitle(active.View()), withoutTitle(plain.View())

	if got != want {
		t.Errorf("hidden panel changed the screen.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestActivityPanel_Hidden_ShowsNeitherItemsNorPlaceholder(t *testing.T) {
	s := newActivityScreen(t)
	if !strings.Contains(s.View(), placeholderText) {
		t.Fatal("placeholder not shown before the panel was hidden")
	}
	pressProgressKey(s, "l")
	s.AppendActivity(1, textItem("hidden thought"))

	view := s.View()

	if strings.Contains(view, "hidden thought") || strings.Contains(view, placeholderText) {
		t.Errorf("hidden panel leaked into the view:\n%s", view)
	}
}

func TestActivityPanel_ItemsAddedWhileHidden_AreShownWhenTheNextToggleShowsThePanel(t *testing.T) {
	s := newActivityScreen(t)
	pressProgressKey(s, "l")
	s.AppendActivity(1, toolItem("Grep", "needle"))

	pressProgressKey(s, "l")
	view := s.View()

	if !strings.Contains(view, "needle") {
		t.Errorf("item added while hidden is missing after re-showing:\n%s", view)
	}
	if strings.Contains(view, placeholderText) {
		t.Errorf("placeholder shown although an item arrived:\n%s", view)
	}
}

func TestActivityPanel_BeforeTheFirstItem_ShowsThePlaceholder(t *testing.T) {
	s := newActivityScreen(t)

	if view := s.View(); !strings.Contains(view, placeholderText) {
		t.Errorf("View() lacks %q before the first item:\n%s", placeholderText, view)
	}
}

func TestActivityPanel_FirstItem_ReplacesThePlaceholder(t *testing.T) {
	s := newActivityScreen(t)

	s.AppendActivity(1, textItem("thinking aloud"))
	view := s.View()

	if strings.Contains(view, placeholderText) {
		t.Errorf("placeholder still shown after the first item:\n%s", view)
	}
	if !strings.Contains(view, "thinking aloud") {
		t.Errorf("first item not shown:\n%s", view)
	}
}

func TestActivityPanel_Items_RenderNameAndShortSummary(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, toolItem("Read", "src/main.go"))
	s.AppendActivity(1, textItem("Looking at the entry point"))
	s.AppendActivity(1, subagentItem("reviewer", "Review the change"))

	view := s.View()

	for _, want := range []string{"Read", "src/main.go", "Looking at the entry point", "reviewer", "Review the change"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() lacks %q:\n%s", want, view)
		}
	}
}

func TestActivityPanel_Items_AreShownOldestFirst(t *testing.T) {
	s := newActivityScreen(t)
	for _, n := range []string{"alpha", "bravo", "charlie"} {
		s.AppendActivity(1, textItem("line-"+n))
	}

	view := s.View()

	a, b, c := strings.Index(view, "line-alpha"), strings.Index(view, "line-bravo"), strings.Index(view, "line-charlie")
	if a < 0 || b < a || c < b {
		t.Errorf("items not in output order (positions %d, %d, %d):\n%s", a, b, c, view)
	}
}

func TestActivityPanel_LongSummary_IsCutToTheScreenWidth(t *testing.T) {
	const width = 60
	s := NewProgressScreen(width, 30, progressStyles())
	s.BeginActivity(1, dispatchInv)
	s.AppendActivity(1, toolItem("Bash", strings.Repeat("Q", 190)))

	shown := false
	for _, line := range strings.Split(s.View(), "\n") {
		if !strings.Contains(line, "QQQQ") {
			continue
		}
		shown = true
		if lipgloss.Width(line) > width {
			t.Errorf("item line is %d columns wide, want <= %d: %q", lipgloss.Width(line), width, line)
		}
	}
	if !shown {
		t.Errorf("the long item is not shown at all:\n%s", s.View())
	}
}

func TestActivityPanel_Header_NamesTheDispatchedAgent_NotAConsultation(t *testing.T) {
	s := newActivityScreen(t)
	s.AppendActivity(1, textItem("hello"))

	view := s.View()

	if strings.Count(view, "agent-a#3") < 2 {
		t.Errorf("panel header does not name the agent next to the step row:\n%s", view)
	}
	if strings.Contains(view, "consultation") {
		t.Errorf("a dispatched agent's panel is labelled as a consultation:\n%s", view)
	}
}

func TestActivityPanel_ConsultationActivity_IsShownDuringAConsultation(t *testing.T) {
	s := NewProgressScreen(120, 30, progressStyles())
	s.StartConsultation(domain.ConsultationActivity{Context: domain.ConsultContextRouting})
	s.BeginActivity(7, consultInv)

	s.AppendActivity(7, toolItem("Read", "Orchestration.md"))
	view := s.View()

	if !strings.Contains(view, "Orchestration.md") {
		t.Errorf("consultation activity not shown:\n%s", view)
	}
	if !strings.Contains(view, "orchestrator-x") {
		t.Errorf("panel header lacks the consultation's agent:\n%s", view)
	}
	if strings.Count(view, "consultation") < 2 {
		t.Errorf("panel header does not say it is a consultation (consultation row plus header expected):\n%s", view)
	}
}

func TestActivityPanel_Elapsed_CountsFromBeginActivity(t *testing.T) {
	s := NewProgressScreen(120, 30, progressStyles())
	time.Sleep(1100 * time.Millisecond)
	s.BeginActivity(1, dispatchInv)
	time.Sleep(1100 * time.Millisecond)

	oneSecond := regexp.MustCompile(`(^|[^0-9.])1s`)
	found := false
	for _, line := range strings.Split(s.View(), "\n") {
		if strings.Contains(line, "agent-a#3") && oneSecond.MatchString(line) {
			found = true
		}
	}

	if !found {
		t.Errorf("no panel header line shows 1s elapsed since the invocation began:\n%s", s.View())
	}
}
