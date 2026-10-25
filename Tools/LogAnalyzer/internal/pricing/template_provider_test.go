package pricing_test

// Tests for the provider-level content of the built-in pricing template.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/pricing"
)

// templateFromPut returns the template as written into a freshly created file.
func templateFromPut(t *testing.T) string {
	t.Helper()
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	return leadingCommentBlock(raw)
}

func TestTemplate_DescribesProviderLevelAndStrictLookup(t *testing.T) {
	tmpl := templateFromPut(t)

	for _, want := range []string{"providers:", "models.dev", "anthropic", "github-copilot", "amazon-bedrock", "google-vertex-anthropic"} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("template should mention %q", want)
		}
	}
	if !strings.Contains(strings.ToLower(tmpl), "strict") {
		t.Error("template should state that lookup is strict, with no fallback between the two sections")
	}
}

func TestTemplate_ShowsProviderExampleWithModelsAndRates(t *testing.T) {
	tmpl := templateFromPut(t)

	providers := strings.Index(tmpl, "providers:")
	if providers < 0 {
		t.Fatal("template should contain a providers: example")
	}
	rest := tmpl[providers:]
	models := strings.Index(rest, "models:")
	if models < 0 {
		t.Fatal("template provider example should contain a models: level")
	}
	if !strings.Contains(rest[models:], "input:") {
		t.Error("template provider example should show an entry with rates")
	}
}

func TestTemplate_UsesCurrentRateKeysAndNoRetiredKeys(t *testing.T) {
	tmpl := templateFromPut(t)

	for _, key := range []string{"cache_write_5m", "cache_write_1h", "cache_read", "long_context"} {
		if !strings.Contains(tmpl, key) {
			t.Errorf("template should mention rate key %q", key)
		}
	}
	for _, retired := range []string{"cached_input", "output_under_threshold", "output_at_threshold", "context_length_threshold"} {
		if strings.Contains(tmpl, retired) {
			t.Errorf("template must not mention retired key %q", retired)
		}
	}
	if strings.Contains(tmpl, "  cache_write:") {
		t.Error("template must not use the retired cache_write key")
	}
}

func TestTemplate_FileHoldingOnlyTemplate_LoadsEmptyWithoutFindings(t *testing.T) {
	tmpl := templateFromPut(t)
	root, path := makeRootWithConfigDir(t)
	if err := os.WriteFile(filepath.Clean(path), []byte(tmpl), 0o644); err != nil {
		t.Fatalf("write template file: %v", err)
	}

	table, findings, err := pricing.NewStore(root).Load(context.Background())

	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	if table.Len() != 0 {
		t.Errorf("table has %d entries, want 0 (template is comments only)", table.Len())
	}
}
