package main

// harness_injection_stack_test.go runs the built mosaic-deploy binary to verify that a built-in
// harness takes its injection content from the layer the catalog stack resolved for it: as one
// unit from the highest layer that supplies the harness's injections, with nothing merged from
// a lower layer and no fallback to the MOSAIC root, and that a harness no layer supplies stops
// the run before anything is written, naming the injection item.
//
// Layers are told apart by markers in their injection text ("<MARKER>-SHARED" and
// "<MARKER>-ORCH") and by the orchestrator injections version.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

const (
	defaultInjMarker = "DEFAULT-INJ"
	userInjMarker    = "USER-INJ"
	defaultOrchVer   = "4.4.4"
	userOrchVer      = "5.5.5"
)

// injectionStackFixture is a MOSAIC root whose default catalog and one user catalog both carry
// marker injections and the regions the injections fill.
type injectionStackFixture struct {
	mosaicRoot string
	user       string
}

func newInjectionStackFixture(t *testing.T) injectionStackFixture {
	t.Helper()
	mosaicRoot := newStackMosaicRoot(t)
	def := filepath.Join(mosaicRoot, "Catalog")
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	writeClaudeMarkerInjections(t, def, defaultInjMarker, defaultOrchVer)
	writeClaudeMarkerInjections(t, user, userInjMarker, userOrchVer)
	addInjectionRegions(t, def, "default-only-agent")
	addInjectionRegions(t, user, "user-a-agent")
	return injectionStackFixture{mosaicRoot: mosaicRoot, user: user}
}

// deploy runs a real claude-code deploy of the given workflows into a fresh workspace.
func (f injectionStackFixture) deploy(t *testing.T, workflows string, extra ...string) (binaryRun, string) {
	t.Helper()
	ws := t.TempDir()
	args := []string{"deploy", "--mosaic-root", f.mosaicRoot, "--harness", "claude-code", "--workspace", ws,
		"--workflows", workflows, "--auto-confirm", "--output", "json"}
	return runDeployBinary(t, append(args, extra...)...), ws
}

func requireSuccess(t *testing.T, run binaryRun) {
	t.Helper()
	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, want success\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
}

func assertContains(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s does not contain %q:\n%s", what, w, text)
		}
	}
}

func assertOmits(t *testing.T, what, text string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(text, u) {
			t.Errorf("%s contains %q although it must not:\n%s", what, u, text)
		}
	}
}

func TestBinary_TwoLayersSupplyInjections_DeployedRegionsComeEntirelyFromTheHigherLayer(t *testing.T) {
	f := newInjectionStackFixture(t)

	run, ws := f.deploy(t, "user-a-wf,default-wf", "--catalog-folder", f.user, "--include-default-catalog")

	requireSuccess(t, run)
	orch := readDeployedAgent(t, ws, "orchestrator")
	assertContains(t, "orchestrator", orch, userInjMarker+"-SHARED", userInjMarker+"-ORCH", userOrchVer)
	assertOmits(t, "orchestrator", orch, defaultInjMarker, defaultOrchVer)
	for _, key := range []string{"user-a-agent", "default-only-agent"} {
		worker := readDeployedAgent(t, ws, key)
		assertContains(t, key, worker, userInjMarker+"-SHARED")
		assertOmits(t, key, worker, defaultInjMarker, userInjMarker+"-ORCH")
	}
}

func TestBinary_HigherLayerSuppliesSharedInjectionsOnly_NothingIsMergedFromTheLowerLayer(t *testing.T) {
	f := newInjectionStackFixture(t)
	writeHarnessInjectionFiles(t, f.user, catalogpaths.HarnessDirNameClaudeCode, "claude-code",
		userInjMarker+"-SHARED", "", "")

	run, ws := f.deploy(t, "user-a-wf", "--catalog-folder", f.user, "--include-default-catalog")

	requireSuccess(t, run)
	orch := readDeployedAgent(t, ws, "orchestrator")
	assertContains(t, "orchestrator", orch, userInjMarker+"-SHARED")
	assertOmits(t, "orchestrator", orch, defaultInjMarker, defaultOrchVer)
}

func TestBinary_OnlyLowerLayerSuppliesInjections_UsesTheLowerLayerContent(t *testing.T) {
	f := newInjectionStackFixture(t)
	removeHarnessInjections(t, f.user)

	run, ws := f.deploy(t, "user-a-wf", "--catalog-folder", f.user, "--include-default-catalog")

	requireSuccess(t, run)
	orch := readDeployedAgent(t, ws, "orchestrator")
	assertContains(t, "orchestrator", orch, defaultInjMarker+"-SHARED", defaultInjMarker+"-ORCH", defaultOrchVer)
	assertOmits(t, "orchestrator", orch, userInjMarker)
	worker := readDeployedAgent(t, ws, "user-a-agent")
	assertContains(t, "user-a-agent", worker, defaultInjMarker+"-SHARED")
}

func TestBinary_UserLayerAloneSuppliesInjections_UsesItsContent(t *testing.T) {
	f := newInjectionStackFixture(t)

	run, ws := f.deploy(t, "user-a-wf", "--catalog-folder", f.user)

	requireSuccess(t, run)
	orch := readDeployedAgent(t, ws, "orchestrator")
	assertContains(t, "orchestrator", orch, userInjMarker+"-SHARED", userInjMarker+"-ORCH", userOrchVer)
	assertOmits(t, "orchestrator", orch, defaultInjMarker)
}

func TestBinary_DefaultCatalogOnly_UsesTheDefaultLayerInjections(t *testing.T) {
	f := newInjectionStackFixture(t)

	run, ws := f.deploy(t, "default-wf")

	requireSuccess(t, run)
	orch := readDeployedAgent(t, ws, "orchestrator")
	assertContains(t, "orchestrator", orch, defaultInjMarker+"-SHARED", defaultInjMarker+"-ORCH", defaultOrchVer)
	assertOmits(t, "orchestrator", orch, userInjMarker)
}

// TestBinary_NoLayerSuppliesTheHarnessInjections_StopsBeforeWritingAndNamesTheItem covers a
// stack with no injections at all (the default catalog on disk has them but is not in the stack)
// and a stack whose layer supplies another harness's injections only.
func TestBinary_NoLayerSuppliesTheHarnessInjections_StopsBeforeWritingAndNamesTheItem(t *testing.T) {
	cases := []struct {
		name    string
		harness string
		prepare func(t *testing.T, user string)
		item    string
	}{
		{"no injections folder at all", "claude-code", removeHarnessInjections,
			catalogpaths.RelHarnessInjectionsDir + "/" + catalogpaths.HarnessDirNameClaudeCode},
		{"only another harness is supplied", "opencode", func(t *testing.T, user string) {
			removeHarnessInjectionDir(t, user, catalogpaths.HarnessDirNameOpenCode)
		}, catalogpaths.RelHarnessInjectionsDir + "/" + catalogpaths.HarnessDirNameOpenCode},
	}
	subcommands := map[string][]string{
		"deploy":    {"deploy", "--auto-confirm", "--workflows", "user-a-wf"},
		"workflows": {"workflows", "--auto-confirm", "--workflows", "user-a-wf"},
	}
	for _, tc := range cases {
		for name, base := range subcommands {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				f := newInjectionStackFixture(t)
				tc.prepare(t, f.user)
				ws := t.TempDir()
				args := append(append([]string{}, base...), "--mosaic-root", f.mosaicRoot,
					"--harness", tc.harness, "--workspace", ws, "--catalog-folder", f.user)

				run := runDeployBinary(t, args...)

				if run.ExitCode == 0 {
					t.Errorf("exit code 0, want a failure: no layer supplies %s and there is no fallback to the MOSAIC root\nstdout: %s",
						tc.item, run.Stdout)
				}
				if out := run.Stdout + run.Stderr; !strings.Contains(out, tc.item) {
					t.Errorf("output does not name the injection item %q\nstdout: %s\nstderr: %s", tc.item, run.Stdout, run.Stderr)
				}
				if n := dirEntryCount(t, ws); n != 0 {
					t.Errorf("workspace holds %d entries after the failed run, want none written", n)
				}
			})
		}
	}
}

func TestBinary_NoLayerSuppliesTheHarnessInjections_OtherHarnessStillDeploys(t *testing.T) {
	f := newInjectionStackFixture(t)
	removeHarnessInjectionDir(t, f.user, catalogpaths.HarnessDirNameOpenCode)

	run, _ := f.deploy(t, "user-a-wf", "--catalog-folder", f.user)

	requireSuccess(t, run)
}
