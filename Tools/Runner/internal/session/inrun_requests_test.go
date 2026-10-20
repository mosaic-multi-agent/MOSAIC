package session_test

// inrun_requests_test.go verifies session.InRunRequests: the one-shot request
// flags the TUI sets and cancels while the session takes them.
//
// The load-bearing property is accounting. Every request that becomes pending
// ends in exactly one of: taken by the session (applied), cancelled by the user,
// or reset (discarded). Never two, never none. The concurrent tests assert that
// invariant and are meant to run under go test -race where cgo is available;
// they also assert the invariant itself, so plain go test is still meaningful.

import (
	"sync"
	"sync/atomic"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

var allKinds = []domain.InRunRequest{
	domain.InRunRequestManualDispatch,
	domain.InRunRequestHITLNext,
}

func TestInRunRequests_NothingPendingInitially(t *testing.T) {
	r := session.NewInRunRequests()
	for _, k := range allKinds {
		if r.Pending(k) {
			t.Errorf("Pending(%q) = true on a new instance, want false", k)
		}
		if r.Take(k) {
			t.Errorf("Take(%q) = true on a new instance, want false", k)
		}
	}
}

func TestInRunRequests_Request_ReportsStateChange(t *testing.T) {
	for _, k := range allKinds {
		t.Run(string(k), func(t *testing.T) {
			r := session.NewInRunRequests()

			first := r.Request(k)
			second := r.Request(k)

			if !first {
				t.Error("first Request = false, want true (state changed)")
			}
			if second {
				t.Error("second Request = true, want false (already pending)")
			}
			if !r.Pending(k) {
				t.Error("Pending = false after Request, want true")
			}
		})
	}
}

func TestInRunRequests_Cancel_ClearsPendingRequest(t *testing.T) {
	for _, k := range allKinds {
		t.Run(string(k), func(t *testing.T) {
			r := session.NewInRunRequests()
			r.Request(k)

			cancelled := r.Cancel(k)
			again := r.Cancel(k)

			if !cancelled {
				t.Error("Cancel of a pending request = false, want true")
			}
			if again {
				t.Error("second Cancel = true, want false (nothing pending)")
			}
			if r.Pending(k) {
				t.Error("Pending = true after Cancel, want false")
			}
			if r.Take(k) {
				t.Error("Take = true after Cancel, want false: a cancelled request must not be applied")
			}
		})
	}
}

func TestInRunRequests_Take_ConsumesExactlyOnce(t *testing.T) {
	for _, k := range allKinds {
		t.Run(string(k), func(t *testing.T) {
			r := session.NewInRunRequests()
			r.Request(k)

			first := r.Take(k)
			second := r.Take(k)

			if !first {
				t.Error("first Take = false, want true")
			}
			if second {
				t.Error("second Take = true, want false: a request applies once")
			}
			if r.Pending(k) {
				t.Error("Pending = true after Take, want false")
			}
			if r.Cancel(k) {
				t.Error("Cancel = true after Take, want false (the cancel lost)")
			}
		})
	}
}

func TestInRunRequests_Kinds_AreIndependent(t *testing.T) {
	r := session.NewInRunRequests()
	r.Request(domain.InRunRequestManualDispatch)

	if r.Pending(domain.InRunRequestHITLNext) {
		t.Error("requesting manual dispatch made HITL pending")
	}
	if r.Take(domain.InRunRequestHITLNext) {
		t.Error("Take(HITL) = true with only manual dispatch requested")
	}
	if !r.Pending(domain.InRunRequestManualDispatch) {
		t.Error("taking HITL cleared the manual-dispatch request")
	}
}

func TestInRunRequests_UnknownKind_IsIgnored(t *testing.T) {
	r := session.NewInRunRequests()
	bogus := domain.InRunRequest("bogus")

	if r.Request(bogus) {
		t.Error("Request(unknown) = true, want false")
	}
	if r.Pending(bogus) || r.Take(bogus) || r.Cancel(bogus) {
		t.Error("an unknown kind became pending")
	}
}

func TestInRunRequests_Reset_ReturnsPendingKindsInOrderAndClears(t *testing.T) {
	r := session.NewInRunRequests()
	r.Request(domain.InRunRequestHITLNext)
	r.Request(domain.InRunRequestManualDispatch)

	got := r.Reset()

	want := []domain.InRunRequest{domain.InRunRequestManualDispatch, domain.InRunRequestHITLNext}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Reset() = %v, want %v (manual dispatch first, then HITL)", got, want)
	}
	for _, k := range allKinds {
		if r.Pending(k) || r.Take(k) {
			t.Errorf("%q still pending or takeable after Reset", k)
		}
	}
}

func TestInRunRequests_Reset_OnlyReportsKindsThatWerePending(t *testing.T) {
	r := session.NewInRunRequests()
	r.Request(domain.InRunRequestHITLNext)

	got := r.Reset()

	if len(got) != 1 || got[0] != domain.InRunRequestHITLNext {
		t.Errorf("Reset() = %v, want only [hitl_next]", got)
	}
}

func TestInRunRequests_Reset_NothingPending_ReturnsNil(t *testing.T) {
	r := session.NewInRunRequests()
	if got := r.Reset(); got != nil {
		t.Errorf("Reset() = %v with nothing pending, want nil", got)
	}
}

// TestInRunRequests_ConcurrentTakeAndCancel_ExactlyOneWins races a session-side
// Take against a TUI-side Cancel over one pending request, many times. Exactly
// one of them may win each round.
func TestInRunRequests_ConcurrentTakeAndCancel_ExactlyOneWins(t *testing.T) {
	const rounds = 2000
	r := session.NewInRunRequests()
	k := domain.InRunRequestManualDispatch

	for i := 0; i < rounds; i++ {
		if !r.Request(k) {
			t.Fatalf("round %d: Request = false on a cleared instance", i)
		}
		var took, cancelled bool
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; took = r.Take(k) }()
		go func() { defer wg.Done(); <-start; cancelled = r.Cancel(k) }()
		close(start)
		wg.Wait()

		if took == cancelled {
			t.Fatalf("round %d: Take=%v Cancel=%v, want exactly one winner (applied twice or lost)", i, took, cancelled)
		}
		if r.Pending(k) {
			t.Fatalf("round %d: request still pending after the race", i)
		}
	}
}

// TestInRunRequests_ConcurrentSetCancelAgainstRead_AccountsForEveryRequest runs
// a TUI-style goroutine that toggles requests against a session-style goroutine
// that keeps taking, then reconciles: every request that became pending was
// applied once, cancelled once, or is still pending at the end. Nothing is lost
// and nothing is applied twice.
func TestInRunRequests_ConcurrentSetCancelAgainstRead_AccountsForEveryRequest(t *testing.T) {
	const toggles = 20000
	for _, k := range allKinds {
		t.Run(string(k), func(t *testing.T) {
			r := session.NewInRunRequests()
			var set, cancelled, taken atomic.Int64
			done := make(chan struct{})

			var readers sync.WaitGroup
			readers.Add(1)
			go func() { // session goroutine
				defer readers.Done()
				for {
					if r.Take(k) {
						taken.Add(1)
					}
					select {
					case <-done:
						return
					default:
					}
				}
			}()

			for i := 0; i < toggles; i++ { // TUI goroutine
				if i%3 == 0 {
					if r.Cancel(k) {
						cancelled.Add(1)
					}
				} else if r.Request(k) {
					set.Add(1)
				}
			}
			close(done)
			readers.Wait()

			residual := int64(0)
			if r.Take(k) {
				residual = 1
			}
			if got := taken.Load() + cancelled.Load() + residual; got != set.Load() {
				t.Fatalf("set=%d but taken=%d + cancelled=%d + still pending=%d = %d: a request was applied twice or silently lost",
					set.Load(), taken.Load(), cancelled.Load(), residual, got)
			}
			if set.Load() == 0 || (taken.Load() == 0 && cancelled.Load() == 0) {
				t.Fatalf("degenerate run: set=%d taken=%d cancelled=%d", set.Load(), taken.Load(), cancelled.Load())
			}
		})
	}
}

// TestInRunRequests_ConcurrentResetAgainstTake_ExactlyOneAccountsForTheRequest
// covers a restart discarding at the instant the session reads.
func TestInRunRequests_ConcurrentResetAgainstTake_ExactlyOneAccountsForTheRequest(t *testing.T) {
	const rounds = 2000
	r := session.NewInRunRequests()
	k := domain.InRunRequestHITLNext

	for i := 0; i < rounds; i++ {
		r.Request(k)
		var took bool
		var reset []domain.InRunRequest
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; took = r.Take(k) }()
		go func() { defer wg.Done(); <-start; reset = r.Reset() }()
		close(start)
		wg.Wait()

		discarded := len(reset) == 1
		if took == discarded {
			t.Fatalf("round %d: took=%v reset=%v, want exactly one to account for the request", i, took, reset)
		}
	}
}
