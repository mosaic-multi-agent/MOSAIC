package catalog_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
)

// writeKeyFixtureAgent writes a minimal subagent at
// <root>/Catalog/Subagents/Utility/<fileName> with the given display name.
func writeKeyFixtureAgent(t *testing.T, root, fileName, name string) {
	t.Helper()
	mustMkdir(t, root, "Catalog", "Subagents", "Utility")
	fm := "---\n" +
		"id: \"1\"\n" +
		"version: \"1.0.0\"\n" +
		"name: " + name + "\n" +
		"description: Synthetic agent for key enumeration tests.\n" +
		"role: subagent\n" +
		"---\nContent.\n"
	mustWriteFile(t, root, filepath.Join("Catalog", "Subagents", "Utility", fileName), []byte(fm))
}

func TestCatalogAgents_AgentMdFileName_KeyDropsWholeSuffix(t *testing.T) {
	root := makeTempMosaicRoot(t)
	writeKeyFixtureAgent(t, root, "x.agent.md", "x")
	writeKeyFixtureAgent(t, root, "y.md", "y")

	cat, err := catalog.Load(root, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	if _, ok := cat.Agent("x"); !ok {
		t.Error(`Agent("x") not found for file x.agent.md`)
	}
	if _, ok := cat.Agent("x.agent"); ok {
		t.Error(`Agent("x.agent") found; the .agent.md suffix must be removed as a whole`)
	}
}

func TestCatalogAgents_PlainMdFileName_KeyUnchanged(t *testing.T) {
	root := makeTempMosaicRoot(t)
	writeKeyFixtureAgent(t, root, "y.md", "y")

	cat, err := catalog.Load(root, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	if _, ok := cat.Agent("y"); !ok {
		t.Error(`Agent("y") not found for file y.md`)
	}
}
