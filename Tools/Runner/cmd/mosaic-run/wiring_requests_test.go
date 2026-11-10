package main

// wiring_requests_test.go verifies that the in-run request object is shared:
// the interactive wiring hands the same instance to the TUI and to every
// session its factory builds, and the non-interactive wiring never receives it.
//
// A second instance anywhere in the chain recreates the orphan-object defect:
// the user's key press would set a request no session ever reads.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

// requestsWiringInput returns a complete wiring input carrying a fresh shared
// request object.
func requestsWiringInput(t *testing.T) (interactiveWiringInput, *session.InRunRequests) {
	t.Helper()
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	reqs := session.NewInRunRequests()
	in.Requests = reqs
	return in, reqs
}

func TestInteractiveWiring_Requests_TheTUIReceivesTheSameInstance(t *testing.T) {
	in, reqs := requestsWiringInput(t)

	w := buildInteractiveWiring(in)

	if w.Options.InRunRequests != reqs {
		t.Errorf("Options.InRunRequests = %p, want the instance passed in (%p): the TUI would set requests the session never reads",
			w.Options.InRunRequests, reqs)
	}
}

func TestInteractiveWiring_Requests_TheSessionTakesWhatTheTUIObjectHolds(t *testing.T) {
	cases := []struct {
		name string
		kind domain.InRunRequest
		take func(session.Deps) func() bool
	}{
		{"manual dispatch", domain.InRunRequestManualDispatch, func(d session.Deps) func() bool { return d.TakeManualDispatchRequest }},
		{"hitl", domain.InRunRequestHITLNext, func(d session.Deps) func() bool { return d.TakeHITLRequest }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, reqs := requestsWiringInput(t)
			w := buildInteractiveWiring(in)
			deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0021"), true, "", runconfig.ConfigSelection{})
			take := tc.take(deps)
			if take == nil {
				t.Fatal("the Deps take function is nil")
			}

			reqs.Request(tc.kind)
			first := take()
			second := take()

			if !first {
				t.Error("take() = false for a pending request: the session reads another object")
			}
			if second {
				t.Error("take() = true twice: a request applied more than once")
			}
		})
	}
}

func TestInteractiveWiring_Requests_TakeFunctionsDoNotCrossKinds(t *testing.T) {
	in, reqs := requestsWiringInput(t)
	deps := buildInteractiveWiring(in).NewDeps(testRunFolder(t, "20260101T000000Z-0022"), true, "", runconfig.ConfigSelection{})

	reqs.Request(domain.InRunRequestHITLNext)
	if deps.TakeManualDispatchRequest == nil || deps.TakeManualDispatchRequest() {
		t.Error("the manual-dispatch take consumed (or lacks) the HITL request")
	}
	if deps.TakeHITLRequest == nil || !deps.TakeHITLRequest() {
		t.Error("the HITL take did not see the HITL request")
	}
}

func TestInteractiveWiring_Requests_EverySessionBuiltSharesTheOneObject(t *testing.T) {
	in, reqs := requestsWiringInput(t)
	w := buildInteractiveWiring(in)
	first := w.NewDeps(testRunFolder(t, "20260101T000000Z-0023"), true, "", runconfig.ConfigSelection{})
	second := w.NewDeps(testRunFolder(t, "20260101T000000Z-0024"), false, "", runconfig.ConfigSelection{Harness: "claude-code"})
	if first.TakeManualDispatchRequest == nil || second.TakeManualDispatchRequest == nil {
		t.Fatal("a built Deps lacks the take function")
	}

	reqs.Request(domain.InRunRequestManualDispatch)
	tookByFirst := first.TakeManualDispatchRequest()
	reqs.Request(domain.InRunRequestManualDispatch)
	tookBySecond := second.TakeManualDispatchRequest()

	if !tookByFirst || !tookBySecond {
		t.Errorf("take results = %v, %v, want both true: a rebuilt session must read the same object", tookByFirst, tookBySecond)
	}
}

func TestInteractiveWiring_Requests_TheSessionObservesAPressMadeBeforeItWasBuilt(t *testing.T) {
	in, reqs := requestsWiringInput(t)
	w := buildInteractiveWiring(in)
	reqs.Request(domain.InRunRequestHITLNext)

	deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0025"), true, "", runconfig.ConfigSelection{})

	if deps.TakeHITLRequest == nil || !deps.TakeHITLRequest() {
		t.Error("a rebuilt session lost the state of the shared object")
	}
}

// TestBuildDeps_CLI_DoesNotWireInRunRequests pins that the non-interactive
// wiring is unchanged: it never receives the shared object.
func TestBuildDeps_CLI_DoesNotWireInRunRequests(t *testing.T) {
	deps := buildDepsFromTransports(&fakeRawInvoker{}, &mainTestNoopInteraction{}, nil, nil)

	if deps.TakeManualDispatchRequest != nil {
		t.Error("buildDeps wired TakeManualDispatchRequest: the CLI must not offer in-run requests")
	}
	if deps.TakeHITLRequest != nil {
		t.Error("buildDeps wired TakeHITLRequest: the CLI must not offer in-run requests")
	}
}
