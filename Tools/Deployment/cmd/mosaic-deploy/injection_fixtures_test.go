package main

// injection_fixtures_test.go holds the fixtures for the harness injections a catalog layer
// supplies: the standard copy every standalone fixture catalog carries (a catalog used as a
// stack layer on its own, or as the only layer, must carry its own HarnessInjections/ because no
// other root is consulted), marker-bearing injection files that name the supplying layer in the
// deployed output, and agent sources with the tool-managed region the injections fill.
// Nothing in this file is a test itself.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// injectionHarnessIDs maps each harness directory name under HarnessInjections/ to the id the
// injection files declare in their frontmatter.
var injectionHarnessIDs = map[string]string{
	catalogpaths.HarnessDirNameClaudeCode: "claude-code",
	catalogpaths.HarnessDirNameGhcpCLI:    "ghcp-cli",
	catalogpaths.HarnessDirNameOpenCode:   "opencode",
	catalogpaths.HarnessDirNameVSCodeGHCP: "vscode-ghcp",
}

// injectionDoc returns an injection file for the harness id with one HarnessConstraints block.
func injectionDoc(harnessID, version, body string) string {
	return "---\nversion: \"" + version + "\"\nharness: " + harnessID + "\n---\n\n# Injections\n\n" +
		"<HarnessConstraints type=\"managed\">\n" + body + "\n</HarnessConstraints>\n"
}

// writeHarnessInjectionCopies gives the catalog root the standard injection files of all four
// built-in harnesses (orchestrator injections version 1.0.0).
func writeHarnessInjectionCopies(t *testing.T, catalogRoot string) {
	t.Helper()
	for dir, id := range injectionHarnessIDs {
		writeHarnessInjectionFiles(t, catalogRoot, dir, id, "Fixture constraint.", "1.0.0", "Fixture constraint.")
	}
}

// writeHarnessInjectionFiles writes the shared and orchestrator injection files of one harness
// directory. An empty orchVersion leaves the orchestrator file out.
func writeHarnessInjectionFiles(t *testing.T, catalogRoot, harnessDir, harnessID, sharedBody, orchVersion, orchBody string) {
	t.Helper()
	dir := catalogpaths.HarnessInjectionsDir(catalogRoot, harnessDir)
	writeStackFile(t, dir, "HarnessInjections.md", []byte(injectionDoc(harnessID, "1.0.0", sharedBody)))
	if orchVersion != "" {
		writeStackFile(t, dir, "HarnessInjectionsOrchestrator.md", []byte(injectionDoc(harnessID, orchVersion, orchBody)))
	}
}

// writeClaudeMarkerInjections gives the catalog root Claude Code injections whose text and
// orchestrator version identify the layer: "<marker>-SHARED", "<marker>-ORCH" and orchVersion.
func writeClaudeMarkerInjections(t *testing.T, catalogRoot, marker, orchVersion string) {
	t.Helper()
	writeHarnessInjectionFiles(t, catalogRoot, catalogpaths.HarnessDirNameClaudeCode, "claude-code",
		marker+"-SHARED", orchVersion, marker+"-ORCH")
}

// removeHarnessInjections deletes the whole HarnessInjections/ folder of the catalog root.
func removeHarnessInjections(t *testing.T, catalogRoot string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(catalogRoot, catalogpaths.RelHarnessInjectionsDir)); err != nil {
		t.Fatalf("remove HarnessInjections of %q: %v", catalogRoot, err)
	}
}

// removeHarnessInjectionDir deletes one harness directory under HarnessInjections/.
func removeHarnessInjectionDir(t *testing.T, catalogRoot, harnessDir string) {
	t.Helper()
	if err := os.RemoveAll(catalogpaths.HarnessInjectionsDir(catalogRoot, harnessDir)); err != nil {
		t.Fatalf("remove injections %q of %q: %v", harnessDir, catalogRoot, err)
	}
}

// addInjectionRegions appends an empty HarnessConstraints region to the orchestrator pair and
// to the named worker agents of the catalog root, so a deploy or render fills it from the
// harness injections.
func addInjectionRegions(t *testing.T, catalogRoot string, agentKeys ...string) {
	t.Helper()
	files := []string{
		filepath.Join("Orchestrator", "orchestrator.md"),
		filepath.Join("Orchestrator", "orchestrator-script.md"),
	}
	for _, key := range agentKeys {
		files = append(files, filepath.Join("Subagents", "Test", key+".md"))
	}
	for _, rel := range files {
		p := filepath.Join(catalogRoot, rel)
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %q: %v", p, err)
		}
		region := "\n<HarnessConstraints type=\"managed\">\n</HarnessConstraints>\n"
		if err := os.WriteFile(p, append(src, []byte(region)...), 0o644); err != nil {
			t.Fatalf("write %q: %v", p, err)
		}
	}
}

// readDeployedAgent returns the text of the Claude Code agent file deployed into the workspace.
func readDeployedAgent(t *testing.T, workspace, key string) string {
	t.Helper()
	p := filepath.Join(workspace, ".claude", "agents", key+".md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read deployed agent %q: %v", p, err)
	}
	return string(b)
}
