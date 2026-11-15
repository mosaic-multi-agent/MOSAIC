package agentfile_test

// Tests for the Runner's per-harness agent file data: the catalog-backed
// lookup, the extensions and stems a Format accepts, and the test-only seam
// that gives a harness without a catalog entry format data.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/agentfile"
)

func TestFor_CatalogHarnesses_ReturnCatalogFormatData(t *testing.T) {
	tests := []struct {
		harnessID string
		want      agentfile.Format
	}{
		{"claude-code", agentfile.Format{AgentsDir: ".claude/agents", FormatID: "markdown", Extension: ".md"}},
		{"opencode", agentfile.Format{AgentsDir: ".opencode/agents", FormatID: "markdown", Extension: ".md"}},
		{"ghcp-cli", agentfile.Format{AgentsDir: ".github/agents", FormatID: "markdown", Extension: ".agent.md"}},
	}
	for _, tc := range tests {
		t.Run(tc.harnessID, func(t *testing.T) {
			got, ok := agentfile.For(tc.harnessID)

			if !ok {
				t.Fatalf("For(%q): want format data from the catalog, got ok=false", tc.harnessID)
			}
			if got != tc.want {
				t.Errorf("For(%q) = %+v, want %+v", tc.harnessID, got, tc.want)
			}
		})
	}
}

func TestFor_HarnessWithoutFormatData_ReturnsZeroFormatNotOK(t *testing.T) {
	for _, id := range []string{"", "no-such-harness"} {
		t.Run("id="+id, func(t *testing.T) {
			got, ok := agentfile.For(id)

			if ok {
				t.Error("want ok=false for a harness without format data")
			}
			if got != (agentfile.Format{}) {
				t.Errorf("want the zero Format, got %+v", got)
			}
		})
	}
}

func TestFormatExtensions_MarkdownAcceptsBothMarkdownExtensions(t *testing.T) {
	want := []string{".agent.md", ".md"}
	formats := map[string]agentfile.Format{
		"zero format":      {},
		"claude-code":      {AgentsDir: ".claude/agents", FormatID: "markdown", Extension: ".md"},
		"ghcp-cli":         {AgentsDir: ".github/agents", FormatID: "markdown", Extension: ".agent.md"},
		"explicit default": {FormatID: "markdown"},
	}
	for name, f := range formats {
		t.Run(name, func(t *testing.T) {
			if got := f.Extensions(); !reflect.DeepEqual(got, want) {
				t.Errorf("Extensions() = %v, want %v (longest first)", got, want)
			}
		})
	}
}

func TestFormatExtensions_MarkdownWithOtherExtension_AddsIt(t *testing.T) {
	f := agentfile.Format{FormatID: "markdown", Extension: ".mdx"}

	got := f.Extensions()

	for _, ext := range []string{".agent.md", ".md", ".mdx"} {
		if !containsString(got, ext) {
			t.Errorf("Extensions() = %v, want it to contain %q", got, ext)
		}
	}
}

func TestFormatExtensions_NonMarkdown_AcceptsOnlyItsOwnExtension(t *testing.T) {
	f := agentfile.Format{FormatID: "codex-toml", Extension: ".toml"}

	got := f.Extensions()

	if want := []string{".toml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Extensions() = %v, want %v", got, want)
	}
}

func TestFormatStem(t *testing.T) {
	markdown := agentfile.Format{}
	toml := agentfile.Format{FormatID: "codex-toml", Extension: ".toml"}
	tests := []struct {
		name     string
		f        agentfile.Format
		filename string
		want     string
	}{
		{"markdown .md", markdown, "foo.md", "foo"},
		{"markdown .agent.md strips the compound extension", markdown, "foo.agent.md", "foo"},
		{"markdown extension is case-insensitive", markdown, "FOO.MD", "FOO"},
		{"markdown stem keeps dots", markdown, "foo.bar.md", "foo.bar"},
		{"markdown bare extension has no stem", markdown, ".md", ""},
		{"markdown bare compound extension has no stem", markdown, ".agent.md", ""},
		{"markdown other extension", markdown, "foo.toml", ""},
		{"toml .toml", toml, "foo.toml", "foo"},
		{"toml extension is case-insensitive", toml, "Foo.TOML", "Foo"},
		{"toml ignores markdown files", toml, "foo.md", ""},
		{"toml bare extension has no stem", toml, ".toml", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.Stem(tc.filename); got != tc.want {
				t.Errorf("Stem(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestRegisterTestOnly_GivesHarnessFormatDataUntilUnregistered(t *testing.T) {
	want := agentfile.Format{AgentsDir: ".seam/agents", FormatID: "mosaic-test-json", Extension: ".json"}

	unregister := agentfile.RegisterTestOnly("seam-harness-lifecycle", want)
	got, ok := agentfile.For("seam-harness-lifecycle")
	unregister()
	_, okAfter := agentfile.For("seam-harness-lifecycle")

	if !ok || got != want {
		t.Errorf("For after RegisterTestOnly = (%+v, %v), want (%+v, true)", got, ok, want)
	}
	if okAfter {
		t.Error("want the harness back to having no format data after unregister")
	}
}

func TestRegisterTestOnly_CatalogHarness_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want a panic when registering format data for a catalog harness")
		}
	}()

	agentfile.RegisterTestOnly("claude-code", agentfile.Format{Extension: ".x"})
}

func TestRegisterTestOnly_AlreadyRegistered_Panics(t *testing.T) {
	unregister := agentfile.RegisterTestOnly("seam-harness-twice", agentfile.Format{Extension: ".x"})
	defer unregister()
	defer func() {
		if recover() == nil {
			t.Error("want a panic when registering a harness that is already test-registered")
		}
	}()

	agentfile.RegisterTestOnly("seam-harness-twice", agentfile.Format{Extension: ".y"})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
