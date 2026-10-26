package session_test

// Shared doubles and the rig for the standing-guidance tests: a guidance store
// double keyed by run folder, an Interaction double that scripts the review
// answers (and models the CLI by answering Unsupported), and a rig that wires
// them, a pre-consultant, a Runner-log recorder and the live guidance holder
// into a block rig.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const (
	savedTask        = "saved-task-sentinel-k3v"
	savedConstraints = "saved-constraints-sentinel-k3v"
	adviceTask       = "advice-task-sentinel-q8w"
	adviceConstraint = "advice-constraints-sentinel-q8w"
)

// ---- memGuidanceStore ----

// guidanceSave is one call to memGuidanceStore.Save.
type guidanceSave struct {
	Folder string
	G      domain.Guidance
}

// memGuidanceStore is an in-memory domain.GuidanceStore keyed by run folder.
type memGuidanceStore struct {
	mu      sync.Mutex
	files   map[string]domain.Guidance
	loads   []string
	saves   []guidanceSave
	loadErr error
	saveErr error
}

func (m *memGuidanceStore) Load(folder string) (domain.Guidance, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loads = append(m.loads, folder)
	if m.loadErr != nil {
		return domain.Guidance{}, false, m.loadErr
	}
	g, ok := m.files[folder]
	return g, ok, nil
}

func (m *memGuidanceStore) Save(folder string, g domain.Guidance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.files == nil {
		m.files = map[string]domain.Guidance{}
	}
	m.files[folder] = g
	m.saves = append(m.saves, guidanceSave{Folder: folder, G: g})
	return nil
}

// preload stores g as the saved guidance file of folder.
func (m *memGuidanceStore) preload(folder string, g domain.Guidance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = map[string]domain.Guidance{}
	}
	m.files[folder] = g
}

func (m *memGuidanceStore) savedGuidance() []domain.Guidance {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Guidance, len(m.saves))
	for i, s := range m.saves {
		out[i] = s.G
	}
	return out
}

func (m *memGuidanceStore) fileOf(folder string) (domain.Guidance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.files[folder]
	return g, ok
}

// ---- guidanceAsk ----

// guidanceAsk is an Interaction double for the guidance questions. Review
// answers and text answers are consumed in order; an exhausted review queue
// answers accept. When unsupported is set, every question is answered
// Unsupported, as the CLI does.
type guidanceAsk struct {
	mu          sync.Mutex
	unsupported bool
	review      []interaction.ChoiceAnswer
	texts       map[interaction.QuestionID][]interaction.TextAnswer

	reviews []interaction.ChoiceQuestion
	asks    []interaction.TextQuestion

	// dispatches reports the dispatches made so far; dispatchesAtReview records
	// its value at each review question.
	dispatches         func() int
	dispatchesAtReview []int
}

func answerReview(optionID string) interaction.ChoiceAnswer {
	return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: optionID}
}

func answerText(text string) interaction.TextAnswer {
	return interaction.TextAnswer{Status: interaction.Answered, Text: text}
}

func (a *guidanceAsk) SelectOne(_ context.Context, q interaction.ChoiceQuestion) (interaction.ChoiceAnswer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if q.ID != domain.QuestionGuidanceReview {
		return interaction.ChoiceAnswer{Status: interaction.Answered}, nil
	}
	a.reviews = append(a.reviews, q)
	if a.dispatches != nil {
		a.dispatchesAtReview = append(a.dispatchesAtReview, a.dispatches())
	}
	if a.unsupported {
		return interaction.ChoiceAnswer{Status: interaction.Unsupported}, nil
	}
	if len(a.review) == 0 {
		return answerReview(domain.GuidanceOptionAccept), nil
	}
	ans := a.review[0]
	a.review = a.review[1:]
	return ans, nil
}

func (a *guidanceAsk) AskText(_ context.Context, q interaction.TextQuestion) (interaction.TextAnswer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if q.ID != domain.QuestionGuidanceTask && q.ID != domain.QuestionGuidanceConstraints {
		return interaction.TextAnswer{Status: interaction.Answered}, nil
	}
	a.asks = append(a.asks, q)
	queue := a.texts[q.ID]
	if a.unsupported || len(queue) == 0 {
		return interaction.TextAnswer{Status: interaction.Unsupported}, nil
	}
	ans := queue[0]
	a.texts[q.ID] = queue[1:]
	return ans, nil
}

func (a *guidanceAsk) SelectMany(context.Context, interaction.ChoiceQuestion) (interaction.MultiChoiceAnswer, error) {
	return interaction.MultiChoiceAnswer{Status: interaction.Answered}, nil
}

func (a *guidanceAsk) Confirm(context.Context, interaction.Question) (interaction.ConfirmAnswer, error) {
	return interaction.ConfirmAnswer{Status: interaction.Unsupported}, nil
}

func (a *guidanceAsk) Notify(context.Context, interaction.Notice)          {}
func (a *guidanceAsk) Progress(context.Context, interaction.ProgressEvent) {}

// queueText scripts the answers of one guidance text question.
func (a *guidanceAsk) queueText(id interaction.QuestionID, answers ...interaction.TextAnswer) {
	if a.texts == nil {
		a.texts = map[interaction.QuestionID][]interaction.TextAnswer{}
	}
	a.texts[id] = append(a.texts[id], answers...)
}

func optionIDs(q interaction.ChoiceQuestion) []string {
	var ids []string
	for _, o := range q.Options {
		ids = append(ids, o.ID)
	}
	return ids
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- guidanceRig ----

// guidanceRig is a blockRig whose session is rebuilt with the guidance doubles.
type guidanceRig struct {
	*blockRig
	files *memGuidanceStore
	ask   *guidanceAsk
	pc    *scriptedPreConsultant
	log   *sessionRecordingLogger
	live  *session.StandingGuidance

	// noStore wires no GuidanceStore into the session.
	noStore bool

	// onInvoke, when set, runs before each dispatch reaches the harness.
	onInvoke func(agent string)
}

// newGuidanceRig builds the rig for fixture with an agent definition per agent.
func newGuidanceRig(t *testing.T, fixture string, agents ...string) *guidanceRig {
	t.Helper()
	g := &guidanceRig{
		blockRig: newBlockRig(t, fixture, nil, agents...),
		files:    &memGuidanceStore{},
		ask:      &guidanceAsk{},
		pc:       &scriptedPreConsultant{},
		log:      &sessionRecordingLogger{},
		live:     session.NewStandingGuidance(),
	}
	g.ask.dispatches = func() int { return len(g.adapter.Invocations()) }
	g.rebuild()
	return g
}

// newLinearGuidanceRig is the rig of the two-agent linear workflow.
func newLinearGuidanceRig(t *testing.T) *guidanceRig {
	t.Helper()
	return newGuidanceRig(t, "linear-orch.md", "agent-a", "agent-b")
}

// rebuild replaces the session so that it carries the rig's current doubles.
func (g *guidanceRig) rebuild() {
	var h domain.HarnessAdapter = g.adapter
	if g.onInvoke != nil {
		h = &beforeInvokeHarness{delegate: g.adapter, before: g.onInvoke}
	}
	var gs domain.GuidanceStore = g.files
	if g.noStore {
		gs = nil
	}
	g.ses = session.New(session.Deps{
		Harness:       h,
		Store:         g.blockRig.store,
		Routing:       g.consult,
		Clock:         fixedClock{t: epoch},
		Interact:      g.ask,
		PreConsult:    g.pc,
		Debug:         g.log,
		GuidanceStore: gs,
		Guidance:      g.live,
	})
}

// autoConfig is the configuration of a new auto run with pre-consultation on.
func (g *guidanceRig) autoConfig(workflow string) domain.RunConfig {
	cfg := g.config(workflow, domain.ExecutionModeAuto)
	cfg.PreConsultation = true
	return cfg
}

// linearResume records the linear run as stopped after agent-a and returns the
// resume configuration, with pre-consultation on.
func (g *guidanceRig) linearResume() domain.RunConfig {
	g.resumeAfterFirstRow("linear", domain.ExecutionModeAuto, "agent-a", "PLANNING")
	g.blockRig.store.state.PreConsultation = true
	cfg := g.resumeConfig("linear", domain.ExecutionModeAuto)
	cfg.PreConsultation = true
	return cfg
}

// queueLinear queues SUCCESS replies for the whole linear workflow.
func (g *guidanceRig) queueLinear() {
	g.queueOK("agent-a")
	g.queueOK("agent-b")
}

// guidanceChanges returns the change field of every session.guidance.changed
// event, in order.
func (g *guidanceRig) guidanceChanges() []string {
	var out []string
	for _, e := range g.guidanceEvents() {
		for _, f := range e.Fields {
			if f.Key == "change" {
				out = append(out, f.Value)
			}
		}
	}
	return out
}

// guidanceEvents returns every session.guidance.changed event, in order.
func (g *guidanceRig) guidanceEvents() []sessionLogEntry {
	g.log.mu.Lock()
	defer g.log.mu.Unlock()
	var out []sessionLogEntry
	for _, e := range g.log.entries {
		if e.Event == domain.EventSessionGuidanceChanged {
			out = append(out, e)
		}
	}
	return out
}

// requestsFor returns the requests of agent in dispatch order.
func (g *guidanceRig) requestsFor(agent string) []domain.ProtocolRequest {
	return g.requestsOf(agent)
}

// carriesGuidance reports whether req carries the task_description and the
// constraints text of want.
func carriesGuidance(req domain.ProtocolRequest, want domain.Guidance) bool {
	return strings.Contains(req.TaskDescription, want.TaskDescription) &&
		strings.Contains(req.Constraints, want.Constraints)
}

// carriesNoGuidance reports whether req carries neither field of g.
func carriesNoGuidance(req domain.ProtocolRequest, g domain.Guidance) bool {
	return !strings.Contains(req.TaskDescription, g.TaskDescription) &&
		!strings.Contains(req.Constraints, g.Constraints) &&
		!strings.Contains(req.TaskDescription, g.Constraints) &&
		!strings.Contains(req.Constraints, g.TaskDescription)
}

// savedGuidanceValue is the guidance the tests preload as the saved file.
var savedGuidanceValue = domain.Guidance{TaskDescription: savedTask, Constraints: savedConstraints}

// adviceValue is the guidance the pre-consultant returns.
var adviceValue = domain.Guidance{TaskDescription: adviceTask, Constraints: adviceConstraint}

// adviceOf returns g as pre-consultation advice.
func adviceOf(g domain.Guidance) domain.PreConsultationAdvice {
	return domain.PreConsultationAdvice{TaskDescription: g.TaskDescription, Constraints: g.Constraints}
}

// firstRequestOf returns the first request of agent, failing the test when the
// agent was never dispatched.
func (g *guidanceRig) firstRequestOf(t *testing.T, agent string) domain.ProtocolRequest {
	t.Helper()
	reqs := g.requestsFor(agent)
	if len(reqs) == 0 {
		t.Fatalf("want %s dispatched, but it was not", agent)
	}
	return reqs[0]
}

// eventAt returns the i-th session.guidance.changed event, failing the test
// when there are fewer.
func (g *guidanceRig) eventAt(t *testing.T, i int) sessionLogEntry {
	t.Helper()
	events := g.guidanceEvents()
	if i >= len(events) {
		t.Fatalf("want at least %d guidance change events, got %d", i+1, len(events))
	}
	return events[i]
}

// firstReviewDispatches returns how many dispatches had run when the first
// review question was asked, failing the test when none was asked.
func (a *guidanceAsk) firstReviewDispatches(t *testing.T) int {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.dispatchesAtReview) == 0 {
		t.Fatal("want a review question, but none was asked")
	}
	return a.dispatchesAtReview[0]
}
