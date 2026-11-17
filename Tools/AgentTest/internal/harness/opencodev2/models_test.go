package opencodev2_test

// Tests for the model helpers: the definition's model key and the form in
// which V2's logs identify the model.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func writeDefinition(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing definition: %v", err)
	}
	return path
}

func TestDefinitionModel_ReadsModelFrontmatterKey(t *testing.T) {
	path := writeDefinition(t, "---\nname: orchestrator\nmodel: anthropic/claude-test\n---\nbody\n")

	field, err := opencodev2.New(opencodev2.Options{}).DefinitionModel(path)
	if err != nil {
		t.Fatalf("DefinitionModel: %v", err)
	}

	if !field.Present || field.Value != "anthropic/claude-test" {
		t.Errorf("field = %+v, want the present value anthropic/claude-test", field)
	}
}

func TestDefinitionModel_AbsentKeyIsNotPresent(t *testing.T) {
	path := writeDefinition(t, "---\nname: orchestrator\n---\nbody\n")

	field, err := opencodev2.New(opencodev2.Options{}).DefinitionModel(path)
	if err != nil {
		t.Fatalf("DefinitionModel: %v", err)
	}

	if field.Present || field.Value != "" {
		t.Errorf("field = %+v, want an absent model key", field)
	}
}

func TestObservedModelForm_IsProviderQualified(t *testing.T) {
	got := opencodev2.New(opencodev2.Options{}).ObservedModelForm()

	if got != domain.ObservedModelProviderQualified {
		t.Errorf("ObservedModelForm = %q, want %q", got, domain.ObservedModelProviderQualified)
	}
}
