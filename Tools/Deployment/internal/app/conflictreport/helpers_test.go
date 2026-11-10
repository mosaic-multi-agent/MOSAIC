package conflictreport_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/conflictreport"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

func newTestMosaicRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, filepath.Join("Catalog", "CatalogFilesFormat.md"), "# format\n")
	return root
}

// newTestCatalogRoot returns a catalog root with identity id (name "Catalog <id>", version 1.0.0).
func newTestCatalogRoot(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, "catalog-identity.yaml", "id: "+id+"\nname: Catalog "+id+"\nversion: \"1.0.0\"\n")
	return root
}

// withTestOverrides rewrites the identity of root so that it declares overridesBody (YAML lines
// indented by two spaces under "overrides:").
func withTestOverrides(t *testing.T, root, id, overridesBody string) {
	t.Helper()
	mustWrite(t, root, "catalog-identity.yaml",
		"id: "+id+"\nname: Catalog "+id+"\nversion: \"1.0.0\"\noverrides:\n"+overridesBody)
}

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

func addTestSubagent(t *testing.T, root, key, version string) {
	t.Helper()
	fm := "---\nid: \"" + key + "-id\"\nversion: \"" + version + "\"\nname: " + key + "\n" +
		"description: Test agent.\nrole: subagent\nrequired_skills: []\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Subagents", "general", key+".md"), fm)
}

func addTestOrchestratorPair(t *testing.T, root, version string) {
	t.Helper()
	fm := "---\nversion: \"" + version + "\"\nname: orchestrator\ndescription: Orchestrator.\nrole: orchestrator\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Orchestrator", "orchestrator.md"), fm)
	fm = "---\nversion: \"" + version + "\"\nname: orchestrator-script\ndescription: Script.\nrole: orchestrator\n---\nBody.\n"
	mustWrite(t, root, filepath.Join("Orchestrator", "orchestrator-script.md"), fm)
}

func addTestWorkflow(t *testing.T, root, id string, referencedAgents ...string) {
	t.Helper()
	fm := "---\nid: " + id + "\nversion: \"1.0.0\"\nname: " + id + "\ndescription: A workflow.\nhint: Hint.\n"
	if len(referencedAgents) > 0 {
		fm += "referenced_agents:\n"
		for _, a := range referencedAgents {
			fm += "  - " + a + "\n"
		}
	}
	mustWrite(t, root, filepath.Join("Workflows", "general", id+".md"), fm+"---\nBody.\n")
}

func addTestSkill(t *testing.T, root, key string) {
	t.Helper()
	mustWrite(t, root, filepath.Join("Skills", key, "SKILL.md"),
		"---\nname: "+key+"\ndescription: Skill.\nversion: \"1.0.0\"\n---\n# Skill\n")
}

func addTestHook(t *testing.T, root, id string) {
	t.Helper()
	y := "schema_version: \"1\"\nid: " + id + "\nversion: 1.0.0\ndescription: Hook.\nplaceholder: false\nvariants:\n" +
		"  h1:\n    supported: true\n    files:\n      - source: h1-hook.sh\n        target: h1-hook.sh\n    registration: []\n"
	mustWrite(t, root, filepath.Join("Hooks", id, "h1", "h1-hook.sh"), "#!/bin/sh\n")
	mustWrite(t, root, filepath.Join("Hooks", id, "hook.yaml"), y)
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

// buildFor runs Build over the stack's shadow records and workflow lookup.
func buildFor(c stack.Catalog, selected, deployed []string) domain.ConflictReport {
	return conflictreport.Build(conflictreport.Input{
		Shadows:           c.Shadows(),
		Workflow:          c.Workflow,
		SelectedWorkflows: selected,
		DeployedWorkflows: deployed,
	})
}

func identityOf(id string) domain.CatalogIdentity {
	return domain.CatalogIdentity{ID: id, Name: "Catalog " + id, Version: "1.0.0"}
}

func agentItem(key string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: key}
}

// lineFor returns the single line for item and fails the test when there is not exactly one.
func lineFor(t *testing.T, r domain.ConflictReport, item domain.CatalogItem) domain.ConflictLine {
	t.Helper()
	var found []domain.ConflictLine
	for _, l := range r.Lines {
		if l.Item == item {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one line for %+v, got %d (report: %+v)", item, len(found), r.Lines)
	}
	return found[0]
}

func joined(s []string) string { return strings.Join(s, ",") }
