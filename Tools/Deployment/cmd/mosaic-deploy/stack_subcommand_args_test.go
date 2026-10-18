package main

// stack_subcommand_args_test.go verifies that every subcommand that loads a catalog, and the
// complete argument sets the Runner and AgentTest pass, resolve a user catalog without the
// default layer when --include-default-catalog is absent, and with the default layer at the
// bottom when it is present.

import (
	"path/filepath"
	"testing"
)

// subcommandArgs returns one argument vector per catalog-loading subcommand, each naming
// the user catalog dir and none naming the default-layer flag.
func subcommandArgs(dir, ws string) map[string][]string {
	return map[string][]string{
		"deploy":              {"deploy", "--catalog-folder", dir, "--harness", "claude-code", "--workspace", ws},
		"deploy auto-confirm": {"deploy", "--harness", "claude-code", "--workspace", ws, "--auto-confirm", "--catalog-folder=" + dir},
		"deploy dry-run":      {"--catalog-folder", dir, "deploy", "--harness", "claude-code", "--workspace", ws, "--dry-run"},
		"update":              {"update", "--catalog-folder", dir, "--harness", "claude-code", "--workspace", ws},
		"workflows":           {"workflows", "--catalog-folder", dir, "--harness", "claude-code", "--workspace", ws, "--workflows", "user-a-wf"},
		"agents":              {"agents", "--catalog-folder", dir, "--harness", "claude-code", "--workspace", ws, "--subagents", ""},
		"hooks":               {"hooks", "--catalog-folder", dir, "--harness", "claude-code", "--workspace", ws, "--hooks", ""},
		"render":              {"render", "--catalog-folder", dir, "--source-agent", "user-a-agent", "--harness", "claude-code", "--workspace", ws},
		"promote":             {"promote", "--catalog-folder", dir, "--file", "x.md", "--harness", "claude-code"},
		"transform":           {"transform", "--catalog-folder", dir, "--harness", "claude-code", "--target-harness", "opencode", "--path", "x.md"},
		"check-index":         {"check-index", "--catalog-folder", dir},
	}
}

// consumerArgs returns the complete argument sets the external tools pass, taken from their
// builder functions. Each names the user catalog via --catalog-folder and no default-layer flag.
func consumerArgs(dir, ws, mosaicRoot string) map[string][]string {
	return map[string][]string{
		"AgentTest deploy": {"deploy", "--harness", "claude-code", "--workspace", ws, "--workflows", "user-a-wf",
			"--auto-confirm", "--output", "json", "--mosaic-root", mosaicRoot, "--catalog-folder", dir},
		"AgentTest deploy dry-run": {"deploy", "--harness", "claude-code", "--workspace", ws, "--workflows", "user-a-wf",
			"--auto-confirm", "--dry-run", "--output", "json", "--catalog-folder", dir},
		"AgentTest deploy empty workflows": {"deploy", "--harness", "claude-code", "--workspace", ws, "--workflows", "",
			"--auto-confirm", "--output", "json", "--catalog-folder", dir},
		"AgentTest render source-agent": {"render", "--source-agent", "user-a-agent", "--harness", "claude-code",
			"--workspace", ws, "--overwrite", "--output", "json", "--catalog-folder", dir},
		"AgentTest render empty workflows": {"render", "--source-agent", "user-a-agent", "--harness", "claude-code",
			"--workspace", ws, "--workflows", "", "--overwrite", "--output", "json", "--catalog-folder", dir},
		"AgentTest render source path": {"render", "--source", "agent.md", "--harness", "claude-code",
			"--workspace", ws, "--overwrite", "--output", "json", "--catalog-folder", dir},
		"Runner deploy": {"deploy", "--catalog-folder", dir, "--workspace", ws, "--harness", "claude-code",
			"--workflows", "user-a-wf", "--auto-confirm"},
		"Runner deploy empty infrastructure": {"deploy", "--catalog-folder", dir, "--workspace", ws, "--harness", "claude-code",
			"--workflows", "user-a-wf", "--infrastructure", "", "--auto-confirm"},
		"Runner deploy empty workflows with mosaic root": {"deploy", "--mosaic-root", mosaicRoot, "--catalog-folder", dir,
			"--workspace", ws, "--harness", "claude-code", "--workflows", "", "--auto-confirm"},
	}
}

// assertUserLayerAlone fails unless the stack for args is the user catalog dir alone and
// supplies none of the default catalog's content.
func assertUserLayerAlone(t *testing.T, mosaicRoot, dir string, args []string) {
	t.Helper()
	cat := stackFor(t, mosaicRoot, args...)

	layers := cat.Layers()
	if len(layers) != 1 || layers[0].Default || filepath.Clean(layers[0].Root) != filepath.Clean(absOf(t, dir)) {
		t.Fatalf("Layers() = %+v, want the user catalog %q alone", layers, dir)
	}
	if _, ok := cat.Agent("user-a-agent"); !ok {
		t.Error("user catalog agent missing")
	}
	for _, key := range []string{"default-only-agent"} {
		if _, ok := cat.Agent(key); ok {
			t.Errorf("default catalog agent %q resolved without the default-layer flag", key)
		}
	}
	if _, ok := cat.Skill("default-skill"); ok {
		t.Error("default skill merged into the user catalog")
	}
	if _, ok := cat.Hook("default-hook"); ok {
		t.Error("default hook merged into the user catalog")
	}
}

func TestStackFromArgs_EverySubcommand_UserCatalogWithoutDefaultFlagResolvesAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	dir := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	for name, args := range subcommandArgs(dir, ws) {
		t.Run(name, func(t *testing.T) {
			assertUserLayerAlone(t, mosaicRoot, dir, args)
		})
	}
}

func TestStackFromArgs_EverySubcommand_DefaultFlagPlacesTheDefaultLayerAtTheBottom(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	dir := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	for name, args := range subcommandArgs(dir, ws) {
		t.Run(name, func(t *testing.T) {
			cat := stackFor(t, mosaicRoot, append(args, "--include-default-catalog")...)

			layers := cat.Layers()
			if len(layers) != 2 || layers[0].Default || !layers[1].Default {
				t.Fatalf("Layers() = %+v, want [user, default]", layers)
			}
			if _, ok := cat.Agent("default-only-agent"); !ok {
				t.Error("default catalog agent missing although the flag was given")
			}
		})
	}
}

func TestStackFromArgs_ConsumerArgumentSets_ResolveFromTheUserCatalogAlone(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	dir := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	for name, args := range consumerArgs(dir, ws, mosaicRoot) {
		t.Run(name, func(t *testing.T) {
			assertUserLayerAlone(t, mosaicRoot, dir, args)
		})
	}
}
