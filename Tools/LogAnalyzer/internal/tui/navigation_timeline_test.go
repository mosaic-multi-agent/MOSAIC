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
	if view := m.View(); !strings.Contains(view, "builder#1") || !strings.Contains(view, "Orchestrator") {
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
	report := navViewsReport(timelineNavRun(runA, 300, "worker-a"), timelineNavRun(runB, 700, "worker-b"))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewTimeline)

	assertOn(t, m, screens.ScreenTimeline)
	if view := m.View(); !strings.Contains(view, "worker-b") || strings.Contains(view, "worker-a") {
		t.Errorf("timeline must show the second run's worker-b and not worker-a\n%s", view)
	}
}

func TestTimeline_ScrollOnAShortTerminal(t *testing.T) {
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
	m := onRunDetail(t, navViewsReport(run))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	m = send(asModel(updated), screens.KeyViewTimeline)

	view := m.View()
	if n := strings.Count(view, "\n") + 1; n > 8 {
		t.Errorf("view has %d lines on an 8-line terminal\n%s", n, view)
	}
	if strings.Contains(view, "step-30") {
		t.Errorf("step-30 must be scrolled out of view\n%s", view)
	}

	m = send(m, strings.Fields(strings.Repeat("down ", 150))...)
	if !strings.Contains(m.View(), "step-30") {
		t.Errorf("scrolling down must reach step-30\n%s", m.View())
	}
}
