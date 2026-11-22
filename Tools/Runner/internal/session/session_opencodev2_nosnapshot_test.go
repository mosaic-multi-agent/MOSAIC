package session_test

// Tests for run start on the opencode-v2 harness: V2 agents run in place. Run
// start takes no snapshot: no backup directory, no per-run copy and no rewrite
// of the deployed agent files, which carry mode: subagent and must keep it.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// writeOpenCodeV2HarnessDir deploys the linear orchestrator script and two
// subagents under <workDir>/.opencode/agents with mode: subagent frontmatter.
func writeOpenCodeV2HarnessDir(t *testing.T) (workDir, orchPath string) {
	t.Helper()
	workDir = t.TempDir()
	agentsDir := filepath.Join(workDir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	orchPath = writeOrchestratorDefinition(t, agentsDir, "opencode", "linear-orch.md")
	// The V1 writer adds mode: subagent to agents, the shape a V1 transform rewrites.
	writeDefinitionFile(t, agentsDir, "opencode", "agent-a")
	writeDefinitionFile(t, agentsDir, "opencode", "agent-b")
	return workDir, orchPath
}

func openCodeV2Config(orchPath, runID string) domain.RunConfig {
	cfg := baseCLIHarnessConfig(orchPath, runID)
	cfg.HarnessID = "opencode-v2"
	return cfg
}

// opencodeDirEntries lists every entry of <workDir>/.opencode.
func opencodeDirEntries(t *testing.T, workDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(workDir, ".opencode"))
	if err != nil {
		t.Fatalf("read .opencode: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSession_Start_OpenCodeV2_RunsInPlaceWithoutSnapshotOrBackup(t *testing.T) {
	workDir, orchPath := writeOpenCodeV2HarnessDir(t)
	agentsDir := filepath.Join(workDir, ".opencode", "agents")
	original := snapshotAgentFiles(t, agentsDir)
	if !strings.Contains(original["agent-a.md"], "mode: subagent") {
		t.Fatalf("fixture agent lacks mode: subagent: %q", original["agent-a.md"])
	}
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	var siblingsDuringRun [][]string
	var contentDuringRun []map[string]string
	cb := &beforeInvokeHarness{delegate: f, before: func(string) {
		siblingsDuringRun = append(siblingsDuringRun, opencodeDirEntries(t, workDir))
		contentDuringRun = append(contentDuringRun, snapshotAgentFiles(t, agentsDir))
	}}
	ses := session.New(session.Deps{Harness: cb, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), openCodeV2Config(orchPath, "ocv2-nosnap-01"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(siblingsDuringRun) == 0 {
		t.Fatal("want at least one dispatch observation")
	}
	for i, names := range siblingsDuringRun {
		if !reflect.DeepEqual(names, []string{"agents"}) {
			t.Errorf("before dispatch %d: .opencode holds %v, want only agents (no backup directory)", i, names)
		}
		if !reflect.DeepEqual(contentDuringRun[i], original) {
			t.Errorf("agent files were rewritten before dispatch %d", i)
		}
	}
	if after := opencodeDirEntries(t, workDir); !reflect.DeepEqual(after, []string{"agents"}) {
		t.Errorf(".opencode holds %v after the run, want only agents", after)
	}
	if after := snapshotAgentFiles(t, agentsDir); !reflect.DeepEqual(after, original) {
		t.Error("agent files changed by the run")
	}
}

func TestSession_Start_OpenCodeV2_AgentReferencesNameDeployedFiles(t *testing.T) {
	workDir, orchPath := writeOpenCodeV2HarnessDir(t)
	agentsDir := filepath.Join(workDir, ".opencode", "agents")
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{Harness: f, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), openCodeV2Config(orchPath, "ocv2-nosnap-02"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := f.Invocations()
	if len(invs) == 0 {
		t.Fatal("want at least one invocation")
	}
	for _, inv := range invs {
		want := filepath.Join(agentsDir, inv.Agent.Identifier+".md")
		if filepath.Clean(inv.Agent.DefinitionPath) != filepath.Clean(want) {
			t.Errorf("agent %q: DefinitionPath %q, want deployed file %q", inv.Agent.Identifier, inv.Agent.DefinitionPath, want)
		}
		if strings.Contains(filepath.ToSlash(inv.Agent.DefinitionPath), "agents-runner-") {
			t.Errorf("agent %q resolved into a per-run copy: %q", inv.Agent.Identifier, inv.Agent.DefinitionPath)
		}
	}
}
