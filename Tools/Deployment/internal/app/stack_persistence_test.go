package app_test

// stack_persistence_test.go verifies which catalog stack the write runs persist with a
// deployment. The stack travels in the executor request and is written by the executor's
// journalled manifest save, never by the app itself.
//
// What a write run hands the executor, for deploy, update, workflows, agents and hooks:
//   - A run whose stack came from stack input supplies the encoded stack: user layers in order,
//     then the default layer when it is part of the stack. The default-layer flag alone gives a
//     single default entry; a user catalog without the flag excludes the default entry.
//   - A run whose stack did not come from stack input supplies none, whatever stack it used.
//
// What the manifest holds afterwards (real executor and manifest store):
//   - A deploy with stack input persists that stack.
//   - An update with different stack input replaces it as a whole; no persisted layer is merged
//     in, and the default-layer flag alone replaces a user-catalog stack.
//   - A run without stack input, full or partial, leaves the persisted stack unchanged.
//   - An update that fails and reverts leaves the persisted stack as it was.

import (
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

func requireStackEqual(t *testing.T, got, want []domain.PersistedCatalogLayer) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("CatalogStack = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CatalogStack = %+v, want %+v", got, want)
		}
	}
}

func TestWritePaths_StackInput_SuppliesTheEncodedStack(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := newSPFixture(t)

			if _, err := op.run(context.Background(), app.New(f.withInput(f.descA())), f.ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			requireStackEqual(t, f.exec.last(t).ManifestMeta.CatalogStack, f.layersA())
		})
	}
}

func TestWritePaths_DefaultLayerFlagAlone_SuppliesASingleDefaultEntry(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := newSPFixture(t)

			d := stackconfig.Description{IncludeDefault: true}
			if _, err := op.run(context.Background(), app.New(f.withInput(d)), f.ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			requireStackEqual(t, f.exec.last(t).ManifestMeta.CatalogStack, []domain.PersistedCatalogLayer{{Default: true}})
		})
	}
}

func TestWritePaths_UserCatalogWithoutDefaultLayerFlag_ExcludesTheDefaultEntry(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := newSPFixture(t)

			if _, err := op.run(context.Background(), app.New(f.withInput(f.descB())), f.ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			requireStackEqual(t, f.exec.last(t).ManifestMeta.CatalogStack, f.layersB())
		})
	}
}

func TestWritePaths_StackNotFromInput_SuppliesNoStack(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := newSPFixture(t)

			if _, err := op.run(context.Background(), app.New(f.withoutInput()), f.ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			if got := f.exec.last(t).ManifestMeta.CatalogStack; got != nil {
				t.Errorf("CatalogStack = %+v, want none: a stack that did not come from input is not persisted", got)
			}
		})
	}
}

func TestDeploy_StackInput_PersistsTheStack(t *testing.T) {
	f := newSPFixture(t)

	f.deployWithInput(t, f.descA())

	f.requirePersisted(t, f.layersA())
}

func TestUpdate_DifferentStackInput_ReplacesThePersistedStackWithoutMerging(t *testing.T) {
	f := newSPFixture(t)
	f.deployWithInput(t, f.descA())

	if _, err := opByName(t, "update").run(context.Background(), app.New(f.real(f.withInput(f.descB()))), f.ws); err != nil {
		t.Fatalf("update: %v", err)
	}

	f.requirePersisted(t, f.layersB())
}

func TestUpdate_DefaultLayerFlagAlone_ReplacesAPersistedUserCatalogStack(t *testing.T) {
	f := newSPFixture(t)
	f.deployWithInput(t, f.descA())

	d := stackconfig.Description{IncludeDefault: true}
	if _, err := opByName(t, "update").run(context.Background(), app.New(f.real(f.withInput(d))), f.ws); err != nil {
		t.Fatalf("update: %v", err)
	}

	f.requirePersisted(t, []domain.PersistedCatalogLayer{{Default: true}})
}

func TestWriteRunWithoutStackInput_LeavesThePersistedStackUnchanged(t *testing.T) {
	for _, op := range provOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := newSPFixture(t)
			f.deployWithInput(t, f.descA())

			if _, err := op.run(context.Background(), app.New(f.real(f.withoutInput())), f.ws); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			f.requirePersisted(t, f.layersA())
		})
	}
}

func TestUpdate_FailsAndReverts_LeavesThePersistedStackUnchanged(t *testing.T) {
	f := newSPFixture(t)
	f.deployWithInput(t, f.descA())
	deps := f.real(f.withInput(f.descB()))
	deps.Executor = revertingExecutor{inner: deps.Executor}

	_, err := opByName(t, "update").run(context.Background(), app.New(deps), f.ws)

	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("update error = %v, want the run to revert (ErrRunReverted)", err)
	}
	f.requirePersisted(t, f.layersA())
}
