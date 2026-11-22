package main

// config_seed_binary_test.go runs the built mosaic-deploy binary to verify that startup creates
// the missing config files from the commented templates, leaves existing ones alone, and that a
// command which does not save config (render) keeps its stdout and exit status either way.
// Templates are compared against this tool's own shipped config/ folder; the MOSAIC root is a
// temporary fixture.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seededConfigPath returns the path of a config file under a MOSAIC root.
func seededConfigPath(root, name string) string {
	return filepath.Join(root, "MosaicDeploy", "config", name)
}

// shippedTemplate returns the bytes of a template shipped in this tool's config/ folder.
func shippedTemplate(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "config", name))
	if err != nil {
		t.Fatalf("read shipped template %s: %v", name, err)
	}
	return string(b)
}

// renderWorker runs the non-saving render subcommand for the fixture worker against root,
// writing into a fresh temporary workspace. It returns the run and the workspace path.
func renderWorker(t *testing.T, root string) (binaryRun, string) {
	t.Helper()
	workspace := t.TempDir()
	run := runDeployBinary(t, "render", "--mosaic-root", root, "--harness", v2HarnessID,
		"--source-agent", v2WorkerKey, "--workspace", workspace, "--output", "json")
	return run, workspace
}

func TestBinary_NonSavingCommand_CreatesMissingConfigFilesFromTemplates(t *testing.T) {
	root := newV2MosaicRoot(t)

	run, _ := renderWorker(t, root)

	if run.ExitCode != 0 {
		t.Fatalf("render: exit %d\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
	for _, name := range []string{"user-config.yaml", "tool-config.yaml"} {
		got, err := os.ReadFile(seededConfigPath(root, name))
		if err != nil {
			t.Fatalf("%s was not created at startup: %v", name, err)
		}
		if string(got) != shippedTemplate(t, name) {
			t.Errorf("%s differs from the shipped template", name)
		}
	}
}

func TestBinary_NonSavingCommand_LeavesExistingConfigFileUntouchedAndSeedsTheOther(t *testing.T) {
	root := newV2MosaicRoot(t)
	existing := "# mine\nschema_version: \"1\"\n"
	writeStackFile(t, root, filepath.Join("MosaicDeploy", "config", "user-config.yaml"), []byte(existing))

	run, _ := renderWorker(t, root)

	if run.ExitCode != 0 {
		t.Fatalf("render: exit %d\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
	got, err := os.ReadFile(seededConfigPath(root, "user-config.yaml"))
	if err != nil || string(got) != existing {
		t.Errorf("existing user-config.yaml changed: err=%v content=%q", err, got)
	}
	tool, err := os.ReadFile(seededConfigPath(root, "tool-config.yaml"))
	if err != nil || string(tool) != shippedTemplate(t, "tool-config.yaml") {
		t.Errorf("missing tool-config.yaml was not seeded from the template: err=%v", err)
	}
}

func TestBinary_NonSavingCommand_StdoutAndExitUnchangedBySeeding(t *testing.T) {
	seededRoot := newV2MosaicRoot(t)
	for _, name := range []string{"user-config.yaml", "tool-config.yaml"} {
		writeStackFile(t, seededRoot, filepath.Join("MosaicDeploy", "config", name), []byte(shippedTemplate(t, name)))
	}
	freshRoot := newV2MosaicRoot(t)

	before, beforeWorkspace := renderWorker(t, seededRoot)
	after, afterWorkspace := renderWorker(t, freshRoot)

	if before.ExitCode != 0 || after.ExitCode != 0 {
		t.Fatalf("render exit codes = %d (files present), %d (files seeded); want 0 and 0", before.ExitCode, after.ExitCode)
	}
	if got, want := normalizeRenderOutput(after.Stdout, freshRoot, afterWorkspace), normalizeRenderOutput(before.Stdout, seededRoot, beforeWorkspace); got != want {
		t.Errorf("render stdout changed when config files were seeded:\n got: %s\nwant: %s", got, want)
	}
	if strings.TrimSpace(after.Stderr) != "" {
		t.Errorf("seeding succeeded but stderr is not empty: %s", after.Stderr)
	}
}

func TestBinary_SeedingFailure_WarnsOnStderrAndKeepsExitStatus(t *testing.T) {
	root := newV2MosaicRoot(t)
	// A regular file where the config directory belongs makes creating the files impossible.
	writeStackFile(t, root, filepath.Join("MosaicDeploy", "config"), []byte("not a directory"))

	run, _ := renderWorker(t, root)

	if run.ExitCode != 0 {
		t.Fatalf("render must still succeed when seeding fails: exit %d\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
	if !strings.Contains(strings.ToLower(run.Stderr), "could not create default config files") {
		t.Errorf("a seeding failure should print the could-not-create-default-config-files warning on stderr, got: %q", run.Stderr)
	}
	if strings.Contains(strings.ToLower(run.Stdout), "warning") {
		t.Errorf("the warning must not appear on stdout: %q", run.Stdout)
	}
}

// normalizeRenderOutput replaces the per-run MOSAIC root and workspace in render output so two
// runs against different temp directories compare equal.
func normalizeRenderOutput(out, root, workspace string) string {
	for _, m := range []struct{ path, mask string }{{workspace, "<workspace>"}, {root, "<root>"}} {
		out = strings.ReplaceAll(out, filepath.ToSlash(m.path), m.mask)
		out = strings.ReplaceAll(out, strings.ReplaceAll(m.path, `\`, `\\`), m.mask)
		out = strings.ReplaceAll(out, m.path, m.mask)
	}
	return out
}
