package runflow

// progress_scroll_fit_test.go verifies that the progress screen never renders
// more lines than the terminal is high, and keeps the title, status, help bar,
// pending lines, banners, guard line, repair and consultation lines and the
// activity panel, in every combination of panel state, optional parts, scroll
// position and width (including narrow widths where the help bar wraps), and
// that every step row is one line no wider than the screen.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-run/internal/domain"
)

const fitRowCount = 80

// flatten removes styling and collapses whitespace, so text that wrapped over
// several lines can be searched as one phrase.
func flatten(view string) string {
	return strings.Join(strings.Fields(ansiRe.ReplaceAllString(view, "")), " ")
}

// squash removes styling and every whitespace character, so a phrase that was
// wrapped over several lines, even in the middle of a hyphenated word, can be
// searched for.
func squash(s string) string {
	return strings.Join(strings.Fields(ansiRe.ReplaceAllString(s, "")), "")
}

type fitPart struct {
	name string
	set  func(s *ProgressScreen)
	want []string // phrases that must stay in the view (flattened)
}

var fitParts = []fitPart{
	{"none", func(s *ProgressScreen) {}, nil},
	{"pending requests", func(s *ProgressScreen) {
		s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})
	}, []string{"Requested: manual dispatch", "Requested: HITL for the next agent"}},
	{"stop confirm banner", func(s *ProgressScreen) { pressProgressKey(s, "s") }, []string{confirmPromptLabel}},
	{"stop notice", func(s *ProgressScreen) {
		pressProgressKey(s, "s")
		pressProgressKey(s, "y")
	}, []string{"Stopping after current step completes"}},
	{"force-stop banner", func(s *ProgressScreen) { pressProgressKey(s, "x") }, []string{forceStopPromptLabel}},
	{"guard indicator", func(s *ProgressScreen) { s.SetGuardWaived(true) }, []string{guardOffIndicator}},
	{"consultation and repair outcomes", func(s *ProgressScreen) {
		s.StartConsultation(routingNoKind)
		s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Problems: 2})
		s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 2, Accepted: true})
	}, []string{"consultation: routing", "artifact repair, attempt 1 of 2", "artifact repair, attempt 2 of 2"}},
	{"everything", func(s *ProgressScreen) {
		s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})
		s.SetGuardWaived(true)
		s.StartConsultation(routingNoKind)
		s.RecordRepairOutcome(domain.RepairOutcome{Attempt: 1, Accepted: true})
		pressProgressKey(s, "s")
		pressProgressKey(s, "y")
		pressProgressKey(s, "x")
	}, []string{"Requested: manual dispatch", guardOffIndicator, "consultation: routing", "Stopping after current step completes", forceStopPromptLabel}},
}

type fitPanel struct {
	name  string
	setup func(s *ProgressScreen)
}

var fitPanels = []fitPanel{
	{"panel not begun", func(s *ProgressScreen) {}},
	{"panel shown", func(s *ProgressScreen) {
		s.BeginActivity(1, dispatchInv)
		for i := 0; i < 30; i++ {
			s.AppendActivity(1, toolItem("Read", fmt.Sprintf("file%02d.txt", i)))
		}
	}},
	{"panel hidden", func(s *ProgressScreen) {
		s.BeginActivity(1, dispatchInv)
		pressProgressKey(s, "l")
	}},
}

var fitPositions = []struct {
	name string
	keys []tea.KeyType
}{
	{"following", nil},
	{"top", []tea.KeyType{tea.KeyHome}},
	{"middle", []tea.KeyType{tea.KeyHome, tea.KeyPgDown}},
	{"one up", []tea.KeyType{tea.KeyUp}},
}

func newFitScreen(width, height int) *ProgressScreen {
	s := NewProgressScreen(width, height, progressStyles())
	addScrollRows(s, 0, fitRowCount-1)
	s.AppendRow(ProgressRow{AgentInstance: scrollRowName(fitRowCount - 1), Phase: "EXECUTION", Status: "running"})
	return s
}

func TestFit_EveryStateStaysWithinTheHeightAndKeepsItsParts(t *testing.T) {
	for _, width := range []int{200, 80, 50} {
		for _, panel := range fitPanels {
			for _, part := range fitParts {
				for _, pos := range fitPositions {
					name := fmt.Sprintf("w%d/%s/%s/%s", width, panel.name, part.name, pos.name)
					t.Run(name, func(t *testing.T) {
						const height = 40
						s := newFitScreen(width, height)
						panel.setup(s)
						part.set(s)
						for _, k := range pos.keys {
							pressScrollKey(s, k, 1)
						}

						view := s.View()

						if got := viewLineCount(view); got > height {
							t.Errorf("view has %d lines, want at most %d:\n%s", got, height, view)
						}
						flat := flatten(view)
						for _, w := range append([]string{"Running", buildHelpBar(false)}, part.want...) {
							if !strings.Contains(squash(flat), squash(w)) {
								t.Errorf("view lacks %q:\n%s", w, view)
							}
						}
						if panel.name == "panel shown" && !strings.Contains(flat, "Activity: agent-a#3") {
							t.Errorf("view lacks the activity panel:\n%s", view)
						}
						if len(visibleRows(view)) == 0 {
							t.Errorf("no step row shown:\n%s", view)
						}
					})
				}
			}
		}
	}
}

func TestFit_ShorterTerminal_StaysWithinTheHeight(t *testing.T) {
	for _, width := range []int{200, 50} {
		for _, panel := range fitPanels {
			for _, pos := range fitPositions {
				t.Run(fmt.Sprintf("w%d/%s/%s", width, panel.name, pos.name), func(t *testing.T) {
					const height = 20
					s := newFitScreen(width, height)
					panel.setup(s)
					s.SetPendingRequests(PendingRequests{HITLNext: true})
					for _, k := range pos.keys {
						pressScrollKey(s, k, 1)
					}

					if got := viewLineCount(s.View()); got > height {
						t.Errorf("view has %d lines, want at most %d:\n%s", got, height, s.View())
					}
				})
			}
		}
	}
}

func TestFit_PanelHiddenWithBanner_RowsGiveWayToTheBanner(t *testing.T) {
	s := newFitScreen(200, 24)
	pressProgressKey(s, "l")
	pressProgressKey(s, "x")

	view := s.View()

	if got := viewLineCount(view); got > 24 {
		t.Errorf("view has %d lines, want at most 24:\n%s", got, view)
	}
	if !strings.Contains(view, forceStopPromptLabel) {
		t.Errorf("force-stop banner missing:\n%s", view)
	}
}

func TestFit_WrappedHelpBar_IsKeptInFullAndCounted(t *testing.T) {
	s := newFitScreen(30, 30)
	pressProgressKey(s, "l")

	view := s.View()

	if got := viewLineCount(view); got > 30 {
		t.Errorf("view has %d lines, want at most 30:\n%s", got, view)
	}
	if !strings.Contains(squash(view), squash(buildHelpBar(false))) {
		t.Errorf("wrapped help bar is not kept in full:\n%s", view)
	}
}

func TestFit_ResizeToAShorterScreen_StaysWithinTheNewHeight(t *testing.T) {
	s := newFitScreen(200, 40)
	pressScrollKey(s, tea.KeyHome, 1)

	s.Resize(200, 18)

	if got := viewLineCount(s.View()); got > 18 {
		t.Errorf("view has %d lines after the resize, want at most 18:\n%s", got, s.View())
	}
}

func TestFit_LongStepRows_AreOneLineCutWithAnEllipsis(t *testing.T) {
	long := strings.Repeat("a", 150) + "#1"
	for _, status := range []string{"running", "SUCCESS"} {
		t.Run(status, func(t *testing.T) {
			s := NewProgressScreen(80, 24, progressStyles())
			addScrollRows(s, 0, 3)
			s.AppendRow(ProgressRow{AgentInstance: long, Phase: "EXECUTION", Stage: "7", Status: status})
			if status != "running" {
				s.CompleteRow(status)
			}
			addScrollRows(s, 3, 5)

			view := s.View()

			if got := viewLineCount(view); got > 24 {
				t.Errorf("view has %d lines, want at most 24:\n%s", got, view)
			}
			cut := false
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > 80 {
					t.Errorf("line is %d columns wide, want at most 80: %q", lipgloss.Width(line), line)
				}
				if strings.Contains(line, "aaaa") && strings.HasSuffix(ansiRe.ReplaceAllString(line, ""), "\u2026") {
					cut = true
				}
			}
			if !cut {
				t.Errorf("the long step row is not cut with an ellipsis:\n%s", view)
			}
		})
	}
}

func TestFit_LongFailedInfrastructureRow_StaysOneLineWithoutAnEllipsis(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	addScrollRows(s, 0, 3)
	s.AppendRow(infraRow("BLOCKED", "E501", strings.Repeat("reason ", 60)))
	addScrollRows(s, 3, 5)

	view := s.View()

	if got := viewLineCount(view); got > 24 {
		t.Errorf("view has %d lines, want at most 24:\n%s", got, view)
	}
	found := false
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 80 {
			t.Errorf("line is %d columns wide, want at most 80: %q", lipgloss.Width(line), line)
		}
		if strings.Contains(line, "infrastructure failed") {
			found = true
			if strings.HasSuffix(strings.TrimSpace(ansiRe.ReplaceAllString(line, "")), "…") {
				t.Errorf("failed infrastructure row is given an ellipsis: %q", line)
			}
		}
	}
	if !found {
		t.Errorf("failed infrastructure row not shown:\n%s", view)
	}
}

func TestFit_StepRows_NeverWiderThanTheScreen(t *testing.T) {
	for _, width := range []int{80, 40, 20} {
		s := newFitScreen(width, 30)
		pressScrollKey(s, tea.KeyUp, 1)

		for _, line := range strings.Split(s.View(), "\n") {
			if scrollRowRe.MatchString(line) && lipgloss.Width(line) > width {
				t.Errorf("width %d: step row is %d columns wide: %q", width, lipgloss.Width(line), line)
			}
		}
	}
}

func TestFit_BelowTheMinimum_StillShowsOneStepRow(t *testing.T) {
	s := newFitScreen(200, 7)

	view := s.View()

	if len(visibleRows(view)) < 1 {
		t.Errorf("no step row shown on a very short screen:\n%s", view)
	}
}
