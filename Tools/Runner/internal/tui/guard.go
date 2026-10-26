package tui

import (
	"strconv"

	"mosaic-run/internal/domain"
)

// Triggers recorded on a tui.guard.switched entry.
const (
	guardTriggerKey      = "key"
	guardTriggerResume   = "resume-guard-off"
	guardTriggerStart    = "start"
	guardSwitchedMessage = "anti-loop guard switched"
)

// applyGuardKeyEvents applies the guard key presses the progress screen
// recorded: each one sets the shared waiver and the screen's display and is
// logged.
func (m *rootModel) applyGuardKeyEvents() {
	for _, ev := range m.progressScreen.TakeGuardKeyEvents() {
		m.setGuardWaived(ev.Waive, guardTriggerKey)
	}
}

// setGuardWaived sets the waiver and the progress display, and logs the switch
// when the waiver changed.
func (m *rootModel) setGuardWaived(waived bool, trigger string) {
	changed := m.guardWaiver.Set(waived)
	if m.progressScreen != nil {
		m.progressScreen.SetGuardWaived(waived)
	}
	if changed {
		m.debug.Log(domain.EventTUIGuardSwitched, guardSwitchedMessage,
			domain.F("waived", strconv.FormatBool(waived)),
			domain.F("trigger", trigger))
	}
}

// applyGuardForStart sets the waiver for a session about to start: off only
// for the resume the user chose on a guard-stop ending screen, on for every
// other start. The choice is consumed, so it never carries into a later start.
func (m *rootModel) applyGuardForStart() {
	waived := m.guardOffNextStart
	m.guardOffNextStart = false
	trigger := guardTriggerStart
	if waived {
		trigger = guardTriggerResume
	}
	m.setGuardWaived(waived, trigger)
}
