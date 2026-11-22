package app_test

// save_offer_refresh_test.go specifies that a saved custom tool answer takes effect for later
// runs: a later run on the same service (the TUI returns to the mode screen after a run and
// starts the next one in the same process) and a later run on a freshly built service and
// registry (another process, another workspace) resolve the generic from the saved entry and do
// not ask for it again. A save that did not happen leaves the next run asking, and a failed
// registry rebuild is a warning that leaves the run unchanged.

import (
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// rebuildState is shared by every generation of a rebuildRegistry.
type rebuildState struct {
	rebuilds int
	err      error // when set, Rebuild fails with it
}

// rebuildRegistry is a registry that builds its modules from the saved config at construction
// and implements registry.Rebuilder: Rebuild returns a new registry built again from the config.
type rebuildRegistry struct {
	state   *rebuildState
	build   func() map[string]domain.HarnessModule
	modules map[string]domain.HarnessModule
}

func newRebuildRegistry(state *rebuildState, build func() map[string]domain.HarnessModule) *rebuildRegistry {
	return &rebuildRegistry{state: state, build: build, modules: build()}
}

func (r *rebuildRegistry) List() []domain.HarnessRef { return []domain.HarnessRef{minimalHarness} }

func (r *rebuildRegistry) Resolve(id string) (domain.HarnessModule, error) {
	if m, ok := r.modules[id]; ok {
		return m, nil
	}
	return nil, errors.New("harness not found: " + id)
}

func (r *rebuildRegistry) RebuildForStack(injections.Result) (registry.Registry, error) {
	return r, nil
}

func (r *rebuildRegistry) Rebuild() (registry.Registry, error) {
	r.state.rebuilds++
	if r.state.err != nil {
		return nil, r.state.err
	}
	return newRebuildRegistry(r.state, r.build), nil
}

// refreshRun is a deploy run whose registry reads the saved entries when it is built.
type refreshRun struct {
	*offerRun
	state *rebuildState
	svc   app.Service
}

func newRefreshRun(t *testing.T, o offerSetup) *refreshRun {
	t.Helper()
	r := newOfferRun(t, listFlowAnswer, o)
	state := &rebuildState{}
	r.deps.Registry = newRebuildRegistry(state, r.buildModules)
	return &refreshRun{offerRun: r, state: state, svc: app.New(r.deps)}
}

// buildModules is the registry's module factory: the saved entries of the spy config are the
// harness's tool_destinations at the moment the registry is built.
func (r *offerRun) buildModules() map[string]domain.HarnessModule {
	return map[string]domain.HarnessModule{
		offerHarnessID: newOfferModule(domain.TierBuiltin, true, []string{"bash"}, savedFrom(r.user)),
	}
}

// deploy runs one deploy on the service.
func (r *refreshRun) deploy(t *testing.T, svc app.Service) {
	t.Helper()
	_, err := svc.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: offerHarnessID, WorkspacePath: r.workDir, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
}

// refreshFlows are the deploy-family flows driven on a given service and workspace folder.
var refreshFlows = []struct {
	name string
	run  func(t *testing.T, svc app.Service, dir string) error
}{
	{"deploy", func(t *testing.T, svc app.Service, dir string) error {
		_, err := svc.DeployNew(context.Background(), app.DeployRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, Scope: domain.ScopeProject,
			WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		return err
	}},
	{"deploy agents", func(t *testing.T, svc app.Service, dir string) error {
		_, err := svc.DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, Scope: domain.ScopeProject,
			SubagentIDs: []string{"test-runner", "second-runner"}, UtilityAgentIDs: []string{},
			InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
			SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		return err
	}},
	{"change workflows", func(t *testing.T, svc app.Service, dir string) error {
		_, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, WorkflowIDs: []string{"quick-fix"},
			SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		return err
	}},
}

func (r *refreshRun) asked() int { return r.spy.Stub.CountAsked(domain.QCustomTool, "bash") }

func TestSaveOffer_SameProcess_SavedAnswerIsNotAskedAgainByTheSameService(t *testing.T) {
	r := newRefreshRun(t, offerSetup{})

	r.deploy(t, r.svc)
	r.deploy(t, r.svc)

	if got := r.asked(); got != 1 {
		t.Errorf("the custom tool question was asked %d times over two runs, want once", got)
	}
	if len(r.offer.offers) != 1 {
		t.Errorf("offers = %d, want one: a saved generic is not asked, so not offered again", len(r.offer.offers))
	}
	if r.state.rebuilds != 1 {
		t.Errorf("registry rebuilt %d times, want once after the save", r.state.rebuilds)
	}
}

func TestSaveOffer_SameProcess_EveryFlowSkipsTheSavedAnswerOnTheSameService(t *testing.T) {
	for _, flow := range refreshFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newRefreshRun(t, offerSetup{})
			if err := flow.run(t, r.svc, r.workDir); err != nil {
				t.Fatalf("first run: %v", err)
			}

			// A fresh workspace makes the agents new again, so only the saved entry can stop the question.
			if err := flow.run(t, r.svc, t.TempDir()); err != nil {
				t.Fatalf("second run: %v", err)
			}

			if got := r.asked(); got != 1 {
				t.Errorf("the custom tool question was asked %d times over two runs, want once", got)
			}
			if len(r.offer.offers) != 1 {
				t.Errorf("offers = %d, want one", len(r.offer.offers))
			}
			if r.state.rebuilds != 1 {
				t.Errorf("registry rebuilt %d times, want once after the save", r.state.rebuilds)
			}
		})
	}
}

func TestSaveOffer_SameProcess_RegistryWithoutRebuild_SaveSucceedsAndRunsAreUnaffected(t *testing.T) {
	r := newOfferRun(t, listFlowAnswer, offerSetup{}) // the default stub registry cannot be rebuilt
	svc := app.New(r.deps)
	deploy := refreshFlows[0].run

	if err := deploy(t, svc, r.workDir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(r.user.puts) != 1 {
		t.Fatalf("saved %d entries, want one although the registry cannot be rebuilt", len(r.user.puts))
	}
	if err := deploy(t, svc, t.TempDir()); err != nil {
		t.Fatalf("second run: %v", err)
	}

	if got := r.spy.Stub.CountAsked(domain.QCustomTool, "bash"); got != 2 {
		t.Errorf("the question was asked %d times, want 2: this registry never learns the saved entry", got)
	}
}

func TestSaveOffer_SameProcess_LaterRunLeavesTheToolMappedNotUnmapped(t *testing.T) {
	r := newRefreshRun(t, offerSetup{})
	r.deploy(t, r.svc)

	r.exec.capturedPlan, r.todo.gaps = nil, nil
	r.deploy(t, r.svc)

	assertToolGapReporting(t, r.customToolRun, []string{"bash"}, nil)
	if r.asked() != 1 {
		t.Errorf("the question was asked %d times, want once", r.asked())
	}
}

func TestSaveOffer_SameProcess_NothingSavedMeansTheNextRunAsksAgainAndNoRebuild(t *testing.T) {
	setups := map[string]offerSetup{
		"declined":    {answer: declineOffer},
		"cancelled":   {answer: cancelOffer},
		"failed save": {lockErr: errors.New("lock timeout")},
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			r := newRefreshRun(t, setup)

			r.deploy(t, r.svc)
			r.deploy(t, r.svc)

			if got := r.asked(); got != 2 {
				t.Errorf("the question was asked %d times over two runs, want 2 (nothing was saved)", got)
			}
			if len(r.offer.offers) != 2 {
				t.Errorf("offers = %d, want one per answered run", len(r.offer.offers))
			}
			if r.state.rebuilds != 0 {
				t.Errorf("registry rebuilt %d times although nothing was saved", r.state.rebuilds)
			}
		})
	}
}

func TestSaveOffer_SameProcess_FailedRebuild_IsAWarningAndLaterRunsKeepAskingWithTheOldRegistry(t *testing.T) {
	r := newRefreshRun(t, offerSetup{})
	r.state.err = errors.New("rebuild backend exploded")

	r.deploy(t, r.svc) // fails the test on a run error
	r.deploy(t, r.svc)

	var warning *domain.Notice
	for _, n := range noticesAt(r.notices(), domain.NoticeWarning) {
		if noticeMentions(n, "rebuild backend exploded") {
			warning = &n
		}
	}
	if warning == nil {
		t.Fatalf("no warning names the rebuild error: %+v", r.notices())
	}
	if got := r.asked(); got != 2 {
		t.Errorf("the question was asked %d times, want 2: the old registry does not know the saved entry", got)
	}
}

func TestSaveOffer_LaterProcess_FreshServiceAndRegistryDoNotAskForASavedGeneric(t *testing.T) {
	first := newRefreshRun(t, offerSetup{})
	first.deploy(t, first.svc)
	if len(first.user.puts) != 1 {
		t.Fatalf("saved %d entries in the first run, want one", len(first.user.puts))
	}

	later := first.deps
	later.Registry = newRebuildRegistry(&rebuildState{}, first.buildModules)
	if err := refreshFlows[0].run(t, app.New(later), t.TempDir()); err != nil { // another workspace
		t.Fatalf("later run: %v", err)
	}

	if got := first.asked(); got != 1 {
		t.Errorf("the question was asked %d times over two services, want once", got)
	}
}

func TestSaveOffer_SameProcess_EntriesAccumulateAcrossRebuilds(t *testing.T) {
	agents := []domain.Agent{toolAgent("bash-runner", "bash"), toolAgent("grep-runner", "grep")}
	generics := []string{"bash", "grep"}
	answers := map[string]string{"bash": listFlowAnswer, "grep": listFlowAnswer}
	r := newOfferRunFor(t, agents, generics, answers, offerSetup{})
	state := &rebuildState{}
	r.deps.Registry = newRebuildRegistry(state, func() map[string]domain.HarnessModule {
		return map[string]domain.HarnessModule{
			offerHarnessID: newOfferModule(domain.TierBuiltin, true, generics, savedFrom(r.user)),
		}
	})
	svc := app.New(r.deps)
	deployAgents := func(ids ...string) {
		t.Helper()
		_, err := svc.DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: offerHarnessID, WorkspacePath: r.workDir, Scope: domain.ScopeProject,
			SubagentIDs: ids, UtilityAgentIDs: []string{},
			InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
			SkipAll: listSkipModels, AutoConfirmPlan: true,
		})
		if err != nil {
			t.Fatalf("DeployAgents: %v", err)
		}
	}

	deployAgents("bash-runner")
	deployAgents("bash-runner", "grep-runner")

	if got := r.spy.Stub.CountAsked(domain.QCustomTool, "bash"); got != 1 {
		t.Errorf("bash was asked %d times over two runs, want once: the first run saved it", got)
	}
	if got := r.spy.Stub.CountAsked(domain.QCustomTool, "grep"); got != 1 {
		t.Errorf("grep was asked %d times, want once", got)
	}
	if len(r.user.puts) != 2 || state.rebuilds != 2 {
		t.Errorf("saved %d entries and rebuilt %d times, want 2 and 2: the second save keeps the first", len(r.user.puts), state.rebuilds)
	}
}
