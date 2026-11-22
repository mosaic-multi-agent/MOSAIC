package agentload_test

// load_markdown_test.go covers loading Markdown agent files: the standard form is
// the file itself, and failures are errors that name their reason.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/docformat"

	"mosaic-common/agentload"
)

func TestLoad_Markdown_DocumentEqualsDocformatParse(t *testing.T) {
	path := fixture("markdown", "valid.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := docformat.Parse(raw)
	if err != nil {
		t.Fatalf("docformat.Parse: %v", err)
	}

	a := mustLoad(t, path, "markdown")

	if string(a.Canonical) != string(raw) {
		t.Errorf("Canonical differs from the file bytes")
	}
	if a.Document == nil || string(a.Document.Bytes()) != string(want.Bytes()) {
		t.Errorf("Document does not equal docformat.Parse of the file")
	}
	for _, key := range want.Frontmatter().Keys() {
		wv, _ := want.Frontmatter().Get(key)
		gv, ok := a.Document.Frontmatter().Get(key)
		if !ok || gv.Scalar != wv.Scalar {
			t.Errorf("frontmatter %q = %q (present %v), want %q", key, gv.Scalar, ok, wv.Scalar)
		}
	}
	if a.Path != path {
		t.Errorf("Path = %q, want %q", a.Path, path)
	}
}

func TestLoad_Markdown_EmptyFormatIDMeansMarkdown(t *testing.T) {
	a := mustLoad(t, fixture("markdown", "valid.md"), "")

	if a.FormatID != "markdown" {
		t.Errorf("FormatID = %q, want %q", a.FormatID, "markdown")
	}
	if got := scalar(t, a, "name"); got != "reviewer" {
		t.Errorf("name = %q, want reviewer", got)
	}
}

func TestLoad_Markdown_HasNoSettings(t *testing.T) {
	a := mustLoad(t, fixture("markdown", "valid.md"), "markdown")

	if a.Settings != nil {
		t.Errorf("Settings = %v, want nil for a format without a settings reader", a.Settings)
	}
}

func TestLoad_Markdown_MalformedFrontmatterIsUndecodable(t *testing.T) {
	path := fixture("markdown", "malformed_frontmatter.md")

	a, err := agentload.Load(path, "markdown")

	if err == nil {
		t.Fatalf("Load returned nil error for malformed frontmatter (agent %v)", a)
	}
	if a != nil {
		t.Errorf("Load returned a non-nil agent alongside an error")
	}
	if !errors.Is(err, agentload.ErrUndecodable) {
		t.Errorf("error %v does not wrap ErrUndecodable", err)
	}
	assertNamesFile(t, err, path)
}

func TestLoad_Markdown_MissingFileIsUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.md")

	_, err := agentload.Load(path, "markdown")

	if err == nil {
		t.Fatal("Load returned nil error for a missing file")
	}
	if !errors.Is(err, agentload.ErrUnreadable) {
		t.Errorf("error %v does not wrap ErrUnreadable", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error %v does not wrap fs.ErrNotExist", err)
	}
	assertNamesFile(t, err, path)
}

func TestLoad_UnknownFormatID(t *testing.T) {
	_, err := agentload.Load(fixture("markdown", "valid.md"), "no-such-format")

	if err == nil {
		t.Fatal("Load returned nil error for an unknown format id")
	}
	if !errors.Is(err, agentload.ErrUnknownFormat) {
		t.Errorf("error %v does not wrap ErrUnknownFormat", err)
	}
	if errors.Is(err, agentload.ErrUnreadable) || errors.Is(err, agentload.ErrUndecodable) {
		t.Errorf("error %v wraps more than one load failure sentinel", err)
	}
}

func TestLoad_Markdown_DoesNotModifyFile(t *testing.T) {
	path := copyFixture(t, "markdown", "valid.md")
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)

	mustLoad(t, path, "markdown")

	assertUnchanged(t, path, before, info)
}

// assertNamesFile checks the error is a *LoadError for path whose text names the path.
func assertNamesFile(t *testing.T, err error, path string) {
	t.Helper()
	var le *agentload.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error %v is not an *agentload.LoadError", err)
	}
	if le.Path != path {
		t.Errorf("LoadError.Path = %q, want %q", le.Path, path)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error text %q does not name the file %q", err.Error(), path)
	}
}
