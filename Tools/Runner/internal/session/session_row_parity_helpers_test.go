package session_test

// Fixtures for the row-parity tests: one double that is the interaction port
// and the step-record observer at once and keeps a single ordered event list,
// a store wrapper that puts every successful Apply into that list, and a rig
// that wires them into a session.

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// rowEvent is one entry of the ordered event list.
type rowEvent struct {
	kind   string // "apply", "record" or "notice"
	seq    int    // apply: the applied step's Seq
	entry  domain.ExecutionLogEntry
	notice interaction.Notice
}

// rowRecorder is the interaction port and the step-record observer of a
// session under test; both feed one ordered event list.
type rowRecorder struct {
	noopInteraction
	mu     sync.Mutex
	events []rowEvent
}

func (r *rowRecorder) add(ev rowEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *rowRecorder) Notify(_ context.Context, n interaction.Notice) {
	r.add(rowEvent{kind: "notice", notice: n})
}

func (r *rowRecorder) StepRecorded(_ context.Context, entry domain.ExecutionLogEntry) {
	r.add(rowEvent{kind: "record", entry: entry})
}

func (r *rowRecorder) snapshot() []rowEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]rowEvent(nil), r.events...)
}

// recorded returns the announced entries in announcement order.
func (r *rowRecorder) recorded() []domain.ExecutionLogEntry {
	var out []domain.ExecutionLogEntry
	for _, ev := range r.snapshot() {
		if ev.kind == "record" {
			out = append(out, ev.entry)
		}
	}
	return out
}

// rowStore is a memStore that reports every successful Apply to the recorder.
type rowStore struct {
	*memStore
	rec *rowRecorder
}

func (s *rowStore) Apply(ctx context.Context, state domain.ArtifactState, step domain.CompletedStep) (domain.ArtifactState, error) {
	out, err := s.memStore.Apply(ctx, state, step)
	if err == nil {
		s.rec.add(rowEvent{kind: "apply", seq: step.Seq})
	}
	return out, err
}

// rowRig is a session over a fixture with the recorder wired as Interact
// and StepRecords.
type rowRig struct {
	ses      session.Session
	f        *harness.MockAdapter
	store    *rowStore
	rec      *rowRecorder
	orchPath string
}

// newRowRig builds the rig. tune may set further Deps (Routing, Approvals).
func newRowRig(t *testing.T, fixture string, agents []string, tune func(*session.Deps)) *rowRig {
	t.Helper()
	dir := t.TempDir()
	rig := &rowRig{f: harness.NewMockAdapter(), rec: &rowRecorder{}}
	rig.orchPath = copyOrchestratorFile(t, dir, fixture)
	for _, a := range agents {
		writeAgentFile(t, dir, a)
	}
	rig.store = &rowStore{memStore: &memStore{}, rec: rig.rec}
	deps := session.Deps{
		Harness: rig.f, Store: rig.store, Clock: fixedClock{t: epoch},
		Interact: rig.rec, StepRecords: rig.rec,
	}
	if tune != nil {
		tune(&deps)
	}
	rig.ses = session.New(deps)
	return rig
}

// start runs the session and ignores its outcome: the parity of the rows holds
// whatever way the run ends.
func (r *rowRig) start(cfg domain.RunConfig) {
	r.ses.Start(context.Background(), cfg) //nolint:errcheck
}

// requireParity asserts that the session announced exactly the entries the
// store recorded, in order and unchanged, and that each successful Apply was
// followed at once by its announcement.
func (r *rowRig) requireParity(t *testing.T, minEntries int) {
	t.Helper()
	r.requireParityAfter(t, 0, minEntries)
}

// requireParityAfter is requireParity for a resumed run: the first seeded
// entries of the log predate the session and are not announced.
func (r *rowRig) requireParityAfter(t *testing.T, seeded, minEntries int) {
	t.Helper()
	log := r.store.state.ExecutionLog[min(seeded, len(r.store.state.ExecutionLog)):]
	if len(log) < minEntries {
		t.Fatalf("scenario did not run as intended: want at least %d Execution Log entries, got %d: %+v",
			minEntries, len(log), log)
	}
	if got := r.rec.recorded(); !reflect.DeepEqual(got, log) {
		t.Errorf("announced entries =\n%+v\nwant exactly the recorded Execution Log\n%+v", got, log)
	}
	events := r.rec.snapshot()
	for i, ev := range events {
		if ev.kind != "apply" {
			continue
		}
		if i+1 >= len(events) || events[i+1].kind != "record" || events[i+1].entry.Seq != ev.seq {
			t.Errorf("successful Apply of seq %d is not immediately followed by its announcement; next event: %+v",
				ev.seq, events[min(i+1, len(events)-1)])
		}
	}
}

// requireEntry asserts the log holds an entry for the given agent instance.
func (r *rowRig) requireEntry(t *testing.T, agentInstance string) {
	t.Helper()
	for _, e := range r.rec.recorded() {
		if e.Agent == agentInstance {
			return
		}
	}
	t.Errorf("want an announced entry for %q, got %+v", agentInstance, r.rec.recorded())
}

// successEntry scripts a SUCCESS reply with the given message.
func successEntry(msg string) harness.ScriptedEntry {
	return statusEntry(domain.StatusSUCCESS, domain.ErrorNone, msg)
}

// firstDisapprovedThenApproved is the approval reader of a step whose first
// attempt fails the HITL gate and whose re-dispatch passes it.
func firstDisapprovedThenApproved() *switchingApprovalReader {
	return &switchingApprovalReader{firstApproval: domain.ApprovalFalse, restApproval: domain.ApprovalTrue}
}
