package main

// Composition-root wiring of the observed-model provider. The e2e harness
// assembles its own runner.Deps, so only these tests notice a composition root
// that forgets the provider: every real run would then be reported as having
// no model evidence.
//
// The Deps field is addressed by name through reflection so these tests
// compile before the field exists and fail on its absence.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/workspace"
)

const observedModelsFieldName = "ObservedModels"

type fakeObservedModelProvider struct{}

func (fakeObservedModelProvider) ObservedSubjectModels(ctx context.Context, q domain.ObservedModelQuery) (domain.ObservedModelEvidence, error) {
	return domain.ObservedModelEvidence{Models: []string{"fake-model"}}, nil
}

func TestBuildDeps_WiresAnObservedModelProvider(t *testing.T) {
	cfg := WiringConfig{
		HarnessID:         "claude-code",
		FixtureRoot:       "C:/fixtures",
		WorkspaceRoot:     "C:/workspaces",
		SelfPath:          "C:/bin/mosaic-agent-test.exe",
		LoggerBundleDir:   "C:/bundles/logger",
		CostToolPath:      "C:/bin/mosaic-log-analyzer.exe",
		CostTimeout:       30 * time.Second,
		Diag:              new(discardWriter),
		CatalogFolder:     "C:/catalog",
		DeployScratchRoot: "C:/scratch",
	}

	d, err := buildDeps(cfg)
	if err != nil {
		t.Fatalf("buildDeps returned an error: %v", err)
	}

	field := reflect.ValueOf(d).FieldByName(observedModelsFieldName)
	if !field.IsValid() {
		t.Fatalf("Deps has no %s field; the composition root must hold the observed-model provider", observedModelsFieldName)
	}
	if field.IsNil() {
		t.Errorf("buildDeps left Deps.%s nil; every run would be reported as having no model evidence", observedModelsFieldName)
	}
}

func TestDeps_RunnerDeps_ProjectsObservedModelProviderUnchanged(t *testing.T) {
	d := testDeps(t)
	field := reflect.ValueOf(&d).Elem().FieldByName(observedModelsFieldName)
	if !field.IsValid() {
		t.Fatalf("Deps has no %s field; RunnerDeps cannot project the observed-model provider", observedModelsFieldName)
	}
	want := fakeObservedModelProvider{}
	field.Set(reflect.ValueOf(want))
	ws := workspace.NewManager(t.TempDir(), d.Clock)

	got := d.RunnerDeps(ws, fakeProgressSink{}, testBundle())

	if got.ObservedModels != want {
		t.Errorf("RunnerDeps(...).ObservedModels = %v, want the provider held by Deps", got.ObservedModels)
	}
}
