package main

// e2e_opencode_v2_render_test.go runs the built mosaic-deploy binary to verify that
// "render --harness opencode-v2 --output json" produces a V2 agent. Tools/AgentTest drives the
// render subcommand as an external subprocess and depends on this to work. Fixtures are built
// in temporary folders; nothing reads the shipped catalog.

import (
	"path/filepath"
	"testing"
)

func TestBinary_RenderOpenCodeV2_ProducesAV2AgentAndReportsItsPath(t *testing.T) {
	root, ws := newV2MosaicRoot(t), t.TempDir()
	want := filepath.Join(ws, ".opencode", "agents", v2WorkerKey+".md")

	run := runDeployBinary(t, "render", "--mosaic-root", root, "--harness", v2HarnessID,
		"--source-agent", v2WorkerKey, "--workspace", ws, "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("render --harness %s: exit %d\nstdout: %s\nstderr: %s", v2HarnessID, run.ExitCode, run.Stdout, run.Stderr)
	}
	result := decodeJSONMap(t, run.Stdout)
	if got, _ := result["targetHarnessId"].(string); got != v2HarnessID {
		t.Errorf("targetHarnessId = %q, want %q", got, v2HarnessID)
	}
	if got, _ := result["destinationPath"].(string); filepath.Clean(got) != filepath.Clean(want) {
		t.Errorf("destinationPath = %q, want %q", got, want)
	}
	agent := parseV2Agent(t, readWorkspaceFile(t, ws, ".opencode/agents/"+v2WorkerKey+".md"))
	requireV2Shape(t, agent, "subagent")
	if got := agent.effectFor("read", "src/a.go"); got != "allow" {
		t.Errorf("rendered worker read = %q, want allow (it declares file_read)", got)
	}
	if got := agent.effectFor("shell", "ls"); got != "deny" {
		t.Errorf("rendered worker shell = %q, want deny (it does not declare terminal)", got)
	}
}

func TestBinary_RenderOpenCodeV2_ExplicitDestinationAndOrchestratorIsPrimary(t *testing.T) {
	root, dir := newV2MosaicRoot(t), t.TempDir()
	dest := filepath.Join(dir, "orchestrator.md")

	run := runDeployBinary(t, "render", "--mosaic-root", root, "--harness", v2HarnessID,
		"--source-agent", "orchestrator", "--dest", dest, "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("render --harness %s: exit %d\nstdout: %s\nstderr: %s", v2HarnessID, run.ExitCode, run.Stdout, run.Stderr)
	}
	agent := parseV2Agent(t, readWorkspaceFile(t, dir, "orchestrator.md"))
	requireV2Shape(t, agent, "primary")
	if got := agent.effectFor("subagent", v2WorkerKey); got != "allow" {
		t.Errorf("rendered orchestrator dispatch of %q = %q, want allow", v2WorkerKey, got)
	}
}
