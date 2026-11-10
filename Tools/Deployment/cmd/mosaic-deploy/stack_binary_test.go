package main

// stack_binary_test.go runs the built mosaic-deploy binary to verify the composition root
// end to end: which catalog layers a run deploys from, and that stack errors stop a run
// before any file is written and name the offending layer.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// deployArgs returns the dry-run deploy arguments shared by the binary tests.
func deployArgs(mosaicRoot, ws, workflows string, extra ...string) []string {
	args := []string{"deploy", "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--workspace", ws,
		"--workflows", workflows, "--auto-confirm", "--dry-run", "--output", "json"}
	return append(args, extra...)
}

func TestBinary_NoStackInput_DeploysFromTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "default-wf")...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	versions := parseRunSummary(t, run).deployedVersions()
	if versions["orchestrator"] != "1.0.0" || versions["default-only-agent"] != "1.0.0" {
		t.Errorf("deployed agents = %v, want the default catalog's orchestrator and default-only-agent at 1.0.0", versions)
	}
}

func TestBinary_DefaultFlagAlone_DeploysFromTheDefaultCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "default-wf", "--include-default-catalog")...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	if v := parseRunSummary(t, run).deployedVersions(); v["default-only-agent"] != "1.0.0" {
		t.Errorf("deployed agents = %v, want the default catalog's default-only-agent", v)
	}
}

func TestBinary_UserCatalogWithoutDefaultFlag_DeploysFromTheUserCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "user-a-wf", "--catalog-folder", user)...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	versions := parseRunSummary(t, run).deployedVersions()
	if versions["orchestrator"] != "2.0.0" || versions["user-a-agent"] != "2.0.0" {
		t.Errorf("deployed agents = %v, want the user catalog's pair and user-a-agent at 2.0.0", versions)
	}
	if _, ok := versions["default-only-agent"]; ok {
		t.Errorf("deployed agents = %v, must not contain default catalog content", versions)
	}
}

func TestBinary_UserCatalogWithoutDefaultFlag_RejectsADefaultOnlyWorkflow(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "default-wf", "--catalog-folder", user)...)

	if run.ExitCode == 0 {
		t.Errorf("exit code 0, want a failure: default-wf is not in the user catalog and the default layer was left out\nstdout: %s", run.Stdout)
	}
}

func TestBinary_UserCatalogWithDefaultFlag_DeploysFromBothLayersUserWinning(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "user-a-wf,default-wf",
		"--catalog-folder", user, "--include-default-catalog")...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	versions := parseRunSummary(t, run).deployedVersions()
	if versions["orchestrator"] != "2.0.0" {
		t.Errorf("orchestrator version = %q, want 2.0.0 (user layer above default)", versions["orchestrator"])
	}
	if versions["user-a-agent"] != "2.0.0" || versions["default-only-agent"] != "1.0.0" {
		t.Errorf("deployed agents = %v, want user-a-agent from the user layer and default-only-agent from the default layer", versions)
	}
}

func TestBinary_RealDeploy_WritesOnlyUserCatalogAgents(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	run := runDeployBinary(t, "deploy", "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--workspace", ws,
		"--workflows", "user-a-wf", "--auto-confirm", "--output", "json", "--catalog-folder", user)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, stderr: %s", run.ExitCode, run.Stderr)
	}
	agents := filepath.Join(ws, ".claude", "agents")
	if _, err := os.Stat(filepath.Join(agents, "user-a-agent.md")); err != nil {
		t.Errorf("user catalog agent not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agents, "default-only-agent.md")); err == nil {
		t.Error("default catalog agent was written although the default layer was left out")
	}
}

// failingStackLayers returns, per defect, the arguments naming the defective layer(s)
// and the strings the error must contain.
func failingStackLayers(t *testing.T) map[string]struct {
	args  []string
	names []string
} {
	t.Helper()
	bare := t.TempDir()
	dupA := newStackUserCatalog(t, "same-id", "2.0.0")
	dupB := newStackUserCatalog(t, "same-id", "3.0.0")
	half := newIdentifiedCatalogRoot(t, "half-pair")
	writeStackOrchestratorFile(t, half, "orchestrator", "2.0.0")
	type c = struct {
		args  []string
		names []string
	}
	return map[string]c{
		"missing identity": {[]string{"--catalog-folder", bare}, []string{bare}},
		"duplicate id":     {[]string{"--catalog-folder", dupA, "--catalog-folder", dupB}, []string{dupA, dupB}},
		"incomplete pair":  {[]string{"--catalog-folder", half}, []string{half}},
	}
}

func TestBinary_StackError_StopsEverySubcommandBeforeWritingAndNamesTheLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	subcommands := map[string][]string{
		"deploy":    {"deploy", "--harness", "claude-code", "--workflows", "", "--auto-confirm"},
		"update":    {"update", "--harness", "claude-code", "--auto-confirm"},
		"workflows": {"workflows", "--harness", "claude-code", "--workflows", "", "--auto-confirm"},
		"agents":    {"agents", "--harness", "claude-code", "--subagents", "", "--yes"},
		"hooks":     {"hooks", "--harness", "claude-code", "--hooks", "", "--yes"},
	}
	for defect, bad := range failingStackLayers(t) {
		for name, base := range subcommands {
			t.Run(defect+"/"+name, func(t *testing.T) {
				ws := t.TempDir()
				args := append(append([]string{}, base...), "--mosaic-root", mosaicRoot, "--workspace", ws)
				args = append(args, bad.args...)

				run := runDeployBinary(t, args...)

				if run.ExitCode == 0 {
					t.Errorf("exit code 0, want a failure\nstdout: %s", run.Stdout)
				}
				for _, n := range bad.names {
					if !strings.Contains(run.Stderr, filepath.Clean(n)) {
						t.Errorf("stderr does not name the layer %q:\n%s", n, run.Stderr)
					}
				}
				if n := dirEntryCount(t, ws); n != 0 {
					t.Errorf("workspace has %d entries after a stack error, want none", n)
				}
			})
		}
	}
}

func TestBinary_StackError_StopsCheckIndexAndNamesTheLayer(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	bare := t.TempDir()

	run := runDeployBinary(t, "check-index", "--mosaic-root", mosaicRoot, "--catalog-folder", bare)

	if run.ExitCode == 0 {
		t.Errorf("exit code 0, want a failure\nstdout: %s", run.Stdout)
	}
	if !strings.Contains(run.Stderr, filepath.Clean(bare)) {
		t.Errorf("stderr does not name the layer %q:\n%s", bare, run.Stderr)
	}
}

func TestBinary_EmptyStack_StopsBeforeWriting(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "", "--include-default-catalog=false")...)

	if run.ExitCode == 0 {
		t.Errorf("exit code 0, want a failure for a stack with no layers\nstdout: %s", run.Stdout)
	}
	if !strings.Contains(run.Stderr, stackerr.ErrEmptyStack.Error()) {
		t.Errorf("stderr does not report the empty stack:\n%s", run.Stderr)
	}
	if n := dirEntryCount(t, ws); n != 0 {
		t.Errorf("workspace has %d entries, want none", n)
	}
}
