package session_test

// Fixtures for runs on a CLI harness: agent and orchestrator definition files
// that declare the frontmatter a CLI-harness run requires (a model on every
// harness, a name equal to the identifier on Claude Code), plus mutators that
// break one field so a test can misdeclare a single agent.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDefinitionModel = "test-model"

// defFields is the frontmatter a definition file declares.
type defFields struct {
	name      string
	model     string
	omitName  bool
	omitModel bool
}

// defMut changes one field of a definition's frontmatter.
type defMut func(*defFields)

// withoutModel drops the model key.
func withoutModel(f *defFields) { f.omitModel = true }

// withEmptyModel declares the model key with no value.
func withEmptyModel(f *defFields) { f.model = "" }

// withBlankModel declares a model value of whitespace only.
func withBlankModel(f *defFields) { f.model = `"   "` }

// withName declares a name other than the identifier.
func withName(name string) defMut { return func(f *defFields) { f.name = name } }

// withoutName drops the name key.
func withoutName(f *defFields) { f.omitName = true }

// definitionText renders the definition of the agent id on the given harness.
// Agents (not the orchestrator) of OpenCode declare mode: subagent so the
// backup-and-transform rules have a field to rewrite.
func definitionText(harnessID, id string, isAgent bool, muts ...defMut) string {
	f := defFields{name: id, model: testDefinitionModel}
	for _, m := range muts {
		m(&f)
	}
	lines := []string{"---"}
	if !f.omitName {
		lines = append(lines, "name: "+f.name)
	}
	lines = append(lines, "description: test definition")
	if !f.omitModel {
		lines = append(lines, "model: "+f.model)
	}
	if harnessID == "opencode" && isAgent {
		lines = append(lines, "mode: subagent")
	}
	lines = append(lines, "---")
	if isAgent {
		lines = append(lines, "# Agent: "+id)
	}
	return strings.Join(lines, "\n") + "\n"
}

// writeDefinitionFile writes the definition of agent id into dir and returns its path.
func writeDefinitionFile(t *testing.T, dir, harnessID, id string, muts ...defMut) string {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(definitionText(harnessID, id, true, muts...)), 0o600); err != nil {
		t.Fatalf("writeDefinitionFile(%q): %v", id, err)
	}
	return path
}

// writeOrchestratorDefinition writes orchestrator.md into dir: the definition
// frontmatter followed by the workflow content of the named testdata fixture.
// Returns the file path.
func writeOrchestratorDefinition(t *testing.T, dir, harnessID, fixtureName string, muts ...defMut) string {
	t.Helper()
	data, err := os.ReadFile(orchFilePath(fixtureName))
	if err != nil {
		t.Fatalf("writeOrchestratorDefinition: read %s: %v", fixtureName, err)
	}
	path := filepath.Join(dir, "orchestrator.md")
	text := definitionText(harnessID, "orchestrator", false, muts...) + string(data)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("writeOrchestratorDefinition: write: %v", err)
	}
	return path
}
