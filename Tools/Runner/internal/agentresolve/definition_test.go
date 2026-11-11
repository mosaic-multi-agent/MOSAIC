package agentresolve_test

// Tests for agentresolve.CheckDefinition: the frontmatter check of a deployed
// agent definition.
//
// Coverage:
//
//   Accepted definitions:
//   - A non-empty scalar model (also when quoted and padded with whitespace).
//   - With the name check requested: a name equal to the identifier (padding
//     around the value is ignored).
//   - Without the name check: any name, or none.
//
//   Missing model (ErrDefinitionModelMissing):
//   - No model key, an empty value, a blank value, a list value, no
//     frontmatter at all.
//
//   Name check (ErrDefinitionNameMismatch, only when requested):
//   - A different name, a name differing in case only, a missing, empty or
//     list-valued name.
//
//   Refusal shape and ordering:
//   - *domain.RefusalError naming the agent (Resource), the frontmatter section
//     and the definition file; Error() carries both identifier and path.
//   - Unreadable file and unparseable frontmatter carry their own cause and are
//     neither of the two sentinels.
//   - The model failure wins over the name failure.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
)

const checkedAgentID = "reviewer-agent"

// writeDefinition writes content as <id>.md in a fresh temp dir and returns the
// reference to it.
func writeDefinition(t *testing.T, id, content string) domain.AgentReference {
	t.Helper()
	path := filepath.Join(t.TempDir(), id+".md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write definition: %v", err)
	}
	return domain.AgentReference{Identifier: id, DefinitionPath: path}
}

// frontmatterOf wraps the given key lines in a frontmatter block followed by a body.
func frontmatterOf(lines ...string) string {
	return "---\n" + strings.Join(lines, "\n") + "\n---\n# Agent\nBody text.\n"
}

func TestCheckDefinition_Accepts(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		requireName bool
	}{
		{"model present", frontmatterOf("model: claude-sonnet-4-5"), false},
		{"model quoted and padded", frontmatterOf(`model: "  claude-sonnet-4-5  "`), false},
		{"model and equal name, name check on", frontmatterOf("name: "+checkedAgentID, "model: m1"), true},
		{"padded equal name, name check on", frontmatterOf(`name: " `+checkedAgentID+` "`, "model: m1"), true},
		{"different name, name check off", frontmatterOf("name: something-else", "model: m1"), false},
		{"no name, name check off", frontmatterOf("description: d", "model: m1"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := writeDefinition(t, checkedAgentID, tc.content)

			if err := agentresolve.CheckDefinition(ref, tc.requireName); err != nil {
				t.Errorf("want nil for a valid definition, got %v", err)
			}
		})
	}
}

func TestCheckDefinition_RefusesMissingModel(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"no model key", frontmatterOf("name: " + checkedAgentID)},
		{"empty model", frontmatterOf("name: "+checkedAgentID, "model:")},
		{"blank quoted model", frontmatterOf("name: "+checkedAgentID, `model: "   "`)},
		{"list model", frontmatterOf("name: "+checkedAgentID, "model: [a, b]")},
		{"no frontmatter", "# Agent\nJust a body, no frontmatter.\n"},
		{"empty file", ""},
	}
	for _, requireName := range []bool{false, true} {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ref := writeDefinition(t, checkedAgentID, tc.content)

				err := agentresolve.CheckDefinition(ref, requireName)

				if !errors.Is(err, agentresolve.ErrDefinitionModelMissing) {
					t.Fatalf("want ErrDefinitionModelMissing (name check %v), got %v", requireName, err)
				}
				requireRefusalNames(t, err, ref)
			})
		}
	}
}

func TestCheckDefinition_NameCheckRefusesMismatch(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"different name", frontmatterOf("name: other-agent", "model: m1")},
		{"case differs", frontmatterOf("name: Reviewer-Agent", "model: m1")},
		{"missing name", frontmatterOf("model: m1")},
		{"empty name", frontmatterOf("name:", "model: m1")},
		{"list name", frontmatterOf("name: ["+checkedAgentID+"]", "model: m1")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := writeDefinition(t, checkedAgentID, tc.content)

			err := agentresolve.CheckDefinition(ref, true)

			if !errors.Is(err, agentresolve.ErrDefinitionNameMismatch) {
				t.Fatalf("want ErrDefinitionNameMismatch, got %v", err)
			}
			if errors.Is(err, agentresolve.ErrDefinitionModelMissing) {
				t.Errorf("a name failure must not be reported as a missing model: %v", err)
			}
			requireRefusalNames(t, err, ref)
		})
	}
}

func TestCheckDefinition_ModelFailureWinsOverNameFailure(t *testing.T) {
	ref := writeDefinition(t, checkedAgentID, frontmatterOf("name: other-agent"))

	err := agentresolve.CheckDefinition(ref, true)

	if !errors.Is(err, agentresolve.ErrDefinitionModelMissing) {
		t.Errorf("want ErrDefinitionModelMissing when both model and name fail, got %v", err)
	}
}

func TestCheckDefinition_UnreadableFile_RefusesWithTheReadError(t *testing.T) {
	ref := domain.AgentReference{
		Identifier:     checkedAgentID,
		DefinitionPath: filepath.Join(t.TempDir(), "gone", checkedAgentID+".md"),
	}

	err := agentresolve.CheckDefinition(ref, false)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want the read error reachable through the refusal, got %v", err)
	}
	if errors.Is(err, agentresolve.ErrDefinitionModelMissing) || errors.Is(err, agentresolve.ErrDefinitionNameMismatch) {
		t.Errorf("an unreadable file is neither a missing model nor a name mismatch: %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_UnparseableFrontmatter_RefusesWithTheParseError(t *testing.T) {
	ref := writeDefinition(t, checkedAgentID, "---\nname: "+checkedAgentID+"\nmodel: [unclosed\n---\nbody\n")

	err := agentresolve.CheckDefinition(ref, true)

	if err == nil {
		t.Fatal("want a refusal for unparseable frontmatter, got nil")
	}
	if errors.Is(err, agentresolve.ErrDefinitionModelMissing) || errors.Is(err, agentresolve.ErrDefinitionNameMismatch) {
		t.Errorf("a parse failure is neither a missing model nor a name mismatch: %v", err)
	}
	re := requireRefusalNames(t, err, ref)
	if re.Cause == nil {
		t.Error("want the parse error as the refusal's cause")
	}
}

// requireRefusalNames asserts err is a *domain.RefusalError that names the agent
// and its definition file, and returns it.
func requireRefusalNames(t *testing.T, err error, ref domain.AgentReference) *domain.RefusalError {
	t.Helper()
	re := asRefusalError(t, err)
	if re.Component != "agentresolve" {
		t.Errorf("Component = %q, want %q", re.Component, "agentresolve")
	}
	if re.Resource != ref.Identifier {
		t.Errorf("Resource = %q, want the agent identifier %q", re.Resource, ref.Identifier)
	}
	if re.Section != "frontmatter" {
		t.Errorf("Section = %q, want %q", re.Section, "frontmatter")
	}
	if !strings.HasPrefix(re.Reason, "definition "+ref.DefinitionPath+": ") {
		t.Errorf("Reason = %q, want it to begin with the definition path", re.Reason)
	}
	text := err.Error()
	if !strings.Contains(text, ref.Identifier) || !strings.Contains(text, ref.DefinitionPath) {
		t.Errorf("Error() = %q, want it to contain identifier %q and path %q", text, ref.Identifier, ref.DefinitionPath)
	}
	return re
}
