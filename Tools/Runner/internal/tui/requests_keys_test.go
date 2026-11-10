package tui

// requests_keys_test.go verifies that the progress-screen request keys reach the
// shared in-run request object, that the pending indicator follows the object
// (including when the session consumes a request), and that requesting and
// cancelling are written to the Runner debug log while ignored keys write none.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runflow"
)

// repaintMsg stands in for the one-second tick that redraws the progress
// screen: any message the root model delegates to the screen.
type repaintMsg struct{}

var requestLogEvents = []string{
	domain.EventTUIRequestSet,
	domain.EventTUIRequestCancelled,
	domain.EventTUIRequestDiscarded,
}

const (
	manualPendingText = "Requested: manual dispatch at the next routing decision (m to cancel)"
	hitlPendingText   = "Requested: HITL for the next agent (h to cancel)"
)

// newRequestsModel returns a rootModel on the progress screen wired to the
// given shared request object and recorder. reqs may be nil to exercise the
// fallback.
func newRequestsModel(rec *tuiRecordingLogger, reqs *session.InRunRequests) *rootModel {
	sess := &stubNavSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	m := newRootModel(context.Background(), sess, Options{
		Theme:         tuicommon.DefaultTheme(),
		Debug:         rec,
		InRunRequests: reqs,
	})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	return m
}

// requestLogCount returns how many request lifecycle entries the recorder holds.
func requestLogCount(rec *tuiRecordingLogger) int {
	n := 0
	for _, e := range requestLogEvents {
		n += len(rec.eventsNamed(e))
	}
	return n
}

func TestRequestKeys_Press_SetsTheSharedRequest(t *testing.T) {
	cases := []struct {
		key  string
		kind domain.InRunRequest
	}{
		{"m", domain.InRunRequestManualDispatch},
		{"M", domain.InRunRequestManualDispatch},
		{"h", domain.InRunRequestHITLNext},
		{"H", domain.InRunRequestHITLNext},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := newRequestsModel(&tuiRecordingLogger{}, reqs)

			pressProgressRune(m, tc.key)

			if !reqs.Pending(tc.kind) {
				t.Fatalf("Pending(%q) = false after pressing %q", tc.kind, tc.key)
			}
			for _, other := range []domain.InRunRequest{domain.InRunRequestManualDispatch, domain.InRunRequestHITLNext} {
				if other != tc.kind && reqs.Pending(other) {
					t.Errorf("pressing %q also made %q pending", tc.key, other)
				}
			}
		})
	}
}

func TestRequestKeys_SessionTake_SeesTheRequestExactlyOnce(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)

	pressProgressRune(m, "m")
	first := reqs.Take(domain.InRunRequestManualDispatch)
	second := reqs.Take(domain.InRunRequestManualDispatch)

	if !first || second {
		t.Errorf("Take results = %v then %v, want true then false", first, second)
	}
}

func TestRequestKeys_SecondPress_CancelsTheRequest(t *testing.T) {
	for _, tc := range []struct {
		key  string
		kind domain.InRunRequest
	}{{"m", domain.InRunRequestManualDispatch}, {"h", domain.InRunRequestHITLNext}} {
		t.Run(tc.key, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := newRequestsModel(&tuiRecordingLogger{}, reqs)

			pressProgressRune(m, tc.key)
			if !reqs.Pending(tc.kind) {
				t.Fatal("precondition: the first press did not make the request pending")
			}
			pressProgressRune(m, tc.key)

			if reqs.Pending(tc.kind) {
				t.Error("request still pending after the second press")
			}
			if reqs.Take(tc.kind) {
				t.Error("a cancelled request was taken by the session")
			}
		})
	}
}

func TestRequestKeys_ThirdPress_RequestsAgain(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)

	pressProgressRune(m, "m")
	pressProgressRune(m, "m")
	pressProgressRune(m, "m")

	if !reqs.Pending(domain.InRunRequestManualDispatch) {
		t.Error("Pending = false after press, cancel, press")
	}
}

func TestRequestKeys_PressAfterSessionTook_RequestsAnewInsteadOfCancelling(t *testing.T) {
	reqs := session.NewInRunRequests()
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, reqs)
	pressProgressRune(m, "m")
	reqs.Take(domain.InRunRequestManualDispatch) // the session applied it

	pressProgressRune(m, "m")

	if !reqs.Pending(domain.InRunRequestManualDispatch) {
		t.Error("Pending = false: a press after the request was applied must set a new request, not cancel a stale display")
	}
	if got := len(rec.eventsNamed(domain.EventTUIRequestSet)); got != 2 {
		t.Errorf("%d set entries, want 2", got)
	}
	if got := len(rec.eventsNamed(domain.EventTUIRequestCancelled)); got != 0 {
		t.Errorf("%d cancelled entries, want 0", got)
	}
}

func TestRequestKeys_NilOption_FallsBackToAWorkingObject(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, nil)

	pressProgressRune(m, "m")

	if !m.progressScreen.PendingRequests().ManualDispatch {
		t.Error("the screen shows nothing pending after m with no Options.InRunRequests; the root model must fall back to its own object")
	}
	if !strings.Contains(m.View(), manualPendingText) {
		t.Error("View() lacks the pending line with the fallback object")
	}
}

func TestRequestKeys_DoNotInterruptTheRun(t *testing.T) {
	stop := session.NewStopSignal()
	sess := &stubNavSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	m := newRootModel(context.Background(), sess, Options{
		Theme:         tuicommon.DefaultTheme(),
		StopSignal:    stop,
		InRunRequests: session.NewInRunRequests(),
	})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress

	pressProgressRune(m, "m")
	pressProgressRune(m, "h")

	if stop.Requested() {
		t.Error("a request key armed the graceful-stop signal")
	}
	if m.ctx.Err() != nil {
		t.Errorf("m.ctx.Err() = %v after request keys, want nil", m.ctx.Err())
	}
	if m.screen != screenProgress {
		t.Errorf("screen = %v after request keys, want screenProgress", m.screen)
	}
	if m.progressScreen.GracefulStop() || m.progressScreen.ConfirmPending() {
		t.Error("a request key changed the stop state")
	}
}

// ---------------------------------------------------------------------------
// Indicator
// ---------------------------------------------------------------------------

func TestRequestKeys_Indicator_ShowsEachPendingRequest(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)
	m.width, m.height = 120, 24
	m.resizeScreens()

	pressProgressRune(m, "m")
	pressProgressRune(m, "h")
	view := m.progressScreen.View()

	if !strings.Contains(view, manualPendingText) || !strings.Contains(view, hitlPendingText) {
		t.Errorf("progress view lacks a pending line; view:\n%s", view)
	}
}

func TestRequestKeys_Indicator_ClearsOnCancel(t *testing.T) {
	m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
	m.width, m.height = 120, 24
	m.resizeScreens()

	pressProgressRune(m, "m")
	if !m.progressScreen.PendingRequests().ManualDispatch || !strings.Contains(m.progressScreen.View(), "Requested:") {
		t.Fatal("precondition: the first press did not show a pending indicator")
	}
	pressProgressRune(m, "m")

	if m.progressScreen.PendingRequests().ManualDispatch {
		t.Error("the screen still shows manual dispatch pending after the cancel")
	}
	if strings.Contains(m.progressScreen.View(), "Requested:") {
		t.Error("the view still shows a pending line after the cancel")
	}
}

// ---------------------------------------------------------------------------
// Status-line acknowledgements
// ---------------------------------------------------------------------------

func TestRequestKeys_Acknowledgements_SetAndCancelTexts(t *testing.T) {
	for _, tc := range []struct {
		key, setText, cancelText string
	}{
		{"m", "Manual dispatch requested - the manual routing dialogue opens at the next routing decision.", "Manual dispatch request cancelled."},
		{"h", "HITL requested for the next agent.", "HITL request cancelled."},
	} {
		t.Run(tc.key, func(t *testing.T) {
			m := newRequestsModel(&tuiRecordingLogger{}, session.NewInRunRequests())
			m.width, m.height = 160, 30
			m.resizeScreens()

			pressProgressRune(m, tc.key)
			if !strings.Contains(m.progressScreen.View(), tc.setText) {
				t.Errorf("view lacks the set acknowledgement %q", tc.setText)
			}

			pressProgressRune(m, tc.key)
			if !strings.Contains(m.progressScreen.View(), tc.cancelText) {
				t.Errorf("view lacks the cancel acknowledgement %q", tc.cancelText)
			}
		})
	}
}

// The already_applied cancel outcome (Cancel losing to the session) cannot be
// forced through the public model API deterministically and is not covered here.

func TestRequestKeys_PendingRequest_SurvivesIgnoredKeysWhileGateIsPending(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)
	pressProgressRune(m, "m")
	pressProgressRune(m, "s") // opens the stop gate

	pressProgressRune(m, "m")
	pressProgressRune(m, "h")

	if !reqs.Pending(domain.InRunRequestManualDispatch) {
		t.Error("the earlier manual request was cancelled by a key ignored during the gate")
	}
	if reqs.Pending(domain.InRunRequestHITLNext) {
		t.Error("a key pressed while the gate is pending set a request")
	}
}

func TestRequestKeys_Indicator_ClearsOnNextUpdateAfterSessionTake(t *testing.T) {
	for _, tc := range []struct {
		key  string
		kind domain.InRunRequest
	}{
		{key: "m", kind: domain.InRunRequestManualDispatch},
		{key: "h", kind: domain.InRunRequestHITLNext},
	} {
		t.Run(tc.key, func(t *testing.T) {
			reqs := session.NewInRunRequests()
			m := newRequestsModel(&tuiRecordingLogger{}, reqs)
			pressProgressRune(m, tc.key)
			if got := m.progressScreen.PendingRequests(); got == (runflow.PendingRequests{}) {
				t.Fatal("precondition: the screen shows nothing pending after the key")
			}

			reqs.Take(tc.kind) // the session consumed it; the screen has seen no message
			m.Update(repaintMsg{})

			if got := m.progressScreen.PendingRequests(); got != (runflow.PendingRequests{}) {
				t.Errorf("PendingRequests() = %+v after the session took the request and the screen updated, want zero", got)
			}
		})
	}
}

func TestRequestKeys_Indicator_KeepsTheOtherRequestWhenOneIsTaken(t *testing.T) {
	reqs := session.NewInRunRequests()
	m := newRequestsModel(&tuiRecordingLogger{}, reqs)
	pressProgressRune(m, "m")
	pressProgressRune(m, "h")

	reqs.Take(domain.InRunRequestManualDispatch)
	m.Update(repaintMsg{})

	got := m.progressScreen.PendingRequests()
	if got.ManualDispatch || !got.HITLNext {
		t.Errorf("PendingRequests() = %+v, want only HITLNext", got)
	}
}

// ---------------------------------------------------------------------------
// Debug log
// ---------------------------------------------------------------------------

func TestRequestKeys_Logging_SetEntryCarriesRequestAndKey(t *testing.T) {
	for _, tc := range []struct {
		key  string
		kind domain.InRunRequest
	}{
		{"m", domain.InRunRequestManualDispatch}, {"M", domain.InRunRequestManualDispatch},
		{"h", domain.InRunRequestHITLNext}, {"H", domain.InRunRequestHITLNext},
	} {
		t.Run(tc.key, func(t *testing.T) {
			rec := &tuiRecordingLogger{}
			m := newRequestsModel(rec, session.NewInRunRequests())

			pressProgressRune(m, tc.key)

			e := requireOneEntry(t, rec, domain.EventTUIRequestSet)
			if got, _ := e.field("request"); got != string(tc.kind) {
				t.Errorf("request field = %q, want %q", got, tc.kind)
			}
			if got, _ := e.field("key"); got != tc.key {
				t.Errorf("key field = %q, want %q verbatim", got, tc.key)
			}
		})
	}
}

func TestRequestKeys_Logging_CancelEntryCarriesOutcomeCancelled(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, session.NewInRunRequests())

	pressProgressRune(m, "h")
	pressProgressRune(m, "h")

	if got := len(rec.eventsNamed(domain.EventTUIRequestSet)); got != 1 {
		t.Errorf("%d set entries, want 1", got)
	}
	e := requireOneEntry(t, rec, domain.EventTUIRequestCancelled)
	if got, _ := e.field("request"); got != string(domain.InRunRequestHITLNext) {
		t.Errorf("request field = %q, want %q", got, domain.InRunRequestHITLNext)
	}
	if got, _ := e.field("key"); got != "h" {
		t.Errorf("key field = %q, want h", got)
	}
	if got, _ := e.field("outcome"); got != "cancelled" {
		t.Errorf("outcome field = %q, want cancelled", got)
	}
}

func TestRequestKeys_Logging_OrderIsSetThenCancelled(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, session.NewInRunRequests())

	pressProgressRune(m, "m")
	pressProgressRune(m, "m")

	order := rec.eventOrder()
	if len(order) != 2 || order[0] != domain.EventTUIRequestSet || order[1] != domain.EventTUIRequestCancelled {
		t.Errorf("events = %v, want [set cancelled]", order)
	}
}

func TestRequestKeys_Logging_IgnoredKeysWriteNothing(t *testing.T) {
	t.Run("gate pending", func(t *testing.T) {
		rec := &tuiRecordingLogger{}
		reqs := session.NewInRunRequests()
		m := newRequestsModel(rec, reqs)
		pressProgressRune(m, "s")
		before := len(rec.snapshot())

		for _, k := range []string{"m", "M", "h", "H"} {
			pressProgressRune(m, k)
		}

		if got := requestLogCount(rec); got != 0 {
			t.Errorf("%d request entries while the gate is pending, want 0", got)
		}
		if len(rec.snapshot()) != before {
			t.Error("ignored keys wrote debug entries")
		}
		if reqs.Pending(domain.InRunRequestManualDispatch) || reqs.Pending(domain.InRunRequestHITLNext) {
			t.Error("a key pressed while the gate is pending set a request")
		}
	})
	t.Run("stop confirmed", func(t *testing.T) {
		rec := &tuiRecordingLogger{}
		reqs := session.NewInRunRequests()
		m := newRequestsModel(rec, reqs)
		triggerGracefulStop(m)
		before := len(rec.snapshot())

		for _, k := range []string{"m", "M", "h", "H"} {
			pressProgressRune(m, k)
		}

		if got := requestLogCount(rec); got != 0 {
			t.Errorf("%d request entries after a confirmed stop, want 0", got)
		}
		if len(rec.snapshot()) != before {
			t.Error("ignored keys wrote debug entries")
		}
		if reqs.Pending(domain.InRunRequestManualDispatch) || reqs.Pending(domain.InRunRequestHITLNext) {
			t.Error("a key pressed after a confirmed stop set a request")
		}
	})
}

func TestRequestKeys_Logging_UnrelatedKeysAndRepaintsWriteNoRequestEntries(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, session.NewInRunRequests())

	for _, k := range []string{"x", "q", "j"} {
		pressProgressRune(m, k)
	}
	m.Update(repaintMsg{})
	m.Update(repaintMsg{})

	if got := requestLogCount(rec); got != 0 {
		t.Errorf("%d request entries from unrelated keys and repaints, want 0", got)
	}
}

func TestRequestKeys_Logging_RepaintsDoNotRepeatTheSetEntry(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newRequestsModel(rec, session.NewInRunRequests())
	pressProgressRune(m, "m")

	m.Update(repaintMsg{})
	m.Update(repaintMsg{})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	if got := len(rec.eventsNamed(domain.EventTUIRequestSet)); got != 1 {
		t.Errorf("%d set entries after repaints, want exactly 1", got)
	}
}
