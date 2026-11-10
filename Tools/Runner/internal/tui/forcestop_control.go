package tui

import (
	"strconv"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/runflow"
)

// Acknowledgements shown after a confirmed force stop.
const (
	forceStopAppliedText = "Force stop requested: stopping the running invocation"
	forceStopIdleText    = "Nothing is running: force stop not applied"
)

// applyForceStopEvents drains the force-stop gate transitions the progress
// screen recorded, logs them, and raises the shared force-stop request for a
// confirmation. The request is a one-shot: it applies to the invocation running
// right now and is discarded when none runs, so it never lingers to kill a later
// invocation.
func (m *rootModel) applyForceStopEvents() {
	for _, ev := range m.progressScreen.TakeForceStopGateEvents() {
		switch ev.Kind {
		case runflow.ForceStopGateEntered:
			m.debug.Log(domain.EventTUIForceStopGateEntered, "force-stop gate entered",
				domain.F("key", ev.Key))
		case runflow.ForceStopGateResolved:
			m.debug.Log(domain.EventTUIForceStopGateResolved, "force-stop gate resolved",
				domain.F("key", ev.Key), domain.F("confirmed", strconv.FormatBool(ev.Confirmed)))
			if ev.Confirmed {
				m.raiseForceStop()
			}
		}
	}
}

// raiseForceStop raises the force-stop request, logs whether it reached a
// running invocation, and shows the matching acknowledgement.
func (m *rootModel) raiseForceStop() {
	applied := m.forceStop != nil && m.forceStop.Request()
	m.debug.Log(domain.EventTUIForceStopRequested, "force stop requested",
		domain.F("applied", strconv.FormatBool(applied)))
	if applied {
		m.progressScreen.SetStatus(forceStopAppliedText, false)
		return
	}
	m.progressScreen.SetStatus(forceStopIdleText, false)
}
