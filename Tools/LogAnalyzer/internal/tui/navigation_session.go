package tui

import (
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessionRequester is a per-run view that can ask for the Session screen of an actor.
type sessionRequester interface {
	SessionRequested() (domain.ActorRef, bool)
	ResetSessionRequest()
}

// openSessionIfRequested shows the Session screen when the active view asked for
// one. The view is kept as it is, so that esc returns to it with its cursor and
// sort.
func (m Model) openSessionIfRequested() (Model, bool) {
	req, ok := m.runView.(sessionRequester)
	if !ok {
		return m, false
	}
	actor, asked := req.SessionRequested()
	if !asked {
		return m, false
	}
	req.ResetSessionRequest()
	run, found := m.report.FindRun(m.viewRunID)
	if !found {
		return m, false
	}
	opener := m.runView
	session := screens.NewSessionScreen(run, actor, opener.ID(), m.width, m.height, m.screenStyles())
	session.SetRunQuality(screens.RunQualityOf(m.report.Quality, run.Run))
	m.sessionOpener = opener
	m.runView = session
	m.screen = screenSession
	return m, true
}

// closeSession returns from the Session screen to the view it was opened from.
func (m Model) closeSession() Model {
	if m.sessionOpener != nil {
		m.runView = m.sessionOpener
		m.screen = screenName(m.sessionOpener.ID())
	} else {
		m.screen = screenRuns
	}
	m.sessionOpener = nil
	return m
}
