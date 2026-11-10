package session_test

// Fixtures for the in-run manual-dispatch tests: a test double for the pending
// request, and a rig over the linear fixtures that records what the manual
// dialogue had been asked at the moment of every harness invocation.

import (
	"sync"
	"testing"

	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// requestFlag is the test double of the shared in-run request: one pending
// flag whose take reads and clears in a single operation.
type requestFlag struct {
	mu      sync.Mutex
	pending bool
	takes   int
}

func (r *requestFlag) request() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = true
}

func (r *requestFlag) cancel() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	was := r.pending
	r.pending = false
	return was
}

func (r *requestFlag) take() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.takes++
	was := r.pending
	r.pending = false
	return was
}

func (r *requestFlag) isPending() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending
}

// linearDialogue scripts a complete manual dialogue that picks the given row of
// the linear workflow with the given task text and accepts every default.
func linearDialogue(t *testing.T, row, task string) *manualUser {
	t.Helper()
	u := newManualUser(t).pickOne(domain.QuestionManualRow, row)
	u.say(domain.QuestionManualTask, task)
	u.acceptDefaultsPlus(domain.QuestionManualInputs)
	u.acceptDefaultsPlus(domain.QuestionManualOutputs)
	u.say(domain.QuestionManualConstraints, "")
	return u.pickOne(domain.QuestionManualHITL, domain.ManualHITLDefault)
}

// rowQuestions counts how often the manual dialogue was opened.
func (u *manualUser) rowQuestions() int {
	n := 0
	for _, id := range u.asked {
		if id == domain.QuestionManualRow {
			n++
		}
	}
	return n
}

// inRunRig is a session over a linear fixture with the in-run request wired in.
type inRunRig struct {
	ses        session.Session
	f          *harness.MockAdapter
	store      *memStore
	user       *manualUser
	flag       *requestFlag
	hitl       *requestFlag // the pending in-run HITL request
	logger     *sessionRecordingLogger
	consultant *scriptedRoutingConsultant
	orchPath   string

	// afterInvoke runs after every successful harness call, before the session
	// accepts its outcome: "while the agent runs".
	afterInvoke func(agentID string)
	// dialogueOpenAtInvoke records, per invocation in order, whether the manual
	// dialogue had already been opened when the harness was called.
	dialogueOpenAtInvoke []bool
	// beforeInvoke runs immediately before every harness call.
	beforeInvoke func(agentID string)
}

// inRunOptions selects the optional parts of an inRunRig.
type inRunOptions struct {
	fixture    string
	agents     []string
	consultant *scriptedRoutingConsultant // nil: Deps.Routing stays unset
	routing    domain.RoutingConsultant   // overrides consultant as Deps.Routing
	noManual   bool                       // leave Deps.Manual unset
	approvals  domain.ApprovalReader      // nil: Deps.Approvals stays unset

	consultations domain.ConsultationObserver // nil: Deps.Consultations stays unset
}

func newInRunRig(t *testing.T, user *manualUser, o inRunOptions) *inRunRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, o.fixture)
	for _, a := range o.agents {
		writeAgentFile(t, dir, a)
	}
	rig := &inRunRig{
		f: harness.NewMockAdapter(), store: &memStore{}, user: user, flag: &requestFlag{}, hitl: &requestFlag{},
		logger: &sessionRecordingLogger{}, consultant: o.consultant, orchPath: orchPath,
	}
	before := &beforeInvokeHarness{delegate: rig.f, before: func(id string) {
		rig.dialogueOpenAtInvoke = append(rig.dialogueOpenAtInvoke, rig.user.rowQuestions() > 0)
		if rig.beforeInvoke != nil {
			rig.beforeInvoke(id)
		}
	}}
	wrapped := &callbackHarness{delegate: before, onInvoke: func(id string) {
		if rig.afterInvoke != nil {
			rig.afterInvoke(id)
		}
	}}
	deps := session.Deps{
		Harness: wrapped, Store: rig.store, Clock: fixedClock{t: epoch},
		Interact: &noopInteraction{}, Debug: rig.logger,
		TakeManualDispatchRequest: rig.flag.take, TakeHITLRequest: rig.hitl.take,
	}
	deps.Approvals = o.approvals
	deps.Consultations = o.consultations
	if o.consultant != nil {
		deps.Routing = o.consultant
	}
	if o.routing != nil {
		deps.Routing = o.routing
	}
	if !o.noManual {
		deps.Manual = &deviation.ManualResolver{Interact: user}
	}
	rig.ses = session.New(deps)
	return rig
}

// requestOnceAt sets the request the first time agent finishes a harness call.
func (r *inRunRig) requestOnceAt(agent string) {
	done := false
	r.afterInvoke = func(id string) {
		if id == agent && !done {
			done = true
			r.flag.request()
		}
	}
}

// requestOnceBefore sets the request the first time agent is about to be
// invoked, for agents whose harness call fails and so never reports completion.
func (r *inRunRig) requestOnceBefore(agent string) {
	done := false
	r.beforeInvoke = func(id string) {
		if id == agent && !done {
			done = true
			r.flag.request()
		}
	}
}

func okEntry() harness.ScriptedEntry {
	return statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done")
}

func (r *inRunRig) queueOK(agent string, n int) {
	queueRepeated(r.f, agent, n, okEntry())
}

// appliedFields returns the fields of the first session.request.applied entry.
func (r *inRunRig) appliedFields(t *testing.T) map[string]string {
	t.Helper()
	out := r.findApplied()
	if out == nil {
		t.Fatalf("want a %q debug entry, got events %v", domain.EventSessionRequestApplied, r.logger.allEvents())
	}
	return out
}

// findApplied snapshots the first applied entry under the logger lock and
// releases it before returning, so callers may read the logger again.
func (r *inRunRig) findApplied() map[string]string {
	r.logger.mu.Lock()
	defer r.logger.mu.Unlock()
	for _, e := range r.logger.entries {
		if e.Event != domain.EventSessionRequestApplied {
			continue
		}
		out := map[string]string{}
		for _, f := range e.Fields {
			out[f.Key] = f.Value
		}
		return out
	}
	return nil
}

func (r *inRunRig) appliedCount() int {
	n := 0
	for _, e := range r.logger.allEvents() {
		if e == domain.EventSessionRequestApplied {
			n++
		}
	}
	return n
}
