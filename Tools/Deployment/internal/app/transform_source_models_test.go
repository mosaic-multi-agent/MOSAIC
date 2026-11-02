package app_test

// transform_source_models_test.go covers ReadSourceModel, the extraction of an agent's source
// model from its frontmatter:
//   - When Frontmatter.ModelKey is declared on the descriptor and that key is present
//     as a non-empty scalar in the frontmatter, its value is returned.
//   - When ModelKey is absent from the frontmatter (or the descriptor does not declare
//     one), the generic "model" key is tried next.
//   - When both ModelKey and "model" are present, ModelKey takes precedence.
//   - When neither is present (or both are empty), UnsetSourceModel ("") is returned.
//   - Unparseable or empty content returns UnsetSourceModel without panicking.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Helpers for building test content and descriptors
// ---------------------------------------------------------------------------

// srcDescWithModelKey returns a minimal HarnessDescriptor whose Frontmatter.ModelKey
// is set to modelKey. All other fields are zero.
func srcDescWithModelKey(modelKey string) domain.HarnessDescriptor {
	return domain.HarnessDescriptor{
		Frontmatter: domain.FrontmatterSpec{ModelKey: modelKey},
	}
}

// fmBytes builds a minimal YAML-frontmatter document from a flat list of key/value pairs
// (alternating: key0, val0, key1, val1, …). The document body is empty.
func fmBytes(keyvals ...string) []byte {
	b := []byte("---\n")
	for i := 0; i+1 < len(keyvals); i += 2 {
		b = append(b, (keyvals[i] + ": " + keyvals[i+1] + "\n")...)
	}
	b = append(b, "---\n"...)
	return b
}

// writeSrcModelFile writes a source-harness agent file with the given src_model value
// (or omits the field when model is empty) to dir/name and returns its absolute path.
func writeSrcModelFile(t *testing.T, dir, name, model string) string {
	t.Helper()
	var content []byte
	if model != "" {
		content = []byte("---\n" +
			"transform_version: \"2.1.0\"\n" +
			"injections_version: \"3.0.0\"\n" +
			"src_model: " + model + "\n" +
			"---\n" +
			"<Identity type=\"core\">\nYou are the agent.\n</Identity>\n")
	} else {
		content = []byte("---\n" +
			"transform_version: \"2.1.0\"\n" +
			"injections_version: \"3.0.0\"\n" +
			"---\n" +
			"<Identity type=\"core\">\nYou are the agent.\n</Identity>\n")
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("writeSrcModelFile: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// ReadSourceModel — ModelKey resolution (happy path)
// ---------------------------------------------------------------------------

// TestReadSourceModel_ModelKeyFieldPresent_ReturnsItsValue verifies that when the
// descriptor's Frontmatter.ModelKey is declared and present as a non-empty scalar in
// the frontmatter, ReadSourceModel returns that scalar value.
func TestReadSourceModel_ModelKeyFieldPresent_ReturnsItsValue(t *testing.T) {
	// Arrange
	content := fmBytes("src_model", "claude-3-sonnet")
	srcDesc := srcDescWithModelKey("src_model")

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert
	// RED: stub returns UnsetSourceModel ("") regardless of content.
	if got != "claude-3-sonnet" {
		t.Errorf("ReadSourceModel = %q, want %q; "+
			"(the value must be read from the frontmatter)",
			got, "claude-3-sonnet")
	}
}

// TestReadSourceModel_ModelKeyAbsentFromFrontmatter_GenericModelPresent_FallsBack verifies
// that when the descriptor's ModelKey is not present in the frontmatter but the generic
// "model" key is, ReadSourceModel returns the generic key's value.
func TestReadSourceModel_ModelKeyAbsentFromFrontmatter_GenericModelPresent_FallsBack(t *testing.T) {
	// Arrange: descriptor says look for "src_model", but only "model" is in the frontmatter.
	content := fmBytes("model", "gpt-4")
	srcDesc := srcDescWithModelKey("src_model") // "src_model" absent from content

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert
	// RED: stub returns "".
	if got != "gpt-4" {
		t.Errorf("ReadSourceModel = %q, want %q; "+
			"(the generic model key must be read)",
			got, "gpt-4")
	}
}

// TestReadSourceModel_BothModelKeyAndGenericPresent_ModelKeyTakesPrecedence verifies that
// when both the descriptor's declared ModelKey field and the generic "model" key are present,
// the descriptor's ModelKey value is used. This is consistent with how retarget.go strips the
// source model: the declared key is the authoritative signal.
func TestReadSourceModel_BothModelKeyAndGenericPresent_ModelKeyTakesPrecedence(t *testing.T) {
	// Arrange: both keys present; ModelKey should win.
	content := fmBytes("src_model", "claude-3-sonnet", "model", "gpt-4")
	srcDesc := srcDescWithModelKey("src_model")

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert: must return the ModelKey value, not the generic one.
	// RED: stub returns "".
	if got != "claude-3-sonnet" {
		t.Errorf("ReadSourceModel = %q, want %q (ModelKey takes precedence over generic \"model\"); "+
			"(RED: stub does not inspect frontmatter)", got, "claude-3-sonnet")
	}
}

// TestReadSourceModel_EmptyModelKeyValue_FallsBackToGenericModel verifies that when the
// descriptor's ModelKey is present in the frontmatter but its value is an empty scalar,
// ReadSourceModel falls back to the generic "model" key. An empty value is treated the same
// as absent: neither triggers an interactive question.
func TestReadSourceModel_EmptyModelKeyValue_FallsBackToGenericModel(t *testing.T) {
	// Arrange: "src_model" has an empty value; "model" has a real value.
	content := fmBytes("src_model", "", "model", "gpt-4-turbo")
	srcDesc := srcDescWithModelKey("src_model")

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert: must return the generic "model" value because ModelKey value is empty.
	// RED: stub returns "".
	if got != "gpt-4-turbo" {
		t.Errorf("ReadSourceModel = %q, want %q; "+
			"(RED: stub does not inspect frontmatter; an empty ModelKey value must fall back to \"model\")",
			got, "gpt-4-turbo")
	}
}

// TestReadSourceModel_EmptyDescriptorModelKey_UsesGenericModel verifies that when the
// descriptor's Frontmatter.ModelKey is empty (the harness does not declare a model key),
// ReadSourceModel uses the generic "model" key as the sole lookup path.
func TestReadSourceModel_EmptyDescriptorModelKey_UsesGenericModel(t *testing.T) {
	// Arrange: descriptor has no declared model key; content has generic "model" only.
	content := fmBytes("model", "claude-opus")
	srcDesc := srcDescWithModelKey("") // no declared model key

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert
	// RED: stub returns "".
	if got != "claude-opus" {
		t.Errorf("ReadSourceModel = %q, want %q; "+
			"(RED: stub does not inspect frontmatter; with empty ModelKey, generic \"model\" must be tried)",
			got, "claude-opus")
	}
}

// ---------------------------------------------------------------------------
// ReadSourceModel — unset / error cases (NOT RED-VERIFIED)
// ---------------------------------------------------------------------------

// TestReadSourceModel_NoModelFields_ReturnsUnsetSourceModel verifies that when neither
// the descriptor's ModelKey nor the generic "model" key is present in the frontmatter,
// ReadSourceModel returns UnsetSourceModel (the empty string).
//
// NOT RED-VERIFIED: the stub always returns UnsetSourceModel, so this test passes
// trivially in the RED phase regardless of content. Its value is in pinning the GREEN
// behaviour when the real implementation lands.
func TestReadSourceModel_NoModelFields_ReturnsUnsetSourceModel(t *testing.T) {
	// Arrange: frontmatter with no model fields at all.
	content := fmBytes("name", "my-agent", "role", "subagent")
	srcDesc := srcDescWithModelKey("src_model")

	// Act
	got := app.ReadSourceModel(content, srcDesc)

	// Assert: must return the empty-string sentinel.
	if got != app.UnsetSourceModel {
		t.Errorf("ReadSourceModel = %q, want UnsetSourceModel (%q); "+
			"a file with no model fields must contribute to the unset group",
			got, app.UnsetSourceModel)
	}
}

// TestReadSourceModel_UnparseableContent_ReturnsUnsetSourceModel verifies that when the
// content bytes cannot be parsed as a document, ReadSourceModel returns UnsetSourceModel
// without panicking or returning an error. The file is classified by the existing per-file
// detection path; ReadSourceModel must not surface a parse failure.
//
// NOT RED-VERIFIED: same trivial pass as NoModelFields above.
func TestReadSourceModel_UnparseableContent_ReturnsUnsetSourceModel(t *testing.T) {
	// Arrange: clearly non-YAML content.
	content := []byte("\x00\x01\x02 not valid yaml at all \xff\xfe")
	srcDesc := srcDescWithModelKey("src_model")

	// Act — must not panic.
	got := app.ReadSourceModel(content, srcDesc)

	// Assert
	if got != app.UnsetSourceModel {
		t.Errorf("ReadSourceModel = %q for unparseable content, want UnsetSourceModel; "+
			"parse failures must be silently treated as unset",
			got)
	}
}

// TestReadSourceModel_NilContent_ReturnsUnsetSourceModel verifies that nil bytes are
// handled gracefully, returning UnsetSourceModel.
//
// NOT RED-VERIFIED: same trivial pass.
func TestReadSourceModel_NilContent_ReturnsUnsetSourceModel(t *testing.T) {
	srcDesc := srcDescWithModelKey("src_model")
	got := app.ReadSourceModel(nil, srcDesc)
	if got != app.UnsetSourceModel {
		t.Errorf("ReadSourceModel(nil) = %q, want UnsetSourceModel", got)
	}
}

// ---------------------------------------------------------------------------
// IndexSourceModels — Files length and content
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// IndexSourceModels — empty input (NOT RED-VERIFIED for trivial pass cases)
// ---------------------------------------------------------------------------

