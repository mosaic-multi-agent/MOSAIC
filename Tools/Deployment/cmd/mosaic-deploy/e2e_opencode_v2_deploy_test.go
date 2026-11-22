package main

// e2e_opencode_v2_deploy_test.go runs the built mosaic-deploy binary to verify that the
// opencode-v2 harness deploys through deploy, update and transform into the V2 layout (agents,
// skills and plugin files), and that deploy never creates or edits a harness config file
// (including opencode.json) and ships nothing to .opencode/tools. Fixtures are built in
// temporary folders; nothing reads the shipped catalog.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinary_DeployOpenCodeV2_WritesAgentsSkillsAndPluginsAtTheV2Paths(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()

	deployV2(t, root, ws)

	for _, rel := range []string{
		v2AgentsDir + "/orchestrator.md", v2AgentsDir + "/" + v2WorkerKey + ".md", v2SkillRel, v2PluginRel, v2HelperRel,
	} {
		if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected deployed file %s: %v", rel, err)
		}
	}
	if got := readWorkspaceFile(t, ws, v2PluginRel); got != v2PluginSrc {
		t.Errorf("plugin file = %q, want the bundle source %q", got, v2PluginSrc)
	}
	if got := readWorkspaceFile(t, ws, v2HelperRel); got != v2HelperSrc {
		t.Errorf("helper file = %q, want the bundle source %q", got, v2HelperSrc)
	}
}

func TestBinary_DeployOpenCodeV2_AgentsCarryAnExplicitModeAndV2Permissions(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()

	deployV2(t, root, ws)

	worker := parseV2Agent(t, readWorkspaceFile(t, ws, v2AgentsDir+"/"+v2WorkerKey+".md"))
	orchestrator := parseV2Agent(t, readWorkspaceFile(t, ws, v2AgentsDir+"/orchestrator.md"))
	requireV2Shape(t, worker, "subagent")
	requireV2Shape(t, orchestrator, "primary")
	for action, want := range map[string]string{"read": "allow", "glob": "allow", "edit": "deny", "shell": "deny", "subagent": "deny"} {
		if got := worker.effectFor(action, "src/a.go"); got != want {
			t.Errorf("worker %s = %q, want %q (it declares file_read and file_search only)", action, got, want)
		}
	}
}

func TestBinary_DeployOpenCodeV2_OrchestratorNeedsNoHandFixingForANativeRun(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()

	deployV2(t, root, ws)

	orchestrator := parseV2Agent(t, readWorkspaceFile(t, ws, v2AgentsDir+"/orchestrator.md"))
	for _, agentID := range []string{v2WorkerKey, "orchestrator-script"} {
		if got := orchestrator.effectFor("subagent", agentID); got != "allow" {
			t.Errorf("dispatching %q = %q, want allow", agentID, got)
		}
	}
	for _, action := range []string{"question", "read", "edit"} {
		if got := orchestrator.effectFor(action, "Orchestration-run/Progress.md"); got != "allow" {
			t.Errorf("orchestrator %s = %q, want allow", action, got)
		}
	}
}

func TestBinary_DeployOpenCodeV2_CreatesNoHarnessConfigAndNothingUnderOpenCodeTools(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()

	deployV2(t, root, ws)

	requireNoHarnessConfigOrTools(t, ws)
	if len(workspaceFiles(t, ws)) == 0 {
		t.Fatal("deploy wrote nothing, so the absence checks proved nothing")
	}
}

func TestBinary_DeployAndUpdateOpenCodeV2_LeaveAnExistingUserConfigUntouched(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()
	writeStackFile(t, ws, "opencode.json", []byte(v2UserConfig))
	writeStackFile(t, ws, filepath.Join(".opencode", "opencode.jsonc"), []byte(v2UserConfig))

	deployV2(t, root, ws)
	run := runV2(t, root, ws, "update", "--auto-confirm", "--output", "json")

	if run.ExitCode != 0 && run.ExitCode != 2 {
		t.Fatalf("update: exit %d\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
	for _, rel := range []string{"opencode.json", ".opencode/opencode.jsonc"} {
		if got := readWorkspaceFile(t, ws, rel); got != v2UserConfig {
			t.Errorf("%s = %q after deploy and update, want it unchanged", rel, got)
		}
	}
	requireNoHarnessConfigOrTools(t, ws, "opencode.json", ".opencode/opencode.jsonc")
}

func TestBinary_UpdateOpenCodeV2_KeepsTheV2PathsAndWritesNoConfig(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()
	deployV2(t, root, ws)

	run := runV2(t, root, ws, "update", "--auto-confirm", "--output", "json")

	if run.ExitCode != 0 && run.ExitCode != 2 {
		t.Fatalf("update: exit %d\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
	if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(v2AgentsDir+"/orchestrator.md"))); err != nil {
		t.Errorf("orchestrator missing after update: %v", err)
	}
	requireNoHarnessConfigOrTools(t, ws)
}

func TestBinary_TransformToOpenCodeV2_WritesV2AgentsAndNoConfig(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()
	setup := runInWorkspace(t, root, ws, "deploy", "--workflows", "default-wf", "--auto-confirm", "--output", "json")
	if setup.ExitCode != 0 {
		t.Fatalf("claude-code setup deploy: exit %d\nstderr: %s", setup.ExitCode, setup.Stderr)
	}

	run := runInWorkspace(t, root, ws, "transform", "--target-harness", v2HarnessID, "--yes")

	if run.ExitCode != 0 && run.ExitCode != 2 {
		t.Fatalf("transform --target-harness %s: exit %d\nstdout: %s\nstderr: %s", v2HarnessID, run.ExitCode, run.Stdout, run.Stderr)
	}
	orchestrator := parseV2Agent(t, readWorkspaceFile(t, ws, v2AgentsDir+"/orchestrator.md"))
	requireV2Shape(t, orchestrator, "primary")
	requireNoHarnessConfigOrTools(t, ws)
}
