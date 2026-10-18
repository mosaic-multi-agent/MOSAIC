package main

// stack_in_effect_test.go runs the built binary to verify that a run reports the stack in
// effect and where it came from: reused from the workspace's persisted stack, built from stack
// input, or the default layer alone. The JSON document carries it under one additive key; the
// human output carries a block for the persisted and stack-input origins only.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
)

// stackInEffectOf decodes the stack in effect from a run's JSON run summary and checks that
// the whole stdout is that one document.
func stackInEffectOf(t *testing.T, run binaryRun) stackconfig.ActiveStack {
	t.Helper()
	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(run.Stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s", err, run.Stdout)
	}
	raw, ok := doc["StackInEffect"]
	if !ok {
		t.Fatalf("run summary has no StackInEffect key:\n%s", run.Stdout)
	}
	if _, ok := doc["Outcome"]; !ok {
		t.Errorf("existing Outcome key missing from the run summary:\n%s", run.Stdout)
	}
	var active stackconfig.ActiveStack
	if err := json.Unmarshal(raw, &active); err != nil {
		t.Fatalf("decode StackInEffect: %v", err)
	}
	return active
}

// requireUserLayers fails unless the stack's layers are exactly the given ids and roots, in
// order, none of them the default layer.
func requireUserLayers(t *testing.T, active stackconfig.ActiveStack, want ...persistedLayer) {
	t.Helper()
	if len(active.Layers) != len(want) {
		t.Fatalf("layers = %+v, want %d user layer(s) %+v", active.Layers, len(want), want)
	}
	for i, w := range want {
		l := active.Layers[i]
		if l.Identity.ID != w.CatalogID || filepath.Clean(l.Root) != filepath.Clean(w.Path) || l.Default {
			t.Errorf("layer %d = %+v, want id %q at %q, not default", i, l, w.CatalogID, w.Path)
		}
	}
}

func TestBinary_Update_NoStackInput_ReportsThePersistedStackInEffect(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)

	run := dryRunUpdate(t, mosaicRoot, ws)

	active := stackInEffectOf(t, run)
	if active.Origin != stackconfig.OriginPersisted {
		t.Errorf("origin = %q, want %q", active.Origin, stackconfig.OriginPersisted)
	}
	requireUserLayers(t, active, persistedLayer{Path: userA, CatalogID: "user-a"})
}

func TestBinary_Update_WithUserCatalogFlag_ReportsOnlyThatInputsLayers(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	userB := newSupersetUserCatalog(t, "user-b", "4.0.0", "user-a")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)

	run := dryRunUpdate(t, mosaicRoot, ws, "--catalog-folder", userB)

	active := stackInEffectOf(t, run)
	if active.Origin != stackconfig.OriginStackInput {
		t.Errorf("origin = %q, want %q", active.Origin, stackconfig.OriginStackInput)
	}
	requireUserLayers(t, active, persistedLayer{Path: userB, CatalogID: "user-b"})
}

func TestBinary_Update_NoStackInputAndNoPersistedStack_ReportsTheDefaultLayerAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "default-wf")

	run := dryRunUpdate(t, mosaicRoot, ws)

	active := stackInEffectOf(t, run)
	if active.Origin != stackconfig.OriginDefault {
		t.Errorf("origin = %q, want %q", active.Origin, stackconfig.OriginDefault)
	}
	if len(active.Layers) != 1 || !active.Layers[0].Default ||
		filepath.Clean(active.Layers[0].Root) != filepath.Clean(filepath.Join(mosaicRoot, "Catalog")) {
		t.Errorf("layers = %+v, want the default layer of %s alone", active.Layers, mosaicRoot)
	}
}

func TestBinary_Update_NoStackInput_HumanOutputNamesThePersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--dry-run")

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if !strings.Contains(run.Stdout, "Catalog stack (persisted):") || !strings.Contains(run.Stdout, "user-a") ||
		!strings.Contains(run.Stdout, userA) {
		t.Errorf("human output does not name the persisted stack and its layer:\n%s", run.Stdout)
	}
}

func TestBinary_Update_NoStackInputAndNoPersistedStack_HumanOutputHasNoStackBlock(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "default-wf")

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--dry-run")

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if strings.Contains(run.Stdout, "Catalog stack") {
		t.Errorf("default-only run printed a stack block:\n%s", run.Stdout)
	}
}
