package runflow

import (
	"github.com/charmbracelet/lipgloss"
)

// Text shown by the force-stop gate. The cancel acknowledgement is set here; the
// confirm acknowledgement depends on whether anything was running, so the root
// model sets it after raising the request.
const (
	forceStopPromptLabel = "Force-stop the running invocation now? (y/n)"
	forceStopCancelText  = "Force stop cancelled - the run continues."
)

// ForceStopGateEventKind names a transition of the force-stop confirmation gate.
type ForceStopGateEventKind int

const (
	// ForceStopGateEntered is the gate opening ('x'/'X').
	ForceStopGateEntered ForceStopGateEventKind = iota

	// ForceStopGateResolved is the gate closing on a confirm or cancel key.
	ForceStopGateResolved
)

// ForceStopGateEvent records one transition of the force-stop gate for the root
// model to log.
type ForceStopGateEvent struct {
	Kind      ForceStopGateEventKind
	Key       string // verbatim key ("x", "y", "esc")
	Confirmed bool   // 'y'/'Y'; meaningful only for ForceStopGateResolved
}

// forceStopGate is the force-stop gate's state, separate from the graceful-stop
// gate so the two never interfere.
type forceStopGate struct {
	pending bool
	events  []ForceStopGateEvent
}

// openForceStopGate opens the gate on key.
func (s *ProgressScreen) openForceStopGate(key string) {
	s.forceStop.pending = true
	s.forceStop.events = append(s.forceStop.events,
		ForceStopGateEvent{Kind: ForceStopGateEntered, Key: key})
}

// updateForceStopGate handles a key while the gate is open. Only a confirm or
// cancel key resolves it; every other key is ignored.
func (s *ProgressScreen) updateForceStopGate(key string) {
	switch key {
	case "y", "Y":
		s.forceStop.pending = false
		s.forceStop.events = append(s.forceStop.events,
			ForceStopGateEvent{Kind: ForceStopGateResolved, Key: key, Confirmed: true})
	case "n", "N", "esc":
		s.forceStop.pending = false
		s.SetStatus(forceStopCancelText, false)
		s.forceStop.events = append(s.forceStop.events,
			ForceStopGateEvent{Kind: ForceStopGateResolved, Key: key})
	}
}

// resetForceStop closes the gate and discards undrained transitions.
func (s *ProgressScreen) resetForceStop() { s.forceStop = forceStopGate{} }

// forceStopBanner renders the force-stop prompt as a bordered box.
func (s *ProgressScreen) forceStopBanner() string { return s.promptBanner(forceStopPromptLabel) }

// promptBanner renders a prompt as a bordered box two columns narrower than the
// screen, so the finished box totals exactly the screen width.
func (s *ProgressScreen) promptBanner(text string) string {
	bodyWidth := s.width - 2
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	return s.styles.Warning.
		Width(bodyWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.styles.Warning.GetForeground()).
		Render(text)
}

// TakeForceStopGateEvents drains the recorded transitions (same drain
// semantics as TakeStopGateEvents).
func (s *ProgressScreen) TakeForceStopGateEvents() []ForceStopGateEvent {
	events := s.forceStop.events
	s.forceStop.events = nil
	return events
}

// ForceStopConfirmPending reports whether the force-stop gate is open.
func (s *ProgressScreen) ForceStopConfirmPending() bool { return s.forceStop.pending }
