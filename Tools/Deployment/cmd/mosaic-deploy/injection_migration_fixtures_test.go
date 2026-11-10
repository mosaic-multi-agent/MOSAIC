package main

// injection_migration_fixtures_test.go holds the fixture workspace for the injection migration
// tests: a small catalog modelled on the shipped one (subagents with and without a harness
// region, both orchestrators) in the layout before and after the move to two harness regions,
// and the conversion of a freshly deployed workspace into the legacy layout a user has today.
// Nothing here reads the shipped catalog. Nothing in this file is a test itself.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

const (
	migSharedOldVersion = "1.0.0"
	migSharedNewVersion = "1.4.0"
	migOrchOldVersion   = "1.2.1"
	migOrchNewVersion   = "1.3.0"

	migOldRule      = "- Old shared rule: keep answers short."
	migHeredocRule  = "- No Heredocs or Stdin Scripts: write a script file instead."
	migOrchRule     = "- Orchestrator rule: end every task description with the skills sentence."
	migProjectNote  = "PROJECT-NOTE-"
	migWorkflowID   = "mig-wf"
	migOrchestrator = "orchestrator"
	migOrchScript   = "orchestrator-script"
)

// migProtocolRegion is the managed protocol region every fixture source carries, so that the
// deployed files carry the protocol stamp and agents are not stale for that reason.
const migProtocolRegion = "<CommunicationProtocol type=\"managed\">\n</CommunicationProtocol>\n"

var (
	migRegionAgents   = []string{"mig-region-a", "mig-region-b", "mig-region-c"}
	migNoRegionAgents = []string{"mig-plain-a", "mig-plain-b"}
	migOrchestrators  = []string{migOrchestrator, migOrchScript}
)

// migAgentSource returns a worker source, with the shared harness region when withRegion is set.
func migAgentSource(key string, withRegion bool) string {
	doc := "---\nid: \"" + key + "-001\"\nversion: \"1.0.0\"\nname: " + key + "\n" +
		"description: Fixture agent.\nrole: subagent\nrequired_skills: []\n---\n\n" +
		"<Identity type=\"core\">\nStatic identity of " + key + ".\n" +
		"<IdentityExtension type=\"project\">\n</IdentityExtension>\n</Identity>\n\n" + migProtocolRegion
	if withRegion {
		doc += "\n<Constraints type=\"core\">\nStatic constraints.\n" +
			"<HarnessConstraints type=\"managed\">\n</HarnessConstraints>\n</Constraints>\n"
	}
	return doc
}

// migOrchestratorSource returns an orchestrator source; twoRegions selects the new layout.
func migOrchestratorSource(name, version string, twoRegions bool) string {
	doc := "---\nversion: \"" + version + "\"\nname: " + name + "\ndescription: Orchestrator.\nrole: orchestrator\n---\n\n" +
		"<Identity type=\"core\">\nStatic identity of " + name + ".\n" +
		"<IdentityExtension type=\"project\">\n</IdentityExtension>\n</Identity>\n\n" + migProtocolRegion +
		"\n<Constraints type=\"core\">\nStatic constraints.\n" +
		"<HarnessConstraints type=\"managed\">\n</HarnessConstraints>\n"
	if twoRegions {
		doc += "<OrchestratorHarnessConstraints type=\"managed\">\n</OrchestratorHarnessConstraints>\n"
	}
	return doc + "</Constraints>\n"
}

// migWorkflowSource references every fixture agent.
func migWorkflowSource() string {
	doc := "---\nid: " + migWorkflowID + "\nversion: \"1.0.0\"\nname: " + migWorkflowID + "\ndescription: A workflow.\n" +
		"hint: Hint.\nreferenced_agents:\n"
	for _, key := range append(append([]string{}, migRegionAgents...), migNoRegionAgents...) {
		doc += "  - " + key + "\n"
	}
	return doc + "---\nContent.\n"
}

// migInjectionFile returns a Claude Code injection file with one region of the given name.
func migInjectionFile(version, regionName string, bullets ...string) string {
	return "---\nversion: \"" + version + "\"\nharness: claude-code\n---\n\n# Injections\n\n" +
		"<" + regionName + " type=\"managed\">\n" + strings.Join(bullets, "\n") + "\n</" + regionName + ">\n"
}

// migWriteInjections writes the Claude Code shared and orchestrator injection files.
func migWriteInjections(t *testing.T, catalogRoot, sharedVersion, orchVersion, orchRegionName string, shared []string) {
	t.Helper()
	dir := catalogpaths.HarnessInjectionsDir(catalogRoot, catalogpaths.HarnessDirNameClaudeCode)
	writeStackFile(t, dir, "HarnessInjections.md", []byte(migInjectionFile(sharedVersion, "HarnessConstraints", shared...)))
	writeStackFile(t, dir, "HarnessInjectionsOrchestrator.md",
		[]byte(migInjectionFile(orchVersion, orchRegionName, migOrchRule)))
}

// migWriteCatalog replaces the sources of the default catalog of the mosaic root with the
// fixture workspace model, in the layout before (migrated false) or after the fix.
func migWriteCatalog(t *testing.T, mosaicRoot string, migrated bool) {
	t.Helper()
	def := filepath.Join(mosaicRoot, "Catalog")
	for _, rel := range []string{"Subagents", "Workflows"} {
		if err := os.RemoveAll(filepath.Join(def, rel)); err != nil {
			t.Fatalf("clear %s: %v", rel, err)
		}
	}
	for _, key := range migRegionAgents {
		writeStackFile(t, def, filepath.Join("Subagents", "Test", key+".md"), []byte(migAgentSource(key, true)))
	}
	for _, key := range migNoRegionAgents {
		writeStackFile(t, def, filepath.Join("Subagents", "Test", key+".md"), []byte(migAgentSource(key, false)))
	}
	writeStackFile(t, def, filepath.Join("Workflows", "Test", migWorkflowID+".md"), []byte(migWorkflowSource()))
	orchVersion, scriptVersion := "8.2.2", "3.3.0"
	if migrated {
		orchVersion, scriptVersion = "8.3.0", "3.4.0"
		migWriteInjections(t, def, migSharedNewVersion, migOrchNewVersion, "OrchestratorHarnessConstraints",
			[]string{migOldRule, migHeredocRule})
	} else {
		migWriteInjections(t, def, migSharedOldVersion, migOrchOldVersion, "HarnessConstraints", []string{migOldRule})
	}
	writeStackFile(t, def, filepath.Join("Orchestrator", "orchestrator.md"),
		[]byte(migOrchestratorSource(migOrchestrator, orchVersion, migrated)))
	writeStackFile(t, def, filepath.Join("Orchestrator", "orchestrator-script.md"),
		[]byte(migOrchestratorSource(migOrchScript, scriptVersion, migrated)))
}

var (
	migSharedRegionRe = regexp.MustCompile(`(?s)<HarnessConstraints[^>]*>.*?</HarnessConstraints>`)
	migOrchRegionRe   = regexp.MustCompile(`(?s)<OrchestratorHarnessConstraints[^>]*>.*?</OrchestratorHarnessConstraints>\n?`)
	migVersionLineRe  = regexp.MustCompile(`(?m)^mosaic_version: .*\n`)
)

// migDeployedPath is the workspace path of a deployed Claude Code agent.
func migDeployedPath(ws, key string) string {
	return filepath.Join(ws, ".claude", "agents", key+".md")
}

// migReadDeployed returns the deployed text of an agent.
func migReadDeployed(t *testing.T, ws, key string) string {
	t.Helper()
	b, err := os.ReadFile(migDeployedPath(ws, key))
	if err != nil {
		t.Fatalf("read deployed %s: %v", key, err)
	}
	return string(b)
}

// migWriteDeployed overwrites the deployed text of an agent.
func migWriteDeployed(t *testing.T, ws, key, text string) {
	t.Helper()
	if err := os.WriteFile(migDeployedPath(ws, key), []byte(text), 0o644); err != nil {
		t.Fatalf("write deployed %s: %v", key, err)
	}
}

// migSharedRegionTag returns the version attribute of the HarnessConstraints region of a
// deployed file, "" when the region is absent or untagged.
func migSharedRegionTag(text string) string {
	open := migSharedRegionRe.FindString(text)
	m := regexp.MustCompile(`^<HarnessConstraints[^>]*version="([^"]*)"`).FindStringSubmatch(open)
	if m == nil {
		return ""
	}
	return m[1]
}

// migToLegacyLayout rewrites a deployed orchestrator, which has two harness regions, into the
// layout deployed before the fix: one HarnessConstraints region tagged with the orchestrator
// injection file's old version that holds the shared rules and the orchestrator rule together.
func migToLegacyLayout(t *testing.T, text string) string {
	t.Helper()
	legacy := migSharedRegionRe.ReplaceAllString(text,
		"<HarnessConstraints type=\"managed\" version=\""+migOrchOldVersion+"\">\n"+migOldRule+"\n"+migOrchRule+"\n</HarnessConstraints>")
	legacy = migOrchRegionRe.ReplaceAllString(legacy, "")
	if strings.Contains(legacy, "<OrchestratorHarnessConstraints") {
		t.Fatalf("legacy conversion left the second region in place:\n%s", legacy)
	}
	return legacy
}

// migWithProjectNote fills the project IdentityExtension region of a deployed file with a
// note the project owns.
func migWithProjectNote(t *testing.T, text, key string) string {
	t.Helper()
	const open = "<IdentityExtension type=\"project\">\n"
	if !strings.Contains(text, open) {
		t.Fatalf("deployed %s has no project region:\n%s", key, text)
	}
	return strings.Replace(text, open, open+migProjectNote+key+"\n", 1)
}

// migStripHarnessRegions removes the version line and both harness regions of a deployed file,
// so what remains is the content an injection-only update must leave alone.
func migStripHarnessRegions(text string) string {
	out := migOrchRegionRe.ReplaceAllString(text, "")
	out = migSharedRegionRe.ReplaceAllString(out, "<HarnessConstraints/>")
	return migVersionLineRe.ReplaceAllString(out, "")
}
