package main

// stack_reuse_helpers_test.go holds the fixtures and binary-run helpers for the persisted-stack
// reuse tests. Nothing in this file is a test itself.
//
// A "persisted workspace" is a temp workspace that a real deploy (with --catalog-folder, which is
// stack input) has already written, so its manifest carries the catalog stack. The reuse tests
// then run further subcommands against it without any stack flag.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// persistedLayer is one entry of the manifest's catalog_stack as stored on disk.
type persistedLayer struct {
	Path      string `yaml:"path"`
	CatalogID string `yaml:"catalog_id"`
	Default   bool   `yaml:"default"`
}

// manifestPath returns the manifest file of a workspace.
func manifestPath(ws string) string {
	return filepath.Join(ws, ".mosaic", "manifest.yaml")
}

// readManifestBytes returns the raw manifest of a workspace.
func readManifestBytes(t *testing.T, ws string) []byte {
	t.Helper()
	b, err := os.ReadFile(manifestPath(ws))
	if err != nil {
		t.Fatalf("read manifest of %s: %v", ws, err)
	}
	return b
}

// readPersistedLayers returns the catalog_stack stored in the workspace manifest.
func readPersistedLayers(t *testing.T, ws string) []persistedLayer {
	t.Helper()
	var doc struct {
		CatalogStack []persistedLayer `yaml:"catalog_stack"`
	}
	if err := yaml.Unmarshal(readManifestBytes(t, ws), &doc); err != nil {
		t.Fatalf("parse manifest of %s: %v", ws, err)
	}
	return doc.CatalogStack
}

// assertPersistedUserLayers fails unless the workspace's stored stack is exactly the given user
// catalog roots (with their ids), highest first, with no default entry.
func assertPersistedUserLayers(t *testing.T, ws string, want ...persistedLayer) {
	t.Helper()
	got := readPersistedLayers(t, ws)
	if len(got) != len(want) {
		t.Fatalf("persisted stack = %+v, want %+v", got, want)
	}
	for i := range want {
		if filepath.Clean(got[i].Path) != filepath.Clean(want[i].Path) || got[i].CatalogID != want[i].CatalogID || got[i].Default {
			t.Errorf("persisted layer %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// assertPersistedDefaultOnly fails unless the stored stack is exactly one default entry.
func assertPersistedDefaultOnly(t *testing.T, ws string) {
	t.Helper()
	got := readPersistedLayers(t, ws)
	if len(got) != 1 || !got[0].Default || got[0].Path != "" || got[0].CatalogID != "" {
		t.Errorf("persisted stack = %+v, want exactly one default entry", got)
	}
}

// runDeployBinaryIn runs the binary with dir as its working directory, so relative flag values
// resolve against dir.
func runDeployBinaryIn(t *testing.T, dir string, args ...string) binaryRun {
	t.Helper()
	cmd := exec.Command(deployBinary(t), args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := binaryRun{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Fatalf("run mosaic-deploy %v in %s: %v", args, dir, err)
	}
	return res
}

// hasAction reports whether the run summary has an action for the artifact kind and key.
func (d runSummaryDoc) hasAction(kind, key string) bool {
	for _, a := range d.Actions {
		if a.Ref.Kind == kind && a.Ref.Key == key {
			return true
		}
	}
	return false
}

// removeAll deletes a fixture folder.
func removeAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}
}

// runInWorkspace runs a subcommand against ws with the fixed harness and MOSAIC root; the
// caller appends the subcommand flags, including any stack flags.
func runInWorkspace(t *testing.T, mosaicRoot, ws string, args ...string) binaryRun {
	t.Helper()
	full := append(append([]string{}, args...), "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--workspace", ws)
	return runDeployBinary(t, full...)
}

// deployWithStack performs a real deploy of workflows into ws with the given stack flags,
// failing the test unless it succeeds. The manifest then persists the stack those flags describe.
func deployWithStack(t *testing.T, mosaicRoot, ws, workflows string, stackFlags ...string) {
	t.Helper()
	args := append([]string{"deploy", "--workflows", workflows, "--auto-confirm", "--output", "json"}, stackFlags...)
	if run := runInWorkspace(t, mosaicRoot, ws, args...); run.ExitCode != 0 {
		t.Fatalf("setup deploy with %v failed (exit %d)\nstderr: %s\nstdout: %s", stackFlags, run.ExitCode, run.Stderr, run.Stdout)
	}
}

// dryRunUpdate runs a dry-run update against ws with the given extra flags and returns the run.
func dryRunUpdate(t *testing.T, mosaicRoot, ws string, extra ...string) binaryRun {
	t.Helper()
	args := append([]string{"update", "--auto-confirm", "--dry-run", "--output", "json"}, extra...)
	return runInWorkspace(t, mosaicRoot, ws, args...)
}

// setUserCatalogVersion rewrites the user catalog's marker content at a new version, so a later
// run's action versions show which catalog it read.
func setUserCatalogVersion(t *testing.T, root, id, version string) {
	t.Helper()
	writeStackOrchestratorPair(t, root, version)
	writeStackAgent(t, root, id+"-agent", id+"-001", version)
	writeStackAgent(t, root, "shared-agent", "shared-001", version)
}

// layerDefect is one way to make the catalog root of a persisted layer stop resolving.
type layerDefect struct {
	name  string
	apply func(t *testing.T, root string)
}

// layerDefects are the three ways a persisted user layer stops resolving.
func layerDefects() []layerDefect {
	return []layerDefect{
		{"missing folder", func(t *testing.T, root string) {
			if err := os.RemoveAll(root); err != nil {
				t.Fatalf("remove layer: %v", err)
			}
		}},
		{"missing identity", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "catalog-identity.yaml")); err != nil {
				t.Fatalf("remove identity: %v", err)
			}
		}},
		{"different id", func(t *testing.T, root string) {
			writeCatalogIdentityFile(t, root, "some-other-id")
		}},
	}
}

// assertStopsNamingLayer fails unless run exited non-zero with the layer root in stderr.
func assertStopsNamingLayer(t *testing.T, run binaryRun, layerRoot string) {
	t.Helper()
	if run.ExitCode == 0 {
		t.Errorf("exit code 0, want a failure for an unresolvable persisted layer\nstdout: %s", run.Stdout)
	}
	if !strings.Contains(run.Stderr, filepath.Clean(layerRoot)) {
		t.Errorf("stderr does not name the layer %q:\n%s", layerRoot, run.Stderr)
	}
}

// assertDoesNotNameLayer fails when stderr mentions the layer root: the run did not read the
// persisted stack.
func assertDoesNotNameLayer(t *testing.T, run binaryRun, layerRoot string) {
	t.Helper()
	if strings.Contains(run.Stderr, filepath.Clean(layerRoot)) {
		t.Errorf("stderr names the persisted layer %q although the run has no workspace to reuse:\n%s", layerRoot, run.Stderr)
	}
}

// newBrokenPersistedWorkspace deploys user catalog "broken-layer" into a fresh workspace and
// then removes the layer folder, so any run that reuses this workspace's stack must stop and
// name the returned layer root.
func newBrokenPersistedWorkspace(t *testing.T, mosaicRoot string) (ws, layerRoot string) {
	t.Helper()
	layerRoot = newStackUserCatalog(t, "broken-layer", "2.0.0")
	ws = t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "broken-layer-wf", "--catalog-folder", layerRoot)
	layerDefects()[0].apply(t, layerRoot)
	return ws, layerRoot
}
