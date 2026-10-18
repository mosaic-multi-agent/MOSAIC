package stack_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
)

// newTestMosaicRoot returns a temp MOSAIC-root-shaped directory containing a Catalog/ folder.
func newTestMosaicRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, filepath.Join("Catalog", "CatalogFilesFormat.md"), "# format\n")
	return root
}

// newTestCatalogRoot returns a temp catalog root carrying a valid identity with the given id.
func newTestCatalogRoot(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	writeTestIdentity(t, root, id)
	return root
}

// writeTestIdentity writes catalog-identity.yaml with the given id into root.
func writeTestIdentity(t *testing.T, root, id string) {
	t.Helper()
	mustWrite(t, root, "catalog-identity.yaml", "id: "+id+"\nname: Catalog "+id+"\nversion: \"1.0.0\"\n")
}

// mustWrite writes content to <root>/<rel>, creating parent directories.
func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", p, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// addTestSubagent writes a worker agent. version distinguishes which layer supplied it.
func addTestSubagent(t *testing.T, root, category, key, version string, extra ...string) {
	t.Helper()
	fm := "---\nid: \"" + key + "-id\"\nversion: \"" + version + "\"\nname: " + key + "\n" +
		"description: Test agent.\nrole: subagent\nrequired_skills: []\n" + strings.Join(extra, "") + "---\nBody.\n"
	mustWrite(t, root, filepath.Join("Subagents", category, key+".md"), fm)
}

// addTestSubagentWithTier writes a worker agent declaring a recommended tier.
func addTestSubagentWithTier(t *testing.T, root, category, key, version, tier string) {
	t.Helper()
	addTestSubagent(t, root, category, key, version, "recommended_tier: "+tier+"\ntier_rationale: why "+tier+"\n")
}

// addTestSubagentRequiringSkill writes a worker agent that lists one required skill.
func addTestSubagentRequiringSkill(t *testing.T, root, category, key, skill string) {
	t.Helper()
	fm := "---\nid: \"" + key + "-id\"\nversion: \"1.0.0\"\nname: " + key + "\n" +
		"description: Test agent.\nrole: subagent\nrequired_skills:\n  - " + skill + "\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Subagents", category, key+".md"), fm)
}

// addTestUtility writes a utility agent.
func addTestUtility(t *testing.T, root, key, version string) {
	t.Helper()
	fm := "---\nversion: \"" + version + "\"\nname: " + key + "\ndescription: Utility.\nrole: utility\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("UtilityAgents", key+".md"), fm)
}

// addTestStandalone writes a standalone agent.
func addTestStandalone(t *testing.T, root, key, version string) {
	t.Helper()
	fm := "---\nversion: \"" + version + "\"\nname: " + key + "\ndescription: Standalone.\nrole: standalone\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("StandaloneAgents", key+".md"), fm)
}

// addTestOrchestrator writes orchestrator.md; version identifies the supplying layer.
func addTestOrchestrator(t *testing.T, root, version string) {
	t.Helper()
	fm := "---\nversion: \"" + version + "\"\nname: orchestrator\ndescription: Orchestrator.\nrole: orchestrator\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Orchestrator", "orchestrator.md"), fm)
}

// addTestOrchestratorScript writes orchestrator-script.md; version identifies the supplying layer.
func addTestOrchestratorScript(t *testing.T, root, version string) {
	t.Helper()
	fm := "---\nversion: \"" + version + "\"\nname: orchestrator-script\ndescription: Script orchestrator.\nrole: orchestrator\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Orchestrator", "orchestrator-script.md"), fm)
}

// addTestWorkflow writes a workflow file under Workflows/<category>/<id>.md.
func addTestWorkflow(t *testing.T, root, category, id, version string, referencedAgents ...string) {
	t.Helper()
	fm := "---\nid: " + id + "\nversion: \"" + version + "\"\nname: " + id + "\ndescription: A workflow.\nhint: Hint.\n"
	if len(referencedAgents) > 0 {
		fm += "referenced_agents:\n"
		for _, a := range referencedAgents {
			fm += "  - " + a + "\n"
		}
	}
	mustWrite(t, root, filepath.Join("Workflows", category, id+".md"), fm+"---\nBody.\n")
}

// addTestSkill writes Skills/<key>/SKILL.md plus extra files (relative paths).
func addTestSkill(t *testing.T, root, key, version string, extraFiles ...string) {
	t.Helper()
	mustWrite(t, root, filepath.Join("Skills", key, "SKILL.md"),
		"---\nname: "+key+"\ndescription: Skill.\nversion: \""+version+"\"\n---\n# Skill\n")
	for _, f := range extraFiles {
		mustWrite(t, root, filepath.Join("Skills", key, f), "extra "+f+"\n")
	}
}

// addTestHook writes a hook bundle with one supported variant per name, each with one file
// named <variant>-hook.sh.
func addTestHook(t *testing.T, root, id string, variants ...string) {
	t.Helper()
	y := "schema_version: \"1\"\nid: " + id + "\nversion: 1.0.0\ndescription: Hook " + id + ".\nplaceholder: false\nvariants:\n"
	for _, v := range variants {
		y += "  " + v + ":\n    supported: true\n    files:\n      - source: " + v + "-hook.sh\n        target: " + v + "-hook.sh\n    registration: []\n"
		mustWrite(t, root, filepath.Join("Hooks", id, v, v+"-hook.sh"), "#!/bin/sh\n# "+v+"\n")
	}
	mustWrite(t, root, filepath.Join("Hooks", id, "hook.yaml"), y)
}

// addTestTamperedHook writes a hook bundle whose stored content hash is wrong, so loading it
// reports a hook-hash-mismatch issue.
func addTestTamperedHook(t *testing.T, root, id string) {
	t.Helper()
	mustWrite(t, root, filepath.Join("Hooks", id, "hook.yaml"),
		"schema_version: \"1\"\nid: "+id+"\nversion: 1.0.0\ndescription: d\nplaceholder: false\n"+
			"content_hash: \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"\n"+
			"variants:\n  h1:\n    supported: true\n    files:\n      - source: f.sh\n        target: f.sh\n    registration: []\n")
	mustWrite(t, root, filepath.Join("Hooks", id, "h1", "f.sh"), "#!/bin/sh\n")
}

// newTestStack composes roots (highest first) and fails the test on error.
func newTestStack(t *testing.T, mosaicRoot string, roots ...string) stack.Catalog {
	t.Helper()
	layers := make([]stack.Layer, len(roots))
	for i, r := range roots {
		layers[i] = stack.Layer{Root: r}
	}
	c, err := stack.New(mosaicRoot, layers)
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	return c
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

// underRoot reports whether path is inside root.
func underRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}
