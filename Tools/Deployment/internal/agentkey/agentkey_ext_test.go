package agentkey_test

import (
	"testing"

	"mosaic-deploy/internal/agentkey"
)

func TestFromFileNameExt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ext  string
		want string
	}{
		{"plain md with md extension", "x.md", ".md", "x"},
		{"agent md with md extension", "x.agent.md", ".md", "x"},
		{"agent md with agent md extension", "x.agent.md", ".agent.md", "x"},
		{"plain md with agent md extension", "x.md", ".agent.md", "x"},
		{"only one agent md suffix is removed", "x.agent.agent.md", ".md", "x.agent"},
		{"plain toml with toml extension", "x.toml", ".toml", "x"},
		{"agent toml with toml extension", "x.agent.toml", ".toml", "x"},
		{"hyphenated toml key", "orchestrator-script.toml", ".toml", "orchestrator-script"},
		{"dotted toml key", "a.b.toml", ".toml", "a.b"},
		{"md name is untouched for toml extension", "x.md", ".toml", "x.md"},
		{"toml name is untouched for md extension", "x.toml", ".md", "x.toml"},
		{"matching is case sensitive for md", "X.MD", ".md", "X.MD"},
		{"matching is case sensitive for toml", "x.TOML", ".toml", "x.TOML"},
		{"empty extension is treated as md", "x.md", "", "x"},
		{"empty extension strips agent md", "x.agent.md", "", "x"},
		{"empty name stays empty", "", ".toml", ""},
		{"toml only gives empty key", ".toml", ".toml", ""},
		{"agent toml only gives empty key", ".agent.toml", ".toml", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := agentkey.FromFileNameExt(tc.in, tc.ext)
			if got != tc.want {
				t.Errorf("FromFileNameExt(%q, %q) = %q, want %q", tc.in, tc.ext, got, tc.want)
			}
		})
	}
}

// The Markdown entry point and the extension-aware one agree for the Markdown extension.
func TestFromFileNameExt_MarkdownMatchesFromFileName(t *testing.T) {
	names := []string{"x.md", "x.agent.md", "x.agent.agent.md", "x", "x.MD", "", ".md", ".agent.md", "a.b.md"}
	for _, n := range names {
		if got, want := agentkey.FromFileNameExt(n, ".md"), agentkey.FromFileName(n); got != want {
			t.Errorf("FromFileNameExt(%q, .md) = %q, FromFileName = %q", n, got, want)
		}
	}
}
