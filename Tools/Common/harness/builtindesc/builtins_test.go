package builtindesc_test

// Tests that every embedded builtin descriptor passes Common's validation and
// yields the shared facts other tools rely on.

import (
	"reflect"
	"testing"

	"mosaic-common/harness/builtindesc"
)

func TestBuiltins_EveryEmbeddedDescriptorIsValid(t *testing.T) {
	got, err := builtindesc.Builtins()

	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}
	if len(got) != len(wantIDs) {
		t.Fatalf("Builtins() returned %d descriptors, want %d", len(got), len(wantIDs))
	}
	for i, d := range got {
		if d.ID != wantIDs[i] {
			t.Errorf("descriptor %d: ID = %q, want %q (builtin order)", i, d.ID, wantIDs[i])
		}
	}
}

func TestBuiltins_CLIBackedHarnessesCarryLabelMechanismAndModels(t *testing.T) {
	want := map[string]builtindesc.CLI{
		"claude-code": {Label: "Claude Code CLI", LoadingMechanism: builtindesc.LoadingMechanismName},
		"opencode":    {Label: "OpenCode CLI", LoadingMechanism: builtindesc.LoadingMechanismName},
		"ghcp-cli":    {Label: "GitHub Copilot CLI", LoadingMechanism: builtindesc.LoadingMechanismName},
		"codex":       {Label: "Codex CLI", LoadingMechanism: builtindesc.LoadingMechanismRunnerLoaded},
		"opencode-v2": {Label: "OpenCode V2 CLI", LoadingMechanism: builtindesc.LoadingMechanismName},
	}

	got, err := builtindesc.Builtins()
	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}

	for _, d := range got {
		wantCLI, cliBacked := want[d.ID]
		if !cliBacked {
			if d.CLI != nil {
				t.Errorf("%s: CLI = %+v, want nil (not CLI-backed)", d.ID, *d.CLI)
			}
			continue
		}
		if d.CLI == nil {
			t.Errorf("%s: CLI is nil, want %+v", d.ID, wantCLI)
			continue
		}
		if *d.CLI != wantCLI {
			t.Errorf("%s: CLI = %+v, want %+v", d.ID, *d.CLI, wantCLI)
		}
		if d.Models == nil || len(d.Models.IDs) == 0 {
			t.Errorf("%s: CLI-backed descriptor has no model data", d.ID)
		}
	}
}

func TestBuiltins_EveryDescriptorDeclaresModels(t *testing.T) {
	got, err := builtindesc.Builtins()
	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}

	for _, d := range got {
		if d.Models == nil || len(d.Models.IDs) == 0 || d.Models.FormatHint == "" {
			t.Errorf("%s: models = %+v, want ids and a format hint", d.ID, d.Models)
		}
	}
}

func TestBuiltins_ExposeDirectoriesFormatAndExtension(t *testing.T) {
	type facts struct{ agents, skills, hooks, ext, format string }
	want := map[string]facts{
		"claude-code": {".claude/agents", ".claude/skills", ".claude/hooks", ".md", "markdown"},
		"opencode":    {".opencode/agents", ".opencode/skills", ".opencode/plugins", ".md", "markdown"},
		"ghcp-cli":    {".github/agents", ".github/skills", ".github/hooks", ".agent.md", "markdown"},
		"codex":       {".codex/agents", ".agents/skills", "", ".toml", "codex-toml"},
		"opencode-v2": {".opencode/agents", ".opencode/skills", ".opencode/plugins", ".md", "markdown"},
	}

	got, err := builtindesc.Builtins()
	if err != nil {
		t.Fatalf("Builtins() error: %v", err)
	}

	for _, d := range got {
		w, ok := want[d.ID]
		if !ok {
			continue
		}
		g := facts{d.AgentsDir, d.SkillsDir, d.HooksDir, d.AgentExtension, d.AgentFormatID}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s: got %+v, want %+v", d.ID, g, w)
		}
	}
}
