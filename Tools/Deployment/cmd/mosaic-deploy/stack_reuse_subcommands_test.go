package main

// stack_reuse_subcommands_test.go runs the built binary to verify which workspace each
// subcommand reuses the persisted stack of. The probe is a persisted layer that no longer
// resolves: a run that reuses the workspace's stack stops and names that layer, a run that does
// not reuse it never mentions the layer.

import (
	"bytes"
	"path/filepath"
	"testing"
)

// writeAgentFile creates a placeholder agent file at path (parents included) and returns it.
func writeAgentFile(t *testing.T, path string) string {
	t.Helper()
	writeStackFile(t, filepath.Dir(path), filepath.Base(path), []byte("---\nname: x\n---\nBody.\n"))
	return path
}

func TestBinary_Workflows_ReusesThePersistedStackOfItsWorkspace(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)

	run := runInWorkspace(t, mosaicRoot, ws, "workflows", "--workflows", "user-a-wf", "--auto-confirm", "--dry-run", "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("workflows without stack input failed (exit %d); user-a-wf exists only in the persisted stack\nstderr: %s", run.ExitCode, run.Stderr)
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userA, CatalogID: "user-a"})
}

func TestBinary_AgentsAndHooks_ReuseThePersistedStackOfTheirWorkspace(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	writeStackHook(t, userA, "user-a-hook")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	cases := map[string]struct {
		args []string
		kind string
		key  string
	}{
		"agents": {[]string{"agents", "--subagents", "user-a-agent", "--yes", "--dry-run", "--output", "json"}, "agent", "user-a-agent"},
		"hooks":  {[]string{"hooks", "--hooks", "user-a-hook", "--yes", "--dry-run", "--output", "json"}, "hook", "user-a-hook"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			run := runInWorkspace(t, mosaicRoot, ws, c.args...)

			if run.ExitCode != 0 {
				t.Fatalf("%s without stack input failed (exit %d); the item exists only in the persisted stack\nstderr: %s", name, run.ExitCode, run.Stderr)
			}
			if !parseRunSummary(t, run).hasAction(c.kind, c.key) {
				t.Errorf("%s run has no %s action for %q, want it deployed from the persisted stack\nstdout: %s", name, c.kind, c.key, run.Stdout)
			}
		})
	}
	assertPersistedUserLayers(t, ws, persistedLayer{Path: userA, CatalogID: "user-a"})
}

func TestBinary_Render_WithWorkspace_ReusesThePersistedStackAndNeverChangesIt(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	before := readManifestBytes(t, ws)

	run := runInWorkspace(t, mosaicRoot, ws, "render", "--source-agent", "user-a-agent", "--workflows", "", "--overwrite", "--output", "json")

	if run.ExitCode != 0 {
		t.Fatalf("render without stack input failed (exit %d); user-a-agent exists only in the persisted stack\nstderr: %s", run.ExitCode, run.Stderr)
	}
	if !bytes.Equal(before, readManifestBytes(t, ws)) {
		t.Error("render changed the workspace manifest")
	}
}

func TestBinary_NonDeployingSubcommands_NeverChangeThePersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	userA := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, "user-a-wf", "--catalog-folder", userA)
	agentFile := writeAgentFile(t, filepath.Join(ws, ".claude", "agents", "harness-only.md"))
	before := readManifestBytes(t, ws)
	cases := map[string][]string{
		"promote":   {"promote", "--file", agentFile, "--harness", "claude-code", "--dry-run", "--output", "json"},
		"transform": {"transform", "--harness", "claude-code", "--target-harness", "opencode", "--path", agentFile, "--dry-run", "--output", "json"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			runDeployBinary(t, append(append([]string{}, args...), "--mosaic-root", mosaicRoot)...)

			if !bytes.Equal(before, readManifestBytes(t, ws)) {
				t.Errorf("%s changed the workspace manifest", name)
			}
		})
	}
}

func TestBinary_WorkspaceSubcommands_ReuseTheStackOfTheirWorkspaceFlag(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	cases := map[string][]string{
		"deploy":    {"deploy", "--workflows", "", "--auto-confirm"},
		"update":    {"update", "--auto-confirm"},
		"workflows": {"workflows", "--workflows", "", "--auto-confirm"},
		"agents":    {"agents", "--subagents", "", "--yes"},
		"hooks":     {"hooks", "--hooks", "", "--yes"},
		"render":    {"render", "--source-agent", "orchestrator", "--overwrite", "--dry-run"},
	}
	for name, base := range cases {
		for _, form := range []string{"space", "equals"} {
			t.Run(name+"/"+form, func(t *testing.T) {
				ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
				args := append([]string{}, base...)
				args = append(args, "--mosaic-root", mosaicRoot, "--harness", "claude-code")
				if form == "space" {
					args = append(args, "--workspace", ws)
				} else {
					args = append(args, "--workspace="+ws)
				}

				run := runDeployBinary(t, args...)

				assertStopsNamingLayer(t, run, layerRoot)
			})
		}
	}
}

func TestBinary_Render_DestWithoutWorkspace_DoesNotReuse(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	dest := filepath.Join(ws, ".claude", "agents", "orchestrator.md")

	run := runDeployBinary(t, "render", "--mosaic-root", mosaicRoot, "--harness", "claude-code",
		"--source-agent", "orchestrator", "--overwrite", "--dest", dest, "--dry-run", "--output", "json")

	assertDoesNotNameLayer(t, run, layerRoot)
	if run.ExitCode != 0 {
		t.Errorf("render --dest without a workspace failed (exit %d); it must use the default layer alone\nstderr: %s", run.ExitCode, run.Stderr)
	}
}

func TestBinary_Promote_ReusesTheStackOfTheWorkspaceContainingFile(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	for _, form := range []string{"space", "equals"} {
		t.Run(form, func(t *testing.T) {
			ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
			file := writeAgentFile(t, filepath.Join(ws, "deep", "er", "harness-only.md"))
			fileFlag := []string{"--file", file}
			if form == "equals" {
				fileFlag = []string{"--file=" + file}
			}

			run := runDeployBinary(t, append([]string{"promote", "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--dry-run"}, fileFlag...)...)

			assertStopsNamingLayer(t, run, layerRoot)
		})
	}
}

func TestBinary_Promote_FileWithoutAWorkspaceAncestor_DoesNotReuse(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	_, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	file := writeAgentFile(t, filepath.Join(t.TempDir(), "harness-only.md"))

	run := runDeployBinary(t, "promote", "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--dry-run", "--file", file)

	assertDoesNotNameLayer(t, run, layerRoot)
}

func TestBinary_Transform_ReusesTheStackOfTheWorkspaceContainingPath(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	cases := map[string]func(ws string) string{
		"path is the workspace itself": func(ws string) string { return ws },
		"path is a file below":         func(ws string) string { return filepath.Join(ws, ".claude", "agents", "a.md") },
		"path is a folder below":       func(ws string) string { return filepath.Join(ws, ".claude", "agents") },
	}
	for name, pathOf := range cases {
		for _, form := range []string{"space", "equals"} {
			t.Run(name+"/"+form, func(t *testing.T) {
				ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
				writeAgentFile(t, filepath.Join(ws, ".claude", "agents", "a.md"))
				pathFlag := []string{"--path", pathOf(ws)}
				if form == "equals" {
					pathFlag = []string{"--path=" + pathOf(ws)}
				}
				args := []string{"transform", "--mosaic-root", mosaicRoot, "--harness", "claude-code", "--target-harness", "opencode", "--dry-run"}

				run := runDeployBinary(t, append(args, pathFlag...)...)

				assertStopsNamingLayer(t, run, layerRoot)
			})
		}
	}
}

func TestBinary_Transform_PathWithoutAWorkspaceAncestor_DoesNotReuse(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	_, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	dir := t.TempDir()
	writeAgentFile(t, filepath.Join(dir, "a.md"))

	run := runDeployBinary(t, "transform", "--mosaic-root", mosaicRoot, "--harness", "claude-code",
		"--target-harness", "opencode", "--dry-run", "--path", dir)

	assertDoesNotNameLayer(t, run, layerRoot)
}

func TestBinary_CheckIndex_NeverReusesAPersistedStack(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	for name, stray := range map[string][]string{
		"workspace flag": {"--workspace", ws},
		"file flag":      {"--file", filepath.Join(ws, "x.md")},
		"path flag":      {"--path", ws},
	} {
		t.Run(name, func(t *testing.T) {
			run := runDeployBinary(t, append([]string{"check-index", "--mosaic-root", mosaicRoot}, stray...)...)

			assertDoesNotNameLayer(t, run, layerRoot)
		})
	}
}

func TestBinary_WorkspaceFlagIsReadOnlyForTheSubcommandThatDeclaresIt(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	file := writeAgentFile(t, filepath.Join(ws, "harness-only.md"))
	cases := map[string][]string{
		"promote with --workspace instead of --file":   {"promote", "--harness", "claude-code", "--workspace", ws},
		"promote with --path instead of --file":        {"promote", "--harness", "claude-code", "--path", ws},
		"transform with --workspace instead of --path": {"transform", "--harness", "claude-code", "--target-harness", "opencode", "--workspace", ws},
		"transform with --file instead of --path":      {"transform", "--harness", "claude-code", "--target-harness", "opencode", "--file", file},
		"update with --file":                           {"update", "--auto-confirm", "--file", file, "--workspace", t.TempDir()},
		"update with --path":                           {"update", "--auto-confirm", "--path", ws, "--workspace", t.TempDir()},
		"deploy with --file":                           {"deploy", "--workflows", "", "--file", file, "--workspace", t.TempDir()},
		"render with --path":                           {"render", "--source-agent", "orchestrator", "--path", ws, "--workspace", t.TempDir()},
		"check-index with --workspace":                 {"check-index", "--workspace", ws},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			run := runDeployBinary(t, append(append([]string{}, args...), "--mosaic-root", mosaicRoot)...)

			assertDoesNotNameLayer(t, run, layerRoot)
		})
	}
}

func TestBinary_FlagValueEqualToASubcommandName_DoesNotChangeTheIdentifiedSubcommand(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	ws, layerRoot := newBrokenPersistedWorkspace(t, mosaicRoot)
	cwd := t.TempDir() // the relative --log-dir value is created below it
	cases := map[string]struct {
		args       []string
		wantReused bool
	}{
		"render after the log-dir value update": {
			[]string{"--log-dir", "update", "render", "--source-agent", "orchestrator", "--overwrite", "--dry-run", "--workspace", ws}, true},
		"check-index after the log-dir value update": {
			[]string{"--log-dir", "update", "check-index", "--workspace", ws}, false},
		"render after the log-dir value check-index": {
			[]string{"--log-dir", "check-index", "render", "--source-agent", "orchestrator", "--overwrite", "--dry-run", "--workspace", ws}, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			args := append([]string{"--mosaic-root", mosaicRoot}, c.args...)
			args = append(args, "--harness", "claude-code")

			run := runDeployBinaryIn(t, cwd, args...)

			if c.wantReused {
				assertStopsNamingLayer(t, run, layerRoot)
			} else {
				assertDoesNotNameLayer(t, run, layerRoot)
			}
		})
	}
}
