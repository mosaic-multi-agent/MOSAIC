package claudecode_test

// The adapter reads the model field of a deployed agent definition (frontmatter
// key `model`) so the runner can check what was deployed independently of what
// the deployment tool reported.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/claudecode"
)

func writeDefinition(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing definition: %v", err)
	}
	return path
}

func definitionReader(t *testing.T) domain.DefinitionModelReader {
	t.Helper()
	r, ok := any(claudecode.New(claudecode.Options{})).(domain.DefinitionModelReader)
	if !ok {
		t.Fatal("claudecode.Adapter does not implement domain.DefinitionModelReader")
	}
	return r
}

func TestDefinitionModel_ParsesTheModelScalar(t *testing.T) {
	cases := []struct {
		name string
		body string
		want domain.DefinitionModelField
	}{
		{"plain scalar", "---\nname: x\nmodel: claude-opus-4-5\ntools: Read\n---\nBody\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"double quoted", "---\nname: x\nmodel: \"claude-opus-4-5\"\n---\nBody\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"single quoted", "---\nname: x\nmodel: 'claude-opus-4-5'\n---\nBody\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"value is not normalised", "---\nmodel: Claude-Opus-4-5\n---\n", domain.DefinitionModelField{Present: true, Value: "Claude-Opus-4-5"}},
		{"key absent", "---\nname: x\n---\nBody mentions model: other\n", domain.DefinitionModelField{}},
		{"key empty", "---\nname: x\nmodel:\n---\n", domain.DefinitionModelField{Present: true, Value: ""}},
		{"key empty quoted", "---\nname: x\nmodel: \"\"\n---\n", domain.DefinitionModelField{Present: true, Value: ""}},
		{"CRLF line endings", "---\r\nname: x\r\nmodel: claude-opus-4-5\r\ntools: Read\r\n---\r\nBody\r\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"CRLF quoted", "---\r\nname: x\r\nmodel: \"claude-opus-4-5\"\r\n---\r\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"trailing comment", "---\nname: x\nmodel: claude-opus-4-5 # note\n---\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"quoted with trailing comment", "---\nname: x\nmodel: \"claude-opus-4-5\" # note\n---\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
		{"nested model key is not the field", "---\nname: x\nmeta:\n  model: nested-model\n---\n", domain.DefinitionModelField{}},
		{"nested key does not shadow top-level", "---\nmeta:\n  model: nested-model\nmodel: claude-opus-4-5\n---\n", domain.DefinitionModelField{Present: true, Value: "claude-opus-4-5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definitionReader(t).DefinitionModel(writeDefinition(t, tc.body))

			if err != nil {
				t.Fatalf("DefinitionModel returned unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("DefinitionModel = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDefinitionModel_UnreadableFileIsAnError(t *testing.T) {
	_, err := definitionReader(t).DefinitionModel(filepath.Join(t.TempDir(), "absent.md"))

	if err == nil {
		t.Error("DefinitionModel of a missing file returned no error")
	}
}

func TestDefinitionModel_UnparseableFrontmatterIsAnError(t *testing.T) {
	_, err := definitionReader(t).DefinitionModel(writeDefinition(t, "---\nname: [unclosed\nmodel: x\n---\nBody\n"))

	if err == nil {
		t.Error("DefinitionModel of a definition with invalid frontmatter returned no error")
	}
}
