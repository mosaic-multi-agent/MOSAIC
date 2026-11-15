package session_test

// Tests for run start on the codex harness: the Runner loads Codex agent files
// itself on every invocation, so run start takes no snapshot. No per-run copy,
// no backup directory and no transform exist during or after the run, and the
// deployed TOML agent files stay byte-identical.

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

// codexAgentText renders a Codex deployed agent file for name with the given
// instructions.
func codexAgentText(name, instructions string) []byte {
	return []byte("# mosaic_id: " + name + "\n\n" +
		"name = \"" + name + "\"\n" +
		"description = \"test definition\"\n" +
		"model = \"" + testDefinitionModel + "\"\n" +
		"sandbox_mode = \"workspace-write\"\n" +
		"web_search = \"disabled\"\n" +
		"developer_instructions = \"\"\"\n" + instructions + "\"\"\"\n")
}

// writeCodexHarnessDir deploys the linear orchestrator script and two agents as
// Codex TOML files under <workDir>/.codex/agents.
func writeCodexHarnessDir(t *testing.T) (workDir, orchPath string) {
	t.Helper()
	workDir = t.TempDir()
	agentsDir := filepath.Join(workDir, ".codex", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	script, err := os.ReadFile(orchFilePath("linear-orch.md"))
	if err != nil {
		t.Fatalf("read orchestrator fixture: %v", err)
	}
	files := map[string][]byte{
		"orchestrator-script.toml": codexAgentText("orchestrator-script", scriptBody(string(script))),
		"agent-a.toml":             codexAgentText("agent-a", "# Agent: agent-a\n"),
		"agent-b.toml":             codexAgentText("agent-b", "# Agent: agent-b\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(agentsDir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return workDir, filepath.Join(agentsDir, "orchestrator-script.toml")
}

func codexConfig(orchPath, runID string) domain.RunConfig {
	cfg := baseCLIHarnessConfig(orchPath, runID)
	cfg.HarnessID = "codex"
	return cfg
}

// codexSiblings lists every entry of <workDir>/.codex.
func codexSiblings(t *testing.T, workDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(workDir, ".codex"))
	if err != nil {
		t.Fatalf("read .codex: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSession_Start_Codex_RunsWithoutSnapshotOrBackup(t *testing.T) {
	workDir, orchPath := writeCodexHarnessDir(t)
	agentsDir := filepath.Join(workDir, ".codex", "agents")
	original := snapshotAgentFiles(t, agentsDir)
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	var duringRun [][]string
	var contentDuringRun []map[string]string
	cb := &beforeInvokeHarness{delegate: f, before: func(string) {
		duringRun = append(duringRun, codexSiblings(t, workDir))
		contentDuringRun = append(contentDuringRun, snapshotAgentFiles(t, agentsDir))
	}}
	ses := session.New(session.Deps{Harness: cb, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), codexConfig(orchPath, "codex-nosnap-01"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(duringRun) == 0 {
		t.Fatal("want at least one dispatch observation")
	}
	for i, names := range duringRun {
		if !reflect.DeepEqual(names, []string{"agents"}) {
			t.Errorf("before dispatch %d: .codex holds %v, want only agents", i, names)
		}
		if !reflect.DeepEqual(contentDuringRun[i], original) {
			t.Errorf("agent files changed before dispatch %d", i)
		}
	}
	if after := codexSiblings(t, workDir); !reflect.DeepEqual(after, []string{"agents"}) {
		t.Errorf(".codex holds %v after the run, want only agents", after)
	}
	if after := snapshotAgentFiles(t, agentsDir); !reflect.DeepEqual(after, original) {
		t.Error("agent files changed by the run")
	}
}

func TestSession_Start_Codex_AgentReferencesNameDeployedTomlFiles(t *testing.T) {
	workDir, orchPath := writeCodexHarnessDir(t)
	agentsDir := filepath.Join(workDir, ".codex", "agents")
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{Harness: f, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), codexConfig(orchPath, "codex-nosnap-02"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := f.Invocations()
	if len(invs) == 0 {
		t.Fatal("want at least one invocation")
	}
	for _, inv := range invs {
		want := filepath.Join(agentsDir, inv.Agent.Identifier+".toml")
		if filepath.Clean(inv.Agent.DefinitionPath) != filepath.Clean(want) {
			t.Errorf("agent %q: DefinitionPath %q, want deployed file %q", inv.Agent.Identifier, inv.Agent.DefinitionPath, want)
		}
		if strings.Contains(filepath.ToSlash(inv.Agent.DefinitionPath), "agents-runner-") {
			t.Errorf("agent %q resolved into a per-run copy: %q", inv.Agent.Identifier, inv.Agent.DefinitionPath)
		}
	}
}

func TestSession_Start_LoadingMechanismRunnerLoaded_IsNotRefusedAsMisconfigured(t *testing.T) {
	_, orchPath := writeCodexHarnessDir(t)
	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{Harness: f, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), codexConfig(orchPath, "codex-nosnap-03"))

	requireRunStatus(t, got, err, domain.RunCompleted)
}
