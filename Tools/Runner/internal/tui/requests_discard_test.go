package tui

// requests_discard_test.go verifies that a pending in-run request never
// survives the end or restart of a session: it is discarded, logged once per
// discarded request, and neither shown nor applied afterwards.
//
// The restart tests place the requests after the run ended and before the
// restart action, so they can only pass when the restart itself discards. The
// run-end discard has its own tests and would otherwise mask a missing restart
// discard.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runflow"
)

// pendingKinds lists the kinds in the order Reset reports them.
var pendingKinds = []domain.InRunRequest{
	domain.InRunRequestManualDispatch,
	domain.InRunRequestHITLNext,
}

// requireNoRequestState fails unless nothing is pending in the shared object,
// shown on the screen, or takeable by the session.
func requireNoRequestState(t *testing.T, m *rootModel, reqs *session.InRunRequests) {
	t.Helper()
	for _, k := range pendingKinds {
		if reqs.Pending(k) {
			t.Errorf("%q is still pending in the shared object", k)
		}
		if reqs.Take(k) {
			t.Errorf("the session could still take %q", k)
		}
	}
	if m.progressScreen != nil {
		if got := m.progressScreen.PendingRequests(); got.ManualDispatch || got.HITLNext {
			t.Errorf("the screen still shows %+v", got)
		}
		if strings.Contains(m.progressScreen.View(), "Requested:") {
			t.Error("the progress view still shows a pending line")
		}
		if got := m.progressScreen.TakeRequestKeyEvents(); len(got) != 0 {
			t.Errorf("undrained request key events %+v survived", got)
		}
	}
}

// requireDiscardedEntries asserts one discarded entry per kind, in order, all
// carrying the given reason.
func requireDiscardedEntries(t *testing.T, rec *tuiRecordingLogger, reason string) {
	t.Helper()
	entries := rec.eventsNamed(domain.EventTUIRequestDiscarded)
	if len(entries) != len(pendingKinds) {
		t.Fatalf("%d discarded entries, want %d (one per discarded request)", len(entries), len(pendingKinds))
	}
	for i, k := range pendingKinds {
		if got, _ := entries[i].field("request"); got != string(k) {
			t.Errorf("discarded entry %d request = %q, want %q", i, got, k)
		}
		if got, _ := entries[i].field("reason"); got != reason {
			t.Errorf("discarded entry %d reason = %q, want %q", i, got, reason)
		}
	}
}

func requestBoth(reqs *session.InRunRequests) {
	for _, k := range pendingKinds {
		reqs.Request(k)
	}
}

// ---------------------------------------------------------------------------
// Restart paths
// ---------------------------------------------------------------------------

// requestRestartPaths drives each restart path in two halves: reach the screen
// the restart action is taken on, then take the action.
var requestRestartPaths = []struct {
	name  string
	reach func(t *testing.T, m *rootModel)
	act   func(t *testing.T, m *rootModel)
}{
	{
		name:  "done-screen continue",
		reach: func(t *testing.T, m *rootModel) { sendRunStoppedDone(m) },
		act:   func(t *testing.T, m *rootModel) { pressContinue(m) },
	},
	{
		name: "exec-override retry",
		reach: func(t *testing.T, m *rootModel) {
			m.Update(runErrorMsg{err: &domain.HarnessLaunchError{
				Harness: "claude-code", Executable: "/usr/bin/claude",
				Err: errors.New("exec: no such file or directory"),
			}})
		},
		act: func(t *testing.T, m *rootModel) {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/opt/claude")})
			m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		},
	},
	{
		name:  "stop-recovery retry",
		reach: func(t *testing.T, m *rootModel) { enterStopRecoveryScreen(t, m) },
		act:   func(t *testing.T, m *rootModel) { m.Update(tea.KeyMsg{Type: tea.KeyEnter}) },
	},
	{
		name:  "stop-recovery manual dispatch",
		reach: func(t *testing.T, m *rootModel) { enterStopRecoveryScreen(t, m) },
		act: func(t *testing.T, m *rootModel) {
			m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		},
	},
}

func TestRequestDiscard_EveryRestartPath_DiscardsPendingRequests(t *testing.T) {
	for _, path := range requestRestartPaths {
		t.Run(path.name, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			rec := &tuiRecordingLogger{}
			m := newRequestsModel(rec, reqs)
			path.reach(t, m)
			requestBoth(reqs)
			m.progressScreen.SetPendingRequests(runflow.PendingRequests{ManualDispatch: true, HITLNext: true})

			path.act(t, m)

			if m.screen != screenProgress {
				t.Fatalf("precondition: screen = %v after the restart action, want screenProgress", m.screen)
			}
			requireNoRequestState(t, m, reqs)
			requireDiscardedEntries(t, rec, "restart")
		})
	}
}

func TestRequestDiscard_EveryRestartPath_ResumedRunStartsWithWorkingKeys(t *testing.T) {
	for _, path := range requestRestartPaths {
		t.Run(path.name, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := newRequestsModel(&tuiRecordingLogger{}, reqs)
			path.reach(t, m)
			requestBoth(reqs)
			path.act(t, m)

			pressProgressRune(m, "m")

			if !reqs.Pending(domain.InRunRequestManualDispatch) {
				t.Error("m did not set a request on the resumed run")
			}
		})
	}
}

func TestRequestDiscard_Restart_WithNothingPending_LogsNothing(t *testing.T) {
	for _, path := range requestRestartPaths {
		t.Run(path.name, func(t *testing.T) {
			rec := &tuiRecordingLogger{}
			m := newRequestsModel(rec, session.NewInRunRequests())
			path.reach(t, m)

			path.act(t, m)

			if got := requestLogCount(rec); got != 0 {
				t.Errorf("%d request entries for a restart with nothing pending, want 0", got)
			}
		})
	}
}

func TestRequestDiscard_Restart_LogsOnlyTheKindThatWasPending(t *testing.T) {
	reqs := session.NewInRunRequests()
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, reqs)
	sendRunStoppedDone(m)
	reqs.Request(domain.InRunRequestHITLNext)

	pressContinue(m)

	entries := rec.eventsNamed(domain.EventTUIRequestDiscarded)
	if len(entries) != 1 {
		t.Fatalf("%d discarded entries, want 1", len(entries))
	}
	if got, _ := entries[0].field("request"); got != string(domain.InRunRequestHITLNext) {
		t.Errorf("request field = %q, want hitl_next", got)
	}
}

// ---------------------------------------------------------------------------
// Fresh launch and run end
// ---------------------------------------------------------------------------

func TestRequestDiscard_SessionLaunch_DiscardsPendingRequests(t *testing.T) {
	reqs := session.NewInRunRequests()
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, reqs)
	requestBoth(reqs)

	m.launchSession()

	requireNoRequestState(t, m, reqs)
	requireDiscardedEntries(t, rec, "session_start")
}

func TestRequestDiscard_RunEnd_DiscardsPendingRequests(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
	}{
		{"run completed", runDoneMsg{outcome: domain.RunOutcome{Status: domain.RunCompleted}}},
		{"run stopped", runDoneMsg{outcome: domain.RunOutcome{Status: domain.RunStopped, Message: "stopped"}}},
		{"run error", runErrorMsg{err: errors.New("session failed")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			rec := &tuiRecordingLogger{}
			m := newRequestsModel(rec, reqs)
			pressProgressRune(m, "m")
			pressProgressRune(m, "h")

			m.Update(tc.msg)

			requireNoRequestState(t, m, reqs)
			requireDiscardedEntries(t, rec, "run_end")
		})
	}
}

func TestRequestDiscard_RunEnd_WithNothingPending_LogsNothing(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, session.NewInRunRequests())

	m.Update(runDoneMsg{outcome: domain.RunOutcome{Status: domain.RunCompleted}})

	if got := len(rec.eventsNamed(domain.EventTUIRequestDiscarded)); got != 0 {
		t.Errorf("%d discarded entries with nothing pending, want 0", got)
	}
}
