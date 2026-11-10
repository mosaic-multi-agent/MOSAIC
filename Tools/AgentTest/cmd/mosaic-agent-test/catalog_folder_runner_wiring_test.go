package main

// The runner's pre-spawn check reads each deployed agent's declared tier from
// the test catalog the run's deployer was built with. These tests pin that the
// composition root hands the runner exactly that folder: the process-wide
// default through Deps.RunnerDeps, and a per-run override through
// composedSuiteRunner.Run.

import (
	"context"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
	"mosaic-agent-test/internal/runner"
	"mosaic-agent-test/internal/suite"
	"mosaic-agent-test/internal/workspace"
)

func TestRunnerDeps_CarriesTheProcessWideCatalogFolder(t *testing.T) {
	const defaultCatalog = "C:/default/catalog"
	d := minimalDepsForCatalogTest(t, defaultCatalog, nil)
	ws := workspace.NewManager(t.TempDir(), fakeClock{})

	got := d.RunnerDeps(ws, fakeProgressSink{}, testBundle())

	if got.CatalogFolder != defaultCatalog {
		t.Errorf("RunnerDeps(...).CatalogFolder = %q, want the process-wide catalog folder %q the default deployer is built with", got.CatalogFolder, defaultCatalog)
	}
}

// capturedRunnerDeps runs an empty plan through a composedSuiteRunner and
// returns the runner.Deps its suite was handed.
func capturedRunnerDeps(t *testing.T, d Deps, plan preflight.Plan) runner.Deps {
	t.Helper()
	var captured runner.Deps
	r := composedSuiteRunner{
		deps:      d,
		ws:        workspace.NewManager(t.TempDir(), fakeClock{}),
		retention: domain.RetainNever,
		newSuite: func(o suite.Options) *suite.Suite {
			adapter, ok := o.Runner.(testRunnerAdapter)
			if !ok {
				t.Fatalf("suite.Options.Runner is %T, want testRunnerAdapter", o.Runner)
			}
			captured = adapter.deps
			return suite.New(o)
		},
	}

	_, _ = r.Run(context.Background(), plan, fakeProgressSink{})

	return captured
}

func TestComposedSuiteRunner_PerRunCatalogOverrideReachesTheRunnersPreSpawnCheck(t *testing.T) {
	const defaultCatalog = "C:/default/catalog"
	const overrideCatalog = "C:/override/catalog"
	var built *catalogFactoryDeployer
	factory := func(folder string) domain.AgentDeployer {
		built = &catalogFactoryDeployer{calledWith: folder}
		return built
	}
	d := minimalDepsForCatalogTest(t, defaultCatalog, factory)

	got := capturedRunnerDeps(t, d, preflight.Plan{CatalogFolder: overrideCatalog})

	if got.CatalogFolder != overrideCatalog {
		t.Errorf("runner.Deps.CatalogFolder = %q, want the per-run override %q", got.CatalogFolder, overrideCatalog)
	}
	if built == nil || got.Deploy != domain.AgentDeployer(built) {
		t.Fatalf("runner.Deps.Deploy = %v, want the per-run deployer built by the factory", got.Deploy)
	}
	if got.CatalogFolder != built.calledWith {
		t.Errorf("runner.Deps.CatalogFolder = %q but its deployer was built with %q; the check must read the folder deploy is given", got.CatalogFolder, built.calledWith)
	}
}

func TestComposedSuiteRunner_WithoutOverrideTheRunnerKeepsTheDefaultCatalogFolder(t *testing.T) {
	const defaultCatalog = "C:/default/catalog"
	factory := func(folder string) domain.AgentDeployer { return &catalogFactoryDeployer{calledWith: folder} }
	d := minimalDepsForCatalogTest(t, defaultCatalog, factory)

	got := capturedRunnerDeps(t, d, preflight.Plan{CatalogFolder: defaultCatalog})

	if got.CatalogFolder != defaultCatalog {
		t.Errorf("runner.Deps.CatalogFolder = %q, want the process-wide default %q", got.CatalogFolder, defaultCatalog)
	}
}
