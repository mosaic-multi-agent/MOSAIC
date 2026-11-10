package session_test

// Shared doubles and helpers for the force-stop session tests: a force-stop
// window double, a harness that blocks an invocation until its context is
// cancelled, a post-kill choice interaction and row selectors.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// forceWindow is a domain.ForceStopWatcher test double. Request cancels the
// context of the open window, if any, like the production signal does.
type forceWindow struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	forced bool
	opened int
}

func (w *forceWindow) Watch(ctx context.Context) (context.Context, func() bool) {
	wctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancel, w.forced = cancel, false
	w.opened++
	w.mu.Unlock()
	return wctx, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.cancel = nil
		cancel()
		return w.forced
	}
}

// Request force-stops the invocation in the open window and reports whether a
// window was open.
func (w *forceWindow) Request() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel == nil {
		return false
	}
	w.forced = true
	w.cancel()
	return true
}

// blockingHarness delegates to a MockAdapter, except that the nth invocation
// of agent (1-based) raises the force-stop request itself (when win is set)
// and then blocks until its context is cancelled, returning the context error
// like a killed subprocess would. onBlock runs just before the block starts.
type blockingHarness struct {
	delegate domain.HarnessAdapter
	win      *forceWindow
	agent    string
	nth      int
	onBlock  func()

	mu    sync.Mutex
	calls map[string]int
}

func newBlockingHarness(f domain.HarnessAdapter, win *forceWindow, agent string, nth int) *blockingHarness {
	return &blockingHarness{delegate: f, win: win, agent: agent, nth: nth, calls: map[string]int{}}
}

func (h *blockingHarness) Invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	h.mu.Lock()
	h.calls[agent.Identifier]++
	n := h.calls[agent.Identifier]
	h.mu.Unlock()
	if agent.Identifier != h.agent || n != h.nth {
		return h.delegate.Invoke(ctx, agent, request)
	}
	if h.onBlock != nil {
		h.onBlock()
	}
	if h.win != nil {
		h.win.Request()
	}
	select {
	case <-ctx.Done():
		return domain.ProtocolResponse{}, fmt.Errorf("harness: invocation killed: %w", ctx.Err())
	case <-time.After(1 * time.Second):
		return domain.ProtocolResponse{}, errors.New("test: invocation context was never cancelled")
	}
}

// choiceUser answers the post-kill question with option and every other
// question as the no-op double does. It records the questions asked.
type choiceUser struct {
	noopInteraction
	option string
	status interaction.AnswerStatus // zero value means Answered
	// stopAfterFirst answers the post-kill question with stop from the second ask on.
	stopAfterFirst bool

	mu       sync.Mutex
	asked    []interaction.QuestionID
	prompted []interaction.ChoiceQuestion
}

func (u *choiceUser) SelectOne(_ context.Context, q interaction.ChoiceQuestion) (interaction.ChoiceAnswer, error) {
	u.mu.Lock()
	u.asked = append(u.asked, q.ID)
	u.prompted = append(u.prompted, q)
	u.mu.Unlock()
	if q.ID != domain.QuestionForceStopNext {
		return interaction.ChoiceAnswer{Status: interaction.Answered}, nil
	}
	status := u.status
	if status == "" {
		status = interaction.Answered
	}
	option := u.option
	if u.stopAfterFirst && u.postKillAsks() > 1 {
		option = domain.ForceStopNextStop
	}
	return interaction.ChoiceAnswer{Status: status, OptionID: option}, nil
}

// postKillAsks counts how often the post-kill question was asked.
func (u *choiceUser) postKillAsks() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for _, id := range u.asked {
		if id == domain.QuestionForceStopNext {
			n++
		}
	}
	return n
}

// forceStoppedSteps returns the recorded steps flagged as force-stopped.
func forceStoppedSteps(store *memStore) []domain.CompletedStep {
	var out []domain.CompletedStep
	for _, s := range store.Applied {
		if s.ForceStopped {
			out = append(out, s)
		}
	}
	return out
}

// countE501Steps counts every recorded BLOCKED/E501 step, whatever its agent.
func countE501Steps(store *memStore) int {
	n := 0
	for _, s := range store.Applied {
		if s.Status == domain.StatusBLOCKED && s.ErrorCode == domain.ErrorTOOL_UNAVAILABLE {
			n++
		}
	}
	return n
}

// requireForceStoppedRow asserts the single force-stopped step is the
// unaccepted attempt of agent: PARTIALLY_DONE with the force-stop summary.
func requireForceStoppedRow(t testing.TB, store *memStore, agentPrefix string, infra bool) domain.CompletedStep {
	t.Helper()
	rows := forceStoppedSteps(store)
	if len(rows) != 1 {
		t.Fatalf("want exactly one force-stopped row, got %d (applied: %+v)", len(rows), store.Applied)
	}
	row := rows[0]
	if !strings.HasPrefix(row.AgentInstance, agentPrefix+"#") {
		t.Errorf("want the row of %s, got %q", agentPrefix, row.AgentInstance)
	}
	if row.Status != domain.StatusPARTIALLY_DONE || row.Summary != domain.ForceStoppedSummary {
		t.Errorf("want PARTIALLY_DONE with the force-stop summary, got %s %q", row.Status, row.Summary)
	}
	if row.IsInfrastructure != infra {
		t.Errorf("want IsInfrastructure=%v, got %v", infra, row.IsInfrastructure)
	}
	return row
}
