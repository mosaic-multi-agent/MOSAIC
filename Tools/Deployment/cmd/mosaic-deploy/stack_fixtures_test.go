package main

// stack_fixtures_test.go builds the MOSAIC root and user catalogs used by the stack-input,
// stack-construction and stack-error tests. Nothing in this file is a test itself.
//
// Marker values: the default catalog supplies "default-only-agent", "default-skill",
// "default-hook" and "default-wf"; every user catalog supplies its own "<id>-agent" and
// "<id>-wf". The agent "shared-agent" exists in the default catalog (version 1.0.0) and in
// user catalogs (version given per catalog), so the winning version names the winning layer.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

const stackFixtureProtocol = `---
version: "1.0"
---

<CommunicationProtocol type="core" name="Subagent">
Subagent text.
</CommunicationProtocol>

<CommunicationProtocol type="core" name="Orchestrator">
Orchestrator text.
</CommunicationProtocol>
`

// stackAgentDoc returns a worker agent document with an explicit numeric id.
func stackAgentDoc(key, numericID, version string) string {
	return "---\nid: \"" + numericID + "\"\nversion: \"" + version + "\"\nname: " + key + "\n" +
		"description: Fixture agent.\nrole: subagent\nrequired_skills: []\n---\nContent.\n"
}

// writeStackAgent writes a worker agent under Subagents/Test of the catalog root.
func writeStackAgent(t *testing.T, catalogRoot, key, numericID, version string) {
	t.Helper()
	writeStackFile(t, catalogRoot, filepath.Join("Subagents", "Test", key+".md"),
		[]byte(stackAgentDoc(key, numericID, version)))
}

// writeStackWorkflow writes a workflow that references the given agent.
func writeStackWorkflow(t *testing.T, catalogRoot, id, agentKey string) {
	t.Helper()
	doc := "---\nid: " + id + "\nversion: \"1.0.0\"\nname: " + id + "\ndescription: A workflow.\n" +
		"hint: Hint.\nreferenced_agents:\n  - " + agentKey + "\n---\nContent.\n"
	writeStackFile(t, catalogRoot, filepath.Join("Workflows", "Test", id+".md"), []byte(doc))
}

// writeStackOrchestratorPair writes both orchestrator files; version names the supplier.
func writeStackOrchestratorPair(t *testing.T, catalogRoot, version string) {
	t.Helper()
	writeStackOrchestratorFile(t, catalogRoot, "orchestrator", version)
	writeStackOrchestratorFile(t, catalogRoot, "orchestrator-script", version)
}

// writeStackOrchestratorFile writes one orchestrator-role file.
func writeStackOrchestratorFile(t *testing.T, catalogRoot, name, version string) {
	t.Helper()
	doc := "---\nversion: \"" + version + "\"\nname: " + name + "\ndescription: Orchestrator.\nrole: orchestrator\n---\nBody.\n"
	writeStackFile(t, catalogRoot, filepath.Join("Orchestrator", name+".md"), []byte(doc))
}

// writeStackSkill writes a skill bundle.
func writeStackSkill(t *testing.T, catalogRoot, key string) {
	t.Helper()
	writeStackFile(t, catalogRoot, filepath.Join("Skills", key, "SKILL.md"),
		[]byte("---\nname: "+key+"\ndescription: Skill.\nversion: \"1.0.0\"\n---\n# Skill\n"))
}

// writeStackHook writes a hook bundle with one supported variant.
func writeStackHook(t *testing.T, catalogRoot, id string) {
	t.Helper()
	y := "schema_version: \"1\"\nid: " + id + "\nversion: 1.0.0\ndescription: Hook " + id + ".\nplaceholder: false\n" +
		"variants:\n  h1:\n    supported: true\n    files:\n      - source: f.sh\n        target: f.sh\n    registration: []\n"
	writeStackFile(t, catalogRoot, filepath.Join("Hooks", id, "hook.yaml"), []byte(y))
	writeStackFile(t, catalogRoot, filepath.Join("Hooks", id, "h1", "f.sh"), []byte("#!/bin/sh\n"))
}

// newStackMosaicRoot returns a MOSAIC root whose default catalog (<root>/Catalog) carries
// identity "stack-default", the orchestrator pair (version 1.0.0), the marker content listed
// in the file comment, and the protocol and bundle documents the app loads from the MOSAIC root.
func newStackMosaicRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	def := filepath.Join(root, "Catalog")
	writeCatalogIdentityFile(t, def, "stack-default")
	writeStackFile(t, root, catalogpaths.MosaicRelSourceFilesFormatFile, []byte("# CatalogFilesFormat\n"))
	writeStackFile(t, root, filepath.Join("Catalog", "CommunicationProtocol.md"), []byte(stackFixtureProtocol))
	writeStackFile(t, root, filepath.FromSlash(catalogpaths.MosaicRelBundleFile), []byte(minimalBundleDoc))
	writeStackHarnessInjections(t, root)
	writeStackOrchestratorPair(t, def, "1.0.0")
	writeStackAgent(t, def, "default-only-agent", "default-only-001", "1.0.0")
	writeStackAgent(t, def, "shared-agent", "shared-001", "1.0.0")
	writeStackWorkflow(t, def, "default-wf", "default-only-agent")
	writeStackSkill(t, def, "default-skill")
	writeStackHook(t, def, "default-hook")
	return root
}

// newStackUserCatalog returns a user catalog root with identity id, the orchestrator pair and
// "shared-agent" at the given version, plus "<id>-agent" and "<id>-wf".
func newStackUserCatalog(t *testing.T, id, version string) string {
	t.Helper()
	root := newIdentifiedCatalogRoot(t, id)
	writeSystemFileCopies(t, root)
	writeHarnessInjectionCopies(t, root)
	writeStackOrchestratorPair(t, root, version)
	writeStackAgent(t, root, id+"-agent", id+"-001", version)
	writeStackAgent(t, root, "shared-agent", "shared-001", version)
	writeStackWorkflow(t, root, id+"-wf", id+"-agent")
	return root
}

// writeStackFile writes content to <root>/<rel>, creating parent directories.
func writeStackFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("create parent of %q: %v", p, err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write %q: %v", p, err)
	}
}

// writeStackHarnessInjections gives the MOSAIC root's default catalog its own copy of the
// harness injections; the stack supplies them from the layer it resolves, not from a fixed root.
func writeStackHarnessInjections(t *testing.T, mosaicRoot string) {
	t.Helper()
	writeHarnessInjectionCopies(t, filepath.Join(mosaicRoot, "Catalog"))
}
