package decision

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens"
)

// StopChoice classifies the user's response on the stop recovery screen.
type StopChoice string

const (
	// StopChoiceRetry directs the runner to resume the run: after a routing
	// stop it re-invokes the routing consultant with the same state, after a
	// pre-consultation stop it repeats pre-consultation. For when the user has
	// fixed something externally and wants the run to continue.
	StopChoiceRetry StopChoice = "retry"

	// StopChoiceManualDispatch directs the runner to invoke the ManualResolver
	// for a single routing decision, after which the run continues normally
	// with the configured resolver.
	StopChoiceManualDispatch StopChoice = "manual-dispatch"
)

// stopOptions lists the available recovery actions in cursor order for a
// routing stop.
var stopOptions = []StopChoice{
	StopChoiceRetry,
	StopChoiceManualDispatch,
}

// preConsultationStopOptions lists the recovery actions after a
// pre-consultation stop. Manual dispatch is absent: it only arms manual
// resolution for routing consultations, so it cannot replace the
// pre-consultation decision.
var preConsultationStopOptions = []StopChoice{
	StopChoiceRetry,
}

// StopScreen presents the stop reason and offers recovery actions when the
// routing consultant has stopped the run.
//
// Navigation contract:
//   - Enter on "Retry"           -> Done() == true, Choice() == StopChoiceRetry.
//   - Enter on "Manual dispatch" -> Done() == true, Choice() == StopChoiceManualDispatch.
//   - Esc                        -> Back() == true.
type StopScreen struct {
	reason string
	// preConsultation selects the pre-consultation variant: title, wording and
	// option list differ from the routing-stop variant.
	preConsultation bool
	cursor int
	done   bool
	back   bool
	width  int
	height int
	styles screens.Styles
}

// NewStopScreen creates a stop recovery screen displaying the given stop reason.
func NewStopScreen(reason string, width, height int, styles screens.Styles) *StopScreen {
	return &StopScreen{
		reason: reason,
		cursor: 0, // Retry is selected by default
		width:  width,
		height: height,
		styles: styles,
	}
}

// NewPreConsultationStopScreen creates the recovery screen for a run the
// orchestrator stopped at pre-consultation. It offers exactly one choice,
// StopChoiceRetry; the cursor cannot move off it.
func NewPreConsultationStopScreen(reason string, width, height int, styles screens.Styles) *StopScreen {
	s := NewStopScreen(reason, width, height, styles)
	s.preConsultation = true
	return s
}

// options returns the recovery actions for the screen's variant.
func (s *StopScreen) options() []StopChoice {
	if s.preConsultation {
		return preConsultationStopOptions
	}
	return stopOptions
}

// Update processes key messages for the stop recovery screen.
func (s *StopScreen) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.options())-1 {
			s.cursor++
		}
	case "enter":
		s.done = true
	case "esc":
		s.back = true
	}
	return nil
}

// View renders the stop recovery screen showing the reason and recovery options.
func (s *StopScreen) View() string {
	titleText := "Run Stopped"
	retryLabel := "Retry — re-invoke the routing decision with the same state"
	if s.preConsultation {
		titleText = "Run Stopped at Pre-Consultation"
		retryLabel = "Retry — resume the run, which asks the orchestrator for pre-consultation again"
	}
	title := s.styles.Title.Width(s.width).Render(titleText)
	border := s.styles.Border.Width(s.width).Render(strings.Repeat("─", s.width))

	reasonText := "Reason: " + s.reason
	if s.preConsultation {
		reasonText = "The orchestrator stopped the run at pre-consultation.\n" + reasonText
	}
	reason := s.styles.Body.Width(s.width).Render(reasonText)

	optionLabels := map[StopChoice]string{
		StopChoiceRetry:          retryLabel,
		StopChoiceManualDispatch: "Manual dispatch — take over as resolver for this one decision",
	}

	var optionsBuilder strings.Builder
	optionsBuilder.WriteString(s.styles.Body.Width(s.width).Render("Recovery options:") + "\n")
	for i, choice := range s.options() {
		label := optionLabels[choice]
		if i == s.cursor {
			optionsBuilder.WriteString("▶ " + s.styles.Selected.Render(label) + "\n")
		} else {
			optionsBuilder.WriteString("  " + s.styles.Body.Render(label) + "\n")
		}
	}

	help := s.styles.Help.Width(s.width).Render("↑/k up  ↓/j down  enter select  esc quit  ctrl+c quit")
	return strings.Join([]string{title, border, reason, "", optionsBuilder.String(), border, help}, "\n")
}

// Done reports whether the user confirmed a recovery choice.
func (s *StopScreen) Done() bool { return s.done }

// Back reports whether the user pressed Esc.
func (s *StopScreen) Back() bool { return s.back }

// Choice returns the selected recovery action. Only valid when Done() is true.
func (s *StopScreen) Choice() StopChoice {
	opts := s.options()
	if s.cursor >= 0 && s.cursor < len(opts) {
		return opts[s.cursor]
	}
	return ""
}

// Resize updates the screen dimensions.
func (s *StopScreen) Resize(width, height int) {
	s.width = width
	s.height = height
}
