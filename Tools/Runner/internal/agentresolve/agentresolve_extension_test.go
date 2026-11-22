package agentresolve_test

// Tests for agent resolution driven by the harness's agent file extension:
// identifiers resolve to <id><ext> files of the harness's format, files with
// other extensions are ignored, and the Markdown acceptance (.md and
// .agent.md) is unchanged.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/agentresolve"
)

// ghcpFormat is the catalog data of GHCP CLI: Markdown with the .agent.md extension.
var ghcpFormat = agentfile.Format{AgentsDir: ".github/agents", FormatID: "markdown", Extension: ".agent.md"}

// --- ResolveAll ---

func TestResolveAll_NonMarkdownFormat_ResolvesIdentifiersToExtensionFiles(t *testing.T) {
	f := useTestJSON(t)

	result, err := agentresolve.ResolveAll(testJSONAgentsDir(), f, []string{"planner", "reviewer"})

	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	for _, id := range []string{"planner", "reviewer"} {
		ref, ok := result[id]
		if !ok {
			t.Fatalf("want %q in the result map, got %v", id, result)
		}
		if ref.Identifier != id {
			t.Errorf("Identifier = %q, want %q", ref.Identifier, id)
		}
		if filepath.Base(ref.DefinitionPath) != id+".json" {
			t.Errorf("DefinitionPath = %q, want the %s.json file", ref.DefinitionPath, id)
		}
	}
}

func TestResolveAll_NonMarkdownFormat_IgnoresFilesWithOtherExtensions(t *testing.T) {
	f := useTestJSON(t)

	_, err := agentresolve.ResolveAll(testJSONAgentsDir(), f, []string{"notes"})

	re := asRefusalError(t, err)
	if re.Component != "agentresolve" || !strings.Contains(err.Error(), "notes") {
		t.Errorf("want a missing-agent refusal naming %q from agentresolve, got %v", "notes", err)
	}
}

func TestResolveAll_NonMarkdownFormat_SameStemMarkdownFileIsNotADuplicate(t *testing.T) {
	f := useTestJSON(t)
	dir := t.TempDir()
	writeFiles(t, dir, "foo.json", "foo.md")

	result, err := agentresolve.ResolveAll(dir, f, []string{"foo"})

	if err != nil {
		t.Fatalf("ResolveAll: want the .md file ignored, not a duplicate: %v", err)
	}
	if got := filepath.Base(result["foo"].DefinitionPath); got != "foo.json" {
		t.Errorf("DefinitionPath file = %q, want foo.json", got)
	}
}

func TestResolveAll_NonMarkdownFormat_EmptyIdentifiers_ReturnsEmptyMap(t *testing.T) {
	f := useTestJSON(t)

	result, err := agentresolve.ResolveAll(testJSONAgentsDir(), f, []string{})

	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want an empty map, got %v", result)
	}
}

func TestResolveAll_MarkdownFormatWithOtherExtension_ResolvesThatExtension(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "foo.mdx")

	result, err := agentresolve.ResolveAll(dir, markdownWithExtension(".mdx"), []string{"foo"})

	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if got := filepath.Base(result["foo"].DefinitionPath); got != "foo.mdx" {
		t.Errorf("DefinitionPath file = %q, want foo.mdx", got)
	}
}

func TestResolveAll_MarkdownFormatWithOtherExtension_StillAcceptsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.md", "b.agent.md", "c.mdx")

	result, err := agentresolve.ResolveAll(dir, markdownWithExtension(".mdx"), []string{"a", "b", "c"})

	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("want all three identifiers resolved, got %v", result)
	}
}

func TestResolveAll_MarkdownFormatWithOtherExtension_DuplicateStemsRefuse(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "foo.md", "foo.mdx")

	_, err := agentresolve.ResolveAll(dir, markdownWithExtension(".mdx"), []string{"foo"})

	re := asRefusalError(t, err)
	if re.Component != "agentresolve" {
		t.Errorf("Component = %q, want agentresolve", re.Component)
	}
}

func TestResolveAll_NonMarkdownFormat_MissingIdentifier_RefusesNamingAgentAndDirectory(t *testing.T) {
	f := useTestJSON(t)
	dir := testJSONAgentsDir()

	_, err := agentresolve.ResolveAll(dir, f, []string{"planner", "absent-agent"})

	re := asRefusalError(t, err)
	if re.Resource != "absent-agent" {
		t.Errorf("Resource = %q, want %q", re.Resource, "absent-agent")
	}
	if !strings.Contains(re.Reason, dir) {
		t.Errorf("Reason = %q, want it to name the directory %q", re.Reason, dir)
	}
}

func TestResolveAll_GHCPFormat_AcceptsBothAgentMdAndMd(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.agent.md", "b.md")

	result, err := agentresolve.ResolveAll(dir, ghcpFormat, []string{"a", "b"})

	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want both identifiers resolved, got %v", result)
	}
}

// --- ResolveOne ---

func TestResolveOne_NonMarkdownFormat_ResolvesExtensionFile(t *testing.T) {
	f := useTestJSON(t)

	ref, err := agentresolve.ResolveOne(testJSONAgentsDir(), f, "planner")

	if err != nil {
		t.Fatalf("ResolveOne: %v", err)
	}
	if ref.Identifier != "planner" || filepath.Base(ref.DefinitionPath) != "planner.json" {
		t.Errorf("ref = %+v, want planner resolved to planner.json", ref)
	}
}

func TestResolveOne_NonMarkdownFormat_OtherExtensionOnly_RefusesAsMissing(t *testing.T) {
	f := useTestJSON(t)

	_, err := agentresolve.ResolveOne(testJSONAgentsDir(), f, "notes")

	re := asRefusalError(t, err)
	if re.Resource != "notes" {
		t.Errorf("Resource = %q, want %q", re.Resource, "notes")
	}
}

func TestResolveOne_NonMarkdownFormat_SameStemMarkdownFileIsNotAmbiguous(t *testing.T) {
	f := useTestJSON(t)
	dir := t.TempDir()
	writeFiles(t, dir, "foo.json", "foo.md")

	ref, err := agentresolve.ResolveOne(dir, f, "foo")

	if err != nil {
		t.Fatalf("ResolveOne: want the .md file ignored, not ambiguous: %v", err)
	}
	if filepath.Base(ref.DefinitionPath) != "foo.json" {
		t.Errorf("DefinitionPath = %q, want foo.json", ref.DefinitionPath)
	}
}

func TestResolveOne_MarkdownFormatWithOtherExtension_BothExtensionsAreAmbiguous(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "foo.md", "foo.mdx")

	_, err := agentresolve.ResolveOne(dir, markdownWithExtension(".mdx"), "foo")

	re := asRefusalError(t, err)
	if re.Resource != "foo" {
		t.Errorf("Resource = %q, want %q", re.Resource, "foo")
	}
}

// --- ResolveOrchestrator ---

func TestResolveOrchestrator_NonMarkdownFormat_IdentifierIsFileNameWithoutExtension(t *testing.T) {
	f := useTestJSON(t)
	path := filepath.Join(testJSONAgentsDir(), "orchestrator-script.json")

	ref, err := agentresolve.ResolveOrchestrator(path, f)

	if err != nil {
		t.Fatalf("ResolveOrchestrator: %v", err)
	}
	if ref.Identifier != "orchestrator-script" {
		t.Errorf("Identifier = %q, want %q", ref.Identifier, "orchestrator-script")
	}
	if !filepath.IsAbs(ref.DefinitionPath) || filepath.Base(ref.DefinitionPath) != "orchestrator-script.json" {
		t.Errorf("DefinitionPath = %q, want the absolute path of the .json file", ref.DefinitionPath)
	}
}

func TestResolveOrchestrator_NonMarkdownFormat_WrongExtension_Refuses(t *testing.T) {
	f := useTestJSON(t)
	dir := t.TempDir()
	writeFiles(t, dir, "orchestrator-script.md")

	_, err := agentresolve.ResolveOrchestrator(filepath.Join(dir, "orchestrator-script.md"), f)

	re := asRefusalError(t, err)
	if re.Component != "agentresolve" {
		t.Errorf("Component = %q, want agentresolve", re.Component)
	}
}

func TestResolveOrchestrator_MarkdownFormatWithOtherExtension_ResolvesThatExtension(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "orchestrator-script.mdx")

	ref, err := agentresolve.ResolveOrchestrator(filepath.Join(dir, "orchestrator-script.mdx"), markdownWithExtension(".mdx"))

	if err != nil {
		t.Fatalf("ResolveOrchestrator: %v", err)
	}
	if ref.Identifier != "orchestrator-script" {
		t.Errorf("Identifier = %q, want %q", ref.Identifier, "orchestrator-script")
	}
}

func TestResolveOrchestrator_GHCPFormat_AgentMdStemStripsCompoundExtension(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "orchestrator-script.agent.md")

	ref, err := agentresolve.ResolveOrchestrator(filepath.Join(dir, "orchestrator-script.agent.md"), ghcpFormat)

	if err != nil {
		t.Fatalf("ResolveOrchestrator: %v", err)
	}
	if ref.Identifier != "orchestrator-script" {
		t.Errorf("Identifier = %q, want %q", ref.Identifier, "orchestrator-script")
	}
}
