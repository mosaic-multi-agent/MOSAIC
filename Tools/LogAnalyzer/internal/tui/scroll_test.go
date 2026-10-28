package tui

// scroll_test.go verifies the scrolling behaviour of RunsScreen and
// RunDetailScreen (the Dispatches view) as table views.
//
// RunsScreen: with more runs than fit in the given height, View shows a bounded
// window; the selected run is always inside it; scrolling to the last run makes
// it appear and scrolling back makes the first appear again; a reduced height
// keeps the selected run visible; a clipped list carries a position hint; nothing
// is ever drawn past the height.
//
// RunDetailScreen: with more dispatch rows than fit, View shows a bounded window;
// scrolling down reveals later rows and stops at the last; scrolling up returns
// to the first; very short heights do not panic and draw nothing past the height.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const manyRuns = 40

// buildScrollRunsReport builds a domain.Report with n distinct named runs of equal
// cost, so the cost order falls back to the name order: run i is listed i-th. Each
// run id ends with a unique tag, runSuffix(i).
func buildScrollRunsReport(n int) domain.Report {
	runs := make([]domain.RunReport, n)
	for i := range runs {
		runs[i] = domain.RunReport{
			Run: domain.NamedRun(fmt.Sprintf("20260101T%06dZ-r%03d", i, i)),
			Totals: domain.Totals{
				Tokens: domain.TokenUsage{Input: domain.Tokens(100)},
				Money: domain.CategoryMoney{
					Total:    domain.KnownMoney(domain.Money(1_000_000_000)),
					Complete: true,
				},
			},
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		}
	}
	return domain.Report{
		Runs:     runs,
		AllRuns:  domain.Totals{},
		Currency: domain.Currency,
		Quality:  domain.NewQualitySummary(nil),
	}
}

// runSuffix returns the unique tag embedded in run[i]'s ID, e.g. "r000".
func runSuffix(i int) string { return fmt.Sprintf("r%03d", i) }

// scrollMoney is a complete $1.00 total.
func scrollMoney() domain.Totals {
	return domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(100)},
		Money:  domain.CategoryMoney{Total: domain.KnownMoney(domain.Money(1_000_000_000)), Complete: true},
	}
}

// buildScrollDetailRun builds a run with one orchestrator and agentCount agents
// labelled "Agent#1" ... "Agent#N". Each agent is both an actor report and a
// dispatch of one agent type, as the analysis delivers them. The first agent costs
// the most, so the default cost order lists them as numbered.
func buildScrollDetailRun(agentCount int) domain.RunReport {
	run := domain.RunReport{
		Run: domain.NamedRun("20260101T120000Z-scrl"),
		Totals: domain.Totals{
			Tokens: domain.TokenUsage{Input: domain.Tokens(int64(agentCount+1) * 100)},
			Money: domain.CategoryMoney{
				Total:    domain.KnownMoney(domain.Money(int64(agentCount+1) * 1_000_000_000)),
				Complete: true,
			},
		},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: scrollMoney()},
	}
	group := domain.AgentTypeGroup{AgentType: "worker", Figures: domain.GroupFigures{Invocations: agentCount}}
	for i := 0; i < agentCount; i++ {
		actor := domain.ActorRef{
			Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(fmt.Sprintf("Agent#%d", i+1)), Type: "worker",
		}
		totals := scrollMoney()
		totals.Money.Total = domain.KnownMoney(domain.Money(int64(agentCount-i) * 10_000_000))
		run.Agents = append(run.Agents, domain.ActorReport{Actor: actor, Totals: totals})
		group.Dispatches = append(group.Dispatches, domain.Dispatch{
			Invocation: domain.Invocation{Actor: actor, State: domain.InvocationClosed, StatusCode: "SUCCESS", EndEvents: 1},
			Totals:     totals,
		})
	}
	if agentCount > 0 {
		run.ByAgentType = []domain.AgentTypeGroup{group}
	}
	return run
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func pressRuns(s *screens.RunsScreen, key string, n int) {
	for i := 0; i < n; i++ {
		s.Update(keyRunes(key))
	}
}

func pressDetail(s *screens.RunDetailScreen, key string, n int) {
	for i := 0; i < n; i++ {
		s.Update(keyRunes(key))
	}
}

func assertNoMoreLinesThan(t *testing.T, view string, height int) {
	t.Helper()
	if n := len(frameLines(view)); n > height {
		t.Errorf("view drew %d lines on a %d-line terminal:\n%s", n, height, view)
	}
}

// ---------------------------------------------------------------------------
// RunsScreen scrolling
// ---------------------------------------------------------------------------

func TestRunsScreen_SmallHeight_ClipsRunList(t *testing.T) {
	const height = 13
	s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, height, plainStyles())

	view := s.View(80, height)

	if !strings.Contains(view, runSuffix(0)) {
		t.Errorf("run[0] should be visible at cursor=0; not found in view:\n%s", view)
	}
	if strings.Contains(view, runSuffix(manyRuns-1)) {
		t.Errorf("the last run should be clipped at height=%d cursor=0:\n%s", height, view)
	}
	assertNoMoreLinesThan(t, view, height)
}

func TestRunsScreen_NavigateToLastRun_LastRunVisible(t *testing.T) {
	const height = 13
	s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, height, plainStyles())

	pressRuns(s, "j", manyRuns-1)
	view := s.View(80, height)

	if !strings.Contains(view, runSuffix(manyRuns-1)) {
		t.Errorf("the last run should be visible after navigating to it:\n%s", view)
	}
	if strings.Contains(view, runSuffix(0)) {
		t.Errorf("the first run should have scrolled off:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, height)
}

func TestRunsScreen_NavigateBackToFirst_FirstRunVisible(t *testing.T) {
	const height = 13
	s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, height, plainStyles())

	pressRuns(s, "j", manyRuns-1)
	pressRuns(s, "k", manyRuns-1)
	view := s.View(80, height)

	if !strings.Contains(view, runSuffix(0)) {
		t.Errorf("the first run should be visible after scrolling back to the top:\n%s", view)
	}
	if strings.Contains(view, runSuffix(manyRuns-1)) {
		t.Errorf("the last run should have scrolled off again:\n%s", view)
	}
}

func TestRunsScreen_ReducedHeight_SelectedRunStillVisible(t *testing.T) {
	const largeHeight, smallHeight = 60, 13
	s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, largeHeight, plainStyles())

	pressRuns(s, "j", 5)
	view := s.View(80, smallHeight)

	if !strings.Contains(view, runSuffix(5)) {
		t.Errorf("run[5] should be visible at the reduced height:\n%s", view)
	}
	if strings.Contains(view, runSuffix(manyRuns-1)) {
		t.Errorf("the last run should be clipped at the reduced height:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, smallHeight)
}

func TestRunsScreen_SelectedRunIsInTheWindowAtEveryHeightThatShowsAnyRun(t *testing.T) {
	const selected = 7
	for height := 3; height <= 30; height++ {
		s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, 60, plainStyles())
		pressRuns(s, "j", selected)

		view := s.View(80, height)

		shown := 0
		for i := 0; i < manyRuns; i++ {
			if strings.Contains(view, runSuffix(i)) {
				shown++
			}
		}
		if shown > 0 && !strings.Contains(view, runSuffix(selected)) {
			t.Errorf("height %d: rows are shown but not the selected run:\n%s", height, view)
		}
		assertNoMoreLinesThan(t, view, height)
	}
}

func TestRunsScreen_ScrollHintVisible_WhenListIsClipped(t *testing.T) {
	const height = 13
	s := screens.NewRunsScreen(buildScrollRunsReport(manyRuns), 80, height, plainStyles())

	view := s.View(80, height)

	// The hint format "X-Y of N" tells a scroll indicator from the run count badge.
	if !strings.Contains(view, fmt.Sprintf("of %d", manyRuns)) {
		t.Errorf("a clipped run list should carry a scroll hint containing \"of %d\":\n%s", manyRuns, view)
	}
}

func TestRunsScreen_ZeroRuns_DoesNotPanic(t *testing.T) {
	s := screens.NewRunsScreen(buildScrollRunsReport(0), 80, 5, plainStyles())

	view := s.View(80, 5)

	if view == "" {
		t.Error("RunsScreen.View with zero runs returned an empty string")
	}
	assertNoMoreLinesThan(t, view, 5)
}

func TestRunsScreen_ShortHeight_DoesNotPanicAndDrawsNothingPastTheHeight(t *testing.T) {
	for _, h := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("height=%d", h), func(t *testing.T) {
			s := screens.NewRunsScreen(buildScrollRunsReport(3), 80, h, plainStyles())

			view := s.View(80, h)

			assertNoMoreLinesThan(t, view, h)
			if h >= 1 && !strings.Contains(view, "Log Analyzer") {
				t.Errorf("the title header leads whenever a line fits:\n%s", view)
			}
			if h >= screens.PageChromeLines && !strings.Contains(view, "? help") {
				t.Errorf("the help line must be present once the chrome fits:\n%s", view)
			}
		})
	}
}

func TestRunsScreen_WithUnattributable_KeepsItsChromeAndTheSelectedRun(t *testing.T) {
	report := buildScrollRunsReport(5)
	report.Unattributable = &domain.RunReport{Run: domain.UnattributableRun(), Totals: scrollMoney()}

	for _, height := range []int{9, 12, 15, 30} {
		s := screens.NewRunsScreen(report, 80, height, plainStyles())

		view := s.View(80, height)

		assertNoMoreLinesThan(t, view, height)
		for _, want := range []string{"Log Analyzer", "? help"} {
			if !strings.Contains(view, want) {
				t.Errorf("height %d: view lacks %q:\n%s", height, want, view)
			}
		}
	}
	view := screens.NewRunsScreen(report, 80, 40, plainStyles()).View(80, 40)
	if !strings.Contains(view, screens.UnattributableLabel) || !strings.Contains(view, runSuffix(0)) {
		t.Errorf("a tall view shows the runs and the unattributable section:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// RunDetailScreen (Dispatches) scrolling
// ---------------------------------------------------------------------------

func TestRunDetailScreen_SmallHeight_ClipsDispatchRows(t *testing.T) {
	const height = 13
	s := screens.NewRunDetailScreen(buildScrollDetailRun(40), 80, height, plainStyles())

	view := s.View(80, height)

	if strings.Contains(view, "Agent#40") {
		t.Errorf("Agent#40 should be clipped at height=%d scroll=0:\n%s", height, view)
	}
	if !strings.Contains(view, "Agent#1") {
		t.Errorf("Agent#1 should be visible at scroll=0:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, height)
}

func TestRunDetailScreen_ScrollDown_RevealsLaterRows(t *testing.T) {
	const height = 13
	s := screens.NewRunDetailScreen(buildScrollDetailRun(40), 80, height, plainStyles())

	pressDetail(s, "j", 100)
	view := s.View(80, height)

	if !strings.Contains(view, "Agent#40") {
		t.Errorf("Agent#40 should be visible after scrolling to the end:\n%s", view)
	}
	if strings.Contains(view, "orchestrator") || strings.Contains(view, "Agent#1 ") {
		t.Errorf("the first rows should have scrolled off:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, height)
}

func TestRunDetailScreen_ScrollUp_ReturnsToTheFirstRows(t *testing.T) {
	const height = 13
	s := screens.NewRunDetailScreen(buildScrollDetailRun(40), 80, height, plainStyles())

	pressDetail(s, "j", 30)
	pressDetail(s, "k", 60)
	view := s.View(80, height)

	if !strings.Contains(view, "orchestrator") || strings.Contains(view, "Agent#40") {
		t.Errorf("scrolling back to the top should show the orchestrator and not the last agent:\n%s", view)
	}
}

func TestRunDetailScreen_ShortHeight_DoesNotPanicAndDrawsNothingPastTheHeight(t *testing.T) {
	run := buildScrollDetailRun(3)
	for _, h := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("height=%d", h), func(t *testing.T) {
			s := screens.NewRunDetailScreen(run, 80, h, plainStyles())

			view := s.View(80, h)

			assertNoMoreLinesThan(t, view, h)
			if h > 0 && !strings.Contains(view, run.Run.Label()) {
				t.Errorf("the header leads whenever a line fits:\n%s", view)
			}
		})
	}
}

func TestRunDetailScreen_ScrollHintVisible_WhenRowsAreClipped(t *testing.T) {
	const height = 13
	s := screens.NewRunDetailScreen(buildScrollDetailRun(40), 80, height, plainStyles())

	view := s.View(80, height)

	if !strings.Contains(view, " of 4") { // "X-Y of 40" or "X-Y of 41" with the orchestrator row
		t.Errorf("a clipped dispatch table should carry a scroll hint:\n%s", view)
	}
}

func TestRunDetailScreen_ZeroAgents_RendersTheOrchestratorOnly(t *testing.T) {
	s := screens.NewRunDetailScreen(buildScrollDetailRun(0), 80, 10, plainStyles())

	view := s.View(80, 10)

	if !strings.Contains(view, "orchestrator") {
		t.Errorf("the orchestrator row must be shown for a run without agents:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, 10)
}

func TestRunDetailScreen_ReducedHeight_RowsAroundTheScrollPositionStayVisible(t *testing.T) {
	const largeHeight, smallHeight = 40, 9
	s := screens.NewRunDetailScreen(buildScrollDetailRun(40), 80, largeHeight, plainStyles())

	pressDetail(s, "j", 5)
	view := s.View(80, smallHeight)

	if !strings.Contains(view, "Agent#5 ") && !strings.Contains(view, "Agent#6 ") && !strings.Contains(view, "Agent#4 ") {
		t.Errorf("the rows around the scroll position must stay visible after the resize:\n%s", view)
	}
	if strings.Contains(view, "Agent#40") {
		t.Errorf("the last row cannot be in the window:\n%s", view)
	}
	assertNoMoreLinesThan(t, view, smallHeight)
}

func TestRunDetailScreen_EveryRowIsOneLineAtEveryHeight(t *testing.T) {
	run := buildScrollDetailRun(12)
	for height := 6; height <= 24; height++ {
		s := screens.NewRunDetailScreen(run, 80, height, plainStyles())

		view := s.View(80, height)

		assertNoMoreLinesThan(t, view, height)
		for _, want := range []string{run.Run.Label(), "? help"} {
			if !strings.Contains(view, want) {
				t.Errorf("height %d: view lacks %q:\n%s", height, want, view)
			}
		}
	}
}

func TestRunDetailScreen_WithUnpricedModels_KeepsTheNoticeAndTheChrome(t *testing.T) {
	const height = 13
	run := buildScrollDetailRun(3)
	run.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}

	s := screens.NewRunDetailScreen(run, 80, height, plainStyles())
	view := s.View(80, height)

	for _, want := range []string{run.Run.Label(), "? help", "model-x", "orchestrator"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	assertNoMoreLinesThan(t, view, height)
}
