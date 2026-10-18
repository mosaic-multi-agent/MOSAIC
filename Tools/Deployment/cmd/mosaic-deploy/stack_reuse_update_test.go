package main

// stack_reuse_update_test.go runs the built binary to verify the reuse and replace rules for
// the subcommands that deploy into an existing workspace (update, deploy): a run without stack
// input reuses the workspace's persisted stack, a run with any stack input uses that input alone
// and replaces the persisted stack, and a persisted layer that no longer resolves stops the run.

import (
	"bytes"
	"path/filepath"
	"testing"
)

// newShadowingUserCatalog returns a user catalog that supplies the default catalog's marker
// content ("default-wf" and "default-only-agent") under its own identity at the given version,
// so a stack of this catalog alone deploys the same artifact keys as the default catalog.
func newShadowingUserCatalog(t *testing.T, id, version string) string {
	t.Helper()
	root := newStackUserCatalog(t, id, version)
	writeStackAgent(t, root, "default-only-agent", "default-only-001", version)
	writeStackWorkflow(t, root, "default-wf", "default-only-agent")
	return root
}

// newSupersetUserCatalog returns a user catalog that also supplies "<other>-agent" and
// "<other>-wf", so a workspace deployed from catalog <other> can be updated from this one.
func newSupersetUserCatalog(t *testing.T, id, version, other string) string {
	t.Helper()
	root := newStackUserCatalog(t, id, version)
	writeStackAgent(t, root, other+"-agent", other+"-001", version)
	writeStackWorkflow(t, root, other+"-wf", other+"-agent")
	return root
}

func TestBinary_Update_NoStackInput_ReusesThePersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	setUserCatalogVersion(t, userA, "user-a", "3.0.0")

	run := dryRunUpdate(t, mosaicRoot, ws)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	versions := parseRunSummary(t, run).deployedVersions()
	if versions["user-a-agent"] != "3.0.0" || versions["orchestrator"] != "3.0.0" {
		t.Errorf("update versions = %v, want the persisted user catalog's 3.0.0 content", versions)
	}
	if _, ok := versions["default-only-agent"]; ok {
		t.Errorf("update versions = %v, must not contain default catalog content", versions)
	}
}

func TestBinary_Update_NoStackInput_LeavesThePersistedStackUnchanged(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	setUserCatalogVersion(t, userA, "user-a", "3.0.0")

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userA, CatalogID: "user-a"})
}

func TestBinary_Deploy_IntoExistingDeploymentWithoutStackInput_ReusesThePersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	setUserCatalogVersion(t, userA, "user-a", "3.0.0")

	run := runInWorkspace(t, mosaicRoot, ws, "deploy", "--workflows", "user-a-wf", "--auto-confirm",
		"--conflict", "overwrite", "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("redeploy without stack input failed (exit %d); the user-a workflow exists only in the persisted stack\nstderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["user-a-agent"] != "3.0.0" {
		t.Errorf("redeploy versions = %v, want the persisted user catalog's 3.0.0 content", v)
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userA, CatalogID: "user-a"})
}

func TestBinary_Update_WithUserCatalogFlag_UsesOnlyThatInputAndReplacesThePersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	userB := newSupersetUserCatalog(t, "user-b", "4.0.0", "user-a")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	setUserCatalogVersion(t, userA, "user-a", "3.0.0")

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json", "--catalog-folder", userB)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["user-a-agent"] != "4.0.0" {
		t.Errorf("update versions = %v, want user-b's 4.0.0 content (user-a's persisted 3.0.0 must not be used)", v)
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userB, CatalogID: "user-b"})
}

func TestBinary_Update_AfterReplace_UsesTheNewStackWithoutTheOldLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	userB := newSupersetUserCatalog(t, "user-b", "4.0.0", "user-a")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	replace := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json", "--catalog-folder", userB)
	if replace.ExitCode != 0 {
		t.Fatalf("setup replace failed (exit %d): %s", replace.ExitCode, replace.Stderr)
	}
	setUserCatalogVersion(t, userB, "user-b", "5.0.0")
	writeStackAgent(t, userB, "user-a-agent", "user-a-001", "5.0.0")
	removeAll(t, userA) // a merged persisted stack would now fail to resolve user-a

	run := dryRunUpdate(t, mosaicRoot, ws)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["user-a-agent"] != "5.0.0" {
		t.Errorf("update versions = %v, want the replaced stack's (user-b) 5.0.0 content", v)
	}
}

func TestBinary_Update_WithOnlyTheDefaultFlag_ReplacesAUserCatalogStackWithTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userC := newShadowingUserCatalog(t, "user-c", "0.5.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "default-wf", "--catalog-folder", userC)

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json", "--include-default-catalog")

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["default-only-agent"] != "1.0.0" {
		t.Errorf("update versions = %v, want the default catalog's 1.0.0 content alone", v)
	}
	assertPersistedDefaultOnly(t, ws)
}

func TestBinary_Update_AfterDefaultOnlyReset_UsesTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userC := newShadowingUserCatalog(t, "user-c", "0.5.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "default-wf", "--catalog-folder", userC)
	reset := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json", "--include-default-catalog")
	if reset.ExitCode != 0 {
		t.Fatalf("setup reset failed (exit %d): %s", reset.ExitCode, reset.Stderr)
	}
	setUserCatalogVersion(t, userC, "user-c", "9.0.0")
	removeAll(t, userC)

	run := dryRunUpdate(t, mosaicRoot, ws)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d; the reset stack must not depend on the old user catalog\nstderr: %s", run.ExitCode, run.Stderr)
	}
}

func TestBinary_Update_WorkspaceWithoutPersistedStackAndNoStackInput_UsesTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "default-wf")
	if got := readPersistedLayers(t, ws); len(got) != 0 {
		t.Fatalf("setup: a no-input deploy persisted %+v, want no stack", got)
	}
	writeStackAgent(t, filepath.Join(mosaicRoot, "Catalog"), "default-only-agent", "default-only-001", "1.1.0")

	run := dryRunUpdate(t, mosaicRoot, ws)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["default-only-agent"] != "1.1.0" {
		t.Errorf("update versions = %v, want the default catalog's 1.1.0 content", v)
	}
	if got := readPersistedLayers(t, ws); len(got) != 0 {
		t.Errorf("persisted stack after a no-input update = %+v, want none", got)
	}
}

func TestBinary_Update_UnresolvablePersistedLayer_StopsBeforeWritingAndNamesTheLayer(t *testing.T) {
	for _, defect := range layerDefects() {
		t.Run(defect.name, func(t *testing.T) {
			mosaicRoot := newStackMosaicRoot(t)
			userA := newStackUserCatalog(t, "user-a", "2.0.0")
			ws := t.TempDir()
			deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
			defect.apply(t, userA)
			before, entries := readManifestBytes(t, ws), dirEntryCount(t, ws)

			run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json")

			assertStopsNamingLayer(t, run, userA)
			if !bytes.Equal(before, readManifestBytes(t, ws)) || dirEntryCount(t, ws) != entries {
				t.Error("the workspace changed although the persisted stack could not be resolved")
			}
		})
	}
}

func TestBinary_Deploy_UnresolvablePersistedLayer_StopsBeforeWritingAndNamesTheLayer(t *testing.T) {
	for _, defect := range layerDefects() {
		t.Run(defect.name, func(t *testing.T) {
			mosaicRoot := newStackMosaicRoot(t)
			userA := newStackUserCatalog(t, "user-a", "2.0.0")
			ws := t.TempDir()
			deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
			defect.apply(t, userA)
			before, entries := readManifestBytes(t, ws), dirEntryCount(t, ws)

			run := runInWorkspace(t, mosaicRoot, ws, "deploy", "--workflows", "", "--auto-confirm", "--output", "json")

			assertStopsNamingLayer(t, run, userA)
			if !bytes.Equal(before, readManifestBytes(t, ws)) || dirEntryCount(t, ws) != entries {
				t.Error("the workspace changed although the persisted stack could not be resolved")
			}
		})
	}
}

func TestBinary_Update_UnresolvablePersistedLayer_IsRecoverableWithAnyStackInput(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	userB := newSupersetUserCatalog(t, "user-b", "4.0.0", "user-a")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	layerDefects()[0].apply(t, userA)

	run := runInWorkspace(t, mosaicRoot, ws, "update", "--auto-confirm", "--output", "json", "--catalog-folder", userB)

	if run.ExitCode != 0 {
		t.Fatalf("update with a new stack failed (exit %d): the persisted layer is replaced, not validated\nstderr: %s", run.ExitCode, run.Stderr)
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userB, CatalogID: "user-b"})
}
