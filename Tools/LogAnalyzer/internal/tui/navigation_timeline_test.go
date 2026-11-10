package tui

// Navigation to and from the timeline per-run view, reusing the helpers of the
// other per-run view navigation tests.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var timelineBase = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// timelineNavRun is navViewsRun plus one closed invocation named after the run,
// so the timeline shows which run it was built from.
func timelineNavRun(id string, cents int64, instance string) domain.RunReport {
	run := navViewsRun(id, cents)
	inv := domain.Invocation{
		Actor:      domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(instance), Type: "builder"},
		State:      domain.InvocationClosed,
		StatusCode: "SUCCESS",
		StartedAt:  timelineBase,
		EndedAt:    timelineBase.Add(30 * time.Second),
		EndEvents:  1,
	}
	run.Timeline = domain.Timeline{
		Entries: []domain.TimelineEntry{{Invocation: inv, Duration: 30 * time.Second, HasDuration: true}},
		Axis:    domain.TimeInterval{Start: timelineBase, End: timelineBase.Add(30 * time.Second)},
		HasAxis: true,
	}
	return run
}

var otherViewKeys = map[string]screens.ScreenID{
	screens.KeyViewDispatches: screens.ScreenRunDetail,
	screens.KeyViewOverview:   screens.ScreenOverview,
	screens.KeyViewAgentTypes: screens.ScreenAgentTypes,
	screens.KeyViewStatus:     screens.ScreenStatus,
	screens.KeyViewTools:      screens.ScreenTools,
}

func TestTimeline_RunDetailReachesTheTimelineWithContent(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))

	m := send(onRunDetail(t, report), screens.KeyViewTimeline)

	assertOn(t, m, screens.ScreenTimeline)
	if view := m.View(); !strings.Contains(view, "worker-a") {
		t.Errorf("timeline view must list the run's invocation worker-a\n%s", view)
	}
}

func TestTimeline_ReachableFromEveryOtherPerRunView(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))
	for start := range otherViewKeys {
		m := send(onRunDetail(t, report), start)

		m = send(m, screens.KeyViewTimeline)

		assertOn(t, m, screens.ScreenTimeline)
	}
}

func TestTimeline_ExistingViewsStayReachableFromTheTimeline(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))
	for key, id := range otherViewKeys {
		m := send(onRunDetail(t, report), screens.KeyViewTimeline)
		assertOn(t, m, screens.ScreenTimeline)

		m = send(m, key)

		assertOn(t, m, id)
	}
}

func TestTimeline_PerDispatchListStillListsOrchestratorAndDispatches(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))

	m := send(onRunDetail(t, report), screens.KeyViewTimeline)
	assertOn(t, m, screens.ScreenTimeline)

	m = send(m, screens.KeyViewDispatches)

	assertOn(t, m, screens.ScreenRunDetail)
	if view := m.View(); !strings.Contains(view, "builder#1") || !strings.Contains(strings.ToLower(view), "orchestrator") {
		t.Errorf("per-dispatch list must list the orchestrator and builder#1\n%s", view)
	}
}

func TestTimeline_BackReturnsToRunsList(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))
	for _, back := range []string{"esc", "q"} {
		m := send(onRunDetail(t, report), screens.KeyViewTimeline)
		assertOn(t, m, screens.ScreenTimeline)

		m = send(m, back)

		assertOn(t, m, screens.ScreenRuns)
	}
}

func TestTimeline_ShowsTheSelectedRunNotTheFirst(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 700, "worker-a"), timelineNavRun(runB, 300, "worker-b"))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewTimeline)

	assertOn(t, m, screens.ScreenTimeline)
	if view := m.View(); !strings.Contains(view, "worker-b") || strings.Contains(view, "worker-a") {
		t.Errorf("timeline must show the second run's worker-b and not worker-a\n%s", view)
	}
}

func manyStepsRun() domain.RunReport {
	run := timelineNavRun(runA, 1000, "worker-a")
	for i := 1; i <= 30; i++ {
		start := timelineBase.Add(time.Duration(i) * time.Minute)
		run.Timeline.Entries = append(run.Timeline.Entries, domain.TimelineEntry{
			Invocation: domain.Invocation{
				Actor:     domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(fmt.Sprintf("step-%02d", i))},
				State:     domain.InvocationClosed,
				StartedAt: start,
				EndedAt:   start.Add(time.Second),
			},
			Duration: time.Second, HasDuration: true,
		})
	}
	run.Timeline.Axis.End = timelineBase.Add(31 * time.Minute)
	return run
}

func TestTimeline_ScrollOnAShortTerminal(t *testing.T) {
	m := onRunDetail(t, navViewsReport(manyStepsRun()))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	m = send(asModel(updated), screens.KeyViewTimeline)

	view := m.View()
	if n := strings.Count(view, "\n") + 1; n > 12 {
		t.Errorf("view has %d lines on a 12-line terminal\n%s", n, view)
	}
	if strings.Contains(view, "step-30") {
		t.Errorf("step-30 must be scrolled out of view\n%s", view)
	}

	m = send(m, strings.Fields(strings.Repeat("down ", 150))...)
	if !strings.Contains(m.View(), "step-30") {
		t.Errorf("moving the cursor down must reach step-30\n%s", m.View())
	}
}

func TestTimeline_NeverDrawsPastAnEightLineTerminal(t *testing.T) {
	m := onRunDetail(t, navViewsReport(manyStepsRun()))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	m = send(asModel(updated), screens.KeyViewTimeline)

	for i := 0; i < 40; i++ {
		if n := strings.Count(m.View(), "\n") + 1; n > 8 {
			t.Fatalf("view has %d lines on an 8-line terminal\n%s", n, m.View())
		}
		m = send(m, "down", "down")
	}
}

func TestTimeline_ListAndGroupKeysStayOnTheTimeline(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))
	m := send(onRunDetail(t, report), screens.KeyViewTimeline)
	gantt := m.View()

	m = send(m, screens.KeyToggleView)
	assertOn(t, m, screens.ScreenTimeline)
	list := m.View()
	if list == gantt || !strings.Contains(list, "worker-a") {
		t.Errorf("v must show the list version of the timeline, still listing worker-a\n%s", list)
	}

	m = send(m, screens.KeyToggleView, screens.KeyGroup)
	assertOn(t, m, screens.ScreenTimeline)
	if !strings.Contains(m.View(), "worker-a") {
		t.Errorf("g must keep the timeline lanes\n%s", m.View())
	}
}

func TestTimeline_BackFromTheListVersionReturnsToRunsList(t *testing.T) {
	report := navViewsReport(timelineNavRun(runA, 1000, "worker-a"))
	for _, back := range []string{"esc", "q"} {
		m := send(onRunDetail(t, report), screens.KeyViewTimeline, screens.KeyToggleView)

		m = send(m, back)

		assertOn(t, m, screens.ScreenRuns)
	}
}
