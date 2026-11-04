package runflow

// guardOffIndicatorText is shown on its own line for as long as the anti-loop
// guard is switched off, so the state is never visible only in the help bar.
const guardOffIndicatorText = "LOOP GUARD OFF for this session - same-agent dispatches are not limited (g to turn it back on)"

// GuardKeyEvent is one press of the loop-guard key on the progress screen.
type GuardKeyEvent struct {
	Key   string // verbatim bubbletea key: "g" or "G"
	Waive bool   // true when the screen showed the guard on at the press
}

// SetGuardWaived sets the guard display state; the root model calls it.
func (s *ProgressScreen) SetGuardWaived(waived bool) { s.guardWaived = waived }

// GuardWaived reports the guard display state.
func (s *ProgressScreen) GuardWaived() bool { return s.guardWaived }

// TakeGuardKeyEvents returns the guard key presses recorded since the last
// call and forgets them (drain semantics, like TakeRequestKeyEvents).
func (s *ProgressScreen) TakeGuardKeyEvents() []GuardKeyEvent {
	events := s.guardEvents
	s.guardEvents = nil
	return events
}

// recordGuardKey records one press of the guard key. The caller has already
// established that no stop is pending or confirmed.
func (s *ProgressScreen) recordGuardKey(key string) {
	s.guardEvents = append(s.guardEvents, GuardKeyEvent{Key: key, Waive: !s.guardWaived})
}

// guardIndicatorLine renders the permanent guard-off line, or "" while the
// guard is on.
func (s *ProgressScreen) guardIndicatorLine() string {
	if !s.guardWaived {
		return ""
	}
	return s.styles.Error.Width(s.width).Render(guardOffIndicatorText)
}
