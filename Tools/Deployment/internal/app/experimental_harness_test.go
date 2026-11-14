package app_test

// experimental_harness_test.go holds the helpers of the experimental-harness tests: a test-only
// harness module whose descriptor and ref both carry an experimental declaration, a service
// wired over the parity sample catalog with such harnesses, and an interaction wrapper that
// records the questions and the plans the app sends to the user.

import (
	"context"
	"sync"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// Test-only harness ids and reasons. Nothing in the app may depend on these values.
const (
	expHarnessID  = "preview-harness"
	expHarnessID2 = "preview-harness-two"
	expPlainID    = "plain-harness"
	expReason     = "Preview only: orchestration is not available."
	expReason2    = "Second preview: agents only."
)

// expModule is a harness module that answers Ref and Descriptor from its own copies and
// delegates everything else to the built-in OpenCode module.
type expModule struct {
	domain.HarnessModule
	ref  domain.HarnessRef
	desc *domain.HarnessDescriptor
}

func (m *expModule) Ref() domain.HarnessRef                { return m.ref }
func (m *expModule) Descriptor() *domain.HarnessDescriptor { return m.desc }

// newTestExperimentalModule returns a usable harness module with the given id. A non-empty
// reason declares the harness experimental on both the descriptor copy and the ref; an empty
// reason yields a plain (not experimental) harness.
func newTestExperimentalModule(t *testing.T, id, reason string) domain.HarnessModule {
	t.Helper()
	return newTestExperimentalModuleOn(t, hOpen, id, reason)
}

// newTestExperimentalModuleOn is newTestExperimentalModule over the built-in module baseID, so
// two test harnesses can write to different destinations.
func newTestExperimentalModuleOn(t *testing.T, baseID, id, reason string) domain.HarnessModule {
	t.Helper()
	inner := rbModule(t, baseID)
	desc := *inner.Descriptor()
	desc.ID = id
	desc.DisplayName = "Display " + id
	desc.ExperimentalReason = reason
	ref := inner.Ref()
	ref.ID = id
	ref.DisplayName = desc.DisplayName
	ref.ExperimentalReason = reason
	return &expModule{HarnessModule: inner, ref: ref, desc: &desc}
}

// withInteraction is an option that replaces the service's interaction port.
func withInteraction(in domain.Interaction) func(*app.Deps) {
	return func(d *app.Deps) { d.Interaction = in }
}

// newTestExperimentalService returns a service over the parity sample catalog whose registry
// holds the given modules in order, with extra dependency options applied after the registry
// is built.
func newTestExperimentalService(t *testing.T, mods []domain.HarnessModule, opts ...func(*app.Deps)) app.Service {
	t.Helper()
	all := append([]func(*app.Deps){withTwinRegistryModules(mods[1:]...)}, opts...)
	return newTestTwinService(t, mods[0], all...)
}

// recordingInteraction wraps a scripted stub, records every choice question and every plan
// sent to Review, and answers chosen questions with a fixed option id.
type recordingInteraction struct {
	domain.Interaction
	mu        sync.Mutex
	questions []domain.ChoiceQuestion
	plans     []domain.Plan
	answers   map[domain.QuestionID]string
}

func newRecordingInteraction(answers map[domain.QuestionID]string) *recordingInteraction {
	return &recordingInteraction{Interaction: interactiontest.NewBuilder().Build(), answers: answers}
}

func (r *recordingInteraction) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	r.mu.Lock()
	r.questions = append(r.questions, q)
	id, ok := r.answers[q.ID]
	r.mu.Unlock()
	if ok {
		return domain.ChoiceAnswer{Status: domain.Answered, OptionID: id}, nil
	}
	return r.Interaction.SelectOne(ctx, q)
}

func (r *recordingInteraction) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	r.mu.Lock()
	r.plans = append(r.plans, p)
	r.mu.Unlock()
	return r.Interaction.Review(ctx, p)
}

// reset forgets the recorded plans and questions.
func (r *recordingInteraction) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans, r.questions = nil, nil
}

// question returns the recorded question with the given id.
func (r *recordingInteraction) question(t *testing.T, id domain.QuestionID) domain.ChoiceQuestion {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.questions {
		if q.ID == id {
			return q
		}
	}
	t.Fatalf("question %q was not asked", id)
	return domain.ChoiceQuestion{}
}

// lastPlan returns the most recently reviewed plan, failing the test when none was reviewed.
func (r *recordingInteraction) lastPlan(t *testing.T) domain.Plan {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.plans) == 0 {
		t.Fatal("no plan was sent to Review")
	}
	return r.plans[len(r.plans)-1]
}

// optionByID returns the option with the given id, failing the test when it is missing.
func optionByID(t *testing.T, q domain.ChoiceQuestion, id string) domain.Option {
	t.Helper()
	for _, o := range q.Options {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("option %q not offered by question %q", id, q.ID)
	return domain.Option{}
}
