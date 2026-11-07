package session_test

// Shared doubles and the session rig for the consultation force-stop tests: a
// routing consultant and a pre-consultant that block until their context is
// cancelled (raising the force-stop request themselves, like a user pressing
// the control while the orchestrator invocation runs).

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// killedByContext is what a killed orchestrator invocation reports. A
// consultant may wrap the cancellation as a consultation error or return it
// plain; the session must classify by the force-stop request, not by this value.
func killedByContext(plain bool, cause error) error {
	if plain {
		return cause
	}
	return &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "invocation killed: " + cause.Error()}
}

// blockUntilKilled raises the request (when win is set), runs onBlock, and
// waits for ctx to be cancelled.
func blockUntilKilled(ctx context.Context, win *forceWindow, onBlock func(), plain bool) error {
	if onBlock != nil {
		onBlock()
	}
	if win != nil {
		win.Request()
	}
	select {
	case <-ctx.Done():
		return killedByContext(plain, ctx.Err())
	case <-time.After(1 * time.Second):
		return errors.New("test: consultation context was never cancelled")
	}
}

// forceBlockingConsultant answers every call from inner except the nth (1-based)
// one, which blocks until its context is cancelled.
type forceBlockingConsultant struct {
	inner   *scriptedRoutingConsultant
	win     *forceWindow // nil: no force-stop request is raised
	nth     int
	plain   bool   // return the bare context error instead of a consultation error
	onBlock func() // runs just before the block starts

	mu      sync.Mutex
	calls   int
	blocked []domain.ConsultationRequest
}

func (c *forceBlockingConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.mu.Lock()
	c.calls++
	n := c.calls
	if n == c.nth {
		c.blocked = append(c.blocked, req)
	}
	c.mu.Unlock()
	if n != c.nth {
		return c.inner.ConsultRouting(ctx, req)
	}
	return domain.RoutingInstruction{}, blockUntilKilled(ctx, c.win, c.onBlock, c.plain)
}

// blockedRequest returns the request of the blocked consultation.
func (c *forceBlockingConsultant) blockedRequest(t testing.TB) domain.ConsultationRequest {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.blocked) != 1 {
		t.Fatalf("want exactly one blocked consultation, got %d", len(c.blocked))
	}
	return c.blocked[0]
}

// blockingPreConsultant blocks its single call until the context is cancelled.
type blockingPreConsultant struct {
	win     *forceWindow
	plain   bool
	onBlock func()

	mu    sync.Mutex
	calls int
}

func (p *blockingPreConsultant) PreConsult(ctx context.Context, _ domain.ConsultationRequest) (domain.PreConsultationAdvice, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return domain.PreConsultationAdvice{}, blockUntilKilled(ctx, p.win, p.onBlock, p.plain)
}

// consultForceRig is an orchestrated run over the linear fixture whose nth
// routing consultation is force-stopped. The consultations before it are
// answered from inner; Manual is the manual routing dialogue.
type consultForceRig struct {
	ses        session.Session
	f          *harness.MockAdapter
	store      *memStore
	win        *forceWindow
	user       *choiceUser
	manual     *scriptedRoutingConsultant
	inner      *scriptedRoutingConsultant
	consultant *forceBlockingConsultant
	trace      *consultTrace
	logger     *sessionRecordingLogger
	cfg        domain.RunConfig
}

// newConsultForceRig builds the rig; the user answers the post-kill question
// with choice. mutate may change the dependencies before the session is built.
func newConsultForceRig(t *testing.T, choice string, nth int, mutate ...func(*session.Deps)) *consultForceRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	r := &consultForceRig{
		f: harness.NewMockAdapter(), store: &memStore{}, win: &forceWindow{},
		user: &choiceUser{option: choice}, manual: &scriptedRoutingConsultant{}, inner: &scriptedRoutingConsultant{},
		trace: &consultTrace{}, logger: &sessionRecordingLogger{}, cfg: baseOrchestratedConfig(orchPath),
	}
	r.consultant = &forceBlockingConsultant{inner: r.inner, win: r.win, nth: nth}
	deps := session.Deps{
		Harness: r.f, Store: r.store, Clock: fixedClock{t: epoch}, Interact: r.user,
		Routing: r.consultant, Manual: r.manual, ForceStop: r.win, Consultations: r.trace, Debug: r.logger,
	}
	for _, m := range mutate {
		m(&deps)
	}
	r.ses = session.New(deps)
	return r
}

// queueFirstDispatch scripts the consultation before the blocked one: it
// dispatches agent-a, which succeeds with message.
func (r *consultForceRig) queueFirstDispatch(message string) {
	r.inner.queueDispatch("agent-a", "start", 0)
	r.f.Queue("agent-a", successEntry(message))
}

// requireNothingRecordedBeyond asserts the run's artifact holds exactly rows
// accepted rows: no extra row, no sequence advance, current_state as the last
// accepted row left it.
func requireNothingRecordedBeyond(t *testing.T, store *memStore, rows int, lastAgent string) {
	t.Helper()
	if len(store.Applied) != rows || len(store.state.ExecutionLog) != rows {
		t.Errorf("want %d recorded rows, got %d applied and %d logged", rows, len(store.Applied), len(store.state.ExecutionLog))
	}
	if store.state.GlobalSequence != rows {
		t.Errorf("want global_sequence %d, got %d", rows, store.state.GlobalSequence)
	}
	if store.state.CurrentState.LastAgent != lastAgent {
		t.Errorf("want current_state last agent %q, got %q", lastAgent, store.state.CurrentState.LastAgent)
	}
	if n := len(forceStoppedSteps(store)); n != 0 {
		t.Errorf("want no force-stopped row for a consultation, got %d", n)
	}
}
