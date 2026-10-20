package session_test

// Fixtures for the consultation-announcement tests: one trace double that is
// the interaction port and the consultation observer at once and keeps a single
// ordered event list, wrappers that put every consultant call and harness
// invocation into that list, and a rig over the linear fixtures.

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// Event names of a consultTrace.
const (
	evStart      = "start"
	evEnd        = "end"
	evCall       = "call"        // Deps.Routing.ConsultRouting
	evPreCall    = "pre-call"    // Deps.PreConsult.PreConsult
	evManualCall = "manual-call" // Deps.Manual.ConsultRouting
	evNotice     = "notice"
	evInvokePfx  = "invoke:"
)

// consultTrace is the interaction port and the consultation observer of a
// session under test; both feed one ordered event list.
type consultTrace struct {
	noopInteraction
	mu         sync.Mutex
	events     []string
	activities []domain.ConsultationActivity
}

func (c *consultTrace) add(ev string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

func (c *consultTrace) ConsultationStarted(_ context.Context, a domain.ConsultationActivity) {
	c.mu.Lock()
	c.activities = append(c.activities, a)
	c.mu.Unlock()
	c.add(evStart)
}

func (c *consultTrace) ConsultationEnded(context.Context) { c.add(evEnd) }

func (c *consultTrace) Notify(context.Context, interaction.Notice) { c.add(evNotice) }

func (c *consultTrace) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.events)
}

func (c *consultTrace) started() []domain.ConsultationActivity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.activities)
}

func (c *consultTrace) count(ev string) int {
	n := 0
	for _, e := range c.snapshot() {
		if e == ev {
			n++
		}
	}
	return n
}

// without returns the events with the given names dropped.
func (c *consultTrace) without(names ...string) []string {
	var out []string
	for _, e := range c.snapshot() {
		if !slices.Contains(names, e) {
			out = append(out, e)
		}
	}
	return out
}

// tracedRouting records evt for every call, and the request, then delegates.
type tracedRouting struct {
	inner domain.RoutingConsultant
	tr    *consultTrace
	evt   string
	mu    sync.Mutex
	reqs  []domain.ConsultationRequest
}

func (r *tracedRouting) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	r.tr.add(r.evt)
	r.mu.Lock()
	r.reqs = append(r.reqs, req)
	r.mu.Unlock()
	return r.inner.ConsultRouting(ctx, req)
}

// tracedPreConsultant records evPreCall for every call, then delegates.
type tracedPreConsultant struct {
	inner domain.PreConsultant
	tr    *consultTrace
}

func (p *tracedPreConsultant) PreConsult(ctx context.Context, req domain.ConsultationRequest) (domain.PreConsultationAdvice, error) {
	p.tr.add(evPreCall)
	return p.inner.PreConsult(ctx, req)
}

// consultRig is a session over a fixture whose interaction port, consultation
// observer, harness and routing consultant all report to one trace.
type consultRig struct {
	ses      session.Session
	f        *harness.MockAdapter
	store    *memStore
	tr       *consultTrace
	routing  *tracedRouting
	consult  *scriptedRoutingConsultant
	orchPath string
}

// newConsultRig builds the rig over fixture. tune may adjust Deps; the rig's
// routing consultant is the scripted one wrapped by the trace.
func newConsultRig(t *testing.T, fixture string, agents []string, tune func(*session.Deps, *consultTrace)) *consultRig {
	t.Helper()
	dir := t.TempDir()
	r := &consultRig{
		f: harness.NewMockAdapter(), store: &memStore{}, tr: &consultTrace{},
		consult: &scriptedRoutingConsultant{},
	}
	r.routing = &tracedRouting{inner: r.consult, tr: r.tr, evt: evCall}
	r.orchPath = copyOrchestratorFile(t, dir, fixture)
	for _, a := range agents {
		writeAgentFile(t, dir, a)
	}
	deps := session.Deps{
		Harness: &beforeInvokeHarness{delegate: r.f, before: func(id string) { r.tr.add(evInvokePfx + id) }},
		Store:   r.store, Clock: fixedClock{t: epoch},
		Routing: r.routing, Interact: r.tr, Consultations: r.tr,
	}
	if tune != nil {
		tune(&deps, r.tr)
	}
	r.ses = session.New(deps)
	return r
}

func newLinearConsultRig(t *testing.T) *consultRig {
	t.Helper()
	return newConsultRig(t, "linear-orch.md", linearAgents, nil)
}

// requireBalanced asserts every announced start is followed by its end before
// the next start, that no end comes without a start, that no agent is invoked
// while a consultation is open, and that none is left open.
func requireBalanced(t *testing.T, tr *consultTrace) {
	t.Helper()
	open := false
	for i, e := range tr.snapshot() {
		switch {
		case e == evStart && open:
			t.Errorf("event %d: a consultation started while another was open: %v", i, tr.snapshot())
		case e == evStart:
			open = true
		case e == evEnd && !open:
			t.Errorf("event %d: a consultation ended without being open: %v", i, tr.snapshot())
		case e == evEnd:
			open = false
		case open && len(e) > len(evInvokePfx) && e[:len(evInvokePfx)] == evInvokePfx:
			t.Errorf("event %d: %s while a consultation was open: %v", i, e, tr.snapshot())
		}
	}
	if open {
		t.Errorf("a consultation was left open: %v", tr.snapshot())
	}
}

// requireBracketed asserts every call event is immediately preceded by a start
// and immediately followed by an end, and that there is at least wantMin of them.
func requireBracketed(t *testing.T, tr *consultTrace, call string, wantMin int) {
	t.Helper()
	events := tr.snapshot()
	n := 0
	for i, e := range events {
		if e != call {
			continue
		}
		n++
		if i == 0 || events[i-1] != evStart || i+1 >= len(events) || events[i+1] != evEnd {
			t.Errorf("%s at event %d is not bracketed by a start and an end: %v", call, i, events)
		}
	}
	if n < wantMin {
		t.Fatalf("scenario did not run as intended: want at least %d %s events, got %d: %v", wantMin, call, n, events)
	}
	if got := tr.count(evStart); got != n {
		t.Errorf("want one announced start per %s (%d), got %d: %v", call, n, got, events)
	}
}

// requireNotOpenAt asserts every event named evt happens while no consultation
// is open, and that there is at least wantMin of them.
func requireNotOpenAt(t *testing.T, tr *consultTrace, evt string, wantMin int) {
	t.Helper()
	open, n := false, 0
	for i, e := range tr.snapshot() {
		switch {
		case e == evStart:
			open = true
		case e == evEnd:
			open = false
		case e == evt:
			n++
			if open {
				t.Errorf("%s at event %d happened inside a consultation: %v", evt, i, tr.snapshot())
			}
		}
	}
	if n < wantMin {
		t.Fatalf("scenario did not run as intended: want at least %d %s events, got %d: %v", wantMin, evt, n, tr.snapshot())
	}
}

// requireKindsMatch asserts the announced activities describe the consultations
// the consultant received, in order: routing context, and the request's
// deviation kind or none.
func requireKindsMatch(t *testing.T, tr *consultTrace, reqs []domain.ConsultationRequest) {
	t.Helper()
	got := tr.started()
	if len(got) != len(reqs) || len(got) == 0 {
		t.Fatalf("want one announced activity per consultation (%d), got %d: %+v", len(reqs), len(got), got)
	}
	for i, req := range reqs {
		want := domain.ConsultationActivity{Context: req.Context}
		if req.Deviation != nil {
			want.Kind = req.Deviation.Kind
		}
		if got[i] != want {
			t.Errorf("consultation %d announced as %+v, want %+v", i, got[i], want)
		}
	}
}

// requireKinds asserts the announced kinds, in order.
func requireKinds(t *testing.T, tr *consultTrace, want ...domain.DeviationKind) {
	t.Helper()
	var got []domain.DeviationKind
	for _, a := range tr.started() {
		got = append(got, a.Kind)
	}
	if !slices.Equal(got, want) {
		t.Errorf("announced kinds = %v, want %v", got, want)
	}
}

func (r *consultRig) requests() []domain.ConsultationRequest {
	r.routing.mu.Lock()
	defer r.routing.mu.Unlock()
	return slices.Clone(r.routing.reqs)
}

func successAgentEntries(f *harness.MockAdapter, agent string, n int) {
	for i := 0; i < n; i++ {
		f.Queue(agent, successEntry(fmt.Sprintf("%s done %d", agent, i)))
	}
}
