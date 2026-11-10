package runflow

// progress_requests_keys_test.go verifies the progress screen's in-run request
// keys: which presses are accepted, what they record for the root model, and
// when they are ignored.
//
// The screen only records intent. It never toggles its own pending display:
// the root model owns the shared request object and sets the display from it,
// so a key's Cancel flag reports what the screen showed at the press.

import (
	"testing"

	"mosaic-run/internal/domain"
)

var requestKeyCases = []struct {
	key  string
	kind domain.InRunRequest
}{
	{"m", domain.InRunRequestManualDispatch},
	{"M", domain.InRunRequestManualDispatch},
	{"h", domain.InRunRequestHITLNext},
	{"H", domain.InRunRequestHITLNext},
}

func newRequestsScreen() *ProgressScreen {
	return NewProgressScreen(120, 24, progressStyles())
}

func TestProgressScreen_RequestKey_RecordsOneEventNamingTheRequest(t *testing.T) {
	for _, tc := range requestKeyCases {
		t.Run(tc.key, func(t *testing.T) {
			s := newRequestsScreen()

			pressProgressKey(s, tc.key)
			events := s.TakeRequestKeyEvents()

			if len(events) != 1 {
				t.Fatalf("TakeRequestKeyEvents() returned %d events after %q, want 1", len(events), tc.key)
			}
			if events[0].Request != tc.kind {
				t.Errorf("event Request = %q, want %q", events[0].Request, tc.kind)
			}
			if events[0].Key != tc.key {
				t.Errorf("event Key = %q, want %q verbatim", events[0].Key, tc.key)
			}
			if events[0].Cancel {
				t.Error("event Cancel = true on a screen showing nothing pending, want false")
			}
		})
	}
}

func TestProgressScreen_RequestKey_WhenShownPending_RecordsACancel(t *testing.T) {
	for _, tc := range requestKeyCases {
		t.Run(tc.key, func(t *testing.T) {
			s := newRequestsScreen()
			s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})

			pressProgressKey(s, tc.key)
			events := s.TakeRequestKeyEvents()

			if len(events) != 1 || !events[0].Cancel || events[0].Request != tc.kind {
				t.Fatalf("events = %+v, want one cancel event for %q: a second press cancels", events, tc.kind)
			}
		})
	}
}

func TestProgressScreen_RequestKey_CancelFlagIsPerKind(t *testing.T) {
	s := newRequestsScreen()
	s.SetPendingRequests(PendingRequests{ManualDispatch: true})

	pressProgressKey(s, "h")
	events := s.TakeRequestKeyEvents()

	if len(events) != 1 || events[0].Request != domain.InRunRequestHITLNext || events[0].Cancel {
		t.Fatalf("events = %+v, want one HITL set event: only manual dispatch is shown pending", events)
	}
}

func TestProgressScreen_RequestKey_DoesNotToggleItsOwnDisplay(t *testing.T) {
	s := newRequestsScreen()

	pressProgressKey(s, "m")
	pressProgressKey(s, "h")

	if got := s.PendingRequests(); got != (PendingRequests{}) {
		t.Errorf("PendingRequests() = %+v after key presses, want zero: the root model sets the display from the shared object", got)
	}
}

func TestProgressScreen_SetPendingRequests_IsReadBack(t *testing.T) {
	s := newRequestsScreen()
	want := PendingRequests{ManualDispatch: true, HITLNext: false}

	s.SetPendingRequests(want)

	if got := s.PendingRequests(); got != want {
		t.Errorf("PendingRequests() = %+v, want %+v", got, want)
	}
}

func TestProgressScreen_TakeRequestKeyEvents_Drains(t *testing.T) {
	s := newRequestsScreen()
	pressProgressKey(s, "m")
	pressProgressKey(s, "h")

	first := s.TakeRequestKeyEvents()
	second := s.TakeRequestKeyEvents()

	if len(first) != 2 || first[0].Request != domain.InRunRequestManualDispatch || first[1].Request != domain.InRunRequestHITLNext {
		t.Fatalf("first drain = %+v, want [manual, hitl] in press order", first)
	}
	if len(second) != 0 {
		t.Errorf("second drain = %+v, want empty: each press is reported once", second)
	}
}

func TestProgressScreen_RequestKey_IgnoredWhileGateIsPending(t *testing.T) {
	for _, tc := range requestKeyCases {
		t.Run(tc.key, func(t *testing.T) {
			s := pendingProgressScreen(t)

			pressProgressKey(s, tc.key)

			if got := s.TakeRequestKeyEvents(); len(got) != 0 {
				t.Errorf("events = %+v while the stop gate is pending, want none", got)
			}
			if !s.ConfirmPending() {
				t.Error("the gate closed: a request key must not dismiss the confirmation")
			}
			if len(s.TakeStopGateEvents()) != 0 {
				t.Error("a request key recorded a stop-gate event")
			}
		})
	}
}

func TestProgressScreen_RequestKey_IgnoredAfterStopConfirmed(t *testing.T) {
	for _, tc := range requestKeyCases {
		t.Run(tc.key, func(t *testing.T) {
			s := newRequestsScreen()
			pressProgressKey(s, "s")
			pressProgressKey(s, "y")
			if !s.GracefulStop() {
				t.Fatal("precondition: GracefulStop() = false after s, y")
			}
			s.TakeRequestKeyEvents()

			pressProgressKey(s, tc.key)

			if got := s.TakeRequestKeyEvents(); len(got) != 0 {
				t.Errorf("events = %+v after a confirmed stop, want none", got)
			}
		})
	}
}

func TestProgressScreen_RequestKey_AfterCancelledGate_IsAcceptedAgain(t *testing.T) {
	s := newRequestsScreen()
	pressProgressKey(s, "s")
	pressProgressKey(s, "n")

	pressProgressKey(s, "m")

	if got := s.TakeRequestKeyEvents(); len(got) != 1 {
		t.Errorf("events = %+v after the gate was cancelled, want 1: only a pending or confirmed stop blocks the keys", got)
	}
}

func TestProgressScreen_RequestKey_ExistingKeysKeepTheirBehaviour(t *testing.T) {
	t.Run("s still opens the gate after a request key", func(t *testing.T) {
		s := newRequestsScreen()
		pressProgressKey(s, "m")
		pressProgressKey(s, "s")
		if !s.ConfirmPending() {
			t.Error("ConfirmPending() = false after m, s")
		}
	})
	t.Run("a still opens the artifact view", func(t *testing.T) {
		s := newRequestsScreen()
		pressProgressKey(s, "h")
		pressProgressKey(s, "a")
		if !s.ArtifactViewRequested() {
			t.Error("ArtifactViewRequested() = false after h, a")
		}
		if got := s.TakeRequestKeyEvents(); len(got) != 1 {
			t.Errorf("events = %+v, want only the h press", got)
		}
	})
	t.Run("s, a, y, n record no request event", func(t *testing.T) {
		s := newRequestsScreen()
		for _, k := range []string{"s", "n", "S", "N", "a", "A", "i", "I", "y", "Y"} {
			pressProgressKey(s, k)
		}
		if got := s.TakeRequestKeyEvents(); len(got) != 0 {
			t.Errorf("events = %+v from existing keys, want none", got)
		}
	})
}

func TestProgressScreen_ResetRequestState_ClearsDisplayAndUndrainedEvents(t *testing.T) {
	s := newRequestsScreen()
	s.SetPendingRequests(PendingRequests{ManualDispatch: true, HITLNext: true})
	pressProgressKey(s, "m")

	s.ResetRequestState()

	if got := s.PendingRequests(); got != (PendingRequests{}) {
		t.Errorf("PendingRequests() = %+v after reset, want zero", got)
	}
	if got := s.TakeRequestKeyEvents(); len(got) != 0 {
		t.Errorf("events = %+v after reset, want none: they belong to the ended session", got)
	}
}
