package app_test

// nested_project_default_test.go covers a generic agent whose managed region contains a project
// region with content. Render and deploy must accept it and carry that content as the default.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

const nestedDefaultText = "Default text kept from the promoted generic."

func nestedDefaultGenericBytes() []byte {
	return []byte("---\n" +
		"id: \"48\"\n" +
		"version: \"1.0.0\"\n" +
		"name: nested-default-agent\n" +
		"description: A generic agent with a nested project default.\n" +
		"role: subagent\n" +
		"model: \"{model-identifier}\"\n" +
		"tools: []\n" +
		"---\n" +
		"<Identity type=\"core\">\n" +
		"You are the nested default agent.\n" +
		"</Identity>\n" +
		"<Constraints type=\"core\">\n" +
		"<HarnessConstraints type=\"managed\">\n" +
		"<ProjectConstraints type=\"project\">\n" +
		nestedDefaultText + "\n" +
		"</ProjectConstraints>\n" +
		"</HarnessConstraints>\n" +
		"</Constraints>\n")
}

func TestRenderAgent_NestedProjectContent_RendersAndCarriesContent(t *testing.T) {
	svc := app.New(newRenderDeps(t))
	src := writeRenderSourceFile(t, t.TempDir(), "nested-default-agent.md", nestedDefaultGenericBytes())
	dest := filepath.Join(t.TempDir(), "nested-default-agent.md")

	_, err := svc.RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, dest))
	if err != nil {
		t.Fatalf("RenderAgent rejected a generic with nested project content: %v", err)
	}

	written, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("rendered file not written: %v", readErr)
	}
	if n := strings.Count(string(written), nestedDefaultText); n != 1 {
		t.Errorf("nested project content appears %d times in rendered output, want 1:\n%s", n, written)
	}
}

// nestedDefaultDeploySource is the shared deploy-fixture worker source (front matter known to
// pass the deploy pipeline) with a nested project default added inside a managed region.
func nestedDefaultDeploySource() []byte {
	return []byte(injWorkerSource +
		"<Constraints type=\"core\">\n" +
		"<HarnessConstraints type=\"managed\">\n" +
		"<ProjectConstraints type=\"project\">\n" +
		nestedDefaultText + "\n" +
		"</ProjectConstraints>\n" +
		"</HarnessConstraints>\n" +
		"</Constraints>\n")
}

func TestDeployAgents_NestedProjectContent_WritesContent(t *testing.T) {
	run := runInfraInjection(t, injectionConfig{
		subagentIDs: []string{injPlainSubagentKey},
		customizeCatalog: func(c *stubCatalog) {
			c.sources["src/"+injPlainSubagentKey+".md"] = nestedDefaultDeploySource()
		},
	})

	if run.err != nil {
		t.Fatalf("DeployAgents rejected a generic with nested project content: %v", run.err)
	}
	got, ok := run.readFile(t, injPlainSubagentKey+".md")
	if !ok {
		t.Fatalf("deployed agent file %s.md not written", injPlainSubagentKey)
	}
	if n := strings.Count(got, nestedDefaultText); n != 1 {
		t.Errorf("nested project content appears %d times in deployed agent, want 1:\n%s", n, got)
	}
}
